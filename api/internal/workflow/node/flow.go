package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"text/template"

	"ai-workbench-api/internal/workflow/engine"
)

// 循环默认配置
const (
	defaultMaxIterations = 100
)

// handleLoop 对数组中的每个元素执行指定动作
func handleLoop(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	items, err := resolveItems(cfg.Data, pool)
	if err != nil {
		return nil, fmt.Errorf("loop: %w", err)
	}

	maxIter := defaultMaxIterations
	if mi, ok := toInt(cfg.Data["max_iterations"]); ok && mi > 0 {
		maxIter = mi
	}

	if len(items) > maxIter {
		items = items[:maxIter]
	}

	results := make([]any, 0, len(items))
	for i, item := range items {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 将当前迭代信息写入变量池
		pool.Set("loop", "index", i)
		pool.Set("loop", "item", item)
		pool.Set("loop", "length", len(items))

		result, err := executeLoopAction(ctx, cfg.Data, pool, deps, item)
		if err != nil {
			return nil, fmt.Errorf("loop iteration %d: %w", i, err)
		}
		results = append(results, result)
	}

	outputs := map[string]any{
		"results": results,
		"count":   len(results),
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// resolveItems 从变量池解析数组
func resolveItems(data map[string]any, pool *engine.VariablePool) ([]any, error) {
	itemsVar, ok := data["items_variable"].(string)
	if !ok || itemsVar == "" {
		// 尝试直接从 items 字段获取
		if raw, ok := data["items"].([]any); ok {
			return raw, nil
		}
		return nil, fmt.Errorf("items_variable or items is required")
	}

	// 解析 nodeID.field 格式
	parts := strings.SplitN(itemsVar, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("items_variable must be in format 'nodeID.field'")
	}

	val, found := pool.Get(parts[0], parts[1])
	if !found {
		return nil, fmt.Errorf("variable %q not found in pool", itemsVar)
	}

	items, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("variable %q is not an array", itemsVar)
	}
	return items, nil
}

// executeLoopAction 执行循环体中的单次动作
func executeLoopAction(ctx context.Context, data map[string]any, pool *engine.VariablePool, deps *Registry, item any) (any, error) {
	action, _ := data["action"].(string)

	switch action {
	case "llm":
		return executeLoopLLM(ctx, data, pool, deps)
	case "http":
		return executeLoopHTTP(ctx, data, pool)
	case "template":
		return executeLoopTemplate(data, pool, item)
	default:
		// 默认直接返回 item（透传）
		return item, nil
	}
}

// executeLoopLLM 在循环中执行 LLM 调用
func executeLoopLLM(ctx context.Context, data map[string]any, pool *engine.VariablePool, deps *Registry) (any, error) {
	if deps.llm == nil {
		return nil, fmt.Errorf("LLMClient not configured")
	}

	prompt := extractAndInterpolate(data, "prompt", pool)
	model := getModelOrDefault(data)

	req := ChatRequest{
		Model:    model,
		Messages: []Message{{Role: "user", Content: prompt}},
	}

	resp, err := deps.llm.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Content, nil
}

// executeLoopHTTP 在循环中执行 HTTP 请求
func executeLoopHTTP(ctx context.Context, data map[string]any, pool *engine.VariablePool) (any, error) {
	url := extractAndInterpolate(data, "url", pool)
	if url == "" {
		return nil, fmt.Errorf("url is required for http action")
	}

	method := strings.ToUpper(getStringOrDefault(data, "method", "GET"))
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	outputs, err := parseHTTPResponse(resp)
	if err != nil {
		return nil, err
	}
	return outputs, nil
}

// executeLoopTemplate 在循环中执行模板转换
func executeLoopTemplate(data map[string]any, pool *engine.VariablePool, item any) (any, error) {
	tmplStr, ok := data["template"].(string)
	if !ok {
		return item, nil
	}
	return renderTemplate(tmplStr, map[string]any{"item": item})
}

// handleIteration loop 的简化版，对每个元素应用模板转换
func handleIteration(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	items, err := resolveItems(cfg.Data, pool)
	if err != nil {
		return nil, fmt.Errorf("iteration: %w", err)
	}

	tmplStr, _ := cfg.Data["template"].(string)
	results := make([]any, 0, len(items))

	for i, item := range items {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pool.Set("iteration", "index", i)
		pool.Set("iteration", "item", item)

		if tmplStr != "" {
			rendered, err := renderTemplate(tmplStr, map[string]any{
				"item":  item,
				"index": i,
			})
			if err != nil {
				return nil, fmt.Errorf("iteration %d: %w", i, err)
			}
			results = append(results, rendered)
		} else {
			results = append(results, item)
		}
	}

	outputs := map[string]any{
		"results": results,
		"count":   len(results),
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// handleListFilter 对数组元素应用过滤规则
func handleListFilter(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	items, err := resolveItems(cfg.Data, pool)
	if err != nil {
		return nil, fmt.Errorf("list_filter: %w", err)
	}

	rules, err := parseFilterRules(cfg.Data)
	if err != nil {
		return nil, err
	}

	filtered := make([]any, 0, len(items))
	for _, item := range items {
		if matchesFilterRules(item, rules) {
			filtered = append(filtered, item)
		}
	}

	outputs := map[string]any{
		"results": filtered,
		"count":   len(filtered),
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// filterRule 过滤规则
type filterRule struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// parseFilterRules 解析过滤规则
func parseFilterRules(data map[string]any) ([]filterRule, error) {
	raw, ok := data["filter_rules"]
	if !ok {
		return nil, nil
	}

	rulesJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("list_filter: marshal rules: %w", err)
	}

	var rules []filterRule
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return nil, fmt.Errorf("list_filter: parse rules: %w", err)
	}
	return rules, nil
}

// matchesFilterRules 检查元素是否满足所有过滤规则
func matchesFilterRules(item any, rules []filterRule) bool {
	if len(rules) == 0 {
		return true
	}

	itemMap, ok := item.(map[string]any)
	if !ok {
		return false
	}

	for _, rule := range rules {
		val := fmt.Sprintf("%v", itemMap[rule.Field])
		if !matchFilterOp(val, rule.Operator, rule.Value) {
			return false
		}
	}
	return true
}

// matchFilterOp 执行单个过滤操作
func matchFilterOp(actual, op, expected string) bool {
	switch op {
	case "==", "equals":
		return actual == expected
	case "!=", "not_equals":
		return actual != expected
	case "contains":
		return strings.Contains(actual, expected)
	case "regex":
		matched, err := regexp.MatchString(expected, actual)
		return err == nil && matched
	default:
		return false
	}
}

// handleTemplateTransform 使用 Go text/template 渲染模板
func handleTemplateTransform(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	tmplStr, ok := cfg.Data["template"].(string)
	if !ok || tmplStr == "" {
		return nil, fmt.Errorf("template_transform: template is required")
	}

	tmplStr = pool.Interpolate(tmplStr)
	// 收集模板数据：合并变量池快照
	data := pool.Snapshot()
	flatData := flattenSnapshot(data)

	rendered, err := renderTemplate(tmplStr, flatData)
	if err != nil {
		return nil, fmt.Errorf("template_transform: %w", err)
	}

	outputs := map[string]any{
		"text":   rendered,
		"result": rendered,
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// renderTemplate 执行 Go text/template 渲染
func renderTemplate(tmplStr string, data any) (string, error) {
	tmpl, err := template.New("node").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// flattenSnapshot 将嵌套的变量池快照展平为单层 map
func flattenSnapshot(snap map[string]map[string]any) map[string]any {
	flat := make(map[string]any, len(snap))
	for nodeID, fields := range snap {
		flat[nodeID] = fields
	}
	return flat
}
