package node

import (
	"context"
	"fmt"
	"time"

	"ai-workbench-api/internal/workflow/engine"
)

const (
	defaultHumanInputTimeout = 300 // 默认 5 分钟
	eventHumanInputRequired  = engine.EventType("human_input_required")
)

// handleHumanInput 等待用户输入，支持超时和默认值
func handleHumanInput(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	prompt, _ := cfg.Data["prompt"].(string)
	inputType := getInputType(cfg)
	timeoutSec := getTimeout(cfg)
	defaultValue, hasDefault := cfg.Data["default_value"]

	notifyFrontend(pool, nodeID, prompt, inputType, cfg)

	inputCh := pool.GetInputChannel(nodeID)
	if inputCh == nil {
		return handleNoChannel(defaultValue, hasDefault)
	}

	return waitForInput(ctx, inputCh, timeoutSec, defaultValue, hasDefault)
}

// getInputType 获取输入类型，默认 text
func getInputType(cfg *engine.NodeConfig) string {
	if t, ok := cfg.Data["input_type"].(string); ok && t != "" {
		return t
	}
	return "text"
}

// getTimeout 获取超时秒数
func getTimeout(cfg *engine.NodeConfig) int {
	if t, ok := toInt(cfg.Data["timeout"]); ok && t > 0 {
		return t
	}
	return defaultHumanInputTimeout
}

// notifyFrontend 通过事件流通知前端需要用户输入
func notifyFrontend(pool *engine.VariablePool, nodeID, prompt, inputType string, cfg *engine.NodeConfig) {
	emitter := pool.GetEventEmitter()
	if emitter == nil {
		return
	}
	emitter.Emit(engine.WorkflowEvent{
		Event:  eventHumanInputRequired,
		NodeID: nodeID,
		Data: map[string]any{
			"prompt":     prompt,
			"input_type": inputType,
			"options":    cfg.Data["options"],
		},
	})
}

// handleNoChannel 没有输入通道时使用默认值
func handleNoChannel(defaultValue any, hasDefault bool) (*engine.NodeResult, error) {
	if hasDefault {
		return &engine.NodeResult{
			Outputs: map[string]any{"user_input": defaultValue},
			Status:  engine.StatusSucceeded,
		}, nil
	}
	return &engine.NodeResult{
		Outputs: map[string]any{"user_input": "", "skipped": true},
		Status:  engine.StatusSucceeded,
	}, nil
}

// waitForInput 等待用户输入或超时
func waitForInput(ctx context.Context, inputCh <-chan any, timeoutSec int, defaultValue any, hasDefault bool) (*engine.NodeResult, error) {
	select {
	case input := <-inputCh:
		return &engine.NodeResult{
			Outputs: map[string]any{"user_input": input},
			Status:  engine.StatusSucceeded,
		}, nil
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		if hasDefault {
			return &engine.NodeResult{
				Outputs: map[string]any{"user_input": defaultValue, "timed_out": true},
				Status:  engine.StatusSucceeded,
			}, nil
		}
		return nil, fmt.Errorf("human_input: timeout after %ds", timeoutSec)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
