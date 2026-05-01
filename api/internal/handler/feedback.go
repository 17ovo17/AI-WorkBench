package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var validRatings = map[string]bool{
	"accurate":   true,
	"inaccurate": true,
	"partial":    true,
}

type feedbackListResponse struct {
	Items []model.DiagnosisFeedback `json:"items"`
	Total int                       `json:"total"`
	Page  int                       `json:"page"`
	Limit int                       `json:"limit"`
}

// SubmitFeedback POST /api/v1/diagnosis/feedback
func SubmitFeedback(c *gin.Context) {
	var input model.DiagnosisFeedback
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.DiagnosisID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "diagnosis_id required"})
		return
	}
	if !validRatings[input.Rating] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be accurate/partial/inaccurate"})
		return
	}
	if input.ID == "" {
		input.ID = store.NewID()
	}
	if input.User == "" {
		input.User = "anonymous"
	}
	input.CreatedAt = time.Now()
	store.AddFeedback(&input)

	auditEvent(c, "diagnosis.feedback.submit", input.DiagnosisID, "low", "ok",
		"rating="+input.Rating, c.GetHeader("X-Test-Batch-Id"))

	// 当反馈为 accurate 时，异步触发知识沉淀工作流
	if input.Rating == "accurate" && input.DiagnosisID != "" {
		go func() {
			_, err := workflow.RunWorkflow(context.Background(), "knowledge_enrich", buildKnowledgeEnrichInputs(input.DiagnosisID))
			if err != nil {
				logrus.Warnf("knowledge_enrich workflow failed for diagnosis %s: %v", input.DiagnosisID, err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true,
		"id": input.ID,
	})
}

// ListFeedbacksByDiagnosisHandler GET /api/v1/diagnosis/feedback
func ListFeedbacksByDiagnosisHandler(c *gin.Context) {
	diagID := c.Query("diagnosis_id")
	if diagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "diagnosis_id required"})
		return
	}
	items := store.ListFeedbacksByDiagnosis(diagID)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// ListAllFeedbacks GET /api/v1/diagnosis/feedback/all
func ListAllFeedbacks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, total := store.ListAllFeedbacks(page, limit)
	c.JSON(http.StatusOK, feedbackListResponse{Items: items, Total: total, Page: page, Limit: limit})
}
