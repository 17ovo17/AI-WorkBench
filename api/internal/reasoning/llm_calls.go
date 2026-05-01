package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
)

type LLMCaller interface {
	Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

func (e *DiagnosticEngine) generateHypotheses(ctx context.Context, input DiagnosticInput, data map[string]interface{}, previous []Hypothesis) ([]Hypothesis, error) {
	sys := `你是资深 SRE，使用 USE 方法论分析故障。基于当前数据生成 3-5 个候选假设。
每个假设包含：id(h1/h2/h3...)、description、category(cpu_high/memory_leak/disk_full/network_issue/db_slow/container_oom/jvm_gc/config_drift/traffic_spike/other)、verification_plan(tool_name+params+purpose)。
可用工具：prometheus_query(ip)、check_alerts(ip)、check_diagnose_history(ip)、knowledge_search(query)。
输出严格 JSON：{"hypotheses":[...]}`

	user := fmt.Sprintf("## 目标: %s\n## 问题: %s\n## 数据:\n%s", input.TargetIP, input.Question, formatData(data))
	if len(previous) > 0 {
		user += "\n## 已排除:\n" + formatRejected(previous) + "\n请生成新方向的假设。"
	}
	resp, err := e.llm.Chat(ctx, sys, user)
	if err != nil {
		return nil, err
	}
	var result struct {
		Hypotheses []Hypothesis `json:"hypotheses"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &result); err != nil {
		return nil, fmt.Errorf("parse hypotheses: %w", err)
	}
	for i := range result.Hypotheses {
		result.Hypotheses[i].Status = "pending"
	}
	return result.Hypotheses, nil
}

func (e *DiagnosticEngine) validateHypotheses(ctx context.Context, input DiagnosticInput, hypotheses []Hypothesis) ([]Hypothesis, error) {
	sys := `你是资深 SRE，基于证据验证每个假设。
对每个假设输出：id、status(verified/rejected/pending)、confidence(0.0-1.0)、reasoning。
输出严格 JSON：{"hypotheses":[{"id":"h1","status":"verified","confidence":0.85,"reasoning":"..."}]}`

	user := fmt.Sprintf("## 目标: %s\n## 假设与证据:\n%s", input.TargetIP, formatHypothesesWithEvidence(hypotheses))
	resp, err := e.llm.Chat(ctx, sys, user)
	if err != nil {
		return hypotheses, err
	}
	var result struct {
		Hypotheses []struct {
			ID         string  `json:"id"`
			Status     string  `json:"status"`
			Confidence float64 `json:"confidence"`
			Reasoning  string  `json:"reasoning"`
		} `json:"hypotheses"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp)), &result); err != nil {
		return hypotheses, nil
	}
	for _, v := range result.Hypotheses {
		for i := range hypotheses {
			if hypotheses[i].ID == v.ID {
				hypotheses[i].Status = v.Status
				hypotheses[i].Confidence = v.Confidence
				hypotheses[i].Reasoning = v.Reasoning
			}
		}
	}
	return hypotheses, nil
}

func (e *DiagnosticEngine) generateTreatment(ctx context.Context, input DiagnosticInput, h *Hypothesis) Treatment {
	if h == nil {
		return Treatment{}
	}
	sys := `你是资深 SRE，基于确认的根因给出处置建议。分 immediate(止血)和 permanent(根治)两层，每层 2-4 条具体命令。
输出 JSON：{"immediate":["..."],"permanent":["..."]}`
	user := fmt.Sprintf("根因: %s\n分类: %s\n证据: %s", h.Description, h.Category, formatEvidence(h.Evidence))
	resp, err := e.llm.Chat(ctx, sys, user)
	if err != nil {
		return Treatment{Immediate: []string{"请人工确认处置方案"}, Permanent: []string{"建立监控告警规则"}}
	}
	var t Treatment
	json.Unmarshal([]byte(extractJSON(resp)), &t)
	return t
}
