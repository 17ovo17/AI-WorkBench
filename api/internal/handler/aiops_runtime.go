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

)

func runAIOpsDiagnosis(sessionID, content string, attachments []model.AIOpsAttachment, audienceHint string) model.AIOpsMessage {
	started := time.Now()
	mode := detectAIOpsMode(content, attachments)
	hosts := extractAIOpsHosts(content)
	businessName := detectBusinessName(content)
	audience := normalizeAIOpsAudience(audienceHint, content, mode)
	steps := []model.ReasoningStep{{Step: 1, Action: "entity_extraction", Input: content, Output: map[string]any{"mode": mode, "hosts": hosts, "business": businessName, "audience": audience}, Status: "completed", Timestamp: time.Now(), Inference: "Parsed intent, hosts, business scope, and audience.", Confidence: "high"}}
	dataSources := []model.DataSourceUsage{}
	suggested := []model.SuggestedAction{}
	topology := model.TopologyHighlight{}
	contentOut := ""

	switch mode {
	case "inspection":
		contentOut, steps, dataSources, suggested, topology = runAIOpsInspectionAnswer(content, businessName, hosts, steps)
	case "topology":
		contentOut, steps, dataSources, suggested, topology = runAIOpsTopologyAnswer(content, businessName, hosts, steps)
	case "report":
		contentOut, steps, dataSources, suggested, topology = runAIOpsReportAnswer(content, attachments, hosts, steps)
	default:
		contentOut, steps, dataSources, suggested, topology = runAIOpsDiagnosticAnswer(content, hosts, steps)
	}
	if len(dataSources) == 0 {
		dataSources = append(dataSources, model.DataSourceUsage{Source: "user", Queries: 1, LatencyMs: time.Since(started).Milliseconds(), Status: "used"})
	}
	summary := buildAIOpsSummaryCard(audience, mode, content, contentOut, hosts, steps, dataSources, topology)
	handoff := buildAIOpsHandoffNote(mode, content, hosts, steps, dataSources, suggested, summary)
	suggested = appendAudienceActions(suggested, summary, handoff)
	return model.AIOpsMessage{MessageID: store.NewID(), ID: store.NewID(), SessionID: sessionID, Role: "assistant", Content: contentOut, Audience: audience, SummaryCard: summary, HandoffNote: handoff, ReasoningChain: steps, DataSources: dataSources, SuggestedActions: suggested, Topology: topology, Attachments: attachments, CreatedAt: time.Now()}
}

func normalizeAIOpsAudience(input, question, mode string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	switch value {
	case "user", "ops", "oncall", "manager":
		return value
	}
	lower := strings.ToLower(question)
	if strings.Contains(question, "\u9886\u5bfc") || strings.Contains(question, "\u7ba1\u7406") || strings.Contains(question, "\u6c47\u62a5") || strings.Contains(question, "\u4e1a\u52a1\u5f71\u54cd") || strings.Contains(lower, "manager") {
		return "manager"
	}
	if strings.Contains(question, "\u503c\u73ed") || strings.Contains(question, "\u4ea4\u63a5") || strings.Contains(lower, "oncall") || strings.Contains(lower, "p0") || strings.Contains(lower, "p1") {
		return "oncall"
	}
	if strings.Contains(lower, "promql") || strings.Contains(question, "\u547d\u4ee4") || strings.Contains(question, "\u8bc1\u636e") || strings.Contains(question, "\u6839\u56e0") || strings.Contains(question, "\u6392\u67e5") {
		return "ops"
	}
	if mode == "inspection" || mode == "topology" {
		return "ops"
	}
	return "user"
}

