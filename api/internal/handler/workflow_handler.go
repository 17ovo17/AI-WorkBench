package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow"
	"ai-workbench-api/internal/workflow/engine"

	"github.com/gin-gonic/gin"
)

// ---------- 列表 ----------

// ListWorkflows GET /api/v1/workflows
func ListWorkflows(c *gin.Context) {
	builtinNames := workflow.ListBuiltinWorkflowNames()
	items := make([]model.Workflow, 0, len(builtinNames)+8)

	for _, name := range builtinNames {
		graph, _, err := engine.LoadBuiltinWorkflow(name)
		if err != nil {
			continue
		}
		items = append(items, model.Workflow{
			ID:          "builtin:" + name,
			Name:        graph.Name,
			Description: graph.Description,
			Builtin:     true,
		})
	}

	for _, w := range store.ListWorkflows() {
		items = append(items, w)
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// ---------- 获取 ----------

// GetWorkflow GET /api/v1/workflows/:id
func GetWorkflow(c *gin.Context) {
	id := c.Param("id")
	if isBuiltinID(id) {
		name := builtinName(id)
		graph, _, err := engine.LoadBuiltinWorkflow(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "builtin workflow not found"})
			return
		}
		c.JSON(http.StatusOK, model.Workflow{
			ID:          id,
			Name:        graph.Name,
			Description: graph.Description,
			Builtin:     true,
		})
		return
	}
	if raw := c.Query("version"); raw != "" {
		version, err := strconv.Atoi(raw)
		if err != nil || version <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workflow version"})
			return
		}
		v, ok := store.GetWorkflowVersion(id, version)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow version not found"})
			return
		}
		c.JSON(http.StatusOK, v)
		return
	}
	w, ok := store.GetWorkflow(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	c.JSON(http.StatusOK, w)
}

// ---------- 创建 ----------

type createWorkflowRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	DSL         string `json:"dsl" binding:"required"`
}

// CreateWorkflow POST /api/v1/workflows
func CreateWorkflow(c *gin.Context) {
	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// validate DSL
	if _, _, err := engine.ParseDSL([]byte(req.DSL)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid DSL: " + err.Error()})
		return
	}
	now := time.Now()
	w := &model.Workflow{
		ID:          store.NewID(),
		Name:        req.Name,
		Description: req.Description,
		DSL:         req.DSL,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.SaveWorkflow(w)
	auditEvent(c, "workflow.create", w.ID, "low", "ok", w.Name, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, w)
}

// ---------- 更新 ----------

// UpdateWorkflow PUT /api/v1/workflows/:id
func UpdateWorkflow(c *gin.Context) {
	id := c.Param("id")
	if isBuiltinID(id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot modify builtin workflow"})
		return
	}
	existing, ok := store.GetWorkflow(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, _, err := engine.ParseDSL([]byte(req.DSL)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid DSL: " + err.Error()})
		return
	}
	if existing.Version <= 0 {
		existing.Version = 1
	}
	store.SaveWorkflowVersion(&model.WorkflowVersion{
		WorkflowID:  existing.ID,
		Version:     existing.Version,
		Name:        existing.Name,
		Description: existing.Description,
		DSL:         existing.DSL,
		CreatedAt:   existing.UpdatedAt,
	})
	existing.Name = req.Name
	existing.Description = req.Description
	existing.DSL = req.DSL
	existing.Version++
	existing.UpdatedAt = time.Now()
	store.SaveWorkflow(existing)
	auditEvent(c, "workflow.update", id, "low", "ok", req.Name, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, existing)
}

// ---------- 删除 ----------

// DeleteWorkflow DELETE /api/v1/workflows/:id
func DeleteWorkflow(c *gin.Context) {
	id := c.Param("id")
	if isBuiltinID(id) {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete builtin workflow"})
		return
	}
	if _, ok := store.GetWorkflow(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	store.DeleteWorkflow(id)
	auditEvent(c, "workflow.delete", id, "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- 执行 ----------

type runWorkflowRequest struct {
	Inputs map[string]any `json:"inputs"`
}

// RunWorkflowAPI POST /api/v1/workflows/:id/run
func RunWorkflowAPI(c *gin.Context) {
	id := c.Param("id")
	name := resolveWorkflowName(id)
	var req runWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取当前工作流版本号
	wfVersion := 1
	if !isBuiltinID(id) {
		if w, ok := store.GetWorkflow(id); ok {
			wfVersion = w.Version
		}
	}

	runID := store.NewID()
	inputJSON, _ := json.Marshal(req.Inputs)
	store.SaveWorkflowRun(&model.WorkflowRun{
		ID: runID, WorkflowID: id, WorkflowVersion: wfVersion,
		Status: "running", Inputs: string(inputJSON), CreatedAt: time.Now(),
	})

	result, err := runWorkflowByVersion(c, id, name, wfVersion, req.Inputs)
	if err != nil {
		store.SaveWorkflowRun(&model.WorkflowRun{
			ID: runID, WorkflowID: id, WorkflowVersion: wfVersion,
			Status: "failed", Inputs: string(inputJSON),
			ErrorMessage: err.Error(), CreatedAt: time.Now(),
		})
		auditEvent(c, "workflow.run", id, "medium", "fail", err.Error(), c.GetHeader("X-Test-Batch-Id"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outputJSON, _ := json.Marshal(result.Outputs)
	store.SaveWorkflowRun(&model.WorkflowRun{
		ID: runID, WorkflowID: id, WorkflowVersion: wfVersion,
		Status: result.Status, Inputs: string(inputJSON),
		Outputs: string(outputJSON), ElapsedMs: result.ElapsedMs,
		CreatedAt: time.Now(),
	})
	auditEvent(c, "workflow.run", id, "low", "ok",
		fmt.Sprintf("elapsed_ms=%d", result.ElapsedMs), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, result)
}

func runWorkflowByVersion(c *gin.Context, id, name string, version int, inputs map[string]any) (*engine.WorkflowResult, error) {
	if isBuiltinID(id) || version <= 0 {
		return workflow.RunWorkflow(c.Request.Context(), name, inputs)
	}
	return workflow.RunWorkflowVersion(c.Request.Context(), id, version, inputs)
}

// StreamWorkflowAPI POST /api/v1/workflows/:id/stream
func StreamWorkflowAPI(c *gin.Context) {
	id := c.Param("id")
	name := resolveWorkflowName(id)
	var req runWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events, err := workflow.RunWorkflowStreaming(c.Request.Context(), name, req.Inputs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, _ := c.Writer.(http.Flusher)
	for evt := range events {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// ---------- 执行历史 ----------

// ListWorkflowRuns GET /api/v1/workflows/:id/runs
func ListWorkflowRuns(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, total := store.ListWorkflowRuns(id, page, limit)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "page": page, "limit": limit})
}

// ---------- 辅助函数 ----------

func isBuiltinID(id string) bool {
	return len(id) > 8 && id[:8] == "builtin:"
}

func builtinName(id string) string {
	if isBuiltinID(id) {
		return id[8:]
	}
	return id
}

func resolveWorkflowName(id string) string {
	if isBuiltinID(id) {
		return builtinName(id)
	}
	// For custom workflows, the store ID is used as the name for loadWorkflowGraph
	return id
}
