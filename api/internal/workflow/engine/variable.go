package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var templatePattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)
var exactTemplatePattern = regexp.MustCompile(`^\s*\{\{([^}]+)\}\}\s*$`)

type VariablePool struct {
	mu   sync.RWMutex
	vars map[string]map[string]any
	sys  map[string]any
}

func NewVariablePool() *VariablePool {
	return &VariablePool{
		vars: make(map[string]map[string]any),
		sys:  make(map[string]any),
	}
}

func (vp *VariablePool) Set(nodeID, field string, val any) {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	if vp.vars[nodeID] == nil {
		vp.vars[nodeID] = make(map[string]any)
	}
	vp.vars[nodeID][field] = val
}

func (vp *VariablePool) SetAll(nodeID string, outputs map[string]any) {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	if vp.vars[nodeID] == nil {
		vp.vars[nodeID] = make(map[string]any)
	}
	for k, v := range outputs {
		vp.vars[nodeID][k] = v
	}
}

func (vp *VariablePool) Get(nodeID, field string) (any, bool) {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	nodeVars, ok := vp.vars[nodeID]
	if !ok {
		return nil, false
	}
	val, ok := nodeVars[field]
	return val, ok
}

func (vp *VariablePool) GetAll(nodeID string) map[string]any {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	nodeVars, ok := vp.vars[nodeID]
	if !ok {
		return nil
	}
	cp := make(map[string]any, len(nodeVars))
	for k, v := range nodeVars {
		cp[k] = v
	}
	return cp
}

func (vp *VariablePool) SetSystem(key string, val any) {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	vp.sys[key] = val
}

func (vp *VariablePool) GetSystem(key string) (any, bool) {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	val, ok := vp.sys[key]
	return val, ok
}

func (vp *VariablePool) Interpolate(template string) string {
	vp.mu.RLock()
	defer vp.mu.RUnlock()

	return templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		path := strings.TrimSpace(match[2 : len(match)-2])
		if shouldLeaveGoTemplateExpression(path) {
			return match
		}
		val, ok := vp.resolveTemplatePath(path)
		if !ok || val == nil {
			return ""
		}
		return anyToString(val)
	})
}

func (vp *VariablePool) InterpolateMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = vp.interpolateValue(v)
	}
	return result
}

func (vp *VariablePool) Snapshot() map[string]map[string]any {
	vp.mu.RLock()
	defer vp.mu.RUnlock()
	snap := make(map[string]map[string]any, len(vp.vars))
	for nodeID, fields := range vp.vars {
		cp := make(map[string]any, len(fields))
		for k, v := range fields {
			cp[k] = v
		}
		snap[nodeID] = cp
	}
	return snap
}

func (vp *VariablePool) resolve(scope, path string) any {
	if scope == "sys" {
		return vp.resolveNested(vp.sys, path)
	}
	nodeVars, ok := vp.vars[scope]
	if !ok {
		return nil
	}
	return vp.resolveNested(nodeVars, path)
}

func (vp *VariablePool) resolveBare(key string) any {
	if key == "" {
		return nil
	}
	if val, ok := vp.sys[key]; ok {
		return val
	}
	var found any
	count := 0
	for _, fields := range vp.vars {
		if val, ok := fields[key]; ok {
			found = val
			count++
		}
	}
	if count == 1 {
		return found
	}
	return nil
}

func (vp *VariablePool) resolveTemplatePath(path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" || shouldLeaveGoTemplateExpression(path) {
		return nil, false
	}
	parts := strings.SplitN(path, ".", 2)
	if len(parts) < 2 {
		if val := vp.resolveBare(path); val != nil {
			return val, true
		}
		return nil, false
	}
	val := vp.resolve(parts[0], parts[1])
	if val == nil {
		return nil, false
	}
	return val, true
}

func shouldLeaveGoTemplateExpression(path string) bool {
	if path == "" || strings.HasPrefix(path, ".") || strings.Contains(path, " ") {
		return true
	}
	switch path {
	case "else", "end":
		return true
	}
	for _, prefix := range []string{"if ", "range ", "with ", "template ", "block "} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func (vp *VariablePool) resolveNested(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data

	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]any:
			val, ok := typed[part]
			if !ok {
				return nil
			}
			current = val
		default:
			return nil
		}
	}
	return current
}

func (vp *VariablePool) interpolateValue(v any) any {
	switch typed := v.(type) {
	case string:
		if matches := exactTemplatePattern.FindStringSubmatch(typed); len(matches) == 2 {
			path := strings.TrimSpace(matches[1])
			vp.mu.RLock()
			val, ok := vp.resolveTemplatePath(path)
			vp.mu.RUnlock()
			if ok {
				return val
			}
			if shouldLeaveGoTemplateExpression(path) {
				return typed
			}
			return ""
		}
		return vp.Interpolate(typed)
	case map[string]any:
		return vp.InterpolateMap(typed)
	case []any:
		result := make([]any, len(typed))
		for i, item := range typed {
			result[i] = vp.interpolateValue(item)
		}
		return result
	default:
		return v
	}
}

// GetEventEmitter 返回事件发射器（占位，由 Engine 层注入）
func (vp *VariablePool) GetEventEmitter() *EventEmitter { return nil }

// GetInputChannel 返回指定节点的用户输入通道（占位，由 Engine 层注入）
func (vp *VariablePool) GetInputChannel(nodeID string) <-chan any { return nil }

func anyToString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
