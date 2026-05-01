package handler

import (
	"fmt"
	"strconv"
	"strings"

	"ai-workbench-api/internal/model"
)

func aiopsTopologyGraphForQuestion(question string) (model.AITopologyGraph, bool) {
	businessName := detectBusinessName(question)
	hosts := extractAIOpsHosts(question)
	business, ok := matchBusinessByNameOrHosts(businessName, hosts)
	lowerQuestion := strings.ToLower(question)
	if !ok && len(hosts) < 2 && !strings.Contains(lowerQuestion, "redis") && !strings.Contains(question, "拓扑") && !strings.Contains(question, "架构") {
		return model.AITopologyGraph{}, false
	}
	if ok {
		return buildAITopologyGraph(aiTopologyGenerateRequest{ServiceName: business.Name, Hosts: business.Hosts, Endpoints: business.Endpoints}, "heuristic_fallback", "AIOps websocket topology update"), true
	}
	return model.AITopologyGraph{Nodes: []model.AITopologyNode{}, Links: []model.AITopologyLink{}, Summary: model.AITopologySummary{Planner: "no_business_matched", NodeCount: 0, LinkCount: 0, Error: "未匹配到业务拓扑数据"}}, true
}

func mapStringAny(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringSliceFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if one, ok := value.(string); ok && strings.TrimSpace(one) != "" {
			return []string{one}
		}
		return nil
	}
	out := []string{}
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func attachmentsFromAny(value any) []model.AIOpsAttachment {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := []model.AIOpsAttachment{}
	for _, item := range items {
		row := mapStringAny(item)
		out = append(out, model.AIOpsAttachment{Type: fmt.Sprint(row["type"]), Content: fmt.Sprint(row["content"]), Filename: fmt.Sprint(row["filename"]), Source: fmt.Sprint(row["source"])})
	}
	return out
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case int:
		if typed > 0 {
			return typed
		}
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func metricNameFromPromQL(query string) string {
	query = strings.TrimSpace(query)
	for i, r := range query {
		if !(r == '_' || r == ':' || r == '.' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			if i > 0 {
				return query[:i]
			}
			break
		}
	}
	return query
}

func firstFloatFromText(text string) (float64, bool) {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !(r == '.' || r == '-' || (r >= '0' && r <= '9'))
	})
	for _, field := range fields {
		if field == "" || field == "." || field == "-" {
			continue
		}
		if value, err := strconv.ParseFloat(field, 64); err == nil {
			return value, true
		}
	}
	return 0, false
}
