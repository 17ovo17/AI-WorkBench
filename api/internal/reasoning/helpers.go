package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func formatData(data map[string]interface{}) string {
	b, _ := json.MarshalIndent(data, "", "  ")
	if len(b) > 3000 {
		return string(b[:3000]) + "\n... (truncated)"
	}
	return string(b)
}

func formatRejected(hypotheses []Hypothesis) string {
	var parts []string
	for _, h := range hypotheses {
		if h.Status == "rejected" {
			parts = append(parts, fmt.Sprintf("- [%s] %s (原因: %s)", h.ID, h.Description, h.Reasoning))
		}
	}
	return strings.Join(parts, "\n")
}

func formatHypothesesWithEvidence(hypotheses []Hypothesis) string {
	var parts []string
	for _, h := range hypotheses {
		if h.Status == "rejected" {
			continue
		}
		part := fmt.Sprintf("### %s: %s (category=%s)\n", h.ID, h.Description, h.Category)
		for _, ev := range h.Evidence {
			part += fmt.Sprintf("  证据[%s]: %s\n", ev.Source, ev.Summary)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "\n")
}

func formatEvidence(evidence []Evidence) string {
	var parts []string
	for _, ev := range evidence {
		parts = append(parts, fmt.Sprintf("[%s] %s", ev.Source, ev.Summary))
	}
	return strings.Join(parts, "; ")
}

func findBestHypothesis(hypotheses []Hypothesis) *Hypothesis {
	var best *Hypothesis
	for i := range hypotheses {
		if hypotheses[i].Status == "rejected" {
			continue
		}
		if best == nil || hypotheses[i].Confidence > best.Confidence {
			best = &hypotheses[i]
		}
	}
	return best
}

func (e *DiagnosticEngine) buildResult(ctx context.Context, input DiagnosticInput, best *Hypothesis, all []Hypothesis, rounds int, duration time.Duration) *DiagnosticResult {
	result := &DiagnosticResult{Hypotheses: all, Rounds: rounds, Duration: duration.String()}
	if best != nil {
		result.RootCause = best.Description
		result.Category = best.Category
		result.Confidence = best.Confidence
		result.Evidence = best.Evidence
		result.Treatment = e.generateTreatment(ctx, input, best)
	} else {
		result.RootCause = "未能确定根因，建议人工排查"
	}
	if result.Confidence >= 0.8 {
		result.ConfidenceLevel = "HIGH"
	} else if result.Confidence >= 0.5 {
		result.ConfidenceLevel = "MEDIUM"
	} else {
		result.ConfidenceLevel = "LOW"
	}
	return result
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
