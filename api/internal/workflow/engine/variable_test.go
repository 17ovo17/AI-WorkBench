package engine

import "testing"

func TestVariablePoolInterpolateMapNestedPath(t *testing.T) {
	pool := NewVariablePool()
	pool.SetAll("llm_diagnosis", map[string]any{
		"result": "CPU bottleneck",
		"json": map[string]any{
			"root_cause": "high iowait",
		},
	})

	got := pool.InterpolateMap(map[string]any{
		"diagnosis": "{{llm_diagnosis.result}}",
		"nested": map[string]any{
			"root": "{{llm_diagnosis.json.root_cause}}",
		},
		"list": []any{"{{llm_diagnosis.result}}"},
	})

	if got["diagnosis"] != "CPU bottleneck" {
		t.Fatalf("diagnosis was not interpolated: %#v", got["diagnosis"])
	}
	nested := got["nested"].(map[string]any)
	if nested["root"] != "high iowait" {
		t.Fatalf("nested path was not interpolated: %#v", nested["root"])
	}
	list := got["list"].([]any)
	if list[0] != "CPU bottleneck" {
		t.Fatalf("list item was not interpolated: %#v", list[0])
	}
}

func TestVariablePoolInterpolateMapPreservesWholeTemplateValueType(t *testing.T) {
	pool := NewVariablePool()
	pool.SetAll("parameter_extractor", map[string]any{
		"root_cause_category": "cpu_high",
		"metric_snapshot": map[string]any{
			"cpu_usage_active": 96.7,
		},
	})

	got := pool.InterpolateMap(map[string]any{
		"category":        "{{parameter_extractor.root_cause_category}}",
		"metric_snapshot": "{{parameter_extractor.metric_snapshot}}",
		"summary":         "cpu={{parameter_extractor.metric_snapshot.cpu_usage_active}}",
	})

	if got["category"] != "cpu_high" {
		t.Fatalf("category was not interpolated: got=%#v want=%q", got["category"], "cpu_high")
	}
	snapshot, ok := got["metric_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("whole-template object should keep map type: got=%T %#v", got["metric_snapshot"], got["metric_snapshot"])
	}
	if snapshot["cpu_usage_active"] != 96.7 {
		t.Fatalf("snapshot value mismatch: got=%#v want=%#v", snapshot["cpu_usage_active"], 96.7)
	}
	if got["summary"] != "cpu=96.7" {
		t.Fatalf("embedded template should render as text: got=%#v want=%q", got["summary"], "cpu=96.7")
	}
}
