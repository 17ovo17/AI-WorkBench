package handler

import (
	"net/http"
	"sort"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// CompareDiagnoses GET /api/v1/diagnose/compare?ip=&limit=2
// 返回同一主机最近 N 次诊断的对比信息。
func CompareDiagnoses(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip required"})
		return
	}
	limit := 2
	all := store.ListRecords()
	matched := make([]*model.DiagnoseRecord, 0)
	for _, r := range all {
		if r.TargetIP == ip && r.Status == model.StatusDone {
			matched = append(matched, r)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreateTime.After(matched[j].CreateTime)
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	if len(matched) < 2 {
		c.JSON(http.StatusOK, gin.H{
			"ip":         ip,
			"records":    matched,
			"diff":       gin.H{"reason": "需要至少 2 次诊断才能对比"},
			"comparable": false,
		})
		return
	}

	diff := buildDiagnosisDiff(matched[0], matched[1])
	c.JSON(http.StatusOK, gin.H{
		"ip":         ip,
		"current":    matched[0],
		"previous":   matched[1],
		"diff":       diff,
		"comparable": true,
	})
}

// buildDiagnosisDiff 构造两次诊断的对比摘要。
func buildDiagnosisDiff(curr, prev *model.DiagnoseRecord) gin.H {
	timeGap := curr.CreateTime.Sub(prev.CreateTime).String()
	out := gin.H{
		"time_gap":       timeGap,
		"current_at":     curr.CreateTime.Format(time.RFC3339),
		"previous_at":    prev.CreateTime.Format(time.RFC3339),
		"alert_changed":  curr.AlertTitle != prev.AlertTitle,
		"source_changed": curr.Source != prev.Source,
	}
	if curr.AlertTitle != "" && prev.AlertTitle != "" {
		out["current_alert"] = curr.AlertTitle
		out["previous_alert"] = prev.AlertTitle
	}
	return out
}
