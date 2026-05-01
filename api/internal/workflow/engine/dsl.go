package engine

import (
	"embed"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

type WorkflowDSL struct {
	Workflow WorkflowSpec `yaml:"workflow"`
}

type WorkflowSpec struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Version     string       `yaml:"version"`
	Nodes       []NodeDSL    `yaml:"nodes"`
	Global      GlobalConfig `yaml:"global"`
}

type NodeDSL struct {
	ID            string         `yaml:"id"`
	Type          string         `yaml:"type"`
	Title         string         `yaml:"title"`
	Description   string         `yaml:"description"`
	Config        map[string]any `yaml:"config"`
	Inputs        map[string]any `yaml:"inputs"`
	Outputs       map[string]any `yaml:"outputs"`
	Next          string         `yaml:"next"`
	Conditions    []BranchDSL    `yaml:"conditions"`
	Fallback      *FallbackDSL   `yaml:"fallback"`
	Transform     map[string]any `yaml:"transform"`
	ParallelGroup string         `yaml:"parallel_group"`
	MaxRetries    *int           `yaml:"max_retries"`
	RetryBackoff  int            `yaml:"retry_backoff"`
	OnFailure     string         `yaml:"on_failure"`
	DefaultValue  any            `yaml:"default_value"`
}

type BranchDSL struct {
	ID    string         `yaml:"id"`
	Logic string         `yaml:"logic"`
	Rules []ConditionDSL `yaml:"rules"`
	Next  string         `yaml:"next"`
}

type ConditionDSL struct {
	Variable string `yaml:"variable"`
	Operator string `yaml:"operator"`
	Value    any    `yaml:"value"`
}

type FallbackDSL struct {
	OnError       string `yaml:"on_error"`
	OnEmpty       string `yaml:"on_empty"`
	FallbackValue any    `yaml:"fallback_value"`
}

type GlobalConfig struct {
	ErrorHandling string `yaml:"error_handling"`
	Timeout       int    `yaml:"timeout"`
	NodeTimeout   int    `yaml:"node_timeout"`
	Retry         struct {
		MaxAttempts int `yaml:"max_attempts"`
		Backoff     int `yaml:"backoff"`
	} `yaml:"retry"`
}

func ParseDSL(data []byte) (*Graph, *EngineConfig, error) {
	var dsl WorkflowDSL
	if err := yaml.Unmarshal(data, &dsl); err != nil {
		return nil, nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	spec := dsl.Workflow
	graph := NewGraph(spec.Name)
	graph.Description = spec.Description
	graph.Version = spec.Version

	for _, n := range spec.Nodes {
		cfg := nodeFromDSL(n)
		graph.AddNode(cfg)
	}

	buildEdges(graph, spec.Nodes)

	config := configFromGlobal(spec.Global)
	return graph, config, nil
}

func LoadBuiltinWorkflow(name string) (*Graph, *EngineConfig, error) {
	path := fmt.Sprintf("builtin/%s.yaml", name)
	data, err := builtinFS.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("load builtin workflow %q: %w", name, err)
	}
	return ParseDSL(data)
}

func nodeFromDSL(n NodeDSL) *NodeConfig {
	cfg := &NodeConfig{
		ID:            n.ID,
		Type:          NodeType(n.Type),
		Title:         n.Title,
		Data:          n.Config,
		Inputs:        n.Inputs,
		Outputs:       n.Outputs,
		Next:          n.Next,
		ParallelGroup: n.ParallelGroup,
		MaxRetries:    n.MaxRetries,
		OnFailure:     n.OnFailure,
		DefaultValue:  n.DefaultValue,
	}
	if n.RetryBackoff > 0 {
		cfg.RetryBackoff = time.Duration(n.RetryBackoff) * time.Millisecond
	}

	for _, cond := range n.Conditions {
		branch := BranchConfig{
			ID:    cond.ID,
			Logic: cond.Logic,
			Next:  cond.Next,
		}
		for _, r := range cond.Rules {
			branch.Rules = append(branch.Rules, ConditionRule{
				Variable: r.Variable,
				Operator: r.Operator,
				Value:    r.Value,
			})
		}
		cfg.Branches = append(cfg.Branches, branch)
	}

	if n.Fallback != nil {
		cfg.Fallback = &FallbackConfig{
			OnError:       n.Fallback.OnError,
			FallbackValue: n.Fallback.FallbackValue,
		}
	}

	return cfg
}

func buildEdges(graph *Graph, nodes []NodeDSL) {
	groups := map[string][]string{}
	for _, n := range nodes {
		if n.ParallelGroup != "" {
			groups[n.ParallelGroup] = append(groups[n.ParallelGroup], n.ID)
		}
	}

	for _, n := range nodes {
		if n.Next != "" {
			graph.AddEdge(Edge{SourceID: n.ID, TargetID: n.Next})
			if targetCfg := graph.Nodes[n.Next]; targetCfg != nil && targetCfg.ParallelGroup != "" {
				for _, peerID := range groups[targetCfg.ParallelGroup] {
					if peerID != n.Next {
						graph.AddEdge(Edge{SourceID: n.ID, TargetID: peerID})
					}
				}
			}
		}
		for _, cond := range n.Conditions {
			if cond.Next != "" {
				graph.AddEdge(Edge{SourceID: n.ID, TargetID: cond.Next, SourceHandle: cond.ID})
			}
		}
	}
}

func configFromGlobal(g GlobalConfig) *EngineConfig {
	cfg := DefaultConfig()

	if g.ErrorHandling != "" {
		cfg.ErrorHandling = g.ErrorHandling
	}
	if g.Timeout > 0 {
		cfg.Timeout = time.Duration(g.Timeout) * time.Millisecond
	}
	if g.NodeTimeout > 0 {
		cfg.NodeTimeout = time.Duration(g.NodeTimeout) * time.Millisecond
	}
	if g.Retry.MaxAttempts > 0 {
		cfg.MaxRetries = g.Retry.MaxAttempts - 1
	}
	if g.Retry.Backoff > 0 {
		cfg.RetryBackoff = time.Duration(g.Retry.Backoff) * time.Millisecond
	}

	return &cfg
}
