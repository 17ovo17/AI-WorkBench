package handler

import (
	"fmt"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
)

func runAIOpsTopologyAnswer(question, businessName string, hosts []string, steps []model.ReasoningStep) (string, []model.ReasoningStep, []model.DataSourceUsage, []model.SuggestedAction, model.TopologyHighlight) {
	if business, ok := matchBusinessByNameOrHosts(businessName, hosts); ok {
		graph := buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: business.Name, Hosts: business.Hosts, Endpoints: business.Endpoints}, "heuristic_fallback", "AIOps topology mode reused deterministic Topo-Architect")
		steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "topology_generate", Input: business.Name, Output: graph.Summary, Status: "completed", Timestamp: time.Now(), Inference: "Generated Topo-Architect JSON and prepared topology highlight."})
		nodes := allAITopologyNodeIDs(graph.Nodes)
		answer := buildTopologyDetailedAnswer(business.Name, graph)
		url := "/topology?highlight=" + strings.Join(nodes, ",")
		return answer, steps, []model.DataSourceUsage{{Source: "topology", Queries: 1, Status: "used"}}, []model.SuggestedAction{{ID: "open-topology", Type: "link", Label: "Open topology", URL: url, Params: map[string]any{"url": url}}}, model.TopologyHighlight{HighlightNodes: nodes, Nodes: aiTopologyNodeSummaries(graph.Nodes)}
	}
	if len(hosts) > 0 {
		endpoints := make([]model.TopologyEndpoint, 0, len(hosts))
		for _, host := range hosts {
			endpoints = append(endpoints, model.TopologyEndpoint{IP: host, Port: 8080, ServiceName: "app-service", Protocol: "tcp"})
		}
		graph := buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: firstNonEmptyAIOps([]string{businessName, "ad-hoc-scope"}), Hosts: hosts, Endpoints: endpoints}, "heuristic_fallback", "AIOps topology mode generated an ad-hoc graph from selected IPs")
		nodes := make([]string, 0, len(graph.Nodes))
		for _, node := range graph.Nodes {
			nodes = append(nodes, node.ID)
		}
		steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "topology_generate", Input: hosts, Output: graph.Summary, Status: "completed", Timestamp: time.Now(), Inference: "Generated an ad-hoc Topo-Architect graph from the provided IP list."})
		answer := buildTopologyDetailedAnswer(firstNonEmptyAIOps([]string{businessName, "ad-hoc"}), graph)
		url := "/topology?highlight=" + strings.Join(nodes, ",")
		return answer, steps, []model.DataSourceUsage{{Source: "topology", Queries: 1, Status: "heuristic_fallback"}}, []model.SuggestedAction{{ID: "open-topology", Type: "link", Label: "Open topology", URL: url, Params: map[string]any{"url": url}}}, model.TopologyHighlight{HighlightNodes: nodes, Nodes: aiTopologyNodeSummaries(graph.Nodes)}
	}
	return "## Topology Scope Required\n\nPlease provide a business name, IP list, or service endpoints before generating topology.", steps, []model.DataSourceUsage{{Source: "topology", Queries: 0, Status: "need_scope"}}, nil, model.TopologyHighlight{}
}

