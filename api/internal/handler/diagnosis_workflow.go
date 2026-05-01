package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ai-workbench-api/internal/workflow"
	"ai-workbench-api/internal/workflow/engine"

	"github.com/gin-gonic/gin"
)

// startDiagnosisRequest models the workflow start payload.
type startDiagnosisRequest struct {
	Hostname     string `json:"hostname"`
	TimeRange    string `json:"time_range"`
	UserQuestion string `json:"user_question"`
	ResponseMode string `json:"response_mode"`
	User         string `json:"user"`
}

// StartDiagnosisWorkflow POST /api/v1/diagnosis/start
func StartDiagnosisWorkflow(c *gin.Context) {
	req := parseDiagnosisRequest(c)
	if req == nil {
		return
	}

	route := RouteToWorkflow(req.UserQuestion, req.Hostname, "", "")
	inputs := route.Inputs
	inputs["time_range"] = req.TimeRange
	inputs["user_question"] = req.UserQuestion

	if req.ResponseMode == "streaming" {
		streamDiagnosisWorkflow(c, route.WorkflowName, inputs, req)
		return
	}
	blockingDiagnosisWorkflow(c, route.WorkflowName, inputs, req)
}

func parseDiagnosisRequest(c *gin.Context) *startDiagnosisRequest {
	var req startDiagnosisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return nil
	}
	if strings.TrimSpace(req.Hostname) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostname required"})
		return nil
	}
	if req.TimeRange == "" {
		req.TimeRange = "1h"
	}
	if req.ResponseMode == "" {
		req.ResponseMode = "blocking"
	}
	if req.User == "" {
		req.User = "ai-workbench"
	}
	return &req
}

func blockingDiagnosisWorkflow(c *gin.Context, name string, inputs map[string]any, req *startDiagnosisRequest) {
	result, err := workflow.RunWorkflow(c.Request.Context(), name, inputs)
	if err != nil {
		auditEvent(c, "diagnosis.workflow.start", req.User, "medium", "fail",
			err.Error(), c.GetHeader("X-Test-Batch-Id"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	archiveWorkflowResult(req, result)
	auditEvent(c, "diagnosis.workflow.start", req.User, "low", "ok",
		fmt.Sprintf("elapsed_ms=%d", result.ElapsedMs), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, result)
}

func streamDiagnosisWorkflow(c *gin.Context, name string, inputs map[string]any, req *startDiagnosisRequest) {
	events, err := workflow.RunWorkflowStreaming(c.Request.Context(), name, inputs)
	if err != nil {
		auditEvent(c, "diagnosis.workflow.start", req.User, "medium", "fail",
			err.Error(), c.GetHeader("X-Test-Batch-Id"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, _ := c.Writer.(http.Flusher)

	var finishedOutputs map[string]any
	for evt := range events {
		if evt.Event == engine.EventWorkflowFinished {
			finishedOutputs = evt.Data
		}
		data, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	archiveWorkflowOutputs(req, finishedOutputs)
	auditEvent(c, "diagnosis.workflow.start", req.User, "low", "ok",
		"streaming", c.GetHeader("X-Test-Batch-Id"))
}
