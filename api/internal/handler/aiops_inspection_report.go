package handler

import (
	"fmt"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
)

const inspectionTrendOffsetSec int64 = 3600

type inspectionMetricSpec struct {
	Name    string
	Metric  string
	Unit    string
	Range   string
	Warning float64
	Danger  float64
	Aliases []string
}

var inspectionCoreMetricSpecs = []inspectionMetricSpec{
	{Name: "CPU 使用率", Metric: "cpu_usage_active", Unit: "%", Range: "<80%", Warning: 80, Danger: 90},
	{Name: "内存使用率", Metric: "mem_used_percent", Unit: "%", Range: "<85%", Warning: 85, Danger: 95},
	{Name: "磁盘使用率", Metric: "disk_used_percent", Unit: "%", Range: "<90%", Warning: 90, Danger: 95},
	{Name: "系统负载(1m)", Metric: "system_load1", Unit: "", Range: "<4.0", Warning: 4, Danger: 8, Aliases: []string{"系统负载 1m", "系统负载(1分钟)"}},
	{Name: "TCP 连接数", Metric: "netstat_tcp_established", Unit: "", Range: "<5000", Warning: 5000, Danger: 8000, Aliases: []string{"TCP 活跃连接数"}},
	{Name: "网络错误率", Metric: "network_error_rate", Unit: "%", Range: "<1%", Warning: 1, Danger: 5},
	{Name: "进程总数", Metric: "processes", Unit: "", Range: "<2000", Warning: 2000, Danger: 4000},
}

type inspectionFinding struct {
	Host      string
	Metric    string
	Current   string
	Threshold string
	Risk      string
	Impact    string
	Status    string
}

func ensureBusinessInspectionMetricCoverage(business model.TopologyBusiness, metrics []model.BusinessMetricSample) []model.BusinessMetricSample {
	out := append([]model.BusinessMetricSample{}, metrics...)
	index := map[string]int{}
	for i := range out {
		out[i].Name = canonicalInspectionMetricName(out[i].Name)
		index[out[i].IP+"|"+out[i].Name] = i
	}
	for _, host := range inspectionUniqueStrings(business.Hosts) {
		target := discoverPromTarget(host)
		for _, spec := range inspectionCoreMetricSpecs {
			sample := buildInspectionMetricSample(host, spec, target)
			key := host + "|" + spec.Name
			if i, ok := index[key]; ok {
				out[i] = mergeInspectionMetric(out[i], sample, spec)
				continue
			}
			index[key] = len(out)
			out = append(out, sample)
		}
	}
	sortBusinessInspectionMetrics(out)
	return out
}

func buildInspectionMetricSample(host string, spec inspectionMetricSpec, target promTarget) model.BusinessMetricSample {
	selector := fmt.Sprintf(`%s="%s"`, target.LabelKey, target.LabelVal)
	query := inspectionMetricPromQL(spec, target)
	if target.LabelKey == "" || query == "" {
		return unknownInspectionMetric(host, spec, query, "未发现该主机的 Prometheus 指标或必要指标名")
	}
	value, raw, ok := inspectionPromValue(query, 0)
	if !ok {
		return unknownInspectionMetric(host, spec, query, "Prometheus 查询无数据")
	}
	if ok && spec.Unit == "%" && value > 100 {
		if fallbackQuery := mappedInspectionMetricPromQL(spec, target, selector); fallbackQuery != "" {
			if v2, r2, ok2 := inspectionPromValue(fallbackQuery, 0); ok2 && v2 <= 100 {
				query, value, raw = fallbackQuery, v2, r2
			}
		}
	}
	prev, _, hasPrev := inspectionPromValue(query, inspectionTrendOffsetSec)
	trend := inspectionTrend(value, prev, hasPrev)
	return model.BusinessMetricSample{IP: host, Name: spec.Name, Value: value, Unit: spec.Unit, Status: inspectionMetricStatus(value, spec), Source: "prometheus", Query: query, Detail: inspectionMetricDetail(spec, prev, hasPrev, trend, raw)}
}

func mergeInspectionMetric(existing, sampled model.BusinessMetricSample, spec inspectionMetricSpec) model.BusinessMetricSample {
	if sampled.Status != "unknown" {
		return sampled
	}
	existing.Name = spec.Name
	if existing.Unit == "" {
		existing.Unit = spec.Unit
	}
	if existing.Status != "unknown" && existing.Source != "" {
		existing.Status = inspectionMetricStatus(existing.Value, spec)
		existing.Detail = inspectionMetricDetail(spec, 0, false, "-", existing.Detail)
		return existing
	}
	if existing.Detail == "" {
		existing.Detail = sampled.Detail
	}
	return existing
}

