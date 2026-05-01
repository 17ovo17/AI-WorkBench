package reasoning

import (
	"context"
	"fmt"

	"ai-workbench-api/internal/store"
)

type ToolFunc func(ctx context.Context, params map[string]string) Evidence

type ToolRegistry struct {
	tools map[string]ToolFunc
}

func NewToolRegistry() *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]ToolFunc)}
	r.registerDefaults()
	return r
}

func (r *ToolRegistry) Register(name string, fn ToolFunc) {
	r.tools[name] = fn
}

func (r *ToolRegistry) Execute(ctx context.Context, name string, params map[string]string) Evidence {
	fn, ok := r.tools[name]
	if !ok {
		return Evidence{Source: name, Summary: fmt.Sprintf("tool %s not registered", name)}
	}
	return fn(ctx, params)
}

func (r *ToolRegistry) registerDefaults() {
	r.Register("prometheus_query", toolPrometheus)
	r.Register("check_alerts", toolAlerts)
	r.Register("check_diagnose_history", toolDiagnoseHistory)
	r.Register("knowledge_search", toolKnowledgeSearch)
}

func toolPrometheus(_ context.Context, params map[string]string) Evidence {
	ip := params["ip"]
	return Evidence{
		Source:  "prometheus",
		Data:    map[string]string{"ip": ip, "note": "metrics via workflow engine"},
		Summary: fmt.Sprintf("主机 %s 指标数据（通过工作流引擎采集）", ip),
	}
}

func toolAlerts(_ context.Context, params map[string]string) Evidence {
	ip := params["ip"]
	alerts := store.ListAlerts()
	var matched []map[string]string
	for _, a := range alerts {
		if a.TargetIP == ip && a.Status == "firing" {
			matched = append(matched, map[string]string{
				"title": a.Title, "severity": a.Severity, "status": a.Status,
			})
		}
	}
	return Evidence{
		Source:  "alerts",
		Data:    matched,
		Summary: fmt.Sprintf("主机 %s 活跃告警 %d 条", ip, len(matched)),
	}
}

func toolDiagnoseHistory(_ context.Context, params map[string]string) Evidence {
	ip := params["ip"]
	records := store.ListRecords()
	var matched []map[string]string
	count := 0
	for _, r := range records {
		if r.TargetIP == ip && count < 5 {
			matched = append(matched, map[string]string{
				"id": r.ID, "status": string(r.Status), "trigger": r.Trigger,
			})
			count++
		}
	}
	return Evidence{
		Source:  "diagnose_history",
		Data:    matched,
		Summary: fmt.Sprintf("主机 %s 最近 %d 条诊断记录", ip, len(matched)),
	}
}

func toolKnowledgeSearch(_ context.Context, params map[string]string) Evidence {
	query := params["query"]
	if query == "" {
		query = params["ip"]
	}
	docs, _ := store.ListDocuments(1, 3, "", "", query)
	var titles []string
	for _, d := range docs {
		titles = append(titles, d.Title)
	}
	return Evidence{
		Source:  "knowledge",
		Data:    titles,
		Summary: fmt.Sprintf("知识库搜索 '%s' 命中 %d 条", query, len(titles)),
	}
}
