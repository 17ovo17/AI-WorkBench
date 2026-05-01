package model

import "time"

type DiagnoseStatus string

const (
	StatusPending DiagnoseStatus = "pending"
	StatusRunning DiagnoseStatus = "running"
	StatusDone    DiagnoseStatus = "done"
	StatusFailed  DiagnoseStatus = "failed"
)

type DiagnoseRecord struct {
	ID            string         `json:"id"`
	TargetIP      string         `json:"target_ip"`
	Trigger       string         `json:"trigger"` // "manual" | "alert"
	Source        string         `json:"source"`  // "prometheus" | "catpaw"
	DataSource    string         `json:"data_source,omitempty"`
	Status        DiagnoseStatus `json:"status"`
	Report        string         `json:"report"`
	SummaryReport string         `json:"summary_report,omitempty"`
	RawReport     string         `json:"raw_report,omitempty"`
	AlertTitle    string         `json:"alert_title,omitempty"`
	CreateTime    time.Time      `json:"create_time"`
	EndTime       *time.Time     `json:"end_time,omitempty"`
}

type CatpawAgent struct {
	IP       string    `json:"ip"`
	Hostname string    `json:"hostname"`
	Version  string    `json:"version"`
	LastSeen time.Time `json:"last_seen"`
	Online   bool      `json:"online"`
}

type AlertEvent struct {
	Title    string            `json:"title"`
	TargetIP string            `json:"target_ip"`
	Labels   map[string]string `json:"labels"`
	StartsAt time.Time         `json:"starts_at"`
}
