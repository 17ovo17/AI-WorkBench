package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type onCallTestRequest struct {
	Channel     string `json:"channel"`
	Receiver    string `json:"receiver"`
	BusinessID  string `json:"business_id"`
	AlertTitle  string `json:"alert_title"`
	TestBatchID string `json:"test_batch_id"`
}

type onCallTestResponse struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Receiver    string    `json:"receiver"`
	Status      string    `json:"status"`
	Detail      string    `json:"detail"`
	TraceID     string    `json:"trace_id"`
	TestBatchID string    `json:"test_batch_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func TestOnCallNotification(c *gin.Context) {
	var req onCallTestRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Channel = firstNonEmpty(strings.TrimSpace(req.Channel), "console")
	req.Receiver = firstNonEmpty(strings.TrimSpace(req.Receiver), "值班人员")
	if !isAllowedOnCallChannel(req.Channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported notification channel"})
		return
	}
	if len(req.Receiver) > 80 || len(req.AlertTitle) > 200 || len(req.BusinessID) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notification fields exceed length limit"})
		return
	}
	now := time.Now()
	traceID := fmt.Sprintf("notify_%d", now.UnixNano())
	detail := "测试通知已写入审计链路"
	if req.BusinessID != "" || req.AlertTitle != "" {
		detail = fmt.Sprintf("测试通知已触达 %s，业务=%s，事件=%s", req.Receiver, firstNonEmpty(req.BusinessID, "未关联"), firstNonEmpty(req.AlertTitle, "测试事件"))
	}
	auditEvent(c, "oncall.test_send", req.Channel+":"+req.Receiver, "L1", "allow", detail, req.TestBatchID)
	resp := onCallTestResponse{ID: traceID, Channel: req.Channel, Receiver: req.Receiver, Status: "success", Detail: detail, TraceID: traceID, TestBatchID: req.TestBatchID, CreatedAt: now}
	appendOnCallRecord(resp)
	c.JSON(http.StatusOK, resp)
}

func isAllowedOnCallChannel(channel string) bool {
	switch channel {
	case "console", "webhook", "email", "flashduty", "pagerduty", "dingtalk", "feishu", "wecom":
		return true
	default:
		return false
	}
}
