package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/spf13/viper"
)

func runAIOpsInspectionAnswer(question, businessName string, hosts []string, steps []model.ReasoningStep) (string, []model.ReasoningStep, []model.DataSourceUsage, []model.SuggestedAction, model.TopologyHighlight) {
	if business, ok := matchBusinessByNameOrHosts(firstNonEmptyAIOps([]string{businessName, question}), hosts); ok {
		inspection := buildBusinessInspection(business)

		llmReport := strings.TrimSpace(inspection.AIAnalysis)
		if llmReport == "" {
			llmReport = renderRichInspectionReport(inspection)
		}
		llmReport = ensureInspectionReportSections(llmReport, inspection)
		inspection.AIAnalysis = llmReport

		steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "business_inspection", Input: business.Name, Output: map[string]any{"score": inspection.Score, "status": inspection.Status, "metrics": len(inspection.Metrics), "processes": len(inspection.Processes), "llm_report_length": len(llmReport)}, Status: "completed", Timestamp: time.Now(), Inference: "collected all host metrics, process status and topology risks, generated LLM report"})
		steps = appendInspectionReasoningSteps(steps, inspection)

		suggested := []model.SuggestedAction{{ID: "open-topology", Type: "link", Label: "打开业务拓扑", URL: "/topology", Params: map[string]any{"url": "/topology"}}}
		dataSources := []model.DataSourceUsage{{Source: "prometheus", Queries: len(inspection.Metrics), Status: "used"}, {Source: "catpaw", Queries: len(inspection.Processes), Status: "used"}, {Source: "ai_provider", Queries: 0, Status: "skipped"}}

		now := time.Now()
		firstHost := ""
		if len(business.Hosts) > 0 {
			firstHost = business.Hosts[0]
		}
		rec := &model.DiagnoseRecord{
			ID: store.NewID(), TargetIP: firstHost, Trigger: "aiops_chat", Source: "business_inspection",
			DataSource: "prometheus", Status: model.StatusDone, Report: llmReport,
			SummaryReport: llmReport, AlertTitle: business.Name + " 业务巡检", CreateTime: now, EndTime: &now,
		}
		store.AddRecord(rec)

		llmReport += "\n\n---\n> 💡 本次巡检报告已保存到「诊断记录」，点击左侧菜单可查看完整报告，阅读体验更佳。"

		suggested = append(suggested, model.SuggestedAction{ID: "view-diagnose", Type: "link", Label: "查看诊断记录", URL: "/diagnose", Params: map[string]any{"url": "/diagnose"}})

		return llmReport, steps, dataSources, suggested, topologyHighlightForBusiness(business)
	}
	answer := "# 系统巡检报告\n\n- 未匹配到具体业务，已进入全局巡检视角。\n- 建议先在业务拓扑中选择业务，或补充业务名/IP 范围。\n\n## 下一步\n- 选择业务后重新发送业务名+巡检。\n- 或输入具体 IP 执行诊断模式。"
	return answer, steps, []model.DataSourceUsage{{Source: "user", Queries: 1, Status: "need_scope"}}, nil, model.TopologyHighlight{}
}

