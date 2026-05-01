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
	"ai-workbench-api/internal/security"
	"ai-workbench-api/internal/store"

)

type DiagnoseOptions struct {
	Prompt       string
	CredentialID string
	Credential   RemoteCredential
}

var promTool = map[string]interface{}{
	"type": "function",
	"function": map[string]interface{}{
		"name":        "query_prometheus",
		"description": "查询 Prometheus 指标数据，支持任意 PromQL",
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"promql":      map[string]string{"type": "string", "description": "PromQL 查询语句"},
				"description": map[string]string{"type": "string", "description": "本次查询的目的说明"},
			},
			"required": []string{"promql"},
		},
	},
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func RunDiagnose(rec *model.DiagnoseRecord, userPrompt string) {
	RunDiagnoseWithOptions(rec, DiagnoseOptions{Prompt: userPrompt})
}

func RunDiagnoseWithOptions(rec *model.DiagnoseRecord, opts DiagnoseOptions) {
	store.UpdateRecord(rec.ID, func(r *model.DiagnoseRecord) {
		r.Status = model.StatusRunning
	})

	report, dataSource := diagnoseWithAI(rec.TargetIP, opts)

	now := time.Now()
	status := model.StatusDone
	if report == "" {
		status = model.StatusFailed
	}

	var rawReport string
	if status == model.StatusDone {
		inspection := buildHostInspection(rec.TargetIP, dataSource, report)
		raw, _ := json.MarshalIndent(inspection, "", "  ")
		rawReport = string(raw)
	}

	rec.Status = status
	rec.Report = report
	rec.SummaryReport = report
	rec.RawReport = rawReport
	rec.DataSource = dataSource
	rec.EndTime = &now
	store.UpdateRecord(rec.ID, func(r *model.DiagnoseRecord) {
		r.Status = status
		r.Report = report
		r.SummaryReport = report
		r.RawReport = rawReport
		r.DataSource = dataSource
		r.EndTime = &now
	})
}

func diagnoseWithAI(targetIP string, opts DiagnoseOptions) (string, string) {
	baseURL := getBaseURL() + "/chat/completions"
	apiKey := getAPIKey()
	mdl := resolveDefaultModel()

	userPrompt := opts.Prompt
	metrics := availableMetrics(targetIP)
	monitorContext := buildMonitorContext(targetIP, userPrompt)
	dataSource := "prometheus"
	if !hasMonitorData(monitorContext) {
		catpawContext, catpawSource := buildCatpawFallbackContext(targetIP, opts)
		if catpawContext != "" {
			monitorContext += catpawContext
			dataSource = catpawSource
		}
	} else if catpawContext, catpawSource := buildCatpawFallbackContext(targetIP, opts); catpawSource == "catpaw_report" && catpawContext != "" {
		monitorContext += catpawContext
		dataSource = "mixed"
	}
	sysMsg := fmt.Sprintf("你是专业运维诊断 AI。目标主机 IP: %s。\n你可以调用 query_prometheus 工具按需查询任意 Prometheus 指标。\n%s\n%s\n请优先依据上方已经查询到的真实监控数据和 Catpaw 巡检数据进行判断；只有当某项指标确实没有出现在任何数据源中时，才说明无数据。报告开头必须写明数据来源：%s。请根据可用指标系统性地诊断主机状态，给出根因分析和处置建议，使用 Markdown 格式输出。", targetIP, metrics, monitorContext, dataSource)
	if isJVMQuestion(userPrompt) {
		sysMsg += "\n\n[JVM 专项诊断] 用户问题涉及 JVM/GC，请重点查询 jvm_gc_pause_seconds_max、jvm_gc_pause_seconds_count、jvm_memory_used_bytes、jvm_memory_max_bytes、process_cpu_usage 等 JVM 指标。如果 Prometheus 中没有 JVM 指标，明确告知用户需要接入 JMX Exporter 或 Micrometer，并基于通用 CPU/内存指标给出初步判断。"
	}

	messages := []map[string]interface{}{
		{"role": "system", "content": sysMsg},
		{"role": "user", "content": userPrompt},
	}

	client := &http.Client{Timeout: chatUpstreamTimeout}
	for i := 0; i < 15; i++ {
		body, _ := json.Marshal(map[string]interface{}{
			"model":      mdl,
			"messages":   messages,
			"tools":      []interface{}{promTool},
			"max_tokens": 4096,
		})
		req, err := http.NewRequest("POST", baseURL, bytes.NewReader(body))
		if err != nil {
			return "构建请求失败: " + err.Error(), dataSource
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			if isChatTimeoutError(err) {
				return buildAIOpsLLMFallbackContent(targetIP, userPrompt, dataSource), dataSource + "_llm_timeout_fallback"
			}
			return "AI请求失败: " + err.Error(), dataSource
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Sprintf("AI返回错误 HTTP %d: %s", resp.StatusCode, string(data[:min(len(data), 300)])), dataSource
		}

		var result struct {
			Choices []struct {
				Message struct {
					Role      string     `json:"role"`
					Content   string     `json:"content"`
					ToolCalls []toolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(data, &result); err != nil || len(result.Choices) == 0 {
			return fmt.Sprintf("AI响应解析失败: %v data=%s", err, string(data[:min(len(data), 200)])), dataSource
		}

		msg := result.Choices[0].Message
		messages = append(messages, map[string]interface{}{
			"role":       msg.Role,
			"content":    msg.Content,
			"tool_calls": msg.ToolCalls,
		})

		if len(msg.ToolCalls) == 0 {
			return msg.Content, dataSource
		}

		for _, tc := range msg.ToolCalls {
			var args map[string]string
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			val, err := queryProm(args["promql"])
			if err != nil {
				val = "查询失败: " + err.Error()
			}
			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      val,
			})
		}
	}
	return "", dataSource
}

func hasMonitorData(ctx string) bool {
	for _, line := range strings.Split(ctx, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") || !strings.Contains(line, ":") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "no data") || strings.Contains(line, "无数据") || strings.Contains(line, "無數據") {
			continue
		}
		return true
	}
	return false
}

