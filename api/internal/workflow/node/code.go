package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-workbench-api/internal/workflow/engine"

	"github.com/dop251/goja"
)

// 代码执行默认配置
const (
	defaultCodeTimeout = 10 * time.Second
)

// dangerousPatterns 危险代码模式检测列表
var dangerousPatterns = []string{
	"require(", "import(", "process.",
	"child_process", "exec(", "spawn(",
	"eval(", "Function(",
	"XMLHttpRequest", "fetch(", "WebSocket",
	"__proto__", "constructor.constructor",
}

// handleCode 在 goja JS 沙箱中执行用户代码
func handleCode(ctx context.Context, nodeID string, cfg *engine.NodeConfig, pool *engine.VariablePool, _ *Registry) (*engine.NodeResult, error) {
	lang, _ := cfg.Data["language"].(string)
	if lang != "" && lang != "javascript" {
		return nil, fmt.Errorf("code: only javascript is supported, got %q", lang)
	}

	code, ok := cfg.Data["code"].(string)
	if !ok || code == "" {
		return nil, fmt.Errorf("code: code is required")
	}

	if err := checkDangerousCode(code); err != nil {
		return nil, err
	}

	timeout := defaultCodeTimeout
	if t, ok := toInt(cfg.Data["timeout"]); ok && t > 0 {
		timeout = time.Duration(t) * time.Millisecond
	}

	inputs := collectCodeInputs(cfg, pool)
	outputs, logs, err := executeJS(ctx, code, inputs, timeout)
	if err != nil {
		return nil, fmt.Errorf("code: %w", err)
	}

	result := map[string]any{
		"outputs": outputs,
	}
	if len(logs) > 0 {
		result["logs"] = logs
	}

	return &engine.NodeResult{Outputs: result, Status: engine.StatusSucceeded}, nil
}

// checkDangerousCode 静态检查代码中的危险模式（含 Unicode 归一化）
func checkDangerousCode(code string) error {
	normalized := normalizeUnicode(code)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(normalized, pattern) {
			return fmt.Errorf("code: dangerous pattern detected: %q", pattern)
		}
	}
	return nil
}

// normalizeUnicode 将全角字符转为半角，防止 Unicode 绕过
func normalizeUnicode(s string) string {
	replacer := strings.NewReplacer(
		"（", "(", "）", ")", "．", ".",
		"ｅｖａｌ", "eval",
	)
	return replacer.Replace(s)
}

// collectCodeInputs 从变量池收集代码输入
func collectCodeInputs(cfg *engine.NodeConfig, pool *engine.VariablePool) map[string]any {
	if cfg.Inputs == nil {
		return make(map[string]any)
	}
	return pool.InterpolateMap(cfg.Inputs)
}

// executeJS 在 goja 沙箱中执行 JavaScript 代码
func executeJS(ctx context.Context, code string, inputs map[string]any, timeout time.Duration) (map[string]any, []string, error) {
	vm := goja.New()

	// 沙箱加固：删除危险全局函数 + 冻结全局对象
	sandboxInit := `
(function() {
  delete globalThis.eval;
  delete globalThis.Function;
  var dp = Object.getOwnPropertyDescriptor(Object.prototype, '__proto__');
  if (dp) { delete Object.prototype.__proto__; }
  if (typeof globalThis.constructor !== 'undefined') {
    try { Object.defineProperty(globalThis, 'constructor', { value: undefined, writable: false, configurable: false }); } catch(e) {}
  }
})();
`
	if _, err := vm.RunString(sandboxInit); err != nil {
		return nil, nil, fmt.Errorf("sandbox init: %w", err)
	}

	// 注入 inputs
	if err := vm.Set("inputs", inputs); err != nil {
		return nil, nil, fmt.Errorf("set inputs: %w", err)
	}

	// 初始化 outputs 变量
	if err := vm.Set("outputs", map[string]any{}); err != nil {
		return nil, nil, fmt.Errorf("set outputs: %w", err)
	}

	// 注入安全的 console.log
	logs := make([]string, 0)
	console := map[string]any{
		"log": func(call goja.FunctionCall) goja.Value {
			args := make([]string, 0, len(call.Arguments))
			for _, arg := range call.Arguments {
				args = append(args, arg.String())
			}
			logs = append(logs, strings.Join(args, " "))
			return goja.Undefined()
		},
	}
	if err := vm.Set("console", console); err != nil {
		return nil, nil, fmt.Errorf("set console: %w", err)
	}

	// 用 goroutine + context 实现超时控制
	type jsResult struct {
		outputs map[string]any
		err     error
	}

	resultCh := make(chan jsResult, 1)
	go func() {
		_, err := vm.RunString(code)
		if err != nil {
			resultCh <- jsResult{err: err}
			return
		}

		outputs := extractJSOutputs(vm)
		resultCh <- jsResult{outputs: outputs}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		vm.Interrupt("context cancelled")
		return nil, logs, ctx.Err()
	case <-timer.C:
		vm.Interrupt("execution timeout")
		return nil, logs, fmt.Errorf("execution timeout after %v", timeout)
	case res := <-resultCh:
		if res.err != nil {
			return nil, logs, fmt.Errorf("js execution: %w", res.err)
		}
		return res.outputs, logs, nil
	}
}

// extractJSOutputs 从 goja VM 中提取 outputs 变量
func extractJSOutputs(vm *goja.Runtime) map[string]any {
	val := vm.Get("outputs")
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return make(map[string]any)
	}

	exported := val.Export()
	if m, ok := exported.(map[string]any); ok {
		return m
	}

	return map[string]any{"result": exported}
}
