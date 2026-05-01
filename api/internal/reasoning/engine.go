package reasoning

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	MaxRounds           = 3
	ConfidenceThreshold = 0.8
)

type DiagnosticInput struct {
	TargetIP    string                 `json:"target_ip"`
	Question    string                 `json:"question"`
	AlertTitle  string                 `json:"alert_title"`
	Severity    string                 `json:"severity"`
	InitialData map[string]interface{} `json:"initial_data"`
}

type DiagnosticResult struct {
	RootCause       string       `json:"root_cause"`
	Category        string       `json:"root_cause_category"`
	Confidence      float64      `json:"confidence"`
	ConfidenceLevel string       `json:"confidence_level"`
	Evidence        []Evidence   `json:"evidence"`
	Hypotheses      []Hypothesis `json:"hypotheses"`
	Treatment       Treatment    `json:"treatment"`
	Rounds          int          `json:"reasoning_rounds"`
	Duration        string       `json:"duration"`
}

type Hypothesis struct {
	ID               string             `json:"id"`
	Description      string             `json:"description"`
	Category         string             `json:"category"`
	Confidence       float64            `json:"confidence"`
	Status           string             `json:"status"`
	VerificationPlan []VerificationStep `json:"verification_plan"`
	Evidence         []Evidence         `json:"evidence"`
	Reasoning        string             `json:"reasoning,omitempty"`
}

type VerificationStep struct {
	ToolName string            `json:"tool_name"`
	Params   map[string]string `json:"params"`
	Purpose  string            `json:"purpose"`
}

type Evidence struct {
	Source   string      `json:"source"`
	Data     interface{} `json:"data"`
	Summary  string      `json:"summary"`
	Supports string      `json:"supports"`
}

type Treatment struct {
	Immediate []string `json:"immediate"`
	Permanent []string `json:"permanent"`
}

type DiagnosticEngine struct {
	llm       LLMCaller
	tools     *ToolRegistry
	maxRounds int
	threshold float64
}

func NewEngine(llm LLMCaller, tools *ToolRegistry) *DiagnosticEngine {
	return &DiagnosticEngine{llm: llm, tools: tools, maxRounds: MaxRounds, threshold: ConfidenceThreshold}
}

func (e *DiagnosticEngine) Run(ctx context.Context, input DiagnosticInput) (*DiagnosticResult, error) {
	start := time.Now()
	initialData := e.collectInitialData(ctx, input)
	hypotheses, err := e.generateHypotheses(ctx, input, initialData, nil)
	if err != nil {
		return nil, fmt.Errorf("generate hypotheses: %w", err)
	}
	for round := 0; round < e.maxRounds; round++ {
		for i := range hypotheses {
			if hypotheses[i].Status != "pending" {
				continue
			}
			for _, step := range hypotheses[i].VerificationPlan {
				ev := e.tools.Execute(ctx, step.ToolName, step.Params)
				ev.Supports = hypotheses[i].ID
				hypotheses[i].Evidence = append(hypotheses[i].Evidence, ev)
			}
		}
		hypotheses, err = e.validateHypotheses(ctx, input, hypotheses)
		if err != nil {
			log.WithError(err).Warn("reasoning: validate failed")
			break
		}
		best := findBestHypothesis(hypotheses)
		if best != nil && best.Confidence >= e.threshold {
			return e.buildResult(ctx, input, best, hypotheses, round+1, time.Since(start)), nil
		}
		if round < e.maxRounds-1 {
			hypotheses, err = e.generateHypotheses(ctx, input, initialData, hypotheses)
			if err != nil {
				break
			}
		}
	}
	best := findBestHypothesis(hypotheses)
	return e.buildResult(ctx, input, best, hypotheses, e.maxRounds, time.Since(start)), nil
}

func (e *DiagnosticEngine) collectInitialData(ctx context.Context, input DiagnosticInput) map[string]interface{} {
	data := make(map[string]interface{})
	if input.InitialData != nil {
		for k, v := range input.InitialData {
			data[k] = v
		}
	}
	metrics := e.tools.Execute(ctx, "prometheus_query", map[string]string{"ip": input.TargetIP})
	data["metrics"] = metrics.Data
	alerts := e.tools.Execute(ctx, "check_alerts", map[string]string{"ip": input.TargetIP})
	data["alerts"] = alerts.Data
	return data
}
