package node

import (
	"context"
	"strings"
	"testing"

	"ai-workbench-api/internal/workflow/engine"
)

type regressionLLMClient struct {
	content string
}

func (c regressionLLMClient) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{Content: c.content}, nil
}

func TestLLMOutputsExposeResultForTemplateVariables(t *testing.T) {
	outputs := buildLLMOutputs(&ChatResponse{Content: `{"confidence":"HIGH","recommended_runbook_category":"CPU"}`}, true)
	if outputs["result"] == "" {
		t.Fatalf("llm result output is missing: %#v", outputs)
	}
	if outputs["confidence"] != "HIGH" {
		t.Fatalf("json_mode fields were not promoted into outputs: %#v", outputs)
	}
}

func TestTemplateTransformInterpolatesVariablePoolNestedPaths(t *testing.T) {
	pool := engine.NewVariablePool()
	pool.SetAll("llm_diagnosis", map[string]any{
		"result":     "CPU bottleneck",
		"confidence": "HIGH",
		"json": map[string]any{
			"root_cause": "high iowait",
		},
	})

	cfg := &engine.NodeConfig{Data: map[string]any{
		"template": "diagnosis={{llm_diagnosis.result}}\nroot={{llm_diagnosis.json.root_cause}}\nconfidence={{.llm_diagnosis.confidence}}",
	}}
	result, err := handleTemplateTransform(context.Background(), "template", cfg, pool, nil)
	if err != nil {
		t.Fatalf("template_transform failed: %v", err)
	}
	text, _ := result.Outputs["text"].(string)
	for _, want := range []string{"diagnosis=CPU bottleneck", "root=high iowait", "confidence=HIGH"} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered template missing %q: %s", want, text)
		}
	}
	if result.Outputs["result"] != text {
		t.Fatalf("template_transform should expose result alias, got %#v", result.Outputs)
	}
}

func TestParameterExtractorPromotesExtractedFieldsForTemplateVariables(t *testing.T) {
	pool := engine.NewVariablePool()
	cfg := &engine.NodeConfig{Data: map[string]any{
		"text": "CPU 使用率 96.7%，接口延迟升高",
		"parameters": map[string]any{
			"root_cause_category":    map[string]any{"type": "string"},
			"root_cause_description": map[string]any{"type": "string"},
			"metric_snapshot":        map[string]any{"type": "object"},
		},
	}}
	reg := NewRegistry(regressionLLMClient{content: `{"root_cause_category":"cpu_high","root_cause_description":"CPU 使用率过高","metric_snapshot":{"cpu_usage_active":96.7}}`}, nil, nil)

	result, err := handleParameterExtractor(context.Background(), "parameter_extractor", cfg, pool, reg)
	if err != nil {
		t.Fatalf("parameter_extractor failed: %v", err)
	}
	pool.SetAll("parameter_extractor", result.Outputs)
	rendered := pool.InterpolateMap(map[string]any{
		"root_cause_category":    "{{parameter_extractor.root_cause_category}}",
		"root_cause_description": "{{parameter_extractor.root_cause_description}}",
		"metric_snapshot":        "{{parameter_extractor.metric_snapshot}}",
	})

	if rendered["root_cause_category"] != "cpu_high" {
		t.Fatalf("promoted category mismatch: got=%#v want=%q outputs=%#v", rendered["root_cause_category"], "cpu_high", result.Outputs)
	}
	if rendered["root_cause_description"] != "CPU 使用率过高" {
		t.Fatalf("promoted description mismatch: got=%#v want=%q outputs=%#v", rendered["root_cause_description"], "CPU 使用率过高", result.Outputs)
	}
	if _, ok := rendered["metric_snapshot"].(map[string]any); !ok {
		t.Fatalf("metric_snapshot should preserve object type: got=%T %#v", rendered["metric_snapshot"], rendered["metric_snapshot"])
	}
}
