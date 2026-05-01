package node

import (
	"context"
	"fmt"

	"ai-workbench-api/internal/workflow/engine"
)

// handleSubWorkflow 执行子工作流，通过 Registry 中注册的 WorkflowRunner 调用
func handleSubWorkflow(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, reg *Registry) (*engine.NodeResult, error) {
	wfName, _ := cfg.Data["workflow_name"].(string)
	if wfName == "" {
		return nil, fmt.Errorf("sub_workflow: workflow_name required")
	}

	subInputs := buildSubInputs(cfg, pool)

	runner := reg.GetWorkflowRunner()
	if runner == nil {
		return nil, fmt.Errorf("sub_workflow: workflow runner not registered")
	}

	result, err := runner(ctx, wfName, subInputs)
	if err != nil {
		return nil, fmt.Errorf("sub_workflow %s: %w", wfName, err)
	}

	outputs := mapSubOutputs(cfg, result)
	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// buildSubInputs 根据 input_mapping 构建子工作流输入
func buildSubInputs(cfg *engine.NodeConfig, pool *engine.VariablePool) map[string]any {
	subInputs := make(map[string]any)
	mapping, ok := cfg.Data["input_mapping"].(map[string]any)
	if !ok {
		return subInputs
	}
	for subKey, poolRef := range mapping {
		if ref, ok := poolRef.(string); ok {
			subInputs[subKey] = pool.Interpolate(ref)
		}
	}
	return subInputs
}

// mapSubOutputs 根据 output_mapping 映射子工作流输出到变量池
func mapSubOutputs(cfg *engine.NodeConfig, result map[string]any) map[string]any {
	outMapping, ok := cfg.Data["output_mapping"].(map[string]any)
	if !ok || result == nil {
		if result != nil {
			return result
		}
		return make(map[string]any)
	}
	outputs := make(map[string]any)
	for poolKey, subKey := range outMapping {
		if key, ok := subKey.(string); ok {
			outputs[poolKey] = result[key]
		}
	}
	return outputs
}
