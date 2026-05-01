package correlator

import (
	"fmt"
	"sync"
	"time"

	"ai-workbench-api/internal/store"
)

type CorrelatedContext struct {
	TimeWindow  [2]string            `json:"time_window"`
	TargetIP    string               `json:"target_ip"`
	Alerts      []map[string]string  `json:"alerts"`
	Diagnoses   []map[string]string  `json:"diagnoses"`
	Changes     []map[string]string  `json:"changes"`
	AuditEvents []map[string]string  `json:"audit_events"`
	Summary     string               `json:"summary"`
}

func Correlate(targetIP string, window time.Duration) *CorrelatedContext {
	now := time.Now()
	start := now.Add(-window)
	result := &CorrelatedContext{
		TimeWindow: [2]string{start.Format(time.RFC3339), now.Format(time.RFC3339)},
		TargetIP:   targetIP,
	}

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		for _, a := range store.ListAlerts() {
			if a.TargetIP == targetIP && a.CreateTime.After(start) {
				result.Alerts = append(result.Alerts, map[string]string{
					"id": a.ID, "title": a.Title, "severity": a.Severity,
					"status": a.Status, "time": a.CreateTime.Format(time.RFC3339),
				})
			}
		}
	}()

	go func() {
		defer wg.Done()
		for _, r := range store.ListRecords() {
			if r.TargetIP == targetIP && r.CreateTime.After(start) {
				result.Diagnoses = append(result.Diagnoses, map[string]string{
					"id": r.ID, "status": string(r.Status), "trigger": r.Trigger,
					"time": r.CreateTime.Format(time.RFC3339),
				})
			}
		}
	}()

	go func() {
		defer wg.Done()
		for _, c := range store.ListChangeEvents(targetIP, start, 20) {
			result.Changes = append(result.Changes, map[string]string{
				"title": c.Title, "type": c.ChangeType, "operator": c.Operator,
				"time": c.StartedAt.Format(time.RFC3339),
			})
		}
	}()

	go func() {
		defer wg.Done()
		for _, e := range store.ListAuditEvents(50) {
			if e.CreatedAt.After(start) {
				result.AuditEvents = append(result.AuditEvents, map[string]string{
					"action": e.Action, "target": e.Target,
					"time": e.CreatedAt.Format(time.RFC3339),
				})
			}
		}
	}()

	wg.Wait()
	result.Summary = fmt.Sprintf("主机 %s 时间窗口内：%d 告警、%d 诊断、%d 变更、%d 审计",
		targetIP, len(result.Alerts), len(result.Diagnoses), len(result.Changes), len(result.AuditEvents))
	return result
}
