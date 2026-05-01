package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ai-workbench-api/internal/workflow/engine"
	"ai-workbench-api/internal/workflow/node"
)

// 本文件覆盖类别8：工作流节点级集成测试。
// 对 18 种节点类型创建最小真实工作流执行，并补充 DSL 与执行时故障边界。

// TestIntegrationNodes_All18TypesExecute 验证 18 种节点至少各有 1 个真实执行场景，输出写入变量池并可到达 end。
func TestIntegrationNodes_All18TypesExecute(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"value":7}`)
	}))
	defer httpServer.Close()

	type scenario struct {
		nodeType engine.NodeType
		run      func(t *testing.T)
	}
	scenarios := []scenario{
		{engine.NodeStart, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			result, _ := runTestGraph(t, testGraph("start", startNode("end"), endNode(map[string]any{"host": "{{start.host}}"})), node.NewRegistry(nil, nil, nil), map[string]any{"host": "10.0.1.21"})
			if result.Outputs["host"] != "10.0.1.21" {
				t.Fatalf("start host=%v, want 10.0.1.21", result.Outputs["host"])
			}
		}},
		{engine.NodeEnd, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			result, _ := runTestGraph(t, testGraph("end", startNode("end"), endNode(map[string]any{"ok": "true"})), node.NewRegistry(nil, nil, nil), nil)
			if result.Outputs["ok"] != "true" {
				t.Fatalf("end output=%#v, want ok=true", result.Outputs)
			}
		}},
		{engine.NodeCondition, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			cond := &engine.NodeConfig{ID: "condition", Type: engine.NodeCondition, Branches: []engine.BranchConfig{{ID: "yes", Logic: "and", Next: "yes", Rules: []engine.ConditionRule{{Variable: "{{start.flag}}", Operator: "equals", Value: "yes"}}}, {ID: "no", Logic: "default", Next: "no"}}}
			result, _ := runTestGraph(t, testGraph("condition", startNode("condition"), cond, templateNode("yes", "yes", "end"), templateNode("no", "no", "end"), endNode(map[string]any{"branch": "{{yes.result}}{{no.result}}"})), node.NewRegistry(nil, nil, nil), map[string]any{"flag": "yes"})
			if result.Outputs["branch"] != "yes" {
				t.Fatalf("condition branch=%v, want yes", result.Outputs["branch"])
			}
		}},
		{engine.NodeVariableAggregator, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("aggregator", startNode("agg"), &engine.NodeConfig{ID: "agg", Type: engine.NodeVariableAggregator, Data: map[string]any{"sources": []any{"start.a", "start.b"}}, Next: "tmpl"}, templateNode("tmpl", `{{index .agg "start.a"}}/{{index .agg "start.b"}}`, "end"), endNode(map[string]any{"value": "{{tmpl.result}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), map[string]any{"a": "A", "b": "B"})
			if result.Outputs["value"] != "A/B" {
				t.Fatalf("aggregator output=%v, want A/B", result.Outputs["value"])
			}
		}},
		{engine.NodeVariableAssigner, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("assigner", startNode("assign"), &engine.NodeConfig{ID: "assign", Type: engine.NodeVariableAssigner, Data: map[string]any{"assignments": []any{map[string]any{"target": "assigned.value", "value": "{{start.value}}"}}}, Next: "end"}, endNode(map[string]any{"value": "{{assigned.value}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), map[string]any{"value": "assigned-ok"})
			if result.Outputs["value"] != "assigned-ok" {
				t.Fatalf("assigner output=%v, want assigned-ok", result.Outputs["value"])
			}
		}},
		{engine.NodeLLM, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(&fakeLLMClient{responses: []node.ChatResponse{{Content: "llm-ok"}}}, nil, nil)
			g := testGraph("llm", startNode("llm"), &engine.NodeConfig{ID: "llm", Type: engine.NodeLLM, Data: map[string]any{"user_prompt": "hello"}, Next: "end"}, endNode(map[string]any{"text": "{{llm.result}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			if result.Outputs["text"] != "llm-ok" {
				t.Fatalf("llm output=%v, want llm-ok", result.Outputs["text"])
			}
		}},
		{engine.NodeParameterExtractor, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(&fakeLLMClient{responses: []node.ChatResponse{{Content: `{"ip":"10.0.1.21"}`}}}, nil, nil)
			g := testGraph("extractor", startNode("extract"), &engine.NodeConfig{ID: "extract", Type: engine.NodeParameterExtractor, Data: map[string]any{"text": "host 10.0.1.21", "parameters": map[string]any{"ip": "string"}}, Next: "end"}, endNode(map[string]any{"raw": "{{extract.raw}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			AssertResponseContains(t, []byte(fmt.Sprint(result.Outputs["raw"])), "10.0.1.21")
		}},
		{engine.NodeQuestionClassifier, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(&fakeLLMClient{responses: []node.ChatResponse{{Content: "CPU"}}}, nil, nil)
			g := testGraph("classifier", startNode("classify"), &engine.NodeConfig{ID: "classify", Type: engine.NodeQuestionClassifier, Data: map[string]any{"query": "cpu high", "classes": []any{map[string]any{"id": "cpu", "name": "CPU"}}}, Next: "end"}, endNode(map[string]any{"class": "{{classify.class}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			if result.Outputs["class"] != "CPU" {
				t.Fatalf("classifier output=%v, want CPU", result.Outputs["class"])
			}
		}},
		{engine.NodeKnowledgeRetrieval, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(nil, fakeKnowledgeSearcher{results: []node.KnowledgeResult{{ID: "k1", Score: 0.9, Category: "CPU", Description: "CPU doc"}}}, nil)
			g := testGraph("knowledge", startNode("kb"), &engine.NodeConfig{ID: "kb", Type: engine.NodeKnowledgeRetrieval, Data: map[string]any{"query": "CPU", "top_k": 1}, Next: "end"}, endNode(map[string]any{"count": "{{kb.count}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			if fmt.Sprint(result.Outputs["count"]) != "1" {
				t.Fatalf("knowledge count=%v, want 1", result.Outputs["count"])
			}
		}},
		{engine.NodeHTTPRequest, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("http", startNode("http"), &engine.NodeConfig{ID: "http", Type: engine.NodeHTTPRequest, Data: map[string]any{"url": httpServer.URL}, Next: "end"}, endNode(map[string]any{"code": "{{http.status_code}}", "value": "{{http.response.value}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), nil)
			if fmt.Sprint(result.Outputs["code"]) != "200" || fmt.Sprint(result.Outputs["value"]) != "7" {
				t.Fatalf("http outputs=%#v, want code=200 value=7", result.Outputs)
			}
		}},
		{engine.NodeTool, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(nil, nil, fakeToolExecutor{result: map[string]any{"value": "tool-ok"}})
			g := testGraph("tool", startNode("tool"), &engine.NodeConfig{ID: "tool", Type: engine.NodeTool, Data: map[string]any{"tool_name": "lookup"}, Next: "end"}, endNode(map[string]any{"value": "{{tool.result.value}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			if result.Outputs["value"] != "tool-ok" {
				t.Fatalf("tool output=%v, want tool-ok", result.Outputs["value"])
			}
		}},
		{engine.NodeLoop, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("loop", startNode("loop"), &engine.NodeConfig{ID: "loop", Type: engine.NodeLoop, Data: map[string]any{"items": []any{1, 2, 3}}, Next: "end"}, endNode(map[string]any{"count": "{{loop.count}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), nil)
			if fmt.Sprint(result.Outputs["count"]) != "3" {
				t.Fatalf("loop count=%v, want 3", result.Outputs["count"])
			}
		}},
		{engine.NodeIteration, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("iteration", startNode("iteration"), &engine.NodeConfig{ID: "iteration", Type: engine.NodeIteration, Data: map[string]any{"items": []any{1, 2, 3}, "template": "{{.item}}!"}, Next: "end"}, endNode(map[string]any{"count": "{{iteration.count}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), nil)
			if fmt.Sprint(result.Outputs["count"]) != "3" {
				t.Fatalf("iteration count=%v, want 3", result.Outputs["count"])
			}
		}},
		{engine.NodeListFilter, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("filter", startNode("filter"), &engine.NodeConfig{ID: "filter", Type: engine.NodeListFilter, Data: map[string]any{"items": []any{map[string]any{"value": 1}, map[string]any{"value": 4}, map[string]any{"value": 5}}, "filter_rules": []any{map[string]any{"field": "value", "operator": "gt", "value": "3"}}}, Next: "end"}, endNode(map[string]any{"count": "{{filter.count}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), nil)
			if fmt.Sprint(result.Outputs["count"]) != "2" {
				t.Fatalf("filter count=%v, want 2", result.Outputs["count"])
			}
		}},
		{engine.NodeTemplateTransform, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			result, _ := runTestGraph(t, testGraph("template", startNode("tmpl"), templateNode("tmpl", "Hello {{name}}", "end"), endNode(map[string]any{"text": "{{tmpl.result}}"})), node.NewRegistry(nil, nil, nil), map[string]any{"name": "World"})
			if result.Outputs["text"] != "Hello World" {
				t.Fatalf("template output=%v, want Hello World", result.Outputs["text"])
			}
		}},
		{engine.NodeCode, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("code", startNode("code"), &engine.NodeConfig{ID: "code", Type: engine.NodeCode, Inputs: map[string]any{"a": "{{start.a}}", "b": "{{start.b}}"}, Data: map[string]any{"language": "javascript", "code": "outputs.sum = Number(inputs.a) + Number(inputs.b);"}, Next: "end"}, endNode(map[string]any{"sum": "{{code.outputs.sum}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), map[string]any{"a": 2, "b": 3})
			if fmt.Sprint(result.Outputs["sum"]) != "5" {
				t.Fatalf("code sum=%v, want 5", result.Outputs["sum"])
			}
		}},
		{engine.NodeAgent, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			reg := node.NewRegistry(&fakeLLMClient{responses: []node.ChatResponse{{Content: "agent-ok"}}}, nil, fakeToolExecutor{})
			g := testGraph("agent", startNode("agent"), &engine.NodeConfig{ID: "agent", Type: engine.NodeAgent, Inputs: map[string]any{"query": "hello"}, Data: map[string]any{"max_iterations": 1}, Next: "end"}, endNode(map[string]any{"text": "{{agent.text}}"}))
			result, _ := runTestGraph(t, g, reg, nil)
			if result.Outputs["text"] != "agent-ok" {
				t.Fatalf("agent output=%v, want agent-ok", result.Outputs["text"])
			}
		}},
		{engine.NodeDocumentExtractor, func(t *testing.T) {
			env := SetupTestEnv(t)
			defer env.Cleanup(t)
			g := testGraph("doc", startNode("doc"), &engine.NodeConfig{ID: "doc", Type: engine.NodeDocumentExtractor, Data: map[string]any{"document": "content"}, Next: "end"}, endNode(map[string]any{"text": "{{doc.text}}"}))
			result, _ := runTestGraph(t, g, node.NewRegistry(nil, nil, nil), nil)
			if strings.TrimSpace(fmt.Sprint(result.Outputs["text"])) == "" {
				t.Fatalf("document extractor output empty: %#v", result.Outputs)
			}
		}},
	}

	covered := map[engine.NodeType]bool{}
	for _, sc := range scenarios {
		sc := sc
		t.Run(string(sc.nodeType), func(t *testing.T) {
			sc.run(t)
			covered[sc.nodeType] = true
		})
	}
	for _, typ := range []engine.NodeType{engine.NodeStart, engine.NodeEnd, engine.NodeCondition, engine.NodeVariableAggregator, engine.NodeVariableAssigner, engine.NodeLLM, engine.NodeParameterExtractor, engine.NodeQuestionClassifier, engine.NodeKnowledgeRetrieval, engine.NodeHTTPRequest, engine.NodeTool, engine.NodeLoop, engine.NodeIteration, engine.NodeListFilter, engine.NodeTemplateTransform, engine.NodeCode, engine.NodeAgent, engine.NodeDocumentExtractor} {
		if !covered[typ] {
			t.Fatalf("node type %s was not covered", typ)
		}
	}
}

// TestIntegrationNodes_WorkflowDSLBoundaries 验证 DSL 空、非法 YAML、缺 start/end、环路、不可达节点、未知节点类型均被拒绝；缺失变量不再泄漏模板语法。
func TestIntegrationNodes_WorkflowDSLBoundaries(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	badDSLs := []struct {
		name string
		dsl  string
	}{
		{"empty_dsl", ""},
		{"invalid_yaml", "workflow: ["},
		{"missing_start", correctnessWorkflowDSL("missing-start", `    - id: end
      type: end
