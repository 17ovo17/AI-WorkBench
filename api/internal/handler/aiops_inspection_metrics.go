package handler

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"ai-workbench-api/internal/model"

	"github.com/spf13/viper"
)

func inspectionMetricPromQL(spec inspectionMetricSpec, target promTarget) string {
	selector := fmt.Sprintf(`%s="%s"`, target.LabelKey, target.LabelVal)
	if query := directInspectionMetricPromQL(spec, target, selector); query != "" {
		return query
	}
	if query := mappedInspectionMetricPromQL(spec, target, selector); query != "" {
		return query
	}
	return ""
}

func directInspectionMetricPromQL(spec inspectionMetricSpec, target promTarget, selector string) string {
	switch spec.Metric {
	case "netstat_tcp_established":
		return promQLForFirstMetric(target.Metrics, selector, "netstat_tcp_established", "netstat_tcp_inuse", "netstat_sockets_used")
	case "network_error_rate":
		return networkErrorRatePromQL(target.Metrics, selector)
	default:
		if !containsString(target.Metrics, spec.Metric) {
			return ""
		}
		return fmt.Sprintf(`max(%s{%s})`, spec.Metric, selector)
	}
}

func networkErrorRatePromQL(metrics []string, selector string) string {
	if hasPromMetrics(metrics, "net_err_in", "net_err_out", "net_packets_recv", "net_packets_sent") {
		return fmt.Sprintf(`100 * (sum(rate(net_err_in{%s}[5m])) + sum(rate(net_err_out{%s}[5m]))) / clamp_min(sum(rate(net_packets_recv{%s}[5m])) + sum(rate(net_packets_sent{%s}[5m])), 1)`, selector, selector, selector, selector)
	}
	if hasPromMetrics(metrics, "node_network_receive_errs_total", "node_network_transmit_errs_total", "node_network_receive_packets_total", "node_network_transmit_packets_total") {
		nodeSelector := networkDeviceSelector(selector)
		return fmt.Sprintf(`100 * (sum(rate(node_network_receive_errs_total{%s}[5m])) + sum(rate(node_network_transmit_errs_total{%s}[5m]))) / clamp_min(sum(rate(node_network_receive_packets_total{%s}[5m])) + sum(rate(node_network_transmit_packets_total{%s}[5m])), 1)`, nodeSelector, nodeSelector, nodeSelector, nodeSelector)
	}
	if hasPromMetrics(metrics, "node_network_receive_errs_total", "node_network_transmit_errs_total") {
		nodeSelector := networkDeviceSelector(selector)
		return fmt.Sprintf(`sum(rate(node_network_receive_errs_total{%s}[5m])) + sum(rate(node_network_transmit_errs_total{%s}[5m]))`, nodeSelector, nodeSelector)
	}
	return ""
}

func hasPromMetrics(metrics []string, names ...string) bool {
	for _, name := range names {
		if !containsString(metrics, name) {
			return false
		}
	}
	return true
}

func networkDeviceSelector(selector string) string {
	deviceFilter := `device!~"lo|docker.*|veth.*|br-.*|cni.*|flannel.*"`
	if strings.TrimSpace(selector) == "" {
		return deviceFilter
	}
	return selector + "," + deviceFilter
}

func promQLForFirstMetric(metrics []string, selector string, names ...string) string {
	for _, name := range names {
		if containsString(metrics, name) {
			return fmt.Sprintf(`sum(%s{%s})`, name, selector)
		}
	}
	return ""
}

func inspectionPromValue(query string, offsetSec int64) (float64, string, bool) {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" || query == "" {
		return 0, "", false
	}
	text := queryPromInstant(base, query, offsetSec)
	if text == "" && offsetSec == 0 {
		text, _ = queryProm(query)
	}
	if text == "" || isNoPromData(text) {
		return 0, text, false
	}
	return parseFloatValue(text), text, true
}

func unknownInspectionMetric(host string, spec inspectionMetricSpec, query, reason string) model.BusinessMetricSample {
	return model.BusinessMetricSample{IP: host, Name: spec.Name, Unit: spec.Unit, Status: "unknown", Source: "prometheus", Query: query, Detail: reason + "；正常范围 " + spec.Range + "；趋势=-"}
}

func inspectionMetricDetail(spec inspectionMetricSpec, prev float64, hasPrev bool, trend, raw string) string {
	prevText := "无历史样本"
	if hasPrev {
		prevText = formatInspectionNumber(prev) + spec.Unit
	}
	return fmt.Sprintf("正常范围 %s；1小时前 %s；趋势=%s；原始=%s", spec.Range, prevText, trend, compactAIOpsText(raw, 160))
}

