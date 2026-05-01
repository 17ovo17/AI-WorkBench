package workflow

import (
	"context"
	"fmt"

	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow/engine"
)

// RunWorkflowVersion executes a custom workflow by an immutable stored version.
func RunWorkflowVersion(ctx context.Context, id string, version int, inputs map[string]any) (*engine.WorkflowResult, error) {
	graph, cfg, err := loadWorkflowGraphVersion(id, version)
	if err != nil {
		return nil, err
	}
	runner := NewDefaultRegistry()
	eng := engine.NewEngine(graph, runner, *cfg)
	return eng.Run(ctx, inputs)
}

func loadWorkflowGraphVersion(id string, version int) (*engine.Graph, *engine.EngineConfig, error) {
	v, ok := store.GetWorkflowVersion(id, version)
	if !ok {
		return nil, nil, fmt.Errorf("workflow %q version %d not found", id, version)
	}
	graph, cfg, err := engine.ParseDSL([]byte(v.DSL))
	if err != nil {
		return nil, nil, fmt.Errorf("parse workflow %q version %d: %w", id, version, err)
	}
	return graph, cfg, nil
}
