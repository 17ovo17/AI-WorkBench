package node

import (
	"context"
	"fmt"

	"ai-workbench-api/internal/workflow/engine"
)

// LLMClient 是 LLM 调用的抽象接口，由外部注入
type LLMClient interface {
	ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ChatRequest 封装 LLM 请求参数
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []ToolDef `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	JSONMode    bool      `json:"json_mode,omitempty"`
}

// Message 表示对话消息
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall 表示 LLM 发起的工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 表示函数调用详情
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDef 定义可供 LLM 调用的工具
type ToolDef struct {
	Type     string         `json:"type"`
	Function ToolFunctionDef `json:"function"`
}

// ToolFunctionDef 工具函数定义
type ToolFunctionDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ChatResponse 封装 LLM 响应
type ChatResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// KnowledgeSearcher 是知识库检索的抽象接口
type KnowledgeSearcher interface {
	Search(ctx context.Context, query string, topK int, category string) ([]KnowledgeResult, error)
}

// KnowledgeResult 知识库检索结果
type KnowledgeResult struct {
	ID          string  `json:"id"`
	Score       float64 `json:"score"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Treatment   string  `json:"treatment"`
	Keywords    string  `json:"keywords"`
}

// ToolExecutor 是外部工具执行的抽象接口
type ToolExecutor interface {
	Execute(ctx context.Context, toolName string, args map[string]any) (any, error)
}

// WorkflowRunnerFunc 是子工作流执行函数签名，由外部注入
type WorkflowRunnerFunc func(ctx context.Context, name string, inputs map[string]any) (map[string]any, error)

// NodeHandler 是节点处理函数签名
type NodeHandler func(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, deps *Registry) (*engine.NodeResult, error)

// Registry 持有所有节点处理函数和外部依赖
type Registry struct {
	handlers       map[engine.NodeType]NodeHandler
	llm            LLMClient
	knowledge      KnowledgeSearcher
	tools          ToolExecutor
	workflowRunner WorkflowRunnerFunc
}

// NewRegistry 创建节点注册表并注册所有内置节点处理器
func NewRegistry(llm LLMClient, knowledge KnowledgeSearcher, tools ToolExecutor) *Registry {
	r := &Registry{
		handlers:  make(map[engine.NodeType]NodeHandler),
		llm:       llm,
		knowledge: knowledge,
		tools:     tools,
	}
	r.registerAll()
	return r
}

// Run 实现 engine.NodeRunner 接口
func (r *Registry) Run(ctx context.Context, nodeID string, config *engine.NodeConfig, pool *engine.VariablePool) (*engine.NodeResult, error) {
	handler, ok := r.handlers[config.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported node type: %s", config.Type)
	}
	return handler(ctx, nodeID, config, pool, r)
}

// registerAll 注册所有节点类型的处理器
func (r *Registry) registerAll() {
	// 基础节点
	r.handlers[engine.NodeStart] = handleStart
	r.handlers[engine.NodeEnd] = handleEnd
	r.handlers[engine.NodeCondition] = handleCondition
	r.handlers[engine.NodeVariableAggregator] = handleVariableAggregator
	r.handlers[engine.NodeVariableAssigner] = handleVariableAssigner

	// LLM 相关节点
	r.handlers[engine.NodeLLM] = handleLLM
	r.handlers[engine.NodeParameterExtractor] = handleParameterExtractor
	r.handlers[engine.NodeQuestionClassifier] = handleQuestionClassifier

	// 数据节点
	r.handlers[engine.NodeKnowledgeRetrieval] = handleKnowledgeRetrieval
	r.handlers[engine.NodeHTTPRequest] = handleHTTPRequest
	r.handlers[engine.NodeTool] = handleTool

	// 流程控制节点
	r.handlers[engine.NodeLoop] = handleLoop
	r.handlers[engine.NodeIteration] = handleIteration
	r.handlers[engine.NodeListFilter] = handleListFilter
	r.handlers[engine.NodeTemplateTransform] = handleTemplateTransform

	// 代码沙箱节点
	r.handlers[engine.NodeCode] = handleCode

	// Agent 节点
	r.handlers[engine.NodeAgent] = handleAgent

	// 文档提取节点（占位）
	r.handlers[engine.NodeDocumentExtractor] = handleDocumentExtractor

	// 子工作流节点
	r.handlers[engine.NodeSubWorkflow] = handleSubWorkflow

	// 人工输入节点
	r.handlers[engine.NodeHumanInput] = handleHumanInput
}

// SetWorkflowRunner 注入子工作流执行函数
func (r *Registry) SetWorkflowRunner(fn WorkflowRunnerFunc) {
	r.workflowRunner = fn
}

// GetWorkflowRunner 获取子工作流执行函数
func (r *Registry) GetWorkflowRunner() WorkflowRunnerFunc {
	return r.workflowRunner
}
