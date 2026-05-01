package model

import "time"

// Workflow represents a workflow definition (builtin or custom).
type Workflow struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DSL         string    `json:"dsl"`
	Builtin     bool      `json:"builtin"`
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkflowVersion struct {
	ID          string    `json:"id"`
	WorkflowID  string    `json:"workflow_id"`
	Version     int       `json:"version"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DSL         string    `json:"dsl"`
	CreatedAt   time.Time `json:"created_at"`
}

// WorkflowRun represents a single execution record of a workflow.
type WorkflowRun struct {
	ID              string    `json:"id"`
	WorkflowID      string    `json:"workflow_id"`
	WorkflowVersion int       `json:"workflow_version"`
	Status          string    `json:"status"` // running, succeeded, failed, cancelled
	Inputs          string    `json:"inputs"`
	Outputs         string    `json:"outputs"`
	ErrorMessage    string    `json:"error_message"`
	ElapsedMs       int64     `json:"elapsed_ms"`
	CreatedAt       time.Time `json:"created_at"`
}
