package node

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-workbench-api/internal/workflow/engine"
)

// Agent 节点默认配置
const (
	defaultAgentMaxIterations = 15
	agentDefaultModel         = "gpt-4o"
)

// handleAgent 实现 Agent 工具调用循环
func handleAgent(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error) {
	if deps.llm == nil {
		return nil, fmt.Errorf("agent: LLMClient not configured")
	}

	params := parseAgentParams(cfg.Data, pool)
	tools := buildAgentTools(cfg.Data)
	messages := buildAgentMessages(params, cfg, pool)

	finalContent, err := runAgentLoop(ctx, deps, params, messages, tools)
	if err != nil {
		return nil, fmt.Errorf("agent: %w", err)
	}

	outputs := map[string]any{
		"text": finalContent,
	}
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// agentParams 封装 Agent 节点参数
type agentParams struct {
	model         string
	systemPrompt  string
	maxIterations int
}

// parseAgentParams 解析 Agent 参数
func parseAgentParams(data map[string]any, pool *engine.VariablePool) agentParams {
	p := agentParams{
		model:         agentDefaultModel,
		maxIterations: defaultAgentMaxIterations,
	}

	if m, ok := data["model"].(string); ok && m != "" {
		p.model = m
	}
	if sp, ok := data["system_prompt"].(string); ok {
		p.systemPrompt = pool.Interpolate(sp)
	}
	if mi, ok := toInt(data["max_iterations"]); ok && mi > 0 {
		p.maxIterations = mi
	}
	return p
}

// buildAgentTools 从 config.Data 构建工具定义列表
func buildAgentTools(data map[string]any) []ToolDef {
	raw, ok := data["tools"]
	if !ok {
		return nil
	}

	toolsJSON, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var tools []ToolDef
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		return nil
	}
	return tools
}

// buildAgentMessages 构建 Agent 初始消息
func buildAgentMessages(params agentParams, cfg *engine.NodeConfig, pool *engine.VariablePool) []Message {
	messages := make([]Message, 0, 2)

	if params.systemPrompt != "" {
		messages = append(messages, Message{
			Role:    "system",
			Content: params.systemPrompt,
		})
	}

	// 从 inputs 中获取用户消息
	if cfg.Inputs != nil {
		if query, ok := cfg.Inputs["query"].(string); ok {
			interpolated := pool.Interpolate(query)
			messages = append(messages, Message{
				Role:    "user",
				Content: interpolated,
			})
		}
	}

	return messages
}

// runAgentLoop 执行 Agent 工具调用循环
func runAgentLoop(ctx context.Context, deps *Registry, params agentParams, messages []Message, tools []ToolDef) (string, error) {
	var lastContent string

	for i := 0; i < params.maxIterations; i++ {
		select {
		case <-ctx.Done():
			return lastContent, ctx.Err()
		default:
		}

		req := ChatRequest{
			Model:    params.model,
			Messages: messages,
			Tools:    tools,
		}

		resp, err := deps.llm.ChatCompletion(ctx, req)
		if err != nil {
			return lastContent, fmt.Errorf("iteration %d: %w", i, err)
		}

		lastContent = resp.Content

		// 如果没有工具调用，循环结束
		if len(resp.ToolCalls) == 0 {
			return lastContent, nil
		}

		// 将 assistant 消息（含 tool_calls）追加到历史
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// 执行每个工具调用并追加结果
		toolMessages, err := executeAgentTools(ctx, deps, resp.ToolCalls)
		if err != nil {
			return lastContent, err
		}
		messages = append(messages, toolMessages...)
	}

	return lastContent, nil
}

// executeAgentTools 执行 Agent 的工具调用列表
func executeAgentTools(ctx context.Context, deps *Registry, calls []ToolCall) ([]Message, error) {
	messages := make([]Message, 0, len(calls))

	for _, call := range calls {
		result, err := executeSingleTool(ctx, deps, call)
		if err != nil {
			// 将错误作为工具结果返回给 LLM
			messages = append(messages, Message{
				Role:       "tool",
				Content:    fmt.Sprintf("error: %v", err),
				ToolCallID: call.ID,
			})
			continue
		}

		resultStr, err := marshalToolResult(result)
		if err != nil {
			return nil, fmt.Errorf("marshal tool result: %w", err)
		}

		messages = append(messages, Message{
			Role:       "tool",
			Content:    resultStr,
			ToolCallID: call.ID,
		})
	}

	return messages, nil
}

// executeSingleTool 执行单个工具调用
func executeSingleTool(ctx context.Context, deps *Registry, call ToolCall) (any, error) {
	if deps.tools == nil {
		return nil, fmt.Errorf("ToolExecutor not configured")
	}

	var args map[string]any
	if call.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return nil, fmt.Errorf("parse tool args: %w", err)
		}
	}

	return deps.tools.Execute(ctx, call.Function.Name, args)
}

// marshalToolResult 将工具执行结果序列化为字符串
func marshalToolResult(result any) (string, error) {
	switch v := result.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}
