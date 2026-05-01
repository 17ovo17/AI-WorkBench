package model

import "time"

type AIOpsScope struct {
	Cluster   string    `json:"cluster,omitempty"`
	Services  []string  `json:"services,omitempty"`
	Hosts     []string  `json:"hosts,omitempty"`
	Layers    []string  `json:"layers,omitempty"`
	TimeRange TimeRange `json:"timeRange,omitempty"`
}

type TimeRange struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type AIOpsAttachment struct {
	Type     string `json:"type"`
	Content  string `json:"content"`
	Filename string `json:"filename,omitempty"`
	Source   string `json:"source,omitempty"`
}

type ReasoningStep struct {
	Step       int       `json:"step"`
	Action     string    `json:"action"`
	Input      any       `json:"input,omitempty"`
	Output     any       `json:"output,omitempty"`
	Query      string    `json:"query,omitempty"`
	Result     any       `json:"result,omitempty"`
	Status     string    `json:"status"`
	Inference  string    `json:"inference,omitempty"`
	Confidence string    `json:"confidence,omitempty"`
	NextStep   string    `json:"next_step,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	LatencyMs  int64     `json:"latencyMs,omitempty"`
}

type DataSourceUsage struct {
	Source    string `json:"source"`
	Queries   int    `json:"queries"`
	LatencyMs int64  `json:"latency_ms"`
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
}

type SuggestedAction struct {
	ID      string         `json:"id,omitempty"`
	Type    string         `json:"type"`
	Label   string         `json:"label"`
	Command string         `json:"command,omitempty"`
	Query   string         `json:"query,omitempty"`
	URL     string         `json:"url,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
}

type TopologyHighlight struct {
	HighlightNodes []string   `json:"highlightNodes,omitempty"`
	HighlightPaths [][]string `json:"highlightPaths,omitempty"`
	Nodes          []any      `json:"nodes,omitempty"`
	Paths          [][]string `json:"paths,omitempty"`
}

type AIOpsSummaryCard struct {
	Problem          string `json:"problem"`
	Impact           string `json:"impact"`
	Severity         string `json:"severity"`
	NextStep         string `json:"nextStep"`
	EscalationNeeded bool   `json:"escalationNeeded"`
	AudienceHint     string `json:"audienceHint,omitempty"`
}

type AIOpsHandoffNote struct {
	Status           string   `json:"status"`
	Owner            string   `json:"owner,omitempty"`
	Summary          string   `json:"summary"`
	VerifiedFacts    []string `json:"verifiedFacts,omitempty"`
	OpenQuestions    []string `json:"openQuestions,omitempty"`
	SuggestedNext    []string `json:"suggestedNext,omitempty"`
	EscalationPolicy string   `json:"escalationPolicy,omitempty"`
}

type AIOpsSession struct {
	SessionID       string         `json:"sessionId"`
	ID              string         `json:"id,omitempty"`
	Title           string         `json:"title"`
	Mode            string         `json:"mode"`
	Status          string         `json:"status"`
	Scope           AIOpsScope     `json:"scope,omitempty"`
	Context         map[string]any `json:"context,omitempty"`
	ContextSnapshot AIOpsScope     `json:"contextSnapshot,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt,omitempty"`
}

type AIOpsMessage struct {
	MessageID        string            `json:"messageId"`
	ID               string            `json:"id,omitempty"`
	SessionID        string            `json:"sessionId,omitempty"`
	Role             string            `json:"role"`
	Content          string            `json:"content"`
	Audience         string            `json:"audience,omitempty"`
	SummaryCard      AIOpsSummaryCard  `json:"summaryCard,omitempty"`
	HandoffNote      AIOpsHandoffNote  `json:"handoffNote,omitempty"`
	ReasoningChain   []ReasoningStep   `json:"reasoningChain,omitempty"`
	DataSources      []DataSourceUsage `json:"dataSources,omitempty"`
	SuggestedActions []SuggestedAction `json:"suggestedActions,omitempty"`
	Topology         TopologyHighlight `json:"topology,omitempty"`
	Attachments      []AIOpsAttachment `json:"attachments,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
}

type AIOpsInspection struct {
	InspectionID string             `json:"inspectionId"`
	Name         string             `json:"name"`
	Status       string             `json:"status"`
	Progress     map[string]any     `json:"progress,omitempty"`
	Report       BusinessInspection `json:"report,omitempty"`
	CreatedAt    time.Time          `json:"createdAt"`
	UpdatedAt    time.Time          `json:"updatedAt"`
}
