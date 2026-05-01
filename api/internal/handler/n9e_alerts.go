package handler

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type N9eAlert struct {
	ID              int64  `json:"id"`
	RuleName        string `json:"rule_name"`
	Severity        string `json:"severity"`
	TargetIdent     string `json:"target_ident"`
	TargetIP        string `json:"target_ip"`
	TriggerTime     int64  `json:"trigger_time"`
	TriggerTimeText string `json:"trigger_time_text"`
	TriggerValue    string `json:"trigger_value"`
	Tags            string `json:"tags"`
	Status          string `json:"status"`
	PromQL          string `json:"prom_ql"`
	RuleNote        string `json:"rule_note"`
	GroupName       string `json:"group_name"`
}

func n9eSeverity(level int) string {
	switch level {
	case 1:
		return "critical"
	case 2:
		return "warning"
	case 3:
		return "info"
	default:
		return "info"
	}
}

func extractIPFromIdent(ident string) string {
	parts := strings.Split(ident, "-")
	for _, p := range parts {
		if strings.Count(p, ".") == 3 {
			return p
		}
	}
	if idx := strings.LastIndex(ident, "-"); idx > 0 {
		candidate := ident[idx+1:]
		if strings.Count(candidate, ".") == 3 {
			return candidate
		}
	}
	return ident
}

func ListN9eAlerts(c *gin.Context) {
	dsn := viper.GetString("n9e.mysql_dsn")
	if dsn == "" {
		c.JSON(http.StatusOK, []N9eAlert{})
		return
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logrus.Warnf("n9e mysql: %v", err)
		c.JSON(http.StatusOK, []N9eAlert{})
		return
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, rule_name, severity, target_ident, trigger_time, trigger_value, tags, prom_ql, COALESCE(rule_note,''), COALESCE(group_name,'') FROM alert_cur_event ORDER BY trigger_time DESC LIMIT 200`)
	if err != nil {
		logrus.Warnf("n9e query: %v", err)
		c.JSON(http.StatusOK, []N9eAlert{})
		return
	}
	defer rows.Close()

	alerts := []N9eAlert{}
	for rows.Next() {
		var a N9eAlert
		var severity int
		if err := rows.Scan(&a.ID, &a.RuleName, &severity, &a.TargetIdent, &a.TriggerTime, &a.TriggerValue, &a.Tags, &a.PromQL, &a.RuleNote, &a.GroupName); err != nil {
			continue
		}
		a.Severity = n9eSeverity(severity)
		a.TargetIP = extractIPFromIdent(a.TargetIdent)
		a.Status = "firing"
		a.TriggerTimeText = time.Unix(a.TriggerTime, 0).Format("2006-01-02 15:04:05")
		alerts = append(alerts, a)
	}
	c.JSON(http.StatusOK, alerts)
}
