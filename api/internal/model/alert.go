package model

import "time"

type AlertRecord struct {
	ID                string              `json:"id"`
	Title             string              `json:"title"`
	TargetIP          string              `json:"target_ip"`
	Severity          string              `json:"severity"` // critical | warning | info
	Status            string              `json:"status"`   // firing | acknowledged | assigned | diagnosing | muted | mitigated | resolved | archived
	Labels            map[string]string   `json:"labels"`
	Annotations       map[string]string   `json:"annotations,omitempty"`
	Source            string              `json:"source"` // catpaw | alertmanager
	BusinessID        string              `json:"business_id,omitempty"`
	Fingerprint       string              `json:"fingerprint,omitempty"`
	Count             int                 `json:"count"`
	FirstSeen         time.Time           `json:"first_seen"`
	LastSeen          time.Time           `json:"last_seen"`
	AckBy             string              `json:"ack_by,omitempty"`
	Assignee          string              `json:"assignee,omitempty"`
	MutedUntil        *time.Time          `json:"muted_until,omitempty"`
	DeletedAt         *time.Time          `json:"deleted_at,omitempty"`
	TestBatchID       string              `json:"test_batch_id,omitempty"`
	AuditTraceID      string              `json:"audit_trace_id,omitempty"`
	DiagnoseRecordID  string              `json:"diagnose_record_id,omitempty"`
	LinkedBusinessID  string              `json:"linked_business_id,omitempty"`
	Resolution        string              `json:"resolution,omitempty"`
	RunbookURL        string              `json:"runbook_url,omitempty"`
	ActionLog         []AlertAction       `json:"action_log,omitempty"`
	NotificationTrail []AlertNotification `json:"notification_trail,omitempty"`
	CreateTime        time.Time           `json:"create_time"`
	ResolvedAt        *time.Time          `json:"resolved_at,omitempty"`
}

type AlertAction struct {
	Action    string    `json:"action"`
	Actor     string    `json:"actor,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	From      string    `json:"from,omitempty"`
	To        string    `json:"to,omitempty"`
	TraceID   string    `json:"trace_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type AlertNotification struct {
	ID        string    `json:"id"`
	Channel   string    `json:"channel"`
	Receiver  string    `json:"receiver"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail,omitempty"`
	Retry     int       `json:"retry"`
	CreatedAt time.Time `json:"created_at"`
}