func buildAIOpsSummaryCard(audience, mode, question, answer string, hosts []string, steps []model.ReasoningStep, dataSources []model.DataSourceUsage, topology model.TopologyHighlight) model.AIOpsSummaryCard {
	severity := inferAIOpsSeverity(question, steps, topology)
	escalate := severity == "P0" || severity == "P1"
	impact := "\u6682\u672a\u53d1\u73b0\u660e\u786e\u4e1a\u52a1\u5f71\u54cd\uff1b\u5efa\u8bae\u7ed3\u5408\u62d3\u6251\u548c\u544a\u8b66\u7ee7\u7eed\u786e\u8ba4\u3002"
	if len(topology.HighlightNodes) > 0 || len(topology.Nodes) > 0 {
		impact = "\u5df2\u5173\u8054\u4e1a\u52a1\u62d3\u6251\u8282\u70b9\uff0c\u9700\u5173\u6ce8\u4e0a\u4e0b\u6e38\u94fe\u8def\u5f71\u54cd\u3002"
	} else if len(hosts) > 1 {
		impact = fmt.Sprintf("\u6d89\u53ca %d \u53f0\u4e3b\u673a\uff0c\u5efa\u8bae\u6309\u540c\u4e00\u6545\u969c\u57df\u6392\u67e5\u3002", len(hosts))
	} else if len(hosts) == 1 && hosts[0] != ".*" {
		impact = "\u5f53\u524d\u4e3b\u8981\u5f71\u54cd\u76ee\u6807\u4e3b\u673a " + hosts[0] + "\uff0c\u4e1a\u52a1\u5f71\u54cd\u9700\u7ed3\u5408\u62d3\u6251\u786e\u8ba4\u3002"
	}
	next := "\u5148\u67e5\u770b\u7b80\u660e\u7ed3\u8bba\uff1b\u5982\u4ecd\u5f02\u5e38\uff0c\u8bf7\u4ea4\u7ed9\u8fd0\u7ef4\u6309 PromQL \u8bc1\u636e\u590d\u6838\u3002"
	switch audience {
	case "ops":
		next = "\u5c55\u5f00\u63a8\u7406\u94fe\uff0c\u590d\u67e5 PromQL \u4e0e Catpaw \u8bc1\u636e\uff0c\u518d\u6309\u53ea\u8bfb\u547d\u4ee4\u9a8c\u8bc1\u3002"
	case "oncall":
		next = "\u8bb0\u5f55\u5f53\u524d\u72b6\u6001\uff0c\u590d\u5236\u4ea4\u63a5\u6458\u8981\uff1b\u82e5 15 \u5206\u949f\u5185\u65e0\u7f13\u89e3\u6216\u5f71\u54cd\u6269\u5927\u5219\u5347\u7ea7\u3002"
	case "manager":
		next = "\u5173\u6ce8\u4e1a\u52a1\u5f71\u54cd\u3001\u5065\u5eb7\u8bc4\u5206\u548c\u6062\u590d\u9884\u4f30\uff0c\u7b49\u5f85\u503c\u73ed\u4eba\u5458\u66f4\u65b0\u5904\u7f6e\u72b6\u6001\u3002"
	}
	return model.AIOpsSummaryCard{Problem: compactAIOpsText(question, 120), Impact: impact, Severity: severity, NextStep: next, EscalationNeeded: escalate, AudienceHint: audience}
}

func buildAIOpsHandoffNote(mode, question string, hosts []string, steps []model.ReasoningStep, dataSources []model.DataSourceUsage, actions []model.SuggestedAction, summary model.AIOpsSummaryCard) model.AIOpsHandoffNote {
	facts := []string{}
	for _, source := range dataSources {
		facts = append(facts, fmt.Sprintf("%s: %d \u6b21\u67e5\u8be2\uff0c\u72b6\u6001 %s", source.Source, source.Queries, emptyAs(source.Status, "used")))
	}
	if len(hosts) > 0 && hosts[0] != ".*" {
		facts = append(facts, "\u76ee\u6807\u4e3b\u673a: "+strings.Join(hosts, ", "))
	}
	open := []string{"\u662f\u5426\u5b58\u5728\u6700\u8fd1\u53d1\u5e03/\u914d\u7f6e\u53d8\u66f4", "\u4e1a\u52a1\u4fa7\u662f\u5426\u4ecd\u6709\u7528\u6237\u62a5\u969c"}
	next := []string{}
	for _, action := range actions {
		if action.Label != "" {
			next = append(next, action.Label)
		}
	}
	if len(next) == 0 {
		next = append(next, summary.NextStep)
	}
	status := "\u5f85\u786e\u8ba4"
	if summary.Severity == "P0" || summary.Severity == "P1" {
		status = "\u9700\u5347\u7ea7"
	} else if mode == "inspection" {
		status = "\u5de1\u68c0\u8bb0\u5f55"
	}
	return model.AIOpsHandoffNote{Status: status, Summary: fmt.Sprintf("[%s] %s\uff1b\u5f71\u54cd\uff1a%s\uff1b\u4e0b\u4e00\u6b65\uff1a%s", summary.Severity, summary.Problem, summary.Impact, summary.NextStep), VerifiedFacts: limitStrings(facts, 6), OpenQuestions: open, SuggestedNext: limitStrings(next, 5), EscalationPolicy: "P0/P1\u3001\u5f71\u54cd\u6269\u5927\u3001\u8fde\u7eed 15 \u5206\u949f\u672a\u7f13\u89e3\u6216 Prometheus/Catpaw \u5747\u63d0\u793a\u5f02\u5e38\u65f6\u5347\u7ea7\u3002"}
}

