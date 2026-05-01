package model

import "time"

// WorkflowRunStep 表示工作流单次执行中某个节点的执行记录，用于断点续跑
type WorkflowRunStep struct {
	ID         int64      `json:"id"`
	RunID      string     `json:"run_id"`
	NodeID     string     `json:"node_id"`
	NodeType   string     `json:"node_type"`
	Status     string     `json:"status"` // pending/running/success/failed/skipped
	Inputs     string     `json:"inputs,omitempty"`
	Outputs    string     `json:"outputs,omitempty"`
	Error      string     `json:"error,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ElapsedMs  int64      `json:"elapsed_ms"`
	RetryCount int        `json:"retry_count"`
}
