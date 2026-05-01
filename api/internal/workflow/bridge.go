package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"ai-workbench-api/internal/aiconfig"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow/engine"
	"ai-workbench-api/internal/workflow/node"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ---------- LLM 配置读取（从 viper，避免 import handler） ----------

var llmSemaphore = make(chan struct{}, 5)
var llmBreaker = newSimpleBreaker(5, 30*time.Second)

type simpleBreaker struct {
	mu           sync.Mutex
	failures     int
	threshold    int
	lastFailure  time.Time
	resetTimeout time.Duration
	open         bool
}

func newSimpleBreaker(threshold int, reset time.Duration) *simpleBreaker {
	return &simpleBreaker{threshold: threshold, resetTimeout: reset}
}

func (b *simpleBreaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.open {
		return true
	}
	if time.Since(b.lastFailure) > b.resetTimeout {
		b.open = false
		b.failures = 0
		return true
	}
	return false
}

func (b *simpleBreaker) recordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.open = false
}

func (b *simpleBreaker) recordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	b.lastFailure = time.Now()
	if b.failures >= b.threshold {
		b.open = true
	}
}

type aiProvider struct {
	BaseURL string   `mapstructure:"base_url"`
	Alias   string   `mapstructure:"baseurl"`
	APIKey  string   `mapstructure:"api_key"`
	KeyA    string   `mapstructure:"apikey"`
	Models  []string `mapstructure:"models"`
	Default bool     `mapstructure:"default"`
}

func loadProviders() []aiProvider {
	var ps []aiProvider
	_ = viper.UnmarshalKey("ai_providers", &ps)
	for i := range ps {
		if ps[i].BaseURL == "" {
			ps[i].BaseURL = ps[i].Alias
		}
		if ps[i].APIKey == "" {
			ps[i].APIKey = ps[i].KeyA
		}
		ps[i].BaseURL = normalizeURL(ps[i].BaseURL)
		ps[i].APIKey = strings.TrimSpace(ps[i].APIKey)
	}
	return ps
}

func normalizeURL(v string) string {
	u := strings.TrimRight(strings.TrimSpace(v), "/")
	return strings.TrimSuffix(u, "/chat/completions")
}

func usableKey(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "******" || strings.Contains(v, "${") || strings.Contains(strings.ToLower(v), "your_") {
		return ""
	}
	return v
}

func bridgeBaseURL() string {
	for _, p := range loadProviders() {
		if p.Default && p.BaseURL != "" {
			return p.BaseURL
		}
	}
	for _, p := range loadProviders() {
		if p.BaseURL != "" {
			return p.BaseURL
		}
	}
	return normalizeURL(viper.GetString("ai.base_url"))
}

func bridgeAPIKey() string {
	if k := usableKey(os.Getenv("AI_WORKBENCH_API_KEY")); k != "" {
		return k
	}
	for _, p := range loadProviders() {
		if p.Default {
			if k := usableKey(p.APIKey); k != "" {
				return k
			}
		}
	}
	for _, p := range loadProviders() {
		if k := usableKey(p.APIKey); k != "" {
			return k
		}
	}
	return usableKey(viper.GetString("ai.api_key"))
}

func bridgeDefaultModel() string {
	return aiconfig.ResolveDefaultModel()
}

// ---------- LLMBridge 实现 node.LLMClient ----------

// LLMBridge bridges the node.LLMClient interface to the configured AI provider.
type LLMBridge struct{}