func inferAIOpsSeverity(question string, steps []model.ReasoningStep, topology model.TopologyHighlight) string {
	lower := strings.ToLower(question)
	if strings.Contains(lower, "p0") || strings.Contains(question, "\u5168\u7ad9") || strings.Contains(question, "\u4e0d\u53ef\u7528") || strings.Contains(question, "\u5b95\u673a") {
		return "P0"
	}
	if strings.Contains(lower, "p1") || strings.Contains(question, "\u544a\u8b66") || strings.Contains(question, "\u5927\u91cf") || len(topology.HighlightNodes) >= 3 || len(topology.Nodes) >= 3 {
		return "P1"
	}
	for _, step := range steps {
		text := fmt.Sprint(step.Output) + step.Inference
		if strings.Contains(text, "danger") || strings.Contains(text, "\u5371\u9669") || strings.Contains(text, "\u5931\u8d25") {
			return "P1"
		}
	}
	if strings.Contains(question, "\u6162") || strings.Contains(question, "\u9ad8") || strings.Contains(question, "\u5f02\u5e38") {
		return "P2"
	}
	return "P3"
}

func handoffNoteText(note model.AIOpsHandoffNote) string {
	parts := []string{note.Summary, "\u72b6\u6001\uff1a" + note.Status}
	if len(note.VerifiedFacts) > 0 {
		parts = append(parts, "\u5df2\u9a8c\u8bc1\uff1a"+strings.Join(note.VerifiedFacts, "\uff1b"))
	}
	if len(note.OpenQuestions) > 0 {
		parts = append(parts, "\u5f85\u786e\u8ba4\uff1a"+strings.Join(note.OpenQuestions, "\uff1b"))
	}
	if len(note.SuggestedNext) > 0 {
		parts = append(parts, "\u5efa\u8bae\u4e0b\u4e00\u6b65\uff1a"+strings.Join(note.SuggestedNext, "\uff1b"))
	}
	parts = append(parts, "\u5347\u7ea7\u6761\u4ef6\uff1a"+note.EscalationPolicy)
	return strings.Join(parts, "\n")
}

func appendAudienceActions(actions []model.SuggestedAction, summary model.AIOpsSummaryCard, handoff model.AIOpsHandoffNote) []model.SuggestedAction {
	copySummary := fmt.Sprintf("\u95ee\u9898\uff1a%s\n\u5f71\u54cd\uff1a%s\n\u7ea7\u522b\uff1a%s\n\u4e0b\u4e00\u6b65\uff1a%s\n\u662f\u5426\u5347\u7ea7\uff1a%t", summary.Problem, summary.Impact, summary.Severity, summary.NextStep, summary.EscalationNeeded)
	actions = append(actions, model.SuggestedAction{ID: "copy-summary", Type: "command", Label: "\u590d\u5236\u95ee\u9898\u6458\u8981", Command: copySummary, Params: map[string]any{"command": copySummary, "copyOnly": true}})
	note := handoffNoteText(handoff)
	actions = append(actions, model.SuggestedAction{ID: "copy-handoff", Type: "command", Label: "\u590d\u5236\u503c\u73ed\u4ea4\u63a5", Command: note, Params: map[string]any{"command": note, "copyOnly": true}})
	return actions
}