func renderRichInspectionReport(inspection model.BusinessInspection) string {
	generatedAt := inspection.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now()
	}
	lines := []string{
		fmt.Sprintf("# 业务巡检报告 - %s", inspection.BusinessName),
		"", "## 总体评估",
		fmt.Sprintf("- 健康评分：%d/100", inspection.Score),
		fmt.Sprintf("- 状态：%s", inspection.Status),
		fmt.Sprintf("- 巡检时间：%s", generatedAt.Format("2006-01-02 15:04")),
		fmt.Sprintf("- 覆盖主机：%d 台", len(inspectionHosts(inspection))),
		"", "## 主机巡检明细",
	}
	for _, host := range inspectionHosts(inspection) {
		lines = append(lines, renderInspectionHostSection(host, inspection)...)
	}
	lines = append(lines, renderInspectionAbnormalSummary(inspection)...)
	lines = append(lines, renderInspectionDisposition(inspection)...)
	lines = append(lines, renderInspectionHistory(inspection)...)
	return strings.Join(lines, "\n")
}

func businessMetricRecommendation(metric model.BusinessMetricSample) string {
	spec, ok := inspectionMetricSpecByName(metric.Name)
	if !ok {
		return fmt.Sprintf("%s 的 %s 异常：当前 %.2f%s，请结合业务拓扑复核。", metric.IP, metric.Name, metric.Value, metric.Unit)
	}
	finding := inspectionFinding{Host: metric.IP, Metric: spec.Name, Current: formatInspectionMetricValue(metric, spec), Threshold: spec.Range, Status: metric.Status}
	return fmt.Sprintf("%s %s 超过阈值：当前 %s，正常范围 %s；建议执行 `%s`。", metric.IP, spec.Name, finding.Current, spec.Range, inspectionCommands(finding)[0])
}

func renderInspectionHostSection(host string, inspection model.BusinessInspection) []string {
	metrics := inspectionMetricsByHost(inspection.Metrics)[host]
	lines := []string{"", "### " + inspectionHostTitle(host, inspection.Processes), "", "| 指标 | 当前值 | 正常范围 | 状态 | 趋势 |", "|------|--------|----------|------|------|"}
	for _, spec := range inspectionCoreMetricSpecs[:6] {
		metric := findInspectionMetric(metrics, spec.Name)
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s | %s |", spec.Name, formatInspectionMetricValue(metric, spec), spec.Range, inspectionStatusLabel(metric.Status), inspectionTrendFromDetail(metric.Detail)))
	}
	lines = append(lines, "", "**进程/端口巡检**", "", "| 进程/服务 | 端口 | 层级 | 状态 | 说明 |", "|----------|------|------|------|------|")
	lines = append(lines, renderInspectionProcessRows(host, inspection.Processes)...)
	return lines
}

func renderInspectionProcessRows(host string, processes []model.BusinessProcess) []string {
	rows := []string{}
	for _, process := range processes {
		if process.IP != host {
			continue
		}
		rows = append(rows, fmt.Sprintf("| %s | %d | %s | %s | %s |", emptyAs(process.Name, "未登记"), process.Port, inspectionLayerName(process.Layer), inspectionStatusLabel(process.Status), emptyAs(process.Alert, process.Description)))
	}
	if len(rows) == 0 {
		return []string{"| 未登记 | - | - | 无数据 | 未登记业务进程或端口，请补齐拓扑端点 |"}
	}
	return rows
}

func renderInspectionAbnormalSummary(inspection model.BusinessInspection) []string {
	lines := []string{"", "## 异常汇总", "", "| 主机 | 异常指标 | 当前值 | 阈值 | 风险等级 | 影响 |", "|------|---------|--------|------|---------|------|"}
	findings := collectInspectionFindings(inspection)
	if len(findings) == 0 {
		return append(lines, "| - | 未发现异常指标 | - | - | info | 当前巡检未发现超过阈值的资源或进程异常 |")
	}
	for _, item := range findings {
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", item.Host, item.Metric, item.Current, item.Threshold, item.Risk, item.Impact))
	}
	return lines
}

func renderInspectionDisposition(inspection model.BusinessInspection) []string {
	lines := []string{"", "## 处置建议"}
	findings := collectInspectionFindings(inspection)
	if len(findings) == 0 {
		return append(lines, "", "> 当前无超过阈值指标，保持监控，24 小时后复查同一业务链路。")
	}
	lines = append(lines, "", "| 优先级 | 主机 | 异常指标 | 当前值 → 阈值 | 风险 | 处置命令 |", "|--------|------|---------|--------------|------|---------|")
	for _, item := range findings {
		priority := inspectionPriority(item.Status)
		cmds := inspectionCommands(item)
		cmdText := "`" + cmds[0] + "`"
		if len(cmds) > 1 {
			cmdText += fmt.Sprintf(" 等 %d 条", len(cmds))
		}
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s → %s | %s | %s |", priority, item.Host, item.Metric, item.Current, item.Threshold, item.Impact, cmdText))
	}
	return lines
}

