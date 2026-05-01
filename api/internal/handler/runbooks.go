package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type runbookListResponse struct {
	Items []model.Runbook `json:"items"`
	Total int             `json:"total"`
	Page  int             `json:"page"`
	Limit int             `json:"limit"`
}

// ListRunbooks GET /api/v1/knowledge/runbooks
func ListRunbooks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	category := c.Query("category")
	items, total := store.ListRunbooks(category, page, limit)
	c.JSON(http.StatusOK, runbookListResponse{Items: items, Total: total, Page: page, Limit: limit})
}

// GetRunbook GET /api/v1/knowledge/runbooks/:id
func GetRunbook(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	item, ok := store.GetRunbook(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// CreateRunbook POST /api/v1/knowledge/runbooks
func CreateRunbook(c *gin.Context) {
	var item model.Runbook
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if item.Title == "" || item.Category == "" || item.Steps == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title, category and steps required"})
		return
	}
	if item.ID == "" {
		item.ID = store.NewID()
	}
	item.CreatedAt = time.Now()
	store.SaveRunbook(&item)
	auditEvent(c, "knowledge.runbook.create", item.ID, "low", "ok",
		"category="+item.Category, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, item)
}

// UpdateRunbook PUT /api/v1/knowledge/runbooks/:id
func UpdateRunbook(c *gin.Context) {
	id := c.Param("id")
	existing, ok := store.GetRunbook(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	var input model.Runbook
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.ID = existing.ID
	input.CreatedAt = existing.CreatedAt
	store.UpdateRunbook(&input)
	auditEvent(c, "knowledge.runbook.update", id, "low", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, input)
}

// DeleteRunbook DELETE /api/v1/knowledge/runbooks/:id
func DeleteRunbook(c *gin.Context) {
	id := c.Param("id")
	if _, ok := store.GetRunbook(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	store.DeleteRunbook(id)
	auditEvent(c, "knowledge.runbook.delete", id, "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// renderSteps 替换步骤中的模板变量 ${variable_name}。
func renderSteps(steps string, variables map[string]string) string {
	for k, v := range variables {
		steps = strings.ReplaceAll(steps, "${"+k+"}", v)
	}
	return steps
}

type executeRunbookRequest struct {
	TargetIP  string            `json:"target_ip"`
	Variables map[string]string `json:"variables"`
	Executor  string            `json:"executor"`
}

// ExecuteRunbook POST /api/v1/knowledge/runbooks/:id/execute
func ExecuteRunbook(c *gin.Context) {
	id := c.Param("id")
	rb, ok := store.GetRunbook(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}

	var req executeRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	varsJSON, _ := json.Marshal(req.Variables)
	renderedSteps := renderSteps(rb.Steps, req.Variables)

	exec := &model.RunbookExecution{
		ID:        store.NewID(),
		RunbookID: id,
		TargetIP:  req.TargetIP,
		Executor:  req.Executor,
		Status:    "running",
		Variables: string(varsJSON),
		StartedAt: time.Now(),
	}
	store.SaveRunbookExecution(exec)

	// 异步触发 runbook_execute 工作流
	go runRunbookWorkflow(exec.ID, id, req.TargetIP, renderedSteps)

	auditEvent(c, "knowledge.runbook.execute", id, "medium", "ok",
		"exec_id="+exec.ID+",target_ip="+req.TargetIP, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"execution_id": exec.ID, "status": "running"})
}

// runRunbookWorkflow 异步执行 runbook_execute 工作流并更新执行记录。
func runRunbookWorkflow(execID, runbookID, targetIP, steps string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	inputs := map[string]any{
		"runbook_id": runbookID,
		"target_ip":  targetIP,
		"steps":      steps,
	}
	result, err := workflow.RunWorkflow(ctx, "runbook_execute", inputs)

	now := time.Now()
	if err != nil {
		logrus.Warnf("runbook workflow failed: %v", err)
		store.UpdateRunbookExecution(execID, func(e *model.RunbookExecution) {
			e.Status = "failed"
			e.ErrorMessage = err.Error()
			e.FinishedAt = &now
		})
		store.IncrementRunbookExecCount(runbookID, false)
		return
	}

	success := result.Status == "completed" || result.Status == "success"
	status := "succeeded"
	if !success {
		status = "failed"
	}
	outputJSON, _ := json.Marshal(result.Outputs)
	store.UpdateRunbookExecution(execID, func(e *model.RunbookExecution) {
		e.Status = status
		e.Output = string(outputJSON)
		if result.Error != "" {
			e.ErrorMessage = result.Error
		}
		e.FinishedAt = &now
	})
	store.IncrementRunbookExecCount(runbookID, success)
}

type runbookHistoryResponse struct {
	Items []model.RunbookExecution `json:"items"`
	Total int                      `json:"total"`
	Page  int                      `json:"page"`
	Limit int                      `json:"limit"`
}

// ListRunbookHistory GET /api/v1/knowledge/runbooks/:id/history
func ListRunbookHistory(c *gin.Context) {
	id := c.Param("id")
	if _, ok := store.GetRunbook(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "runbook not found"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, total := store.ListRunbookExecutions(id, page, limit)
	c.JSON(http.StatusOK, runbookHistoryResponse{
		Items: items, Total: total, Page: page, Limit: limit,
	})
}
