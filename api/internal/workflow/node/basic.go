package node

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"ai-workbench-api/internal/workflow/engine"
)

// handleStart merges YAML defaults with user-provided inputs (user inputs take priority)
func handleStart(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	existing := pool.GetAll(nodeID)
	outputs := make(map[string]any)
	if cfg.Inputs != nil {
		for k, v := range cfg.Inputs {
			outputs[k] = v
		}
	}
	for k, v := range existing {
		if v != nil && v != "" {
			outputs[k] = v
		}
	}
	return &engine.NodeResult{
		Outputs: outputs,
		Status:  engine.StatusSucceeded,
	}, nil
}

// handleEnd 从变量池收集 config.Outputs 中定义的字段，组装最终结果
func handleEnd(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	outputs := make(map[string]any)
	if cfg.Outputs != nil {
		interpolated := pool.InterpolateMap(cfg.Outputs)
		for k, v := range interpolated {
			outputs[k] = v
		}
	}
	return &engine.NodeResult{
		Outputs: outputs,
		Status:  engine.StatusSucceeded,
	}, nil
}

// handleCondition 遍历 branches，对每个 branch 的 rules 求值
func handleCondition(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	var defaultBranch *engine.BranchConfig

	for i := range cfg.Branches {
		branch := &cfg.Branches[i]
		if branch.Logic == "default" {
			defaultBranch = branch
			continue
		}
		if evaluateBranch(branch, pool) {
			return &engine.NodeResult{
				NextID: branch.Next,
				Status: engine.StatusSucceeded,
			}, nil
		}
	}

	if defaultBranch != nil {
		return &engine.NodeResult{
			NextID: defaultBranch.Next,
			Status: engine.StatusSucceeded,
		}, nil
	}

	return &engine.NodeResult{Status: engine.StatusSkipped}, nil
}

// evaluateBranch 根据 logic 模式（and/or）评估分支规则
func evaluateBranch(branch *engine.BranchConfig, pool *engine.VariablePool) bool {
	if len(branch.Rules) == 0 {
		return false
	}
	if branch.Logic == "or" {
		for _, rule := range branch.Rules {
			if evaluateRule(&rule, pool) {
				return true
			}
		}
		return false
	}
	// 默认 and 逻辑
	for _, rule := range branch.Rules {
		if !evaluateRule(&rule, pool) {
			return false
		}
	}
	return true
}

// evaluateRule 对单条规则求值
func evaluateRule(rule *engine.ConditionRule, pool *engine.VariablePool) bool {
	actual := pool.Interpolate(rule.Variable)
	expected := fmt.Sprintf("%v", rule.Value)

	switch rule.Operator {
	case "equals", "eq", "==":
		return actual == expected
	case "not_equals", "neq", "!=":
		return actual != expected
	case "gt", ">":
		return compareNumeric(actual, expected) > 0
	case "lt", "<":
		return compareNumeric(actual, expected) < 0
	case "gte", ">=":
		return compareNumeric(actual, expected) >= 0
	case "lte", "<=":
		return compareNumeric(actual, expected) <= 0
	case "contains":
		return strings.Contains(actual, expected)
	case "not_contains":
		return !strings.Contains(actual, expected)
	case "empty":
		return actual == "" || actual == rule.Variable
	case "not_empty":
		return actual != "" && actual != rule.Variable
	case "regex":
		matched, err := regexp.MatchString(expected, actual)
		return err == nil && matched
	case "starts_with":
		return strings.HasPrefix(actual, expected)
	case "ends_with":
		return strings.HasSuffix(actual, expected)
	case "in":
		for _, v := range strings.Split(expected, ",") {
			if strings.TrimSpace(v) == actual {
				return true
			}
		}
		return false
	case "not_in":
		for _, v := range strings.Split(expected, ",") {
			if strings.TrimSpace(v) == actual {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// compareNumeric 将两个字符串解析为浮点数进行比较
func compareNumeric(a, b string) int {
	fa, errA := strconv.ParseFloat(a, 64)
	fb, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return strings.Compare(a, b)
	}
	switch {
	case fa < fb:
		return -1
	case fa > fb:
		return 1
	default:
		return 0
	}
}

// handleVariableAggregator 从多个节点收集变量合并到一个 map
func handleVariableAggregator(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	outputs := make(map[string]any)

	sources, ok := cfg.Data["sources"]
	if !ok {
		return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
	}

	sourceList, ok := sources.([]any)
	if !ok {
		return nil, fmt.Errorf("variable_aggregator: sources must be an array")
	}

	for _, src := range sourceList {
		ref, ok := src.(string)
		if !ok {
			continue
		}
		parts := strings.SplitN(ref, ".", 2)
		if len(parts) == 2 {
			val, found := pool.Get(parts[0], parts[1])
			if found {
				outputs[ref] = val
			}
		} else if len(parts) == 1 {
			all := pool.GetAll(parts[0])
			if all != nil {
				outputs[parts[0]] = all
			}
		}
	}

	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// handleVariableAssigner 从赋值列表读取并写入变量池
func handleVariableAssigner(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	assignments, ok := cfg.Data["assignments"]
	if !ok {
		return &engine.NodeResult{Status: engine.StatusSucceeded}, nil
	}

	assignList, ok := assignments.([]any)
	if !ok {
		return nil, fmt.Errorf("variable_assigner: assignments must be an array")
	}

	outputs := make(map[string]any)
	for _, item := range assignList {
		assign, ok := item.(map[string]any)
		if !ok {
			continue
		}
		target, _ := assign["target"].(string)
		value := assign["value"]

		// 对字符串值做变量池插值
		if strVal, ok := value.(string); ok {
			value = pool.Interpolate(strVal)
		}

		parts := strings.SplitN(target, ".", 2)
		if len(parts) == 2 {
			pool.Set(parts[0], parts[1], value)
		}
		outputs[target] = value
	}

	return &engine.NodeResult{Outputs: outputs, Status: engine.StatusSucceeded}, nil
}

// handleDocumentExtractor 文档提取节点（占位实现）
func handleDocumentExtractor(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	return &engine.NodeResult{
		Outputs: map[string]any{"text": "document extraction not yet implemented"},
		Status:  engine.StatusSucceeded,
	}, nil
}