`)},
		{"missing_end", correctnessWorkflowDSL("missing-end", `    - id: start
      type: start
`)},
		{"cycle", correctnessWorkflowDSL("cycle", `    - id: start
      type: start
      next: a
    - id: a
      type: template_transform
      config: {template: "a"}
      next: start
    - id: end
      type: end
`)},
		{"unreachable", correctnessWorkflowDSL("unreachable", `    - id: start
      type: start
      next: end
    - id: orphan
      type: template_transform
      config: {template: "orphan"}
    - id: end
      type: end
`)},
		{"unknown_type", correctnessWorkflowDSL("unknown-type", `    - id: start
      type: start
      next: mystery
    - id: mystery
      type: not_exists
      next: end
    - id: end
      type: end
`)},
	}
	for _, tc := range badDSLs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, status := requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/workflows", map[string]any{"name": tc.name, "dsl": tc.dsl}))
			assertStatusWithBody(t, status, body, http.StatusBadRequest)
		})
	}

	missingVarDSL := correctnessWorkflowDSL("missing-var", `    - id: start
      type: start
      next: end
    - id: end
      type: end
      outputs:
        value: "prefix-{{missing.value}}-suffix"
`)
	id := createWorkflowViaAPI(t, env, correctnessID(t, "wf-missing-var"), missingVarDSL)
	body, status := requestBody(t, env, env.NoAuthRequest(http.MethodPost, "/api/v1/workflows/"+id+"/run", map[string]any{"inputs": map[string]any{}}))
	assertStatusWithBody(t, status, body, http.StatusOK)
	AssertResponseContains(t, body, "prefix--suffix")
	if strings.Contains(string(body), "{{missing.value}}") {
		t.Fatalf("missing variable leaked template syntax: %s", string(body))
	}
}

// TestIntegrationNodes_ExecutionBoundaries 验证 LLM/http/code/tool/knowledge 故障、SSE 断开、并发执行、超大 inputs、内置工作流篡改和失败策略。
func TestIntegrationNodes_ExecutionBoundaries(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)

	t.Run("stop_and_continue_on_failure", func(t *testing.T) {
		g := testGraph("failure-policy", startNode("tool"), &engine.NodeConfig{ID: "tool", Type: engine.NodeTool, Data: map[string]any{"tool_name": "missing"}, Next: "end"}, endNode(map[string]any{"ok": "done"}))
		cfg := engine.DefaultConfig()
		cfg.Timeout = time.Second
		cfg.NodeTimeout = 50 * time.Millisecond
		stopEngine := engine.NewEngine(g, node.NewRegistry(nil, nil, nil), cfg)
		stopResult, err := stopEngine.Run(context.Background(), nil)
		if err != nil || stopResult.Status != engine.StatusFailed {
			t.Fatalf("stop_on_failure result=%+v err=%v, want failed result without panic", stopResult, err)
		}
		cfg.ErrorHandling = engine.ErrorContinue
		continueEngine := engine.NewEngine(g, node.NewRegistry(nil, nil, nil), cfg)
		continueResult, err := continueEngine.Run(context.Background(), nil)
		if err != nil || continueResult.Status != engine.StatusSucceeded {
			t.Fatalf("continue_on_failure result=%+v err=%v, want succeeded", continueResult, err)
		}
	})

	t.Run("timeouts_and_empty_dependencies", func(t *testing.T) {
		timeoutHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond) }))
		defer timeoutHTTP.Close()
		for _, graph := range []*engine.Graph{
			testGraph("llm-timeout", startNode("llm"), &engine.NodeConfig{ID: "llm", Type: engine.NodeLLM, Data: map[string]any{"user_prompt": "x"}, Next: "end"}, endNode(nil)),
			testGraph("http-timeout", startNode("http"), &engine.NodeConfig{ID: "http", Type: engine.NodeHTTPRequest, Data: map[string]any{"url": timeoutHTTP.URL, "timeout": 1}, Next: "end"}, endNode(nil)),
			testGraph("code-loop", startNode("code"), &engine.NodeConfig{ID: "code", Type: engine.NodeCode, Data: map[string]any{"language": "javascript", "timeout": 1, "code": "while(true){}"}, Next: "end"}, endNode(nil)),
			testGraph("tool-missing", startNode("tool"), &engine.NodeConfig{ID: "tool", Type: engine.NodeTool, Data: map[string]any{"tool_name": "missing"}, Next: "end"}, endNode(nil)),
		} {
			cfg := engine.DefaultConfig()
			cfg.Timeout = time.Second
			cfg.NodeTimeout = 20 * time.Millisecond
			cfg.MaxRetries = 0
			result, err := engine.NewEngine(graph, node.NewRegistry(nil, fakeKnowledgeSearcher{}, nil), cfg).Run(context.Background(), nil)
			if err != nil || result == nil || result.Status != engine.StatusFailed {
				t.Fatalf("%s result=%+v err=%v, want failed result", graph.Name, result, err)
			}
		}
	})

	t.Run("stream_disconnect_and_concurrent_runs", func(t *testing.T) {
		id := createWorkflowViaAPI(t, env, correctnessID(t, "wf-stream"), minimalWorkflowDSL("stream-ok"))
		req := env.NoAuthRequest(http.MethodPost, "/api/v1/workflows/"+id+"/stream", map[string]any{"inputs": map[string]any{"ok": "yes"}})
		resp := env.do(t, req)
		_ = resp.Body.Close()

		var wg sync.WaitGroup
		errs := make(chan string, 10)
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				body, status := requestBody(t, env, env.NoAuthRequest(http.MethodPost, "/api/v1/workflows/"+id+"/run", map[string]any{"inputs": map[string]any{"ok": fmt.Sprintf("yes-%d", i)}}))
				if status != http.StatusOK || strings.Contains(string(body), "{{") {
					errs <- fmt.Sprintf("run[%d] status=%d body=%s", i, status, string(body))
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		if len(errs) > 0 {
			t.Fatalf("concurrent workflow run failed: %s", <-errs)
		}

		body, status := requestBody(t, env, env.NoAuthRequest(http.MethodPost, "/api/v1/workflows/"+id+"/run", map[string]any{"inputs": map[string]any{"payload": strings.Repeat("x", 1<<20)}}))
		assertStatusWithBody(t, status, body, http.StatusOK, http.StatusBadRequest, http.StatusRequestEntityTooLarge)
	})

	t.Run("builtin_tamper_forbidden", func(t *testing.T) {
		body, status := requestBody(t, env, env.AdminRequest(http.MethodPut, "/api/v1/workflows/builtin:health_inspection", workflowBody("tamper")))
		assertStatusWithBody(t, status, body, http.StatusForbidden)
		body, status = requestBody(t, env, env.AdminRequest(http.MethodDelete, "/api/v1/workflows/builtin:health_inspection", nil))
		assertStatusWithBody(t, status, body, http.StatusForbidden)
	})
}