func runAIOpsDiagnosticAnswer(question string, hosts []string, steps []model.ReasoningStep) (string, []model.ReasoningStep, []model.DataSourceUsage, []model.SuggestedAction, model.TopologyHighlight) {
	chainName := selectDiagnosisChain(question)
	if len(hosts) == 0 {
		if ip := extractIP(question); ip != "" {
			hosts = []string{ip}
		}
	}
	if len(hosts) == 0 {
		hosts = []string{".*"}
	}
	targetIP := hosts[0]

	steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "chain_selection", Output: chainName, Status: "completed", Timestamp: time.Now(), Inference: fmt.Sprintf("选择诊断链: %s，目标: %s", chainName, targetIP)})

	report, dataSource := diagnoseWithAI(targetIP, DiagnoseOptions{Prompt: question})

	steps = append(steps, model.ReasoningStep{Step: len(steps) + 1, Action: "llm_diagnosis", Output: fmt.Sprintf("LLM 完成诊断，数据源: %s，报告长度: %d 字符", dataSource, len(report)), Status: "completed", Timestamp: time.Now(), Confidence: "high", Inference: "LLM 基于 Prometheus 指标和 Catpaw 数据完成多轮工具调用诊断。"})

	dataSources := []model.DataSourceUsage{{Source: dataSource, Queries: 1, Status: "used"}}
	suggested := []model.SuggestedAction{}
	topology := topologyHighlightForHosts(hosts)
	if len(topology.HighlightNodes) > 0 {
		suggested = append(suggested, model.SuggestedAction{ID: "topology-highlight", Type: "link", Label: "查看关联拓扑", URL: "/topology?highlight=" + strings.Join(topology.HighlightNodes, ","), Params: map[string]any{"url": "/topology?highlight=" + strings.Join(topology.HighlightNodes, ",")}})
	}
	suggested = append(suggested, model.SuggestedAction{ID: "copy-safe-command", Type: "command", Label: "复制只读排查命令", Command: readonlyCommandForChain(chainName), Params: map[string]any{"command": readonlyCommandForChain(chainName)}})

	if report == "" {
		report = "诊断未能获取有效结果，请检查 AI 服务配置和 Prometheus 数据源连接。"
	}

	now := time.Now()
	rec := &model.DiagnoseRecord{
		ID: store.NewID(), TargetIP: targetIP, Trigger: "aiops_chat", Source: "prometheus",
		DataSource: dataSource, Status: model.StatusDone, Report: report,
		SummaryReport: report, CreateTime: now, EndTime: &now,
	}
	store.AddRecord(rec)

	report += "\n\n---\n> 💡 本次诊断报告已保存到「诊断记录」，点击左侧菜单可查看完整报告，阅读体验更佳。"

	suggested = append(suggested, model.SuggestedAction{ID: "view-diagnose", Type: "link", Label: "查看诊断记录", URL: "/diagnose", Params: map[string]any{"url": "/diagnose"}})

	return report, steps, dataSources, suggested, topology
}

func callLLMWithEvidence(question, chainName string, hosts []string, observations []string, catpawText string) (string, string) {
	baseURL := getBaseURL() + "/chat/completions"
	apiKey := getAPIKey()
	mdl := resolveDefaultModel()
	if apiKey == "" || baseURL == "" {
		fallback := inferAIOpsRootCause(chainName, observations, catpawText)
		return fallback, renderDiagnosticMarkdown(question, chainName, hosts, observations, catpawText, fallback)
	}

	evidenceText := strings.Join(observations, "\n")
	if catpawText != "" {
		evidenceText += "\n\nCatpaw 探针巡检数据：\n" + catpawText
	}

	sysPrompt := fmt.Sprintf(`你是专业的 AIOps 智能运维诊断专家。用户提出了一个运维问题，系统已自动从 Prometheus 查询了相关指标数据。

请基于以下证据进行深度分析：
1. 判断各项指标是否异常（对比行业基线）
2. 如果数据缺失，明确标注"[数据缺失]"而不是编造数据
3. 给出根因分析（从最可能到最不可能排列）
4. 给出分级处置建议（立即/短期/长期）
5. 给出健康评分（0-100）

目标主机：%s
诊断链：%s

== Prometheus 查询结果 ==
%s

输出要求：
- 使用中文，技术术语保留英文
- Markdown 格式
- 必须包含：诊断结论、各维度分析、根因分析、风险评估、处置建议
- 如果所有指标都缺失，重点分析监控盲区风险并给出接入建议`, strings.Join(hosts, ", "), chainName, evidenceText)

	messages := []map[string]interface{}{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": question},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model":      mdl,
		"messages":   messages,
		"max_tokens": 4096,
	})
	req, err := http.NewRequest("POST", baseURL, bytes.NewReader(body))
	if err != nil {
		fallback := inferAIOpsRootCause(chainName, observations, catpawText)
		return fallback, renderDiagnosticMarkdown(question, chainName, hosts, observations, catpawText, fallback)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: chatUpstreamTimeout}
	resp, err := client.Do(req)
	if err != nil {
		fallback := inferAIOpsRootCause(chainName, observations, catpawText)
		return fallback, renderDiagnosticMarkdown(question, chainName, hosts, observations, catpawText, fallback)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		fallback := inferAIOpsRootCause(chainName, observations, catpawText)
		return fallback, renderDiagnosticMarkdown(question, chainName, hosts, observations, catpawText, fallback)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Choices) == 0 {
		fallback := inferAIOpsRootCause(chainName, observations, catpawText)
		return fallback, renderDiagnosticMarkdown(question, chainName, hosts, observations, catpawText, fallback)
	}

	llmContent := result.Choices[0].Message.Content
	rootCause := llmContent
	if idx := strings.Index(llmContent, "根因"); idx > 0 && idx < 500 {
		rootCause = llmContent[idx:]
		if end := strings.Index(rootCause, "\n##"); end > 0 {
			rootCause = rootCause[:end]
		}
	}
	return rootCause, llmContent
}
