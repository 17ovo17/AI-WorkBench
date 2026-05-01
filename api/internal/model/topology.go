package model

import "time"

type TopologyNode struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	IP          string    `json:"ip,omitempty"`
	ServiceName string    `json:"service_name,omitempty"`
	Port        int       `json:"port,omitempty"`
	Status      string    `json:"status"`
	Layer       int       `json:"layer,omitempty"`
	AgentRole   string    `json:"agent_role,omitempty"`
	X           float64   `json:"x"`
	Y           float64   `json:"y"`
	Meta        string    `json:"meta,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TopologyEdge struct {
	ID        string    `json:"id"`
	SourceID  string    `json:"source_id"`
	TargetID  string    `json:"target_id"`
	Protocol  string    `json:"protocol,omitempty"`
	Direction string    `json:"direction,omitempty"`
	Label     string    `json:"label,omitempty"`
	Status    string    `json:"status,omitempty"`
	LatencyMs int       `json:"latency_ms,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TopologyGraph struct {
	Nodes     []TopologyNode     `json:"nodes"`
	Edges     []TopologyEdge     `json:"edges"`
	Discovery *TopologyDiscovery `json:"discovery,omitempty"`
}

type TopologyDiscovery struct {
	Planner       string   `json:"planner"`
	Status        string   `json:"status"`
	Summary       string   `json:"summary"`
	DataSources   []string `json:"data_sources"`
	ScopeHosts    []string `json:"scope_hosts"`
	BusinessChain []string `json:"business_chain"`
	Notes         []string `json:"notes,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type TopologyEndpoint struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	ServiceName string `json:"service_name,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
}

type TopologyBusiness struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Hosts      []string           `json:"hosts"`
	Endpoints  []TopologyEndpoint `json:"endpoints,omitempty"`
	Attributes map[string]string  `json:"attributes,omitempty"`
	Graph      TopologyGraph      `json:"graph"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

type AITopologyService struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	Role string `json:"role"`
}

type AITopologyHealth struct {
	Score  int    `json:"score"`
	Status string `json:"status"`
}

type AITopologyMetrics struct {
	CPU  float64 `json:"cpu"`
	Mem  float64 `json:"mem"`
	Disk float64 `json:"disk"`
	Load float64 `json:"load"`
}

type AITopologyNode struct {
	ID       string              `json:"id"`
	IP       string              `json:"ip"`
	Hostname string              `json:"hostname"`
	Layer    string              `json:"layer"`
	Services []AITopologyService `json:"services"`
	Health   AITopologyHealth    `json:"health"`
	Metrics  AITopologyMetrics   `json:"metrics"`
	Alerts   []string            `json:"alerts,omitempty"`
}

type AITopologyLink struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type"`
	Label    string `json:"label"`
	Dashed   bool   `json:"dashed,omitempty"`
	Relation string `json:"relation,omitempty"`
}

type AITopologyRisk struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Nodes       []string `json:"nodes,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
}

type AITopologySummary struct {
	ServiceName        string         `json:"service_name,omitempty"`
	Planner            string         `json:"planner"`
	NodeCount          int            `json:"node_count"`
	LinkCount          int            `json:"link_count"`
	LayerCounts        map[string]int `json:"layer_counts"`
	HealthDistribution map[string]int `json:"health_distribution"`
	CriticalPath       []string       `json:"critical_path,omitempty"`
	Error              string         `json:"error,omitempty"`
}

type AITopologyGraph struct {
	Nodes   []AITopologyNode  `json:"nodes"`
	Links   []AITopologyLink  `json:"links"`
	Risks   []AITopologyRisk  `json:"risks,omitempty"`
	Summary AITopologySummary `json:"summary"`
}

type BusinessMetricSample struct {
	IP     string  `json:"ip"`
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit,omitempty"`
	Status string  `json:"status"`
	Source string  `json:"source"`
	Query  string  `json:"query,omitempty"`
	Detail string  `json:"detail,omitempty"`
}

type BusinessProcess struct {
	IP          string `json:"ip"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
	Port        int    `json:"port,omitempty"`
	Layer       string `json:"layer"`
	Status      string `json:"status"`
	Alert       string `json:"alert,omitempty"`
}

type BusinessResource struct {
	IP      string            `json:"ip"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Owner   string            `json:"owner,omitempty"`
	Purpose string            `json:"purpose,omitempty"`
	Status  string            `json:"status"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

type BusinessInspection struct {
	BusinessID        string                 `json:"business_id"`
	BusinessName      string                 `json:"business_name"`
	Status            string                 `json:"status"`
	Score             int                    `json:"score"`
	Summary           string                 `json:"summary"`
	GeneratedAt       time.Time              `json:"generated_at"`
	Attributes        map[string]string      `json:"attributes,omitempty"`
	Metrics           []BusinessMetricSample `json:"metrics"`
	Processes         []BusinessProcess      `json:"processes"`
	Resources         []BusinessResource     `json:"resources"`
	Alerts            []*AlertRecord         `json:"alerts"`
	TopologyFindings  []string               `json:"topology_findings"`
	Recommendations   []string               `json:"recommendations"`
	AISuggestions     []string               `json:"ai_suggestions,omitempty"`
	DataSources       []string               `json:"data_sources"`
	Planner           string                 `json:"planner,omitempty"`
	AIAnalysis        string                 `json:"ai_analysis,omitempty"`
	AIError           string                 `json:"ai_error,omitempty"`
	ExecutiveSummary  string                 `json:"executive_summary,omitempty"`
	RiskLevel         string                 `json:"risk_level,omitempty"`
	TopFindings       []string               `json:"top_findings,omitempty"`
	AIRecommendations []string               `json:"ai_recommendations,omitempty"`
	EvidenceRefs      []string               `json:"evidence_refs,omitempty"`
	DiagnoseRecordID  string                 `json:"diagnose_record_id,omitempty"`
}
