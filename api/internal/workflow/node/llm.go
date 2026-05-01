package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-workbench-api/internal/workflow/engine"
)

// llm 相关默认配置
const (
	defaultModel       = ""
	defaultMaxTokens   = 4096
	defaultTemperature = 0.7
)

// handleLLM 调用 LLM 生成响应
func handleLLM(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.llm == nil {
		return nil, fmt.Errorf("llm node: LLMClient not configured")
	}

	params := parseLLMParams(cfg.Data, pool)
	messages := buildLLMMessages(params.systemPrompt, params.userPrompt)

	req := ChatRequest{
		Model:       params.model,
		Messages:    messages,
		MaxTokens:   params.maxTokens,
		Temperature: params.temperature,
		JSONMode:    params.jsonMode,
	}

	resp, err := deps.llm.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("llm node: %w", err)
	}

	outputs := buildLLMOutputs(resp, params.jsonMode)
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// llmParams 封装 LLM 节点参数
type llmParams struct {
	model        string
	systemPrompt string
	userPrompt   string
	temperature  float64
	maxTokens    int
	jsonMode     bool
}

// parseLLMParams 从 config.Data 解析 LLM 参数并做变量池插值
func parseLLMParams(data map[string]any, pool *engine.VariablePool) llmParams {
	p := llmParams{
		model:       defaultModel,
		maxTokens:   defaultMaxTokens,
		temperature: defaultTemperature,
	}

	if m, ok := data["model"].(string); ok && m != "" {
		p.model = m
	}
	if sp, ok := data["system_prompt"].(string); ok {
		p.systemPrompt = pool.Interpolate(sp)
	}
	if up, ok := data["user_prompt"].(string); ok {
		p.userPrompt = pool.Interpolate(up)
	}
	if t, ok := toFloat64(data["temperature"]); ok {
		p.temperature = t
	}
	if mt, ok := toInt(data["max_tokens"]); ok {
		p.maxTokens = mt
	}
	if jm, ok := data["json_mode"].(bool); ok {
		p.jsonMode = jm
	}

	return p
}

// buildLLMMessages 构建消息列表
func buildLLMMessages(systemPrompt, userPrompt string) []Message {
	messages := make([]Message, 0, 2)
	if systemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}
	if userPrompt != "" {
		messages = append(messages, Message{Role: "user", Content: userPrompt})
	}
	return messages
}

// buildLLMOutputs 构建 LLM 输出
func buildLLMOutputs(resp *ChatResponse, jsonMode bool) map[string]any {
	outputs := map[string]any{
		"text":   resp.Content,
		"result": resp.Content,
	}
	if jsonMode {
		parsed, err := tryParseJSON(resp.Content)
		if err == nil {
			outputs["json"] = parsed
			if m, ok := parsed.(map[string]any); ok {
				for k, v := range m {
					outputs[k] = v
				}
			}
		}
	}
	if len(resp.ToolCalls) > 0 {
		outputs["tool_calls"] = resp.ToolCalls
	}
	return outputs
}

