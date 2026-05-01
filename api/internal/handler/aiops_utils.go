package handler

import (
	"sort"
	"strings"

	"ai-workbench-api/internal/model"
)

func getAIOpsInspection(id string) (model.AIOpsInspection, bool) {
	aiopsMu.Lock()
	defer aiopsMu.Unlock()
	inspection, ok := aiopsInspections[id]
	return inspection, ok
}

func sanitizeAIOpsAttachments(items []model.AIOpsAttachment) []model.AIOpsAttachment {
	out := make([]model.AIOpsAttachment, 0, len(items))
	for _, item := range items {
		item.Content = sanitizeAIOpsText(item.Content)
		out = append(out, item)
	}
	return out
}

func sanitizeAIOpsText(text string) string {
	for _, key := range []string{"password", "passwd", "token", "api_key", "apikey", "secret", "Authorization"} {
		text = redactKeyValue(text, key)
	}
	return text
}

func redactKeyValue(text, key string) string {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(key))
	for idx >= 0 {
		end := idx + len(key)
		for end < len(text) && (text[end] == ' ' || text[end] == ':' || text[end] == '=' || text[end] == '\t') {
			end++
		}
		valueEnd := end
		for valueEnd < len(text) && text[valueEnd] != ' ' && text[valueEnd] != '\n' && text[valueEnd] != '&' {
			valueEnd++
		}
		if valueEnd > end {
			text = text[:end] + "***" + text[valueEnd:]
		}
		lower = strings.ToLower(text)
		next := strings.Index(lower[idx+len(key):], strings.ToLower(key))
		if next < 0 {
			break
		}
		idx = idx + len(key) + next
	}
	return text
}

func compactAIOpsText(text string, limit int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r", ""))
	if len([]rune(text)) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "..."
}

func markdownList(items []string, limit int) string {
	if len(items) == 0 {
		return "- 无有效指标返回"
	}
	if len(items) > limit {
		items = items[:limit]
	}
	lines := make([]string, len(items))
	for i, item := range items {
		lines[i] = "- " + item
	}
	return strings.Join(lines, "\n")
}

func firstPromQLFromObservation(items []string) string {
	for _, item := range items {
		start := strings.Index(item, "（")
		end := strings.Index(item, "）：")
		if start >= 0 && end > start {
			return item[start+len("（") : end]
		}
	}
	return "up"
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func stringParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	if v, ok := params[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmptyAIOps(items []string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func modeTitle(mode string) string {
	switch mode {
	case "inspection":
		return "AIOps 主动巡检"
	case "report":
		return "AIOps 异常上报"
	case "topology":
		return "AIOps 拓扑问诊"
	default:
		return "AIOps 智能问诊"
	}
}

func listPromQLTemplateIDsForChain(chainName string) []string {
	lib, err := loadPromQLLibrary()
	if err != nil {
		return nil
	}
	chain := lib.DiagnosisChains[chainName]
	ids := []string{}
	for _, step := range chain.Steps {
		ids = append(ids, step.TemplateID)
	}
	return ids
}

func sortedAIOpsModes() []string {
	items := []string{"diagnostic", "inspection", "report", "topology"}
	sort.Strings(items)
	return items
}
