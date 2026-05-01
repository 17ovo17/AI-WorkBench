package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RouteResult 工作流路由结果
type RouteResult struct {
	WorkflowName string
	Inputs       map[string]interface{}
}

type routeAlternative struct {
	WorkflowName string  `json:"workflow_name"`
	Confidence   float64 `json:"confidence"`
	Reason       string  `json:"reason"`
}

// RouteToWorkflow 根据用户输入自动路由到最匹配的工作流
func RouteToWorkflow(question, hostname, alertID, severity string) RouteResult {
	result := RouteResult{
		Inputs: map[string]interface{}{
			"hostname":      hostname,
			"time_range":    "1h",
			"user_question": question,
		},
	}

	if alertID != "" {
		result.WorkflowName = "smart_diagnosis"
		result.Inputs["trigger"] = "alert"
		result.Inputs["alert_id"] = alertID
		result.Inputs["severity"] = severity
		logrus.WithField("workflow", result.WorkflowName).Info("route: alert triggered")
		return result
	}

	q := strings.ToLower(question)
	result.WorkflowName = matchWorkflow(q, &result)

	logrus.WithFields(logrus.Fields{
		"workflow": result.WorkflowName,
		"question": question[:min(len(question), 50)],
	}).Info("route: workflow selected")
	return result
}

// matchWorkflow 按优先级匹配关键词，返回工作流名称并填充路由参数
func matchWorkflow(q string, r *RouteResult) string {
	if wf := matchDomain(q, r); wf != "" {
		return wf
	}
	if wf := matchHealth(q, r); wf != "" {
		return wf
	}
	if wf := matchMetrics(q, r); wf != "" {
		return wf
	}
	if wf := matchSecurity(q, r); wf != "" {
		return wf
	}
	if wf := matchIncident(q, r); wf != "" {
		return wf
	}
	if wf := matchNetwork(q, r); wf != "" {
		return wf
	}
	r.Inputs["trigger"] = "manual"
	return "smart_diagnosis"
}

func matchDomain(q string, r *RouteResult) string {
	switch {
	case containsAny(q, "jvm", "gc", "堆内存", "full gc", "老年代", "young gen"):
		r.Inputs["domain"] = "jvm"
	case containsAny(q, "容器", "pod", "k8s", "kubernetes", "docker", "oom kill", "crashloop"):
		r.Inputs["domain"] = "container"
	case containsAny(q, "慢查询", "slow query", "sql", "死锁", "deadlock", "锁等待", "innodb"):
		r.Inputs["domain"] = "db"
	case containsAny(q, "日志", "log", "error log", "异常日志"):
		r.Inputs["domain"] = "log"
	default:
		return ""
	}
	return "domain_diagnosis"
}

func matchHealth(q string, r *RouteResult) string {
	switch {
	case containsAny(q, "业务巡检", "业务健康", "全链路巡检"):
		r.Inputs["scope"] = "business"
	case containsAny(q, "巡检", "健康检查", "health check", "全面检查"):
		r.Inputs["scope"] = "single"
	case containsAny(q, "中间件", "kafka", "rabbitmq", "elasticsearch", "mq"):
		r.Inputs["scope"] = "middleware"
	case containsAny(q, "存储", "nfs", "iops"):
		r.Inputs["scope"] = "storage"
	case containsAny(q, "依赖", "redis", "mysql连接", "数据库连接"):
		r.Inputs["scope"] = "dependency"
	default:
		return ""
	}
	return "health_inspection"
}

func matchMetrics(q string, r *RouteResult) string {
	switch {
	case containsAny(q, "slo", "error budget", "可用性", "达标"):
		r.Inputs["analysis_type"] = "slo"
	case containsAny(q, "容量", "预测", "forecast", "趋势"):
		r.Inputs["analysis_type"] = "forecast"
	case containsAny(q, "流量", "qps", "突增", "spike", "traffic"):
		r.Inputs["analysis_type"] = "traffic"
	default:
		return ""
	}
	return "metrics_insight"
}

func matchSecurity(q string, r *RouteResult) string {
	switch {
	case containsAny(q, "证书", "ssl", "tls", "https"):
		r.Inputs["audit_type"] = "ssl"
	case containsAny(q, "配置漂移", "config drift", "配置变更"):
		r.Inputs["audit_type"] = "config"
	case containsAny(q, "安全", "审计", "security", "风险"):
		r.Inputs["audit_type"] = "security"
	default:
		return ""
	}
	return "security_compliance"
}

func matchIncident(q string, r *RouteResult) string {
	switch {
	case containsAny(q, "复盘", "postmortem", "故障回顾", "根因分析"):
		r.Inputs["mode"] = "postmortem"
		r.Inputs["incident_summary"] = r.Inputs["user_question"]
	case containsAny(q, "时间线", "timeline", "故障经过"):
		r.Inputs["mode"] = "timeline"
	default:
		return ""
	}
	return "incident_review"
}

func matchNetwork(q string, r *RouteResult) string {
	if containsAny(q, "网络", "丢包", "延迟", "ping", "连通") {
		r.Inputs["target_ips"] = r.Inputs["hostname"]
		return "network_check"
	}
	return ""
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func RoutePreviewHandler(c *gin.Context) {
	var req struct {
		Question string `json:"question"`
		Hostname string `json:"hostname"`
		AlertID  string `json:"alert_id"`
		Severity string `json:"severity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	route := RouteToWorkflow(req.Question, req.Hostname, req.AlertID, req.Severity)
	confidence, reason := routeConfidence(route, req.AlertID)
	c.JSON(http.StatusOK, gin.H{
		"workflow_name":      route.WorkflowName,
		"inputs":             route.Inputs,
		"confidence":         confidence,
		"reason":             reason,
		"needs_confirmation": true,
		"confirm_hint":       "确认后调用对应工作流执行接口，并传入返回的 workflow_name 与 inputs",
		"alternatives":       buildRouteAlternatives(route.WorkflowName, confidence),
	})
}

func routeConfidence(route RouteResult, alertID string) (float64, string) {
	if alertID != "" {
		return 0.95, "alert_id 命中告警诊断路由"
	}
	switch route.WorkflowName {
	case "domain_diagnosis":
		return 0.86, "问题关键词命中领域诊断"
	case "health_inspection":
		return 0.84, "问题关键词命中健康巡检"
	case "metrics_insight", "security_compliance", "incident_review", "network_check":
		return 0.82, "问题关键词命中专项工作流"
	default:
		return 0.50, "未命中明确关键词，使用智能诊断兜底"
	}
}

func buildRouteAlternatives(selected string, selectedConfidence float64) []routeAlternative {
	all := []string{"smart_diagnosis", "domain_diagnosis", "health_inspection", "metrics_insight", "security_compliance", "incident_review", "network_check"}
	alts := make([]routeAlternative, 0, len(all)-1)
	for _, wf := range all {
		if wf == selected {
			continue
		}
		alts = append(alts, routeAlternative{WorkflowName: wf, Confidence: alternativeConfidence(wf, selectedConfidence), Reason: "可由用户手动确认切换"})
	}
	return alts
}

func alternativeConfidence(workflowName string, selectedConfidence float64) float64 {
	if workflowName == "smart_diagnosis" {
		return 0.45
	}
	if selectedConfidence <= 0.5 {
		return 0.40
	}
	return 0.30
}
