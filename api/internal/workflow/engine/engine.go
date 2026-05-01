package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultTimeout      = 120 * time.Second
	defaultNodeTimeout  = 30 * time.Second
	defaultMaxRetries   = 1
	defaultRetryBackoff = 2 * time.Second

	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusSkipped   = "skipped"

	ErrorStop     = "stop_on_failure"
	ErrorContinue = "continue_on_failure"
)

type EngineConfig struct {
	Timeout       time.Duration
	NodeTimeout   time.Duration
	MaxRetries    int
	RetryBackoff  time.Duration
	ErrorHandling string
}

type NodeRunner interface {
	Run(ctx context.Context, nodeID string, config *NodeConfig, pool *VariablePool) (*NodeResult, error)
}

type NodeResult struct {
	Outputs map[string]any
	NextID  string
	Status  string
}

type WorkflowResult struct {
	Status    string         `json:"status"`
	Outputs   map[string]any `json:"outputs"`
	Error     string         `json:"error,omitempty"`
	ElapsedMs int64          `json:"elapsed_ms"`
	NodeCount int            `json:"node_count"`
}

type StepRecorder func(runID, nodeID, nodeType, status string, elapsedMs int64)

type Engine struct {
	graph        *Graph
	pool         *VariablePool
	emitter      *EventEmitter
	runner       NodeRunner
	config       EngineConfig
	stepRecorder StepRecorder
}

func (e *Engine) SetStepRecorder(fn StepRecorder) {
	e.stepRecorder = fn
}

func DefaultConfig() EngineConfig {
	return EngineConfig{
		Timeout:       defaultTimeout,
		NodeTimeout:   defaultNodeTimeout,
		MaxRetries:    defaultMaxRetries,
		RetryBackoff:  defaultRetryBackoff,
		ErrorHandling: ErrorStop,
	}
}

func NewEngine(graph *Graph, runner NodeRunner, config EngineConfig) *Engine {
	return &Engine{
		graph:   graph,
		pool:    NewVariablePool(),
		emitter: NewEventEmitter(128),
		runner:  runner,
		config:  config,
	}
}

func (e *Engine) Pool() *VariablePool {
	return e.pool
}

func (e *Engine) Emitter() *EventEmitter {
	return e.emitter
}

func (e *Engine) Run(ctx context.Context, inputs map[string]any) (*WorkflowResult, error) {
	if err := e.graph.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	start := time.Now()
	e.initRun(inputs)
	return e.runFromStart(ctx, start)
}

func (e *Engine) initRun(inputs map[string]any) {
	e.pool.SetSystem("workflow_id", e.graph.ID)
	e.pool.SetSystem("run_id", newID())
	e.pool.SetSystem("timestamp", time.Now().Unix())
	e.pool.SetAll(e.graph.StartNodeID, inputs)
	EmitWorkflowStarted(e.emitter, e.graph.ID)

}

func (e *Engine) runFromStart(ctx context.Context, start time.Time) (*WorkflowResult, error) {
	nodeCount := 0
	currentID := e.graph.StartNodeID

	for currentID != "" {
		if timedOut(ctx) {
			return e.finishFailed("workflow timeout", start, nodeCount), nil
		}

		node := e.graph.Nodes[currentID]

		// 并行组检测：收集同组节点并行执行
		if node != nil && node.ParallelGroup != "" {
			groupIDs := e.collectParallelGroup(currentID, node.ParallelGroup)
			if len(groupIDs) > 1 {
				if err := e.executeParallelGroup(ctx, groupIDs); err != nil {
					return e.finishFailed(err.Error(), start, nodeCount+len(groupIDs)), nil
				}
				nodeCount += len(groupIDs)
				currentID = e.findGroupExit(groupIDs)
				continue
			}
		}

		result, err := e.executeNode(ctx, currentID)
		nodeCount++

		if err != nil {
			return e.finishFailed(err.Error(), start, nodeCount), nil
		}

		if result.Outputs != nil {
			e.pool.SetAll(currentID, result.Outputs)
		}

		node = e.graph.Nodes[currentID]
		if node.Type == NodeEnd {
			return e.finishSucceeded(result.Outputs, start, nodeCount), nil
		}

		currentID = e.resolveNextNode(currentID, result)
	}

	return e.finishSucceeded(e.pool.GetAll(e.graph.StartNodeID), start, nodeCount), nil
}

