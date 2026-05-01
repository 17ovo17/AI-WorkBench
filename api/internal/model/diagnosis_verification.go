package model

import "time"

// DiagnosisVerification represents an automated verification result for a diagnosis.
type DiagnosisVerification struct {
	ID                  int64     `json:"id"`
	DiagnosisID         string    `json:"diagnosis_id"`
	AlertResolutionRate float64   `json:"alert_resolution_rate"`
	RecurrenceCount     int       `json:"recurrence_count"`
	RunbookExecuted     bool      `json:"runbook_executed"`
	RunbookSuccess      bool      `json:"runbook_success"`
	Score               float64   `json:"score"`
	VerifiedAt          time.Time `json:"verified_at"`
	CreatedAt           time.Time `json:"created_at"`
}
