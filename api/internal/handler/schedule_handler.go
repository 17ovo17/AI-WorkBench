package handler

import (
	"fmt"
	"net/http"
	"time"

	"ai-workbench-api/internal/scheduler"

	"github.com/gin-gonic/gin"
)

// ListSchedulesHandler GET /api/v1/schedules
func ListSchedulesHandler(c *gin.Context) {
	items := scheduler.ListSchedules()
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// CreateScheduleHandler POST /api/v1/schedules
func CreateScheduleHandler(c *gin.Context) {
	var req struct {
		WorkflowName string         `json:"workflow_name" binding:"required"`
		CronExpr     string         `json:"cron_expr" binding:"required"`
		Inputs       map[string]any `json:"inputs"`
		Enabled      bool           `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 校验 cron 表达式
	if _, err := scheduler.ParseCron(req.CronExpr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cron: " + err.Error()})
		return
	}
	s := scheduler.WorkflowSchedule{
		ID:           fmt.Sprintf("sched_%d", time.Now().UnixNano()),
		WorkflowName: req.WorkflowName,
		CronExpr:     req.CronExpr,
		Inputs:       req.Inputs,
		Enabled:      req.Enabled,
	}
	scheduler.AddSchedule(s)
	auditEvent(c, "schedule.create", s.ID, "low", "ok", s.WorkflowName, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, s)
}

// DeleteScheduleHandler DELETE /api/v1/schedules/:id
func DeleteScheduleHandler(c *gin.Context) {
	id := c.Param("id")
	if !scheduler.RemoveSchedule(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found"})
		return
	}
	auditEvent(c, "schedule.delete", id, "low", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