func (b *LLMBridge) ChatCompletion(ctx context.Context, req node.ChatRequest) (*node.ChatResponse, error) {
	if !llmBreaker.allow() {
		return nil, fmt.Errorf("LLM service circuit breaker open, try again later")
	}
	select {
	case llmSemaphore <- struct{}{}:
		defer func() { <-llmSemaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	baseURL := bridgeBaseURL()
	apiKey := bridgeAPIKey()
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("AI provider not configured: base_url or api_key missing")
	}
	model := req.Model
	if model == "" {
		model = bridgeDefaultModel()
	}

	payload := buildLLMPayload(model, req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal llm request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		llmBreaker.recordFailure()
		return nil, fmt.Errorf("llm http call: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		llmBreaker.recordFailure()
		return nil, fmt.Errorf("llm returned HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	llmBreaker.recordSuccess()
	return parseLLMResponse(data)
}

func buildLLMPayload(model string, req node.ChatRequest) map[string]any {
	msgs := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msg := map[string]any{"role": m.Role, "content": m.Content}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		if len(m.ToolCalls) > 0 {
			msg["tool_calls"] = m.ToolCalls
		}
		msgs = append(msgs, msg)
	}
	payload := map[string]any{"model": model, "messages": msgs}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		payload["temperature"] = req.Temperature
	}
	if req.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	if len(req.Tools) > 0 {
		payload["tools"] = req.Tools
	}
	return payload
}

func parseLLMResponse(data []byte) (*node.ChatResponse, error) {
	var raw struct {
		Choices []struct {
			Message struct {
				Content   string          `json:"content"`
				ToolCalls []node.ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse llm json: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("llm returned empty choices")
	}
	return &node.ChatResponse{
		Content:   raw.Choices[0].Message.Content,
		ToolCalls: raw.Choices[0].Message.ToolCalls,
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- ToolBridge 实现 node.ToolExecutor ----------

// ToolBridge bridges the node.ToolExecutor interface to platform capabilities.
type ToolBridge struct{}

func (b *ToolBridge) Execute(ctx context.Context, toolName string, args map[string]any) (any, error) {
	switch toolName {
	case "query_prometheus":
		return toolQueryPrometheus(args)
	case "search_knowledge":
		return toolSearchKnowledge(ctx, args)
	default:
		return nil, fmt.Errorf("unsupported tool: %s", toolName)
	}
}

func toolQueryPrometheus(args map[string]any) (any, error) {
	promQL, _ := args["promql"].(string)
	if promQL == "" {
		return nil, fmt.Errorf("promql argument required")
	}
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return nil, fmt.Errorf("prometheus.url not configured")
	}
	endpoint := base + "/api/v1/query?query=" + url.QueryEscape(promQL)
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read prometheus response: %w", err)
	}
	return string(data), nil
}

func toolSearchKnowledge(_ context.Context, args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	category, _ := args["category"].(string)
	topK := 5
	if v, ok := args["top_k"].(float64); ok && v > 0 {
		topK = int(v)
	}
	cases, total := store.ListCases(1, topK, query, category)
	return map[string]any{"cases": cases, "total": total}, nil
}

// ---------- 便捷函数 ----------

// NewDefaultRegistry creates a Registry with default bridge implementations.
func NewDefaultRegistry() *node.Registry {
	return node.NewRegistry(&LLMBridge{}, &KnowledgeBridge{}, &ToolBridge{})
}

// RunWorkflow loads a builtin/custom workflow by name and executes it.
func RunWorkflow(ctx context.Context, name string, inputs map[string]any) (*engine.WorkflowResult, error) {
	// 0. 别名解析：旧名称自动转发到新工作流
	resolvedName, defaultInputs := ResolveAlias(name)
	if defaultInputs != nil {
		for k, v := range defaultInputs {
			if _, exists := inputs[k]; !exists {
				inputs[k] = v
			}
		}
		name = resolvedName
	}

	// 1. 检查缓存
	if CacheEnabled() {
		if cached, ok := CacheGet(name, inputs); ok {
			return cached, nil
		}
	}

	// 2. 执行工作流
	result, err := runWorkflowInternal(ctx, name, inputs)

	// 3. 写入缓存
	if err == nil && CacheEnabled() {
		CacheSet(name, inputs, result)
	}

	return result, err
}

// runWorkflowInternal 执行工作流的核心逻辑（不含缓存）。
func runWorkflowInternal(ctx context.Context, name string, inputs map[string]any) (*engine.WorkflowResult, error) {
	graph, cfg, err := loadWorkflowGraph(name)
	if err != nil {
		return nil, err
	}
	runner := NewDefaultRegistry()
	eng := engine.NewEngine(graph, runner, *cfg)
	return eng.Run(ctx, inputs)
}

// RunWorkflowStreaming loads a workflow and executes it in streaming mode.
func RunWorkflowStreaming(ctx context.Context, name string, inputs map[string]any) (<-chan engine.WorkflowEvent, error) {
	resolvedName, defaultInputs := ResolveAlias(name)
	if defaultInputs != nil {
		for k, v := range defaultInputs {
			if _, exists := inputs[k]; !exists {
				inputs[k] = v
			}
		}
		name = resolvedName
	}

	graph, cfg, err := loadWorkflowGraph(name)
	if err != nil {
		return nil, err
	}
	runner := NewDefaultRegistry()
	eng := engine.NewEngine(graph, runner, *cfg)
	return eng.RunStreaming(ctx, inputs)
}

// loadWorkflowGraph tries builtin first, then custom from store.
func loadWorkflowGraph(name string) (*engine.Graph, *engine.EngineConfig, error) {
	// Try builtin
	graph, cfg, err := engine.LoadBuiltinWorkflow(name)
	if err == nil {
		return graph, cfg, nil
	}

	// Try custom from store
	w, ok := store.GetWorkflow(name)
	if !ok {
		return nil, nil, fmt.Errorf("workflow %q not found (builtin err: %v)", name, err)
	}
	graph, cfg, parseErr := engine.ParseDSL([]byte(w.DSL))
	if parseErr != nil {
		return nil, nil, fmt.Errorf("parse custom workflow %q: %w", name, parseErr)
	}
	return graph, cfg, nil
}

// ListBuiltinWorkflowNames returns the names of all builtin workflows.
func ListBuiltinWorkflowNames() []string {
	return []string{
		"smart_diagnosis",
		"domain_diagnosis",
		"health_inspection",
		"metrics_insight",
		"security_compliance",
		"incident_review",
		"network_check",
		"runbook_execute",
		"knowledge_enrich",
	}
}

func init() {
	logrus.Debug("workflow bridge initialized")
}