func inspectionMetricStatus(value float64, spec inspectionMetricSpec) string {
	if value >= spec.Danger {
		return "critical"
	}
	if value >= spec.Warning {
		return "warning"
	}
	return "healthy"
}

func inspectionTrend(current, previous float64, ok bool) string {
	if !ok {
		return "-"
	}
	delta := current - previous
	if math.Abs(delta) < math.Max(1, math.Abs(previous)*0.05) {
		return "稳定"
	}
	if delta > 0 {
		return "持续升高"
	}
	return "下降"
}

func formatInspectionMetricValue(metric model.BusinessMetricSample, spec inspectionMetricSpec) string {
	if metric.Status == "unknown" {
		return "无数据"
	}
	return formatInspectionNumber(metric.Value) + spec.Unit
}

func formatInspectionNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "无数据"
	}
	if math.Abs(value-math.Round(value)) < 0.05 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func inspectionHosts(inspection model.BusinessInspection) []string {
	seen := map[string]bool{}
	for _, metric := range inspection.Metrics {
		seen[metric.IP] = metric.IP != ""
	}
	for _, process := range inspection.Processes {
		seen[process.IP] = process.IP != ""
	}
	out := []string{}
	for host, ok := range seen {
		if ok {
			out = append(out, host)
		}
	}
	sort.Strings(out)
	return out
}

func inspectionMetricsByHost(metrics []model.BusinessMetricSample) map[string][]model.BusinessMetricSample {
	out := map[string][]model.BusinessMetricSample{}
	for _, metric := range metrics {
		metric.Name = canonicalInspectionMetricName(metric.Name)
		out[metric.IP] = append(out[metric.IP], metric)
	}
	return out
}

func findInspectionMetric(metrics []model.BusinessMetricSample, name string) model.BusinessMetricSample {
	for _, metric := range metrics {
		if canonicalInspectionMetricName(metric.Name) == name {
			return metric
		}
	}
	return model.BusinessMetricSample{Name: name, Status: "unknown"}
}

func canonicalInspectionMetricName(name string) string {
	name = strings.TrimSpace(name)
	for _, spec := range inspectionCoreMetricSpecs {
		if name == spec.Name || containsString(spec.Aliases, name) {
			return spec.Name
		}
	}
	return name
}

func inspectionMetricSpecByName(name string) (inspectionMetricSpec, bool) {
	name = canonicalInspectionMetricName(name)
	for _, spec := range inspectionCoreMetricSpecs {
		if spec.Name == name {
			return spec, true
		}
	}
	return inspectionMetricSpec{}, false
}

func inspectionTrendFromDetail(detail string) string {
	for _, trend := range []string{"持续升高", "下降", "稳定"} {
		if strings.Contains(detail, "趋势="+trend) {
			return trend
		}
	}
	return "-"
}

func inspectionStatusLabel(status string) string {
	switch status {
	case "healthy", "running":
		return "正常"
	case "warning":
		return "警告"
	case "critical", "danger":
		return "危险"
	default:
		return "无数据"
	}
}

func inspectionMetricAbnormal(status string) bool {
	return status == "warning" || status == "critical" || status == "unknown"
}

func inspectionRisk(status string) string {
	if status == "critical" {
		return "critical"
	}
	if status == "unknown" {
		return "warning"
	}
	return "warning"
}

func inspectionPriority(status string) string {
	if status == "critical" {
		return "P0"
	}
	if status == "warning" {
		return "P1"
	}
	return "P2"
}

func riskActionText(status string) string {
	if status == "critical" {
		return "立即处理"
	}
	if status == "warning" {
		return "尽快复核"
	}
	return "补齐采集"
}

func inspectionImpact(metricName, status string) string {
	if status == "unknown" {
		return "监控盲区，无法确认真实健康状态"
	}
	switch {
	case strings.Contains(metricName, "CPU"), strings.Contains(metricName, "负载"):
		return "应用响应变慢，线程调度和请求排队风险升高"
	case strings.Contains(metricName, "内存"):
		return "OOM 风险，可能触发进程重启或缓存抖动"
	case strings.Contains(metricName, "磁盘"):
		return "写入失败、日志落盘失败或数据库不可用风险"
	case strings.Contains(metricName, "TCP"), strings.Contains(metricName, "网络"):
		return "连接耗尽、丢包或重传导致链路超时"
	default:
		return "需要结合业务拓扑确认影响范围"
	}
}

