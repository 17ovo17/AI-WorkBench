package handler

import (
	"net/http"
	"time"

	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

func ListAuditEvents(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListAuditEvents(500))
}

func auditEvent(c *gin.Context, action, target, risk, decision, detail, batchID string) {
	createdAt := time.Now()
	store.AddAuditEvent(store.AuditEvent{
		ID:          time.Now().Format("20060102150405.000000000"),
		Action:      action,
		Target:      target,
		Risk:        risk,
		Decision:    decision,
		Detail:      detail,
		Operator:    currentOperator(c),
		Timestamp:   createdAt.Format(time.RFC3339),
		Description: auditDescription(action, target, decision, detail),
		TestBatchID: batchID,
		ClientIP:    c.ClientIP(),
		CreatedAt:   createdAt,
	})
}

func currentOperator(c *gin.Context) string {
	if value, ok := c.Get("username"); ok {
		if username, ok := value.(string); ok && username != "" {
			return username
		}
	}
	if token := extractToken(c); token != "" {
		if value, ok := tokenStore.Load(token); ok {
			entry := value.(tokenEntry)
			if time.Now().Before(entry.expiresAt) && entry.username != "" {
				return entry.username
			}
		}
	}
	if c.GetHeader("X-Admin-Token") != "" {
		return "admin-token"
	}
	return "anonymous"
}

func auditDescription(action, target, decision, detail string) string {
	description := action + " 操作对象 " + target + "，结果 " + decision
	if detail != "" {
		description += "，说明：" + detail
	}
	return description
}
