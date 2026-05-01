package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/workflow/engine"
)

// HTTP 请求默认配置
const (
	defaultHTTPTimeout = 30 * time.Second
	maxResponseBody    = 1 << 20 // 1MB
)

// handleKnowledgeRetrieval 调用知识库检索接口
func handleKnowledgeRetrieval(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.knowledge == nil {
		return nil, fmt.Errorf("knowledge_retrieval: KnowledgeSearcher not configured")
	}

	query := extractAndInterpolate(cfg.Data, "query", pool)
	if query == "" {
		return nil, fmt.Errorf("knowledge_retrieval: query is required")
	}

	topK := 5
	if tk, ok := toInt(cfg.Data["top_k"]); ok && tk > 0 {
		topK = tk
	}

	category, _ := cfg.Data["category"].(string)
	category = pool.Interpolate(category)

	results, err := deps.knowledge.Search(ctx, query, topK, category)
	if err != nil {
		return nil, fmt.Errorf("knowledge_retrieval: %w", err)
	}

	filtered := filterByScore(results, cfg.Data)
	outputs := buildKnowledgeOutputs(filtered)

	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// filterByScore 过滤低于阈值的知识库结果
func filterByScore(results []KnowledgeResult, data map[string]any) []KnowledgeResult {
	threshold := 0.0
	if t, ok := toFloat64(data["score_threshold"]); ok {
		threshold = t
	}
	if threshold <= 0 {
		return results
	}

	filtered := make([]KnowledgeResult, 0, len(results))
	for _, r := range results {
		if r.Score >= threshold {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// buildKnowledgeOutputs 构建知识库检索输出
func buildKnowledgeOutputs(results []KnowledgeResult) map[string]any {
	items := make([]any, 0, len(results))
	for _, r := range results {
		items = append(items, map[string]any{
			"id":          r.ID,
			"score":       r.Score,
			"category":    r.Category,
			"description": r.Description,
			"treatment":   r.Treatment,
			"keywords":    r.Keywords,
		})
	}
	return map[string]any{
		"results": items,
		"count":   len(items),
	}
}

// handleHTTPRequest 执行 HTTP 请求
func handleHTTPRequest(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	method := strings.ToUpper(getStringOrDefault(cfg.Data, "method", "GET"))
	rawURL := extractAndInterpolate(cfg.Data, "url", pool)
	if rawURL == "" {
		return nil, fmt.Errorf("http_request: url is required")
	}

	timeout := defaultHTTPTimeout
	if t, ok := toInt(cfg.Data["timeout"]); ok && t > 0 {
		timeout = time.Duration(t) * time.Millisecond
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := buildHTTPBody(cfg.Data, pool)
	if err != nil {
		return nil, fmt.Errorf("http_request: %w", err)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("http_request: create request: %w", err)
	}

	applyHTTPHeaders(req, cfg.Data, pool)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http_request: %w", err)
	}
	defer resp.Body.Close()

	outputs, err := parseHTTPResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("http_request: %w", err)
	}

	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// buildHTTPBody 构建 HTTP 请求体
func buildHTTPBody(data map[string]any, pool *engine.VariablePool) (io.Reader, error) {
	rawBody, ok := data["body"]
	if !ok {
		return nil, nil
	}

	switch b := rawBody.(type) {
	case string:
		interpolated := pool.Interpolate(b)
		return strings.NewReader(interpolated), nil
	case map[string]any:
		interpolated := pool.InterpolateMap(b)
		bodyBytes, err := json.Marshal(interpolated)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		return bytes.NewReader(bodyBytes), nil
	default:
		bodyBytes, err := json.Marshal(rawBody)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		return bytes.NewReader(bodyBytes), nil
	}
}

// applyHTTPHeaders 设置 HTTP 请求头
func applyHTTPHeaders(req *http.Request, data map[string]any, pool *engine.VariablePool) {
	req.Header.Set("Content-Type", "application/json")

	headers, ok := data["headers"].(map[string]any)
	if !ok {
		return
	}
	interpolated := pool.InterpolateMap(headers)
	for k, v := range interpolated {
		if strVal, ok := v.(string); ok {
			req.Header.Set(k, strVal)
		}
	}
}

// parseHTTPResponse 解析 HTTP 响应
func parseHTTPResponse(resp *http.Response) (map[string]any, error) {
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	outputs := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     flattenHeaders(resp.Header),
	}

	// 尝试解析为 JSON
	var jsonBody any
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
		outputs["body"] = jsonBody
		outputs["response"] = jsonBody
	} else {
		outputs["body"] = string(bodyBytes)
		outputs["response"] = string(bodyBytes)
	}

	return outputs, nil
}

// flattenHeaders 将 http.Header 转为 map[string]string
func flattenHeaders(h http.Header) map[string]string {
	flat := make(map[string]string, len(h))
	for k, v := range h {
		flat[k] = strings.Join(v, ", ")
	}
	return flat
}

// handleTool 调用外部工具
func handleTool(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.tools == nil {
		return nil, fmt.Errorf("tool: ToolExecutor not configured")
	}

	toolName, ok := cfg.Data["tool_name"].(string)
	if !ok || toolName == "" {
		return nil, fmt.Errorf("tool: tool_name is required")
	}

	toolArgs := extractToolArgs(cfg.Data, pool)

	result, err := deps.tools.Execute(ctx, toolName, toolArgs)
	if err != nil {
		return nil, fmt.Errorf("tool %q: %w", toolName, err)
	}

	outputs := map[string]any{
		"result":    result,
		"tool_name": toolName,
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// extractToolArgs 提取并插值工具参数
func extractToolArgs(data map[string]any, pool *engine.VariablePool) map[string]any {
	raw, ok := data["tool_args"].(map[string]any)
	if !ok {
		return make(map[string]any)
	}
	return pool.InterpolateMap(raw)
}

// getStringOrDefault 从 map 中获取字符串，不存在则返回默认值
func getStringOrDefault(data map[string]any, key, defaultVal string) string {
	if v, ok := data[key].(string); ok && v != "" {
		return v
	}
	return defaultVal
}