func renderInspectionHistory(inspection model.BusinessInspection) []string {
	lines := []string{"", "## 历史对比（当前 vs 1 小时前）"}
	byHost := inspectionMetricsByHost(inspection.Metrics)
	specs := inspectionCoreMetricSpecs[:6]
	lines = append(lines, "")
	header := "| 主机"
	sep := "|------"
	for _, spec := range specs {
		header += " | " + spec.Name
		sep += " |------"
	}
	lines = append(lines, header+" |", sep+" |")
	var noHistoryHosts []string
	for _, host := range inspectionHosts(inspection) {
		metrics := byHost[host]
		hasAnyData := false
		cells := "| " + host
		for _, spec := range specs {
			metric := findInspectionMetric(metrics, spec.Name)
			trend := inspectionTrendFromDetail(metric.Detail)
			if metric.Status == "unknown" {
				cells += " | -"
			} else {
				val := formatInspectionNumber(metric.Value) + spec.Unit
				if trend == "-" || trend == "" {
					cells += " | " + val + " 首次"
				} else {
					cells += " | " + val + " " + trend
					hasAnyData = true
				}
			}
		}
		lines = append(lines, cells+" |")
		if !hasAnyData {
			noHistoryHosts = append(noHistoryHosts, host)
		}
	}
	if len(noHistoryHosts) > 0 {
		lines = append(lines, "", fmt.Sprintf("> %s 采集时间不足 1 小时，暂无历史对比数据。", strings.Join(noHistoryHosts, "、")))
	}
	return lines
}

func collectInspectionFindings(inspection model.BusinessInspection) []inspectionFinding {
	items := []inspectionFinding{}
	for _, metric := range inspection.Metrics {
		spec, ok := inspectionMetricSpecByName(metric.Name)
		if !ok || !inspectionMetricAbnormal(metric.Status) {
			continue
		}
		items = append(items, inspectionFinding{Host: metric.IP, Metric: spec.Name, Current: formatInspectionMetricValue(metric, spec), Threshold: spec.Range, Risk: inspectionRisk(metric.Status), Impact: inspectionImpact(spec.Name, metric.Status), Status: metric.Status})
	}
	for _, process := range inspection.Processes {
		if process.Status == "running" {
			continue
		}
		items = append(items, inspectionFinding{Host: process.IP, Metric: "进程/端口 " + process.Name, Current: process.Status, Threshold: "running", Risk: "warning", Impact: emptyAs(process.Alert, "业务进程可用性需要复核"), Status: "warning"})
	}
	sortInspectionFindings(items)
	return items
}

func appendInspectionReasoningSteps(steps []model.ReasoningStep, inspection model.BusinessInspection) []model.ReasoningStep {
	checks := []struct {
		Action, Name string
		Metrics      []string
	}{
		{"inspection_alive", "主机存活与采集状态", nil},
		{"inspection_cpu", "CPU 分项巡检", []string{"CPU 使用率"}},
		{"inspection_memory", "内存分项巡检", []string{"内存使用率"}},
		{"inspection_disk", "磁盘分项巡检", []string{"磁盘使用率"}},
		{"inspection_load", "负载分项巡检", []string{"系统负载(1m)"}},
		{"inspection_network", "网络分项巡检", []string{"TCP 连接数", "网络错误率"}},
		{"inspection_process", "进程分项巡检", []string{"进程总数"}},
	}
	for _, check := range checks {
		steps = append(steps, buildInspectionReasoningStep(len(steps)+1, check.Action, check.Name, check.Metrics, inspection))
	}
	return steps
}

func buildInspectionReasoningStep(step int, action, name string, metricNames []string, inspection model.BusinessInspection) model.ReasoningStep {
	output := inspectionSubcheckOutput(metricNames, inspection)
	if action == "inspection_alive" {
		output = map[string]any{"hosts": inspectionHosts(inspection), "processes": len(inspection.Processes), "alerts": len(inspection.Alerts), "status": inspection.Status}
	}
	return model.ReasoningStep{Step: step, Action: action, Output: output, Status: "completed", Timestamp: time.Now(), Inference: name + "完成，已纳入巡检报告。", Confidence: "high"}
}

func inspectionSubcheckOutput(metricNames []string, inspection model.BusinessInspection) map[string]any {
	matched, abnormal, missing := 0, 0, 0
	for _, metric := range inspection.Metrics {
		if !containsString(metricNames, metric.Name) {
			continue
		}
		matched++
		if metric.Status == "unknown" {
			missing++
		} else if inspectionMetricAbnormal(metric.Status) {
			abnormal++
		}
	}
	return map[string]any{"metrics": metricNames, "samples": matched, "abnormal": abnormal, "missing": missing, "host_count": len(inspectionHosts(inspection))}
}
