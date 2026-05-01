package model

import (
	"encoding/json"
	"time"
)

// DiagnosisCase represents a knowledge base case derived from diagnosis.
// MetricSnapshot 使用 json.RawMessage 以同时支持 JSON 对象输入和数据库 JSON 字段存储。
type DiagnosisCase struct {
	ID                     string          `json:"id"`
	MetricSnapshot         json.RawMessage `json:"metric_snapshot"`
	RootCauseCategory      string          `json:"root_cause_category"`
	RootCauseDescription   string          `json:"root_cause_description"`
	TreatmentSteps         string          `json:"treatment_steps"`
	Keywords               string          `json:"keywords"`
	SourceDiagnosisID      string          `json:"source_diagnosis_id"`
	CreatedAt              time.Time       `json:"created_at"`
	CreatedBy              string          `json:"created_by"`
	EvaluationAvg          float64         `json:"evaluation_avg"`
	DiagnosticPath         json.RawMessage `json:"diagnostic_path,omitempty"`
	DistinguishingFeatures string          `json:"distinguishing_features,omitempty"`
	NegativeFindings       json.RawMessage `json:"negative_findings,omitempty"`
	AvgFeedbackRating      float64         `json:"avg_feedback_rating"`
	VerificationStatus     string          `json:"verification_status"`
}

// DiagnosisFeedback represents user feedback on a diagnosis result.
type DiagnosisFeedback struct {
	ID            string    `json:"id"`
	DiagnosisID   string    `json:"diagnosis_id"`
	User          string    `json:"user"`
	Rating        string    `json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
}

// Runbook represents an operational runbook for incident handling.
type Runbook struct {
	ID                string          `json:"id"`
	Title             string          `json:"title"`
	Category          string          `json:"category"`
	TriggerConditions json.RawMessage `json:"trigger_conditions"`
	Steps             string          `json:"steps"`
	AutoExecutable    bool            `json:"auto_executable"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Version           int             `json:"version"`
	Severity          string          `json:"severity"`       // critical/high/medium/low
	EstimatedTime     string          `json:"estimated_time"` // 预计执行时间如 "5m", "30m"
	Prerequisites     string          `json:"prerequisites"`  // 前置条件
	RollbackSteps     string          `json:"rollback_steps"` // 回滚步骤
	Variables         json.RawMessage `json:"variables"`      // 模板变量定义 JSON
	LastExecutedAt    *time.Time      `json:"last_executed_at"`
	ExecutionCount    int             `json:"execution_count"`
	SuccessRate       float64         `json:"success_rate"`
}

// RunbookExecution represents a single execution record of a runbook.
type RunbookExecution struct {
	ID           string     `json:"id"`
	RunbookID    string     `json:"runbook_id"`
	TargetIP     string     `json:"target_ip"`
	Executor     string     `json:"executor"`
	Status       string     `json:"status"` // running/succeeded/failed/cancelled/manual
	Variables    string     `json:"variables"`
	Output       string     `json:"output"`
	ErrorMessage string     `json:"error_message"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}