func inspectionCommands(item inspectionFinding) []string {
	name := item.Metric
	switch {
	case strings.Contains(name, "CPU"):
		return []string{"top -bn1 | head -20", "ps aux --sort=-%cpu | head -10"}
	case strings.Contains(name, "内存"):
		return []string{"free -h && ps aux --sort=-%mem | head -10"}
	case strings.Contains(name, "磁盘"):
		return []string{"df -h && du -xhd1 / 2>/dev/null | sort -h | tail -20"}
	case strings.Contains(name, "负载"):
		return []string{"uptime && ps -eo pid,ppid,stat,pcpu,pmem,comm --sort=-pcpu | head -15"}
	case strings.Contains(name, "TCP"):
		return []string{"ss -s && ss -ant state established | wc -l"}
	case strings.Contains(name, "网络"):
		return []string{"ip -s link && netstat -s | egrep 'error|drop|retrans'"}
	default:
		return []string{"systemctl status categraf --no-pager || ps aux | grep categraf"}
	}
}

func inspectionHostTitle(host string, processes []model.BusinessProcess) string {
	for _, process := range processes {
		if process.IP == host && process.Name != "" {
			return fmt.Sprintf("%s（%s %s）", host, inspectionLayerName(process.Layer), process.Name)
		}
	}
	return host
}

func sortBusinessInspectionMetrics(items []model.BusinessMetricSample) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].IP != items[j].IP {
			return items[i].IP < items[j].IP
		}
		return inspectionMetricOrder(items[i].Name) < inspectionMetricOrder(items[j].Name)
	})
}

func inspectionMetricOrder(name string) int {
	name = canonicalInspectionMetricName(name)
	for i, spec := range inspectionCoreMetricSpecs {
		if spec.Name == name {
			return i
		}
	}
	return 100
}

func sortInspectionFindings(items []inspectionFinding) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return inspectionPriority(items[i].Status) < inspectionPriority(items[j].Status)
		}
		if items[i].Host != items[j].Host {
			return items[i].Host < items[j].Host
		}
		return items[i].Metric < items[j].Metric
	})
}

func inspectionUniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func isNoPromData(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "no data") || strings.Contains(text, "无数据")
}

func buildAIOpsLLMFallbackContent(targetIP, question, dataSource string) string {
	lines := []string{
		"## AI 问诊降级响应",
		"",
		fmt.Sprintf("上游模型在 %d 秒内未返回结果，已切换为确定性运维排查流程。", int(chatUpstreamTimeout/time.Second)),
		"",
		fmt.Sprintf("- 目标主机：%s", emptyAs(targetIP, "未知")),
		fmt.Sprintf("- 用户问题：%s", compactAIOpsText(question, 160)),
		fmt.Sprintf("- 已用数据源：%s", emptyAs(dataSource, "prometheus")),
	}
	target := discoverPromTarget(targetIP)
	if target.LabelKey != "" {
		selector := fmt.Sprintf(`%s="%s"`, target.LabelKey, target.LabelVal)
		lines = append(lines, "", "### 主机实时指标（Prometheus）", "", "| 指标 | 当前值 | 正常范围 | 状态 |", "|------|--------|----------|------|")
		for _, spec := range inspectionCoreMetricSpecs[:6] {
			query := inspectionMetricPromQL(spec, target)
			if query == "" {
				lines = append(lines, fmt.Sprintf("| %s | 无数据 | %s | - |", spec.Name, spec.Range))
				continue
			}
			value, _, ok := inspectionPromValue(query, 0)
			if !ok {
				lines = append(lines, fmt.Sprintf("| %s | 无数据 | %s | - |", spec.Name, spec.Range))
				continue
			}
			if spec.Unit == "%" && value > 100 {
				if fq := mappedInspectionMetricPromQL(spec, target, selector); fq != "" {
					if v2, _, ok2 := inspectionPromValue(fq, 0); ok2 && v2 <= 100 {
						value = v2
					}
				}
			}
			status := inspectionMetricStatus(value, spec)
			statusLabel := inspectionStatusLabel(status)
			lines = append(lines, fmt.Sprintf("| %s | %s%s | %s | %s |", spec.Name, formatInspectionNumber(value), spec.Unit, spec.Range, statusLabel))
		}
	}
	lines = append(lines, "", "### 排查命令",
		fmt.Sprintf("1. `ssh %s \"uptime && top -bn1 | head -20\"`", targetIP),
		fmt.Sprintf("2. `ssh %s \"free -h && df -h && ss -s\"`", targetIP),
		fmt.Sprintf("3. `ssh %s \"journalctl -xe --no-pager | tail -80\"`", targetIP),
		"",
		"> P0/P1 场景请立即升级值班负责人。",
	)
	return strings.Join(lines, "\n")
}
