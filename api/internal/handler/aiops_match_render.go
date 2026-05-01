package handler

import (
	"fmt"
	"strings"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
)

func renderPromQLTemplate(tpl string, hosts []string) string {
	hostPattern := strings.Join(hosts, "|")
	if hostPattern == "" {
		hostPattern = ".*"
	}
	return strings.ReplaceAll(tpl, "{{hosts}}", hostPattern)
}

func inferTemplateObservation(tpl promQLTemplate, output string) string {
	if strings.Contains(strings.ToLower(output), "missing") || strings.Contains(output, "????") || strings.Contains(output, "???") || strings.Contains(output, "????") || strings.TrimSpace(output) == "" {
		return "No valid metric series was returned. Check Categraf collection, labels, and monitoring blind spots."
	}
	if tpl.Threshold != nil {
		return fmt.Sprintf("Observed %s with thresholds: warning=%v critical=%v.", tpl.Name, tpl.Threshold["warning"], tpl.Threshold["critical"])
	}
	return "Metric data was collected for correlation analysis."
}

func inferAIOpsRootCause(chain string, observations []string, catpaw string) string {
	joined := strings.ToLower(strings.Join(observations, "\n"))
	if strings.Contains(joined, "????") || strings.Contains(joined, "???") || strings.Contains(joined, "????") || strings.TrimSpace(joined) == "" {
		return "Prometheus evidence is insufficient for a deterministic root cause. Verify Categraf metrics, ident/instance labels, and scrape status first."
	}
	switch chain {
	case "cpu_high":
		if strings.Contains(joined, "iowait") || strings.Contains(joined, "disk") {
			return "CPU anomaly may be related to IO wait or disk bottleneck. Verify iowait, load, and disk IO utilization."
		}
		return "CPU anomaly chain was triggered. Check hot processes, thread pools, GC, or compute-intensive jobs first."
	case "memory_high":
		return "Memory pressure chain was triggered. Focus on Swap, memory growth trend, and process RSS TOP."
	case "disk_io_bottleneck":
		if catpaw != "" {
			return "Disk IO anomaly should be cross-checked with Catpaw SMART/RAID data to separate hardware risk from write amplification."
		}
		return "Disk IO bottleneck chain was triggered. Add Catpaw SMART/RAID evidence for cross-validation."
	case "network_issue":
		return "Network issue chain was triggered. Focus on retransmission, packet drops, TIME_WAIT, and bandwidth utilization."
	default:
		return "Service latency chain was triggered. Narrow down by P99 latency, error rate, QPS, host resources, and downstream dependencies."
	}
}

func confidenceFromObservations(observations []string) string {
	valid := 0
	for _, item := range observations {
		if !strings.Contains(item, "????") && !strings.Contains(item, "???") && !strings.Contains(item, "????") {
			valid++
		}
	}
	if valid >= 3 {
		return "high"
	}
	if valid >= 1 {
		return "medium - needs verification"
	}
	return "low - more data required"
}

func renderDiagnosticMarkdown(question, chain string, hosts []string, observations []string, catpawText, rootCause string) string {
	return fmt.Sprintf("## Diagnosis Conclusion\n**Problem**: %s\n**Root cause**: %s\n**Confidence**: %s\n\n## Evidence\n- Prometheus: executed `%s` chain with %d templates.\n%s\n- Catpaw: %s\n- User report: %s\n\n## Impact Scope\n- Affected objects: %s\n- Business impact should be confirmed with topology paths and active alerts.\n- If multiple nodes or critical paths are involved, on-call escalation is recommended.\n\n## Recommended Actions\n1. **Immediate (0-30min)**: Re-run PromQL checks and confirm whether the anomaly is still active.\n2. **Short term (1-4h)**: Locate bottlenecks in hot processes, disk IO, network, or downstream dependencies.\n3. **Long term (1-7d)**: Improve monitoring labels, capacity baselines, alert thresholds, and topology dependency data.\n\n## Verification PromQL\n```promql\n%s\n```", compactAIOpsText(question, 120), rootCause, confidenceFromObservations(observations), chain, len(observations), markdownList(observations, 8), emptyAs(catpawText, "No Catpaw snapshot found; Catpaw is only used for cross-validation and should not be used alone for final conclusions."), compactAIOpsText(question, 120), strings.Join(hosts, ", "), firstPromQLFromObservation(observations))
}