// handleParameterExtractor 使用 LLM 从文本中提取结构化参数
func handleParameterExtractor(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.llm == nil {
		return nil, fmt.Errorf("parameter_extractor: LLMClient not configured")
	}

	text := extractAndInterpolate(cfg.Data, "text", pool)
	if text == "" {
		return nil, fmt.Errorf("parameter_extractor: text is required")
	}

	params, ok := cfg.Data["parameters"]
	if !ok {
		return nil, fmt.Errorf("parameter_extractor: parameters schema is required")
	}

	schemaJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("parameter_extractor: marshal schema: %w", err)
	}

	prompt := buildExtractionPrompt(text, string(schemaJSON))
	model := getModelOrDefault(cfg.Data)

	req := ChatRequest{
		Model:    model,
		Messages: []Message{{Role: "user", Content: prompt}},
		JSONMode: true,
	}

	resp, err := deps.llm.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("parameter_extractor: %w", err)
	}

	extracted, err := tryParseJSON(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parameter_extractor: parse response: %w", err)
	}

	outputs := map[string]any{
		"extracted": extracted,
		"raw":       resp.Content,
	}
	if fields, ok := extracted.(map[string]any); ok {
		for key, value := range fields {
			outputs[key] = value
		}
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// buildExtractionPrompt 构建参数提取提示词
func buildExtractionPrompt(text, schema string) string {
	return fmt.Sprintf(
		"从以下文本中提取结构化参数，严格按照 JSON Schema 输出。\n\n"+
			"JSON Schema:\n```json\n%s\n```\n\n"+
			"文本内容:\n%s\n\n"+
			"请直接输出 JSON，不要包含其他内容。",
		schema, text,
	)
}

// handleQuestionClassifier 使用 LLM 对问题进行分类
func handleQuestionClassifier(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.llm == nil {
		return nil, fmt.Errorf("question_classifier: LLMClient not configured")
	}

	query := extractAndInterpolate(cfg.Data, "query", pool)
	if query == "" {
		return nil, fmt.Errorf("question_classifier: query is required")
	}

	classes, err := parseClasses(cfg.Data)
	if err != nil {
		return nil, err
	}

	prompt := buildClassificationPrompt(query, classes)
	model := getModelOrDefault(cfg.Data)

	req := ChatRequest{
		Model:    model,
		Messages: []Message{{Role: "user", Content: prompt}},
	}

	resp, err := deps.llm.ChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("question_classifier: %w", err)
	}

	label := strings.TrimSpace(resp.Content)
	nextID := resolveClassNextID(label, classes, cfg.Branches)

	outputs := map[string]any{
		"class": label,
		"query": query,
	}
	return &engine.NodeResult{
		Outputs: outputs,
		NextID:  nextID,
		Status:  engine.StatusSucceeded,
	}, nil
}

// classItem 表示分类项
type classItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// parseClasses 从 config.Data 解析分类列表
func parseClasses(data map[string]any) ([]classItem, error) {
	raw, ok := data["classes"]
	if !ok {
		return nil, fmt.Errorf("question_classifier: classes is required")
	}

	classesJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("question_classifier: marshal classes: %w", err)
	}

	var classes []classItem
	if err := json.Unmarshal(classesJSON, &classes); err != nil {
		return nil, fmt.Errorf("question_classifier: parse classes: %w", err)
	}
	return classes, nil
}

// buildClassificationPrompt 构建分类提示词
func buildClassificationPrompt(query string, classes []classItem) string {
	var sb strings.Builder
	sb.WriteString("将以下问题分类到指定类别之一。只输出类别名称，不要输出其他内容。\n\n")
	sb.WriteString("可选类别:\n")
	for _, c := range classes {
		sb.WriteString(fmt.Sprintf("- %s\n", c.Name))
	}
	sb.WriteString(fmt.Sprintf("\n问题: %s\n\n类别:", query))
	return sb.String()
}

// resolveClassNextID 根据分类结果匹配下一个节点 ID
func resolveClassNextID(label string, classes []classItem, branches []engine.BranchConfig) string {
	label = strings.TrimSpace(label)
	for _, c := range classes {
		if strings.EqualFold(c.Name, label) {
			for _, b := range branches {
				if b.ID == c.ID {
					return b.Next
				}
			}
		}
	}
	return ""
}

// --- 辅助函数 ---

// extractAndInterpolate 从 data 中提取字符串字段并做变量池插值
func extractAndInterpolate(data map[string]any, key string, pool *engine.VariablePool) string {
	val, ok := data[key].(string)
	if !ok {
		return ""
	}
	return pool.Interpolate(val)
}

// getModelOrDefault 获取模型名称，不存在则返回默认值
func getModelOrDefault(data map[string]any) string {
	if m, ok := data["model"].(string); ok && m != "" {
		return m
	}
	return defaultModel
}

// tryParseJSON 尝试将字符串解析为 JSON
func tryParseJSON(s string) (any, error) {
	s = strings.TrimSpace(s)
	// 去除可能的 markdown 代码块标记
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var result any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// toFloat64 安全地将 any 转为 float64
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// toInt 安全地将 any 转为 int
func toInt(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