func callLLMForInspection(businessName string, inspection model.BusinessInspection) string {
	apiKey := getAPIKey()
	if apiKey == "" {
		return renderInspectionAnswer(inspection)
	}

	baseURL := viper.GetString("ai.base_url")
	if baseURL == "" {
		providers := configuredAIProviders()
		for _, p := range providers {
			if p.Default && p.BaseURL != "" {
				baseURL = p.BaseURL
				break
			}
		}
	}
	if baseURL == "" {
		return renderInspectionAnswer(inspection)
	}

	mdl := resolveDefaultModel()

	var evidenceLines []string
	evidenceLines = append(evidenceLines, fmt.Sprintf("业务名称: %s", businessName))
	evidenceLines = append(evidenceLines, fmt.Sprintf("确定性评分: %d/100, 状态: %s", inspection.Score, inspection.Status))
	evidenceLines = append(evidenceLines, "")
	evidenceLines = append(evidenceLines, "== 各主机指标数据 ==")
	for _, m := range inspection.Metrics {
		name := MetricDisplayName(m.Name)
		evidenceLines = append(evidenceLines, fmt.Sprintf("  %s %s: %.2f %s [%s]", m.IP, name, m.Value, m.Unit, m.Status))
	}
	evidenceLines = append(evidenceLines, "")
	evidenceLines = append(evidenceLines, "== 进程/端口状态 ==")
	for _, p := range inspection.Processes {
		evidenceLines = append(evidenceLines, fmt.Sprintf("  %s:%d %s (%s) -> %s %s", p.IP, p.Port, p.Name, p.Layer, p.Status, p.Alert))
	}
	evidenceLines = append(evidenceLines, "")
	evidenceLines = append(evidenceLines, "== 拓扑结构风险 ==")
	for _, f := range inspection.TopFindings {
		evidenceLines = append(evidenceLines, "  "+f)
	}
	evidence := strings.Join(evidenceLines, "\n")

	sysPrompt := fmt.Sprintf(`你是专业的业务拓扑巡检专家。请基于以下监控数据，生成一份**分层、重点突出、可行动**的业务系统巡检报告。

**核心原则：**
1. **拓扑驱动**：从业务拓扑出发，逐层检查（入口层→应用层→中间件层→数据层）
2. **健康优先**：正常的一笔带过，异常的详细分析并用 **加粗** 标注
3. **风险分级**：按 P0（致命）/P1（高危）/P2（中危）分级
4. **可行动**：每个问题给出具体的处置建议

**报告结构：**

# 业务巡检报告：%s

## 一、总体健康评估

| 指标 | 值 |
|------|-----|
| **系统整体健康分** | **%d/100** |
| 拓扑节点总数 | %d |
| 健康节点（≥85分） | X |
| 亚健康节点（70-84分） | X |
| 危险节点（<70分） | X |
| **当前状态** | %s |

**一句话结论**：...（20字以内）

---

## 二、分层巡检详情

### 2.1 入口层（Gateway/Nginx）

| 节点 | IP:端口 | 健康分 | 关键指标 | 状态 |
|------|---------|--------|----------|------|
| nginx | 198.18.20.20:80 | X | QPS=X, 连接数=X, 5xx=X%% | ✓ 正常 / ⚠️ 异常 |

**本层分析**：
- 基础资源：[正常] 或 [⚠️ CPU 78%%，超过阈值 65%%]
- 业务指标：[正常] 或 [⚠️ 5xx 比例 0.8%%，建议检查后端服务]
- **结论**：...

### 2.2 应用层（JVM/App）

| 节点 | IP:端口 | 健康分 | 关键指标 | 状态 |
|------|---------|--------|----------|------|
| jvm-01 | 198.18.20.11:8081 | X | 堆内存=X%%, GC=X次 | ✓ 正常 / ⚠️ 异常 |
| jvm-02 | 198.18.20.12:8081 | X | 堆内存=X%%, GC=X次 | ✓ 正常 / ⚠️ 异常 |

**本层分析**：
- 基础资源：[正常] 或 [⚠️ 内存异常]
- 业务指标：[正常] 或 [⚠️ Full GC 频繁]
- **结论**：...

### 2.3 中间件层（Redis/Cache）

| 节点 | IP:端口 | 健康分 | 关键指标 | 状态 |
|------|---------|--------|----------|------|
| redis | 198.18.20.20:6375 | X | 命中率=X%%, 连接数=X | ✓ 正常 / ⚠️ 异常 |

**本层分析**：
- 基础资源：[正常] 或 [⚠️ 内存异常]
- 业务指标：[正常] 或 [⚠️ 命中率下降]
- **结论**：...

### 2.4 数据层（Oracle/Database）

| 节点 | IP:端口 | 健康分 | 关键指标 | 状态 |
|------|---------|--------|----------|------|
| oracle-01 | 198.18.22.11:1521 | X | 会话数=X, 表空间=X%% | ✓ 正常 |
| oracle-02 | 198.18.22.12:1521 | X | 会话数=X, 表空间=X%% | ✓ 正常 |
| oracle-03 | 198.18.22.13:1521 | X | 会话数=X, 表空间=X%% | ✓ 正常 |

**本层分析**：
- 基础资源：[正常]（内存 76%% 符合数据库特性）
- 业务指标：[正常] Oracle 集群健康
- **结论**：...

---

## 三、拓扑结构风险

| 优先级 | 风险类型 | 描述 | 影响 |
|--------|---------|------|------|
| **P1** | 单点风险 | 入口层 gateway 仅 1 个节点 | 故障将导致业务整体不可用 |
| **P1** | 单点风险 | 缓存层 redis 仅 1 个节点 | 故障将导致性能严重下降 |
| **P2** | 共址风险 | nginx 与 redis 部署在同一主机 | 单机故障影响多层 |

---

## 四、优先级修复路线图

| 优先级 | 问题 | 影响 | 建议操作 | 预计耗时 |
|--------|------|------|----------|----------|
| **P0** | [仅在有致命问题时列出] | - | - | - |
| **P1** | 入口层单点 | 业务可用性风险 | 增加 nginx 节点，配置负载均衡 | 2h |
| **P1** | 缓存层单点 | 性能风险 | 部署 Redis 主从或集群 | 4h |
| **P2** | 业务监控缺失 | 可观测性不足 | 补齐 nginx/jvm/redis 业务指标采集 | 1h |

---

## 五、附录

- **数据来源**：Prometheus（实时指标）+ Catpaw（主机巡检）+ 夜莺（告警）
- **巡检时间**：%s
- **下次巡检建议**：24小时后

---

**重要提示：**
- 如果某个指标显示 0.00 或 unknown，说明"业务监控缺失，建议补齐"
- 如果基础资源全部正常，直接说"基础资源正常"，不要逐项列举
- 只在有异常时才详细展开分析

== 巡检证据 ==
%s`, businessName, inspection.Score, len(inspection.Processes), inspection.Status, time.Now().Format("2006-01-02 15:04:05"), evidence)

	messages := []map[string]interface{}{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": fmt.Sprintf("请对 %s 业务系统进行全面巡检分析，输出纯 Markdown 格式报告", businessName)},
		{"role": "assistant", "content": "# 业务巡检报告：" + businessName + "\n\n## 一、总览\n\n"},
	}
	body, _ := json.Marshal(map[string]interface{}{"model": mdl, "messages": messages, "max_tokens": 4096})
	req, _ := http.NewRequest("POST", baseURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: chatUpstreamTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return renderInspectionAnswer(inspection)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return renderInspectionAnswer(inspection)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Choices) == 0 {
		return renderInspectionAnswer(inspection)
	}

	content := result.Choices[0].Message.Content
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	trimmed := strings.TrimSpace(content)

	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		fmt.Printf("[DEBUG] LLM returned JSON, converting to Markdown (length: %d)\n", len(trimmed))
		return renderInspectionMarkdownFromJSON(trimmed, inspection)
	}
	fmt.Printf("[DEBUG] LLM returned Markdown directly (length: %d)\n", len(content))
	return content
}