func buildCatpawFallbackContext(ip string, opts DiagnoseOptions) (string, string) {
	if report, ok := store.LatestCatpawReport(ip); ok && strings.TrimSpace(report.Report) != "" {
		return fmt.Sprintf("\n\n[Catpaw 最近巡检报告 - 主机: %s]\n%s\n[Catpaw 报告结束]\n", ip, report.Report), "catpaw_report"
	}

	if !store.HasOnlineAgent(ip) {
		return fmt.Sprintf("\n\n[Catpaw fallback]\nPrometheus has no usable data for %s, and no online Catpaw report/agent is available. Install or start Catpaw and ensure SSH/WinRM ports 22/5985/5986 are reachable.\n", ip), "prometheus_no_data_catpaw_unavailable"
	}

	cred, ok := resolveDiagnoseCredential(ip, opts)
	if ok {
		cred.IP = ip
		if decision := security.ValidateRemoteHost(cred.IP); !decision.Allowed {
			return fmt.Sprintf("\n\n[Catpaw fallback blocked]\nRemote inspect target %s is outside safety whitelist: %s\n", ip, decision.Reason), "catpaw_blocked_by_safety"
		}
		if cred.Protocol == "" {
			cred.Protocol = "ssh"
		}
		cmd := "catpaw inspect system --configs /etc/catpaw/conf.d"
		if strings.EqualFold(cred.Protocol, "winrm") {
			cmd = `C:\catpaw\catpaw.exe inspect system --configs C:\catpaw\conf.d`
		}
		out := ""
		var err error
		if strings.EqualFold(cred.Protocol, "winrm") {
			out, err = execWinRM(RemoteExecRequest{RemoteCredential: cred, Command: cmd})
		} else {
			out, err = execSSH(RemoteExecRequest{RemoteCredential: cred, Command: cmd})
		}
		if err == nil && strings.TrimSpace(out) != "" {
			return fmt.Sprintf("\n\n[Catpaw 即时巡检 - 主机: %s]\n%s\n[Catpaw 即时巡检结束]\n", ip, out), "catpaw_inspect"
		}
		return fmt.Sprintf("\n\n[Catpaw 兜底诊断]\nPrometheus 无数据；Catpaw 探针在线，但即时巡检执行失败：%v。请检查远程凭据、SSH/WinRM 连通性与 catpaw 安装路径。\n", err), "catpaw_online_exec_failed"
	}

	return fmt.Sprintf("\n\n[Catpaw 兜底诊断]\nPrometheus 无数据；Catpaw 探针在线，但请求未提供 credential_id 或内联远程凭据，无法执行即时巡检。下一步：在凭证管理中新增 SSH/WinRM 凭据，或等待探针上报 /api/v1/catpaw/report。\n"), "catpaw_online_no_credential"
}

func resolveDiagnoseCredential(ip string, opts DiagnoseOptions) (RemoteCredential, bool) {
	if opts.Credential.Username != "" || opts.Credential.Password != "" || opts.Credential.SSHKey != "" {
		cred := opts.Credential
		if cred.IP == "" {
			cred.IP = ip
		}
		return cred, true
	}
	if opts.CredentialID == "" {
		return RemoteCredential{}, false
	}
	if saved, ok := store.GetCredential(opts.CredentialID); ok {
		return RemoteCredential{
			IP:       ip,
			Protocol: saved.Protocol,
			Port:     saved.Port,
			Username: saved.Username,
			Password: saved.Password,
			SSHKey:   saved.SSHKey,
		}, true
	}
	return RemoteCredential{}, false
}

func buildHostInspection(ip, dataSource, llmReport string) model.BusinessInspection {
	now := time.Now()
	hosts := []string{ip}
	metrics := businessMetricSamples(hosts, nil)
	alerts := businessAlerts(hosts)
	resources := businessResources(model.TopologyBusiness{ID: "host:" + ip, Name: ip, Hosts: hosts})

	score := 100
	findings := []string{}
	for _, a := range alerts {
		if a.Status == "firing" {
			score -= 18
			findings = append(findings, fmt.Sprintf("存在未恢复告警：%s（%s）", a.Title, a.TargetIP))
		}
	}
	for _, m := range metrics {
		if m.Status == "critical" {
			score -= 12
		} else if m.Status == "warning" {
			score -= 6
		}
	}
	if score < 0 {
		score = 0
	}
	status := "healthy"
	if score < 60 {
		status = "critical"
	} else if score < 85 {
		status = "warning"
	}

	return model.BusinessInspection{
		BusinessID:       "host:" + ip,
		BusinessName:     ip + " 单机诊断",
		Status:           status,
		Score:            score,
		Summary:          fmt.Sprintf("%s 单机诊断完成，数据源: %s", ip, dataSource),
		GeneratedAt:      now,
		Metrics:          metrics,
		Resources:        resources,
		Alerts:           alerts,
		TopologyFindings: findings,
		DataSources:      []string{dataSource},
		AIAnalysis:       llmReport,
	}
}

func isJVMQuestion(text string) bool {
	q := strings.ToLower(text)
	return strings.Contains(q, "jvm") || strings.Contains(q, "gc") || strings.Contains(q, "堆内存") || strings.Contains(q, "老年代") || strings.Contains(q, "full gc") || strings.Contains(q, "young gc") || strings.Contains(q, "元空间")
}