func buildTopologyDetailedAnswer(businessName string, graph model.AITopologyGraph) string {
	lines := []string{
		fmt.Sprintf("## %s 拓扑分析", businessName),
		"",
		fmt.Sprintf("| 项目 | 值 |"),
		fmt.Sprintf("|------|------|"),
		fmt.Sprintf("| 节点数 | %d |", len(graph.Nodes)),
		fmt.Sprintf("| 连线数 | %d |", len(graph.Links)),
		fmt.Sprintf("| 风险数 | %d |", len(graph.Risks)),
	}
	if len(graph.Summary.CriticalPath) > 0 {
		lines = append(lines, fmt.Sprintf("| 关键路径 | %s |", strings.Join(graph.Summary.CriticalPath, " → ")))
	}
	layers := map[string][]model.AITopologyNode{}
	for _, node := range graph.Nodes {
		layers[node.Layer] = append(layers[node.Layer], node)
	}
	layerOrder := []string{"gateway", "app", "cache", "mq", "db", "infra", "monitor"}
	layerNames := map[string]string{"gateway": "入口层", "app": "应用层", "cache": "中间件层-缓存", "mq": "中间件层-消息队列", "db": "数据库层", "infra": "基础设施层", "monitor": "观测层"}
	lines = append(lines, "", "### 分层节点详情", "")
	for _, layer := range layerOrder {
		nodes, ok := layers[layer]
		if !ok || len(nodes) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("#### %s（%d 节点）", layerNames[layer], len(nodes)), "", "| 节点 | IP | 健康分 | 状态 | 服务 |", "|------|------|------|------|------|")
		for _, n := range nodes {
			svcNames := []string{}
			for _, s := range n.Services {
				svcNames = append(svcNames, fmt.Sprintf("%s:%d", s.Name, s.Port))
			}
			lines = append(lines, fmt.Sprintf("| %s | %s | %d | %s | %s |", n.Hostname, n.IP, n.Health.Score, n.Health.Status, strings.Join(svcNames, ", ")))
		}
		lines = append(lines, "")
	}
	if len(graph.Risks) > 0 {
		lines = append(lines, "### 风险分析", "", "| 级别 | 风险 | 描述 | 影响节点 |", "|------|------|------|------|")
		for _, risk := range graph.Risks {
			lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s |", risk.Severity, risk.Title, risk.Description, strings.Join(risk.Nodes, ", ")))
		}
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func runAIOpsReportAnswer(question string, attachments []model.AIOpsAttachment, hosts []string, steps []model.ReasoningStep) (string, []model.ReasoningStep, []model.DataSourceUsage, []model.SuggestedAction, model.TopologyHighlight) {
	facts := []string{}
	for _, attachment := range attachments {
		facts = append(facts, fmt.Sprintf("%s:%s", attachment.Type, compactAIOpsText(attachment.Content, 180)))
	}
	steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "user_report_extract", Input: question, Output: facts, Status: "completed", Timestamp: time.Now(), Inference: "\u7528\u6237\u4e0a\u62a5\u4f5c\u4e3a\u7ebf\u7d22\uff0c\u5fc5\u987b\u518d\u7528 Prometheus \u6216 Catpaw \u9a8c\u8bc1\u3002"})
	answer, steps, ds, actions, topo := runAIOpsDiagnosticAnswer(question, hosts, steps)
	answer = "## \u4e0a\u62a5\u786e\u8ba4\n\n**\u60a8\u63cf\u8ff0\u7684\u95ee\u9898**\uff1a" + compactAIOpsText(question, 160) + "\n\n**\u5904\u7406\u539f\u5219**\uff1a\u7528\u6237\u4e0a\u62a5\u662f\u7ebf\u7d22\uff0c\u4e0d\u76f4\u63a5\u4f5c\u4e3a\u7ed3\u8bba\uff1b\u4ee5\u4e0b\u4e3a\u5e73\u53f0\u9a8c\u8bc1\u7ed3\u679c\u3002\n\n" + answer
	return answer, steps, ds, actions, topo
}

func detectAIOpsMode(text string, attachments []model.AIOpsAttachment) string {
	q := strings.ToLower(text)
	switch {
	case strings.Contains(q, "\u5de1\u68c0") || strings.Contains(q, "\u5168\u9762\u68c0\u67e5") || strings.Contains(q, "\u5065\u5eb7\u5ea6") || strings.Contains(q, "inspection"):
		return "inspection"
	case strings.Contains(q, "\u62d3\u6251") || strings.Contains(q, "\u67b6\u6784\u56fe") || strings.Contains(q, "\u94fe\u8def") || strings.Contains(q, "\u5f71\u54cd\u54ea\u4e9b") || strings.Contains(q, "topology"):
		return "topology"
	case len(attachments) > 0 || strings.Contains(q, "\u4e0a\u62a5") || strings.Contains(q, "\u62a5\u9519") || strings.Contains(q, "error") || strings.Contains(q, "exception"):
		return "report"
	default:
		return "diagnostic"
	}
}

func normalizeAIOpsMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "diagnostic", "inspection", "report", "topology":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}

func selectDiagnosisChain(text string) string {
	q := strings.ToLower(text)
	switch {
	case strings.Contains(q, "gc") || strings.Contains(q, "jvm") || strings.Contains(q, "\u5806\u5185\u5b58") || strings.Contains(q, "\u8001\u5e74\u4ee3") || strings.Contains(q, "full gc") || strings.Contains(q, "young gc"):
		return "jvm_gc"
	case strings.Contains(q, "redis") || strings.Contains(q, "\u6162") || strings.Contains(q, "\u8d85\u65f6") || strings.Contains(q, "timeout") || strings.Contains(q, "p99") || strings.Contains(q, "502") || strings.Contains(q, "503"):
		return "service_slow"
	case strings.Contains(q, "\u5185\u5b58") || strings.Contains(q, "memory") || strings.Contains(q, "oom"):
		return "memory_high"
	case strings.Contains(q, "\u78c1\u76d8") || strings.Contains(q, "io") || strings.Contains(q, "iowait") || strings.Contains(q, "\u7a7a\u95f4"):
		return "disk_io_bottleneck"
	case strings.Contains(q, "\u7f51\u7edc") || strings.Contains(q, "\u4e22\u5305") || strings.Contains(q, "\u91cd\u4f20") || strings.Contains(q, "\u8fde\u63a5"):
		return "network_issue"
	case strings.Contains(q, "cpu") || strings.Contains(q, "\u8d1f\u8f7d") || strings.Contains(q, "\u98d9") || strings.Contains(q, "\u9ad8"):
		return "cpu_high"
	default:
		return "service_slow"
	}
}

func extractAIOpsHosts(text string) []string {
	matches := ipRe.FindAllString(text, -1)
	seen := map[string]bool{}
	out := []string{}
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			out = append(out, match)
		}
	}
	return out
}

func detectBusinessName(text string) string {
	for _, business := range store.ListTopologyBusinesses() {
		if businessNameFuzzyMatch(text, business.Name) {
			return business.Name
		}
	}
	return ""
}
