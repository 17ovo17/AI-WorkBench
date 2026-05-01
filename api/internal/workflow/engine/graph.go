package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type NodeType string

const (
	NodeStart              NodeType = "start"
	NodeEnd                NodeType = "end"
	NodeLLM                NodeType = "llm"
	NodeKnowledgeRetrieval NodeType = "knowledge_retrieval"
	NodeHTTPRequest        NodeType = "http_request"
	NodeCondition          NodeType = "condition"
	NodeCode               NodeType = "code"
	NodeLoop               NodeType = "loop"
	NodeIteration          NodeType = "iteration"
	NodeParameterExtractor NodeType = "parameter_extractor"
	NodeQuestionClassifier NodeType = "question_classifier"
	NodeTemplateTransform  NodeType = "template_transform"
	NodeVariableAggregator NodeType = "variable_aggregator"
	NodeVariableAssigner   NodeType = "variable_assigner"
	NodeTool               NodeType = "tool"
	NodeAgent              NodeType = "agent"
	NodeDocumentExtractor  NodeType = "document_extractor"
	NodeListFilter         NodeType = "list_filter"
	NodeSubWorkflow        NodeType = "sub_workflow"
	NodeHumanInput         NodeType = "human_input"
)

type Edge struct {
	SourceID     string
	TargetID     string
	SourceHandle string
}

type NodeConfig struct {
	ID            string
	Type          NodeType
	Title         string
	Data          map[string]any
	Inputs        map[string]any
	Outputs       map[string]any
	Next          string
	Branches      []BranchConfig
	Fallback      *FallbackConfig
	MaxRetries    *int
	RetryBackoff  time.Duration
	OnFailure     string
	DefaultValue  any
	ParallelGroup string // 同组节点并行执行
}

type BranchConfig struct {
	ID    string
	Rules []ConditionRule
	Logic string
	Next  string
}

type ConditionRule struct {
	Variable string
	Operator string
	Value    any
}

type FallbackConfig struct {
	OnError       string
	FallbackValue any
}

type Graph struct {
	ID          string
	Name        string
	Description string
	Version     string
	Nodes       map[string]*NodeConfig
	Edges       []Edge
	StartNodeID string
	EndNodeIDs  []string
	sorted      []string
	adjacency   map[string][]string
	inDegree    map[string]int
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func NewGraph(name string) *Graph {
	return &Graph{
		ID:        newID(),
		Name:      name,
		Nodes:     make(map[string]*NodeConfig),
		Edges:     make([]Edge, 0),
		adjacency: make(map[string][]string),
		inDegree:  make(map[string]int),
	}
}

func (g *Graph) AddNode(cfg *NodeConfig) {
	g.Nodes[cfg.ID] = cfg
	if _, exists := g.adjacency[cfg.ID]; !exists {
		g.adjacency[cfg.ID] = make([]string, 0)
	}
	if _, exists := g.inDegree[cfg.ID]; !exists {
		g.inDegree[cfg.ID] = 0
	}
	if cfg.Type == NodeStart {
		g.StartNodeID = cfg.ID
	}
	if cfg.Type == NodeEnd {
		g.EndNodeIDs = append(g.EndNodeIDs, cfg.ID)
	}
}

func (g *Graph) AddEdge(e Edge) {
	g.Edges = append(g.Edges, e)
	g.adjacency[e.SourceID] = append(g.adjacency[e.SourceID], e.TargetID)
	g.inDegree[e.TargetID]++
}

func (g *Graph) TopologicalSort() ([]string, error) {
	if len(g.sorted) > 0 {
		return g.sorted, nil
	}

	inDeg := make(map[string]int)
	for id := range g.Nodes {
		inDeg[id] = 0
	}
	for _, e := range g.Edges {
		inDeg[e.TargetID]++
	}

	queue := make([]string, 0)
	for id, deg := range inDeg {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	sorted := make([]string, 0, len(g.Nodes))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		sorted = append(sorted, current)

		for _, neighbor := range g.adjacency[current] {
			inDeg[neighbor]--
			if inDeg[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(sorted) != len(g.Nodes) {
		return nil, errors.New("graph contains a cycle")
	}

	g.sorted = sorted
	return sorted, nil
}

func (g *Graph) Successors(nodeID string) []string {
	return g.adjacency[nodeID]
}

func (g *Graph) Predecessors(nodeID string) []string {
	preds := make([]string, 0)
	for _, e := range g.Edges {
		if e.TargetID == nodeID {
			preds = append(preds, e.SourceID)
		}
	}
	return preds
}

func (g *Graph) Validate() error {
	startCount := 0
	endCount := 0
	for _, node := range g.Nodes {
		if node.Type == NodeStart {
			startCount++
		}
		if node.Type == NodeEnd {
			endCount++
		}
	}

	if startCount != 1 {
		return fmt.Errorf("graph must have exactly one start node, found %d", startCount)
	}
	if endCount < 1 {
		return errors.New("graph must have at least one end node")
	}

	reachable := make(map[string]bool)
	g.walkReachable(g.StartNodeID, reachable)
	for id := range g.Nodes {
		if !reachable[id] {
			return fmt.Errorf("node %q is not reachable from start", id)
		}
	}

	if _, err := g.TopologicalSort(); err != nil {
		return err
	}

	return nil
}

func (g *Graph) walkReachable(nodeID string, visited map[string]bool) {
	if visited[nodeID] {
		return
	}
	visited[nodeID] = true
	for _, next := range g.adjacency[nodeID] {
		g.walkReachable(next, visited)
	}
}
