package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-workbench-api/internal/eventbus"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow"

	log "github.com/sirupsen/logrus"
)

// RunDiagnoseViaWorkflow 通过工作流引擎执行诊断（替代直连 LLM 的 RunDiagnose）
func RunDiagnoseViaWorkflow(rec *model.DiagnoseRecord, question string) {
	store.UpdateRecord(rec.ID, func(r *model.DiagnoseRecord) {
		r.Status = model.StatusRunning
	})

	route := RouteToWorkflow(question, rec.TargetIP, rec.AlertTitle, "")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := workflow.RunWorkflow(ctx, route.WorkflowName, route.Inputs)

	now := time.Now()
	status := model.StatusDone
	var report string

	if err != nil {
		status = model.StatusFailed
		report = fmt.Sprintf("工作流执行失败: %v", err)
		log.WithError(err).WithField("workflow", route.WorkflowName).Warn("diagnose via workflow failed")
	} else if result != nil {
		report = normalizeReportText(extractDiagnoseReport(result.Outputs))
	}

	if report == "" {
		status = model.StatusFailed
		report = "工作流未返回诊断结果"
	}

	updateWorkflowDiagnoseRecord(rec.ID, status, report, now)

	go SendNotification(
		"诊断完成: "+rec.TargetIP,
		fmt.Sprintf("状态: %s\n工作流: %s", status, route.WorkflowName),
		"info",
	)

	publishWorkflowDiagnoseEvent(rec, route.WorkflowName, status, now)

	if status == model.StatusDone {
		go triggerKnowledgeEnrich(rec.ID)
	}
}

// extractDiagnoseReport 从工作流输出中提取诊断报告
func updateWorkflowDiagnoseRecord(id string, status model.DiagnoseStatus, report string, now time.Time) {
	store.UpdateRecord(id, func(r *model.DiagnoseRecord) {
		r.Status = status
		r.Report = report
		r.SummaryReport = report
		r.EndTime = &now
		r.Source = "workflow"
	})
}

func publishWorkflowDiagnoseEvent(rec *model.DiagnoseRecord, workflowName string, status model.DiagnoseStatus, now time.Time) {
	eventbus.Global().Publish(eventbus.Event{
		Type: "diagnosis_completed",
		Data: map[string]interface{}{
			"diagnosis_id": rec.ID, "target_ip": rec.TargetIP,
			"status": string(status), "workflow": workflowName,
		},
		Timestamp: now,
	})
}

func extractDiagnoseReport(outputs map[string]any) string {
	for _, key := range []string{"diagnosis", "report", "analysis"} {
		if v, ok := outputs[key]; ok {
			switch val := v.(type) {
			case string:
				return val
			default:
				b, _ := json.Marshal(val)
				return string(b)
			}
		}
	}
	b, _ := json.MarshalIndent(outputs, "", "  ")
	return string(b)
}

// triggerKnowledgeEnrich 诊断完成后自动触发知识沉淀工作流
func triggerKnowledgeEnrich(diagnosisID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := workflow.RunWorkflow(ctx, "knowledge_enrich", buildKnowledgeEnrichInputs(diagnosisID))
	if err != nil {
		log.WithError(err).WithField("diagnosis_id", diagnosisID).Warn("knowledge enrich failed")
	} else {
		log.WithField("diagnosis_id", diagnosisID).Info("knowledge enrich completed")
	}
}
