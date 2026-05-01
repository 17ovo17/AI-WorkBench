package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
)

// TestKnowledge_APIEndpointMatrix 覆盖知识库案例、文档、Runbook 共 21 个端点。
func TestKnowledge_APIEndpointMatrix(t *testing.T) {
	runEndpointModuleTests(t, "knowledge")
}

// TestKnowledge_CaseChain 验证案例创建、详情、更新、导出、导入、删除链路。
func TestKnowledge_CaseChain(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)
	id := "aiw-case-chain-a3f7b2c1"
	payload := map[string]any{"id": id, "root_cause_category": "CPU", "root_cause_description": "aiw case chain", "treatment_steps": `["check cpu"]`, "keywords": "aiw"}
	_, status := requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/cases", payload))
	assertStatusIn(t, status, http.StatusOK)
	body, status := requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/cases/"+id, nil))
	assertStatusIn(t, status, http.StatusOK)
	AssertResponseContains(t, body, "aiw case chain")
	payload["root_cause_description"] = "aiw case chain updated"
	_, status = requestBody(t, env, env.AdminRequest(http.MethodPut, "/api/v1/knowledge/cases/"+id, payload))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodGet, "/api/v1/knowledge/cases/export", nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/cases/import", []map[string]any{payload}))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodDelete, "/api/v1/knowledge/cases/"+id, nil))
	assertStatusIn(t, status, http.StatusOK)
}

// TestKnowledge_CaseTemplatePollutionGuard 验证未渲染模板不会进入案例库，历史脏数据也不会占据列表首页。
func TestKnowledge_CaseTemplatePollutionGuard(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)
	badID := "aiw-case-bad-template-a3f7b2c1"
	goodID := "aiw-case-good-template-a3f7b2c1"
	defer store.DeleteCase(badID)
	defer store.DeleteCase(goodID)

	_, status := requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/cases", map[string]any{
		"id":                     badID,
		"root_cause_category":    "{{parameter_extractor.root_cause_category}}",
		"root_cause_description": "{{parameter_extractor.root_cause_description}}",
		"treatment_steps":        "check",
		"keywords":               "{{parameter_extractor.keywords}}",
	}))
	assertStatusIn(t, status, http.StatusBadRequest)

	store.SaveCase(&model.DiagnosisCase{
		ID:                   badID,
		RootCauseCategory:    "{{parameter_extractor.root_cause_category}}",
		RootCauseDescription: "{{parameter_extractor.root_cause_description}}",
		Keywords:             "{{parameter_extractor.keywords}}",
		TreatmentSteps:       "bad",
		MetricSnapshot:       json.RawMessage(`"{{parameter_extractor.metric_snapshot}}"`),
		CreatedAt:            time.Now(),
	})
	store.SaveCase(&model.DiagnosisCase{
		ID:                   goodID,
		RootCauseCategory:    "CPU",
		RootCauseDescription: "CPU 使用率过高导致接口延迟",
		Keywords:             "cpu,latency",
		TreatmentSteps:       "检查 CPU 热点进程",
		MetricSnapshot:       json.RawMessage("{}"),
		CreatedAt:            time.Now().Add(time.Second),
	})

	body, status := requestBody(t, env, env.AdminRequest(http.MethodGet, "/api/v1/knowledge/cases?limit=20", nil))
	assertStatusIn(t, status, http.StatusOK)
	AssertResponseContains(t, body, "CPU 使用率过高")
	if strings.Contains(string(body), "{{") || strings.Contains(string(body), "parameter_extractor.") {
		t.Fatalf("case list leaked unrendered template data: %s", string(body))
	}

	body, status = requestBody(t, env, env.AdminRequest(http.MethodPut, "/api/v1/knowledge/cases/"+goodID, map[string]any{
		"root_cause_category":    "CPU",
		"root_cause_description": "{{parameter_extractor.root_cause_description}}",
		"keywords":               "cpu",
		"treatment_steps":        "check",
		"metric_snapshot":        map[string]any{"cpu": 95},
	}))
	assertStatusIn(t, status, http.StatusBadRequest)
	AssertResponseContains(t, body, "unrendered template")
}

// TestKnowledge_DocumentChain 验证文档上传、列表、详情、重建索引、搜索、删除链路。
func TestKnowledge_DocumentChain(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)
	body, status := requestBody(t, env, multipartUploadRequest(t, env, "/api/v1/knowledge/documents/upload", "file", "aiw-doc.md", "# AIW\n测试文档", map[string]string{"title": "aiw doc", "doc_type": "md"}))
	assertStatusIn(t, status, http.StatusOK)
	var doc struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &doc); err != nil || doc.ID == "" {
		t.Fatalf("decode document id: err=%v body=%s", err, string(body))
	}
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/documents", nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/documents/"+doc.ID, nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/documents/"+doc.ID+"/reindex", map[string]any{}))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodPost, "/api/v1/knowledge/search", map[string]any{"query": "AIW", "top_k": 5}))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/search/stats", nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/search/badcase", map[string]any{"query": "AIW", "doc_id": doc.ID, "reason": "不相关"}))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodDelete, "/api/v1/knowledge/documents/"+doc.ID, nil))
	assertStatusIn(t, status, http.StatusOK)
}

// TestKnowledge_RunbookChain 验证 Runbook 创建、详情、执行、历史、删除链路。
func TestKnowledge_RunbookChain(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup(t)
	id := "aiw-runbook-chain-a3f7b2c1"
	payload := map[string]any{"id": id, "title": "aiw runbook chain", "category": "故障处置", "steps": `[{"title":"检查","action":"echo ok","expected_result":"ok"}]`}
	_, status := requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/runbooks", payload))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/runbooks/"+id, nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodPost, "/api/v1/knowledge/runbooks/"+id+"/execute", map[string]any{"target_ip": "127.0.0.1", "variables": map[string]string{"x": "y"}, "executor": "tester"}))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.NoAuthRequest(http.MethodGet, "/api/v1/knowledge/runbooks/"+id+"/history", nil))
	assertStatusIn(t, status, http.StatusOK)
	_, status = requestBody(t, env, env.AdminRequest(http.MethodDelete, "/api/v1/knowledge/runbooks/"+id, nil))
	assertStatusIn(t, status, http.StatusOK)
}
