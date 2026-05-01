package handler

import (
	"encoding/json"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow/engine"
)

// archiveWorkflowResult 将工作流结果保存为 DiagnoseRecord（仅在执行成功时归档）。
func archiveWorkflowResult(req *startDiagnosisRequest, result *engine.WorkflowResult) {
	if result == nil || result.Status != engine.StatusSucceeded {
		return
	}
	rec := buildDiagnoseRecord(req, result.Outputs)
	store.AddRecord(rec)
	go triggerKnowledgeEnrich(rec.ID)
}

// archiveWorkflowOutputs 将 streaming 模式下监听到的 outputs 归档为 DiagnoseRecord。
func archiveWorkflowOutputs(req *startDiagnosisRequest, outputs map[string]any) {
	if outputs == nil {
		return
	}
	rec := buildDiagnoseRecord(req, outputs)
	store.AddRecord(rec)
	go triggerKnowledgeEnrich(rec.ID)
}

// buildDiagnoseRecord 构造一条工作流来源的诊断记录。
func buildDiagnoseRecord(req *startDiagnosisRequest, outputs map[string]any) *model.DiagnoseRecord {
	now := time.Now()
	end := now
	return &model.DiagnoseRecord{
		ID:            "wf_" + store.NewID(),
		TargetIP:      req.Hostname,
		Trigger:       "workflow",
		Source:        "workflow",
		DataSource:    "diagnosis_workflow",
		Status:        model.StatusDone,
		Report:        normalizeReportText(formatWorkflowReport(outputs)),
		SummaryReport: normalizeReportText(extractWorkflowSummary(outputs)),
		RawReport:     jsonString(outputs),
		AlertTitle:    req.UserQuestion,
		CreateTime:    now,
		EndTime:       &end,
	}
}

// formatWorkflowReport 提取主报告文本：优先 outputs.diagnosis，否则整体序列化。
func formatWorkflowReport(outputs map[string]any) string {
	if outputs == nil {
		return ""
	}
	if d, ok := outputs["diagnosis"]; ok {
		switch v := d.(type) {
		case string:
			return v
		case map[string]any:
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	return jsonString(outputs)
}

// extractWorkflowSummary 提取摘要：outputs.diagnosis.summary（如果存在且为字符串）。
func extractWorkflowSummary(outputs map[string]any) string {
	if outputs == nil {
		return ""
	}
	d, ok := outputs["diagnosis"].(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := d["summary"].(string); ok {
		return s
	}
	return ""
}

// jsonString 将任意值序列化为 JSON 字符串（错误时返回空字符串）。
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
