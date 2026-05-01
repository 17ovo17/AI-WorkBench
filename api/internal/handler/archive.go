package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

type archiveRequest struct {
	DiagnosisID          string `json:"diagnosis_id"`
	RootCauseCategory    string `json:"root_cause_category"`
	RootCauseDescription string `json:"root_cause_description"`
	TreatmentSteps       string `json:"treatment_steps"`
	Keywords             string `json:"keywords"`
}

// ArchiveDiagnosis POST /api/v1/diagnosis/archive
// 将诊断记录归档为知识库案例。
func ArchiveDiagnosis(c *gin.Context) {
	var req archiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DiagnosisID == "" || req.RootCauseCategory == "" || req.RootCauseDescription == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "diagnosis_id, root_cause_category and root_cause_description required"})
		return
	}

	diagRecord := findDiagnoseByID(req.DiagnosisID)
	snapshot := extractSnapshot(diagRecord)

	now := time.Now()
	caseItem := &model.DiagnosisCase{
		ID:                   store.NewID(),
		MetricSnapshot:       snapshot,
		RootCauseCategory:    req.RootCauseCategory,
		RootCauseDescription: req.RootCauseDescription,
		TreatmentSteps:       req.TreatmentSteps,
		Keywords:             req.Keywords,
		SourceDiagnosisID:    req.DiagnosisID,
		CreatedAt:            now,
		CreatedBy:            "archive",
	}
	store.SaveCase(caseItem)

	auditEvent(c, "diagnosis.archive", req.DiagnosisID, "low", "ok",
		"category="+req.RootCauseCategory, c.GetHeader("X-Test-Batch-Id"))

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"case": caseItem,
	})
}

// findDiagnoseByID 通过 ID 查找诊断记录，找不到返回 nil。
func findDiagnoseByID(id string) *model.DiagnoseRecord {
	for _, r := range store.ListRecords() {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// extractSnapshot 从诊断记录中提取指标快照（JSON），失败返回空对象。
func extractSnapshot(r *model.DiagnoseRecord) json.RawMessage {
	if r == nil || r.RawReport == "" {
		return json.RawMessage("{}")
	}
	if !json.Valid([]byte(r.RawReport)) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(r.RawReport)
}