func timedOut(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (e *Engine) finishFailed(message string, start time.Time, nodeCount int) *WorkflowResult {
	elapsed := time.Since(start)
	EmitWorkflowFailed(e.emitter, message, elapsed.Seconds())
	e.emitter.Close()
	return &WorkflowResult{Status: StatusFailed, Error: message, ElapsedMs: elapsed.Milliseconds(), NodeCount: nodeCount}
}

func (e *Engine) finishSucceeded(outputs map[string]any, start time.Time, nodeCount int) *WorkflowResult {
	elapsed := time.Since(start)
	EmitWorkflowFinished(e.emitter, outputs, elapsed.Seconds())
	e.emitter.Close()
	return &WorkflowResult{Status: StatusSucceeded, Outputs: outputs, ElapsedMs: elapsed.Milliseconds(), NodeCount: nodeCount}
}

func (e *Engine) RunStreaming(ctx context.Context, inputs map[string]any) (<-chan WorkflowEvent, error) {
	if err := e.graph.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	go func() {
		_, _ = e.Run(ctx, inputs)
	}()

	return e.emitter.Events(), nil
}

func (e *Engine) executeNode(ctx context.Context, nodeID string) (*NodeResult, error) {
	node, ok := e.graph.Nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %q not found in graph", nodeID)
	}

	EmitNodeStarted(e.emitter, nodeID, string(node.Type), node.Title)
	nodeStart := time.Now()
	e.recordNodeStep(nodeID, string(node.Type), "running", 0)

	nodeCtx, cancel := context.WithTimeout(ctx, e.config.NodeTimeout)
	defer cancel()

	result, lastErr := e.runNodeWithRetry(nodeCtx, nodeID, node)
	elapsed := time.Since(nodeStart).Seconds()
	if lastErr != nil {
		EmitNodeError(e.emitter, nodeID, lastErr.Error(), elapsed)
		if fallback := e.nodeFailureResult(node); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("node %q failed: %w", nodeID, lastErr)
	}

	EmitNodeFinished(e.emitter, nodeID, result.Outputs, elapsed)
	e.recordNodeStep(nodeID, string(node.Type), "success", int64(elapsed*1000))
	if result.Status == "" {
		result.Status = StatusSucceeded
	}
	return result, nil
}

func (e *Engine) runNodeWithRetry(ctx context.Context, nodeID string, node *NodeConfig) (*NodeResult, error) {
	maxRetries, retryBackoff := e.nodeRetryPolicy(node)
	var result *NodeResult
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay(retryBackoff, attempt))
		}
		result, lastErr = e.runner.Run(ctx, nodeID, node, e.pool)
		if lastErr == nil {
			break
		}
	}
	return result, lastErr
}

func retryDelay(backoff time.Duration, attempt int) time.Duration {
	delay := backoff * time.Duration(1<<uint(attempt-1))
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}

func (e *Engine) recordNodeStep(nodeID, nodeType, status string, elapsedMs int64) {
	if e.stepRecorder == nil {
		return
	}
	runID, _ := e.pool.GetSystem("run_id")
	e.stepRecorder(fmt.Sprintf("%v", runID), nodeID, nodeType, status, elapsedMs)
}

func (e *Engine) nodeRetryPolicy(node *NodeConfig) (int, time.Duration) {
	maxRetries := e.config.MaxRetries
	if node.MaxRetries != nil && *node.MaxRetries >= 0 {
		maxRetries = *node.MaxRetries
	}
	backoff := e.config.RetryBackoff
	if node.RetryBackoff > 0 {
		backoff = node.RetryBackoff
	}
	return maxRetries, backoff
}

func (e *Engine) nodeFailureResult(node *NodeConfig) *NodeResult {
	switch node.OnFailure {
	case "skip":
		return &NodeResult{Status: StatusSkipped}
	case "default_value":
		return &NodeResult{Outputs: map[string]any{"fallback": node.DefaultValue}, Status: StatusFailed}
	}
	if node.Fallback != nil && node.Fallback.OnError == "continue" {
		return &NodeResult{Outputs: map[string]any{"fallback": node.Fallback.FallbackValue}, Status: StatusFailed}
	}
	if e.config.ErrorHandling == ErrorContinue {
		return &NodeResult{Status: StatusSkipped}
	}
	return nil
}

func (e *Engine) resolveNextNode(current string, result *NodeResult) string {
	if result.NextID != "" {
		return result.NextID
	}

	node := e.graph.Nodes[current]
	if node.Next != "" {
		return node.Next
	}

	successors := e.graph.Successors(current)
	if len(successors) == 1 {
		return successors[0]
	}

	return ""
}

// collectParallelGroup 从 graph 中收集所有属于同一并行组的节点 ID。
func (e *Engine) collectParallelGroup(startID, group string) []string {
	ids := make([]string, 0, 4)
	for _, node := range e.graph.Nodes {
		if node.ParallelGroup == group {
			ids = append(ids, node.ID)
		}
	}
	// 确保 startID 在列表中
	found := false
	for _, id := range ids {
		if id == startID {
			found = true
			break
		}
	}
	if !found {
		ids = append([]string{startID}, ids...)
	}
	return ids
}

// executeParallelGroup 并行执行一组节点，等待全部完成。
func (e *Engine) executeParallelGroup(ctx context.Context, nodeIDs []string) error {
	var wg sync.WaitGroup
	errs := make([]error, len(nodeIDs))

	for i, id := range nodeIDs {
		wg.Add(1)
		go func(idx int, nodeID string) {
			defer wg.Done()
			result, err := e.executeNode(ctx, nodeID)
			if err != nil {
				errs[idx] = err
				return
			}
			if result.Outputs != nil {
				e.pool.SetAll(nodeID, result.Outputs)
			}
		}(i, id)
	}
	wg.Wait()

	// 返回第一个错误（如果有）
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// findGroupExit 找到并行组的出口节点（组内节点的 Next 指向的非组内节点）。
func (e *Engine) findGroupExit(groupIDs []string) string {
	groupSet := make(map[string]bool, len(groupIDs))
	for _, id := range groupIDs {
		groupSet[id] = true
	}

	for _, id := range groupIDs {
		node := e.graph.Nodes[id]
		if node.Next != "" && !groupSet[node.Next] {
			return node.Next
		}
	}
	return ""
}
