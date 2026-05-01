package handler

import (
	"fmt"
	"sort"
	"strings"

	"ai-workbench-api/internal/model"
)

func ensureInspectionReportSections(report string, inspection model.BusinessInspection) string {
	text := localizeInspectionReportText(strings.TrimSpace(report), inspection.BusinessName)
	if text == "" {
		text = renderInspectionAnswer(inspection)
	}
	if inspectionReportComplete(text) {
		return text
	}
	return text + "\n\n" + buildInspectionGuardrailSection(inspection)
}

func inspectionReportComplete(text string) bool {
	checks := []bool{
		strings.Contains(text, "巡检"),
		strings.Contains(text, "健康评分"),
		strings.Contains(text, "异常层级") || strings.Contains(text, "分层"),
		strings.Contains(text, "根因"),
		strings.Contains(text, "处置建议"),
		strings.Contains(text, "数据源"),
	}
	for _, ok := range checks {
		if !ok {
			return false
		}
	}
	return true
}

func buildInspectionGuardrailSection(inspection model.BusinessInspection) string {
	sections := []string{
		"## 巡检保障摘要",
		fmt.Sprintf("- **巡检模式**：业务巡检，已强制纳入 Prometheus 指标、Catpaw 进程、告警和拓扑证据。"),
		fmt.Sprintf("- **健康评分**：%d/100，状态：%s。", inspection.Score, inspection.Status),
		fmt.Sprintf("- **数据源**：%s。", strings.Join(inspection.DataSources, ", ")),
		"",
		"## 异常层级",
	}
	sections = append(sections, inspectionLayerLines(inspection)...)
	sections = append(sections, "", "## 根因分析", "- "+inspectionRootCause(inspection), "", "## 处置建议")
	for _, item := range limitStrings(inspection.AIRecommendations, 5) {
		sections = append(sections, "- "+item)
	}
	return strings.Join(sections, "\n")
}

func localizeInspectionReportText(report, businessName string) string {
	text := strings.TrimSpace(report)
	if text == "" {
		return text
	}
	replacements := []string{
		"# Inspection Report:", "# 业务巡检报告：",
		"# Inspection Report", "# 业务巡检报告",
		"## Overview", "## 总体概览",
		"## Key Findings", "## 关键发现",
		"## Suggested Actions", "## 处置建议",
		"## Root Cause", "## 根因分析",
		"## Root cause", "## 根因分析",
		"## Evidence", "## 证据链",
		"## Impact Scope", "## 影响范围",
		"## Recommended Actions", "## 处置建议",
		"- Health score:", "- 健康评分：",
		"- Current status:", "- 当前状态：",
		"- Data sources:", "- 数据源：",
		"Health score:", "健康评分：",
		"Current status:", "当前状态：",
		"Data sources:", "数据源：",
	}
	text = strings.NewReplacer(replacements...).Replace(text)
	if strings.HasPrefix(text, "# 业务巡检报告\n") && strings.TrimSpace(businessName) != "" {
		text = strings.Replace(text, "# 业务巡检报告", "# 业务巡检报告："+businessName, 1)
	}
	return text
}

func inspectionLayerLines(inspection model.BusinessInspection) []string {
	layers := map[string]int{}
	for _, process := range inspection.Processes {
		if process.Status != "running" {
			layers[inspectionLayerName(process.Layer)]++
		}
	}
	for _, metric := range inspection.Metrics {
		if metric.Status == "warning" || metric.Status == "critical" || metric.Status == "unknown" {
			layers[inspectionLayerName(metricLayerByName(metric.Name))]++
		}
	}
	if len(layers) == 0 {
		return []string{"- 未发现明确异常层级；入口层、应用层、中间件层、数据库层继续按健康巡检基线观测。"}
	}
	names := make([]string, 0, len(layers))
	for name := range layers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("- %s：%d 项巡检证据需要复核。", name, layers[name]))
	}
	return out
}

func inspectionRootCause(inspection model.BusinessInspection) string {
	for _, finding := range inspection.TopFindings {
		if strings.TrimSpace(finding) != "" {
			return "最可能根因来自巡检证据：" + finding
		}
	}
	return "当前未发现阻断性根因；如用户仍感知异常，优先补齐 Prometheus 指标标签并沿业务拓扑逐层复核。"
}

func inspectionLayerName(layer string) string {
	switch strings.ToLower(strings.TrimSpace(layer)) {
	case "frontend", "gateway", "entry":
		return "入口层"
	case "app", "application", "worker":
		return "应用层"
	case "middleware", "cache", "mq":
		return "中间件层"
	case "database", "db":
		return "数据库层"
	default:
		return "基础资源层"
	}
}

func metricLayerByName(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "redis") || strings.Contains(name, "缓存"):
		return "middleware"
	case strings.Contains(lower, "mysql") || strings.Contains(lower, "oracle") || strings.Contains(name, "数据库"):
		return "database"
	case strings.Contains(lower, "nginx") || strings.Contains(name, "入口"):
		return "frontend"
	default:
		return "resource"
	}
}

func allAITopologyNodeIDs(nodes []model.AITopologyNode) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node.ID)
	}
	return out
}

func aiTopologyNodeSummaries(nodes []model.AITopologyNode) []any {
	out := make([]any, 0, len(nodes))
	for _, node := range nodes {
		service := ""
		port := 0
		if len(node.Services) > 0 {
			service = node.Services[0].Name
			port = node.Services[0].Port
		}
		out = append(out, map[string]any{
			"id":           node.ID,
			"ip":           node.IP,
			"service_name": service,
			"port":         port,
			"layer":        node.Layer,
			"layer_name":   aiTopologyLayerName(node.Layer),
			"status":       node.Health.Status,
		})
	}
	return out
}

func aiTopologyLayerName(layer string) string {
	switch layer {
	case "gateway":
		return "入口层"
	case "app":
		return "应用层"
	case "cache", "mq":
		return "中间件层"
	case "db":
		return "数据库层"
	case "infra":
		return "基础设施层"
	case "monitor":
		return "观测层"
	default:
		return "业务组件"
	}
}

func sortAITopologyNodes(nodes []model.AITopologyNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		li, lj := aiTopologyLayerOrder(nodes[i].Layer), aiTopologyLayerOrder(nodes[j].Layer)
		if li != lj {
			return li < lj
		}
		if nodes[i].IP != nodes[j].IP {
			return nodes[i].IP < nodes[j].IP
		}
		return nodes[i].ID < nodes[j].ID
	})
}

func aiTopologyLayerOrder(layer string) int {
	switch layer {
	case "gateway":
		return 0
	case "app":
		return 1
	case "cache", "mq":
		return 2
	case "db":
		return 3
	case "infra":
		return 4
	case "monitor":
		return 5
	default:
		return 9
	}
}