func renderInspectionMarkdownFromJSON(jsonContent string, inspection model.BusinessInspection) string {
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &parsed); err != nil {
		return renderInspectionAnswer(inspection)
	}

	var md strings.Builder
	md.WriteString(fmt.Sprintf("# 业务巡检报告：%s\n\n", inspection.BusinessName))
	md.WriteString("## 一、总览\n\n")
	md.WriteString(fmt.Sprintf("- **健康评分**：%d/100\n", inspection.Score))
	md.WriteString(fmt.Sprintf("- **状态**：%s\n", inspection.Status))

	if summary, ok := parsed["summary"].(map[string]interface{}); ok {
		if judgment, ok := summary["overall_judgment"].(string); ok {
			md.WriteString(fmt.Sprintf("- **一句话结论**：%s\n\n", judgment))
		}
	} else if summaryStr, ok := parsed["summary"].(string); ok {
		md.WriteString(fmt.Sprintf("- **一句话结论**：%s\n\n", summaryStr))
	}

	md.WriteString("## 二、各主机详细分析\n\n")
	hostsByIP := make(map[string][]model.BusinessMetricSample)
	for _, m := range inspection.Metrics {
		hostsByIP[m.IP] = append(hostsByIP[m.IP], m)
	}

	processByIP := make(map[string][]model.BusinessProcess)
	for _, p := range inspection.Processes {
		processByIP[p.IP] = append(processByIP[p.IP], p)
	}

	layers := []struct{ name, ips string }{
		{"入口层（nginx）", "198.18.20.20"},
		{"应用层（jvm）", "198.18.20.11,198.18.20.12"},
		{"中间件层（redis）", "198.18.20.20"},
		{"数据库层（oracle）", "198.18.22.11,198.18.22.12,198.18.22.13"},
	}

	for _, layer := range layers {
		md.WriteString(fmt.Sprintf("### %s\n\n", layer.name))
		for _, ip := range strings.Split(layer.ips, ",") {
			if metrics, ok := hostsByIP[ip]; ok {
				md.WriteString(fmt.Sprintf("**%s**\n\n", ip))
				for _, m := range metrics {
					status := "正常"
					if m.Status == "warning" {
						status = "**警告**"
					} else if m.Status == "critical" {
						status = "**异常**"
					}
					md.WriteString(fmt.Sprintf("- %s：%.2f %s [%s]\n", MetricDisplayName(m.Name), m.Value, m.Unit, status))
				}
				if procs, ok := processByIP[ip]; ok {
					for _, p := range procs {
						md.WriteString(fmt.Sprintf("- 进程：%s:%d (%s) → %s\n", p.IP, p.Port, p.Name, p.Status))
					}
				}
				md.WriteString("\n")
			}
		}
	}

	md.WriteString("## 三、拓扑结构风险\n\n")
	for _, f := range inspection.TopFindings {
		md.WriteString(fmt.Sprintf("- %s\n", f))
	}

	md.WriteString("\n## 四、处置建议\n\n")
	for _, r := range inspection.AIRecommendations {
		md.WriteString(fmt.Sprintf("- %s\n", r))
	}

	return md.String()
}
