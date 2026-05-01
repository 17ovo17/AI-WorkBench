package model

import "time"

type ChangeEvent struct {
	ID          int64     `json:"id"`
	TargetIP    string    `json:"target_ip"`
	ChangeType  string    `json:"change_type"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Operator    string    `json:"operator"`
	Source      string    `json:"source"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
