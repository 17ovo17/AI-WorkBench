package handler

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-workbench-api/internal/eventbus"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	alertWindow    = make(map[string][]time.Time)
	alertWindowMu  sync.Mutex
	stormThreshold = 20
	stormWindowSec = int64(60)
)

// shouldSuppressAlert 检查是否应该抑制告警（风暴检测）
func shouldSuppressAlert(ip string) bool {
	alertWindowMu.Lock()
	defer alertWindowMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(stormWindowSec) * time.Second)

	times := alertWindow[ip]
	valid := make([]time.Time, 0, len(times))
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	alertWindow[ip] = valid

	if len(valid) > stormThreshold {
		logrus.WithFields(logrus.Fields{"ip": ip, "count": len(valid)}).Warn("alert storm detected, suppressing auto-diagnosis")
		return true
	}
	return false
}

// CatpawAlert 接收 catpaw run 模式产生的告警（WebAPI 通知格式）
func CatpawAlert(c *gin.Context) {
	var body struct {
		Title       string            `json:"title"`
		Severity    string            `json:"severity"`
		Status      string            `json:"status"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ip := body.Labels["from_hostip"]
	if ip == "" {
		ip = body.Labels["instance"]
	}

	now := time.Now()
	alert := &model.AlertRecord{
		ID:          fmt.Sprintf("%d", now.UnixNano()),
		Title:       body.Title,
		TargetIP:    ip,
		Severity:    body.Severity,
		Status:      body.Status,
		Labels:      body.Labels,
		Annotations: body.Annotations,
		Source:      "catpaw",
		CreateTime:  now,
	}
	if alert.Status == "" {
		alert.Status = "firing"
	}
	isNew := store.AddAlert(alert)
	go pushToPushgateway(alert)
	go SendNotification("告警: "+alert.Title, fmt.Sprintf("主机: %s\n严重度: %s", ip, alert.Severity), alert.Severity)

	eventbus.Global().Publish(eventbus.Event{
		Type: "alert_created",
		Data: map[string]interface{}{
			"alert_id": alert.ID, "title": alert.Title,
			"target_ip": ip, "severity": alert.Severity,
		},
		Timestamp: now,
	})

	// 告警触发时自动发起 AI 诊断
	if isNew && alert.Status == "firing" && ip != "" {
		rec := &model.DiagnoseRecord{
			ID:         fmt.Sprintf("alert_%s", alert.ID),
			TargetIP:   ip,
			Trigger:    "alert",
			Source:     "catpaw",
			Status:     model.StatusPending,
			AlertTitle: alert.Title,
			CreateTime: now,
		}
		store.AddRecord(rec)
		store.UpdateAlertAction(alert.ID, model.AlertAction{Action: "link-diagnose", Actor: "system", Reason: "auto diagnose triggered by catpaw alert", CreatedAt: now}, func(a *model.AlertRecord) {
			a.DiagnoseRecordID = rec.ID
		})
		prompt := fmt.Sprintf("catpaw 告警「%s」，主机 %s。请立即诊断根因并给出处置建议。", alert.Title, ip)
		if shouldSuppressAlert(ip) {
			logrus.WithField("ip", ip).Info("alert suppressed by storm detection")
		} else {
			go RunDiagnoseViaWorkflow(rec, prompt)
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ListAlerts(c *gin.Context) {
	items := store.ListAlerts()
	out := []*model.AlertRecord{}
	for _, item := range items {
		if matchAlertQuery(item, c) {
			out = append(out, item)
		}
	}
	c.JSON(http.StatusOK, out)
}

func matchAlertQuery(a *model.AlertRecord, c *gin.Context) bool {
	checks := map[string]string{
		"status":        a.Status,
		"severity":      a.Severity,
		"business_id":   a.BusinessID,
		"target_ip":     a.TargetIP,
		"source":        a.Source,
		"test_batch_id": a.TestBatchID,
	}
	for key, value := range checks {
		query := strings.TrimSpace(c.Query(key))
		if query != "" && query != value {
			return false
		}
	}
	keyword := strings.ToLower(strings.TrimSpace(c.Query("q")))
	if keyword != "" {
		joined := strings.ToLower(strings.Join([]string{a.Title, a.TargetIP, a.Severity, a.Status, a.Source, a.Assignee, a.AckBy}, " "))
		if !strings.Contains(joined, keyword) {
			return false
		}
	}
	return true
}

func ResolveAlert(c *gin.Context) {
	if !store.ResolveAlert(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	auditEvent(c, "alert.resolve", c.Param("id"), "L2", "allow", "alert marked resolved", c.Query("test_batch_id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteAlert(c *gin.Context) {
	if !store.DeleteAlert(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	auditEvent(c, "alert.delete", c.Param("id"), "L3", "allow", "alert soft deleted by user confirmation", c.Query("test_batch_id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func AlertAction(c *gin.Context) {
	var req struct {
		Actor            string `json:"actor"`
		Assignee         string `json:"assignee"`
		Reason           string `json:"reason"`
		BusinessID       string `json:"business_id"`
		DiagnoseRecordID string `json:"diagnose_record_id"`
		MutedMinutes     int    `json:"muted_minutes"`
	}
	_ = c.ShouldBindJSON(&req)
	action := c.Param("action")
	if !isAllowedAlertAction(action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported alert action"})
		return
	}
	now := time.Now()
	ok := store.UpdateAlertAction(c.Param("id"), model.AlertAction{Action: action, Actor: req.Actor, Reason: req.Reason, CreatedAt: now}, func(a *model.AlertRecord) {
		switch action {
		case "acknowledge":
			a.Status = "acknowledged"
			a.AckBy = firstNonEmpty(req.Actor, "值班人员")
		case "assign":
			a.Status = "assigned"
			a.Assignee = firstNonEmpty(req.Assignee, req.Actor, "待分派处理人")
		case "mute":
			a.Status = "muted"
			minutes := req.MutedMinutes
			if minutes <= 0 {
				minutes = 60
			}
			until := now.Add(time.Duration(minutes) * time.Minute)
			a.MutedUntil = &until
		case "archive":
			a.Status = "archived"
		case "diagnosing":
			a.Status = "diagnosing"
			a.DiagnoseRecordID = req.DiagnoseRecordID
		case "mitigate":
			a.Status = "mitigated"
			a.Resolution = req.Reason
		case "link-business":
			a.LinkedBusinessID = req.BusinessID
			a.BusinessID = firstNonEmpty(req.BusinessID, a.BusinessID)
		case "link-diagnose":
			a.DiagnoseRecordID = req.DiagnoseRecordID
		}
	})
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "alert not found"})
		return
	}
	auditEvent(c, "alert."+action, c.Param("id"), "L2", "allow", req.Reason, c.Query("test_batch_id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func isAllowedAlertAction(action string) bool {
	switch action {
	case "acknowledge", "assign", "mute", "archive", "diagnosing", "mitigate", "link-business", "link-diagnose":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// AlertWebhook 兼容 Alertmanager / 夜莺 webhook 格式
func AlertWebhook(c *gin.Context) {
	var body struct {
		Alerts []struct {
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			Status      string            `json:"status"`
			StartsAt    time.Time         `json:"startsAt"`
		} `json:"alerts"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(body.Alerts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alerts is required"})
		return
	}

	for _, a := range body.Alerts {
		if len(a.Labels) == 0 {
			continue
		}
		ip := a.Labels["instance"]
		if ip == "" {
			ip = a.Labels["host"]
		}
		for i, ch := range ip {
			if ch == ':' {
				ip = ip[:i]
				break
			}
		}
		now := time.Now()
		status := a.Status
		if status == "" {
			status = "firing"
		}
		alert := &model.AlertRecord{
			ID:          fmt.Sprintf("%d", now.UnixNano()),
			Title:       a.Labels["alertname"],
			TargetIP:    ip,
			Severity:    a.Labels["severity"],
			Status:      status,
			Labels:      a.Labels,
			Annotations: a.Annotations,
			Source:      "alertmanager",
			CreateTime:  now,
		}
		isNew := store.AddAlert(alert)

		if isNew && ip != "" && status == "firing" {
			rec := &model.DiagnoseRecord{
				ID:         fmt.Sprintf("alert_%s", alert.ID),
				TargetIP:   ip,
				Trigger:    "alert",
				Source:     "prometheus",
				Status:     model.StatusPending,
				AlertTitle: alert.Title,
				CreateTime: now,
			}
			store.AddRecord(rec)
			store.UpdateAlertAction(alert.ID, model.AlertAction{Action: "link-diagnose", Actor: "system", Reason: "auto diagnose triggered by alertmanager", CreatedAt: now}, func(a *model.AlertRecord) {
				a.DiagnoseRecordID = rec.ID
			})
			if shouldSuppressAlert(ip) {
				logrus.WithField("ip", ip).Info("alert suppressed by storm detection")
			} else {
				go RunDiagnoseViaWorkflow(rec, fmt.Sprintf("告警「%s」，主机 %s，请诊断根因。", alert.Title, ip))
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