func renderInspectionAnswer(inspection model.BusinessInspection) string {
	lines := []string{
		fmt.Sprintf("# 业务巡检报告：%s", inspection.BusinessName),
		"## 总体概览",
		fmt.Sprintf("- 健康评分：%d/100", inspection.Score),
		fmt.Sprintf("- 当前状态：%s", inspection.Status),
		fmt.Sprintf("- 数据源：%s", strings.Join(inspection.DataSources, ", ")),
		"",
		"## 关键发现",
	}
	for _, item := range limitStrings(inspection.TopFindings, 5) {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "## 处置建议")
	for _, item := range limitStrings(inspection.AIRecommendations, 5) {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func readonlyCommandForChain(chain string) string {
	switch chain {
	case "cpu_high":
		return "top -o %CPU && iostat -x 1 5"
	case "memory_high":
		return "free -m && ps aux --sort=-rss | head"
	case "disk_io_bottleneck":
		return "iostat -x 1 5 && df -h"
	case "network_issue":
		return "ss -s && netstat -s | egrep 'retrans|drop|error'"
	case "jvm_gc":
		return "jstat -gcutil <java_pid> 1000 5 && jmap -heap <java_pid>"
	default:
		return "curl -s -o /dev/null -w '%{http_code} %{time_total}\n' http://127.0.0.1/health"
	}
}

func matchBusinessByScope(scope model.AIOpsScope) (model.TopologyBusiness, bool) {
	return matchBusinessByNameOrHosts("", scope.Hosts)
}

func matchBusinessByNameOrHosts(name string, hosts []string) (model.TopologyBusiness, bool) {
	hostSet := map[string]bool{}
	for _, host := range hosts {
		hostSet[host] = true
	}
	for _, business := range store.ListTopologyBusinesses() {
		if businessNameFuzzyMatch(name, business.Name) {
			return business, true
		}
		for _, host := range business.Hosts {
			if hostSet[host] {
				if strings.TrimSpace(name) == "" {
					return scopeBusinessToHosts(business, hostSet), true
				}
				return business, true
			}
		}
	}
	return model.TopologyBusiness{}, false
}

func scopeBusinessToHosts(business model.TopologyBusiness, hostSet map[string]bool) model.TopologyBusiness {
	scoped := business
	scoped.Hosts = []string{}
	for _, host := range business.Hosts {
		if hostSet[host] {
			scoped.Hosts = append(scoped.Hosts, host)
		}
	}
	scoped.Endpoints = []model.TopologyEndpoint{}
	for _, endpoint := range business.Endpoints {
		if hostSet[endpoint.IP] {
			scoped.Endpoints = append(scoped.Endpoints, endpoint)
		}
	}
	return scoped
}

var businessMatchStopwords = []string{
	"业务链路", "巡检", "诊断", "检查", "排查", "分析", "拓扑", "业务", "链路",
	"系统", "服务", "请", "帮我", "帮忙", "看下", "看一下", "一下", "的",
}

func businessNameFuzzyMatch(input, businessName string) bool {
	query := normalizeBusinessPhrase(input)
	target := normalizeBusinessPhrase(businessName)
	if query == "" || target == "" {
		return false
	}
	if strings.Contains(target, query) || strings.Contains(compactPhrase(target), compactPhrase(query)) {
		return true
	}
	matched := 0
	for _, token := range strings.Fields(query) {
		if len([]rune(token)) >= 2 && strings.Contains(target, token) {
			matched++
		}
	}
	return matched > 0 && (matched == len(strings.Fields(query)) || matched >= 2)
}

func normalizeBusinessPhrase(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("-", " ", "_", " ", "：", " ", ":", " ", "，", " ", ",", " ", "。", " ", "/", " ")
	text = replacer.Replace(text)
	for _, stopword := range businessMatchStopwords {
		text = strings.ReplaceAll(text, strings.ToLower(stopword), " ")
	}
	return strings.Join(strings.Fields(text), " ")
}

func compactPhrase(value string) string {
	return strings.ReplaceAll(value, " ", "")
}

func topologyHighlightForHosts(hosts []string) model.TopologyHighlight {
	if business, ok := matchBusinessByNameOrHosts("", hosts); ok {
		return topologyHighlightForBusiness(business)
	}
	return model.TopologyHighlight{}
}

func topologyHighlightForBusiness(business model.TopologyBusiness) model.TopologyHighlight {
	graph := buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: business.Name, Hosts: business.Hosts, Endpoints: business.Endpoints}, "heuristic_fallback", "AIOps highlight")
	nodes := []string{}
	for _, node := range graph.Nodes {
		if node.Health.Status == "danger" || len(nodes) < 3 {
			nodes = append(nodes, node.ID)
		}
	}
	return model.TopologyHighlight{HighlightNodes: nodes, Nodes: aiTopologyNodeSummaries(graph.Nodes)}
}
