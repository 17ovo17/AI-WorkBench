package handler

import (
	"encoding/json"
	"strings"
)

var reportTextKeys = []string{"report", "summary_report", "markdownReport", "markdown", "analysis", "diagnosis", "result", "content"}

func normalizeReportText(text string) string {
	trimmed := stripJSONFence(text)
	if trimmed == "" || (!strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[")) {
		return text
	}
	var value any
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return text
	}
	if report := firstReportText(value); strings.TrimSpace(report) != "" {
		return report
	}
	return text
}

func stripJSONFence(text string) string {
	value := strings.TrimSpace(text)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	return strings.TrimSpace(value)
}

func firstReportText(value any) string {
	switch current := value.(type) {
	case string:
		return current
	case map[string]any:
		for _, key := range reportTextKeys {
			if report := firstReportText(current[key]); strings.TrimSpace(report) != "" {
				return report
			}
		}
	case []any:
		for _, item := range current {
			if report := firstReportText(item); strings.TrimSpace(report) != "" {
				return report
			}
		}
	}
	return ""
}
