package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

var changeEvents []*model.ChangeEvent

func AddChangeEvent(event *model.ChangeEvent) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	changeEvents = append(changeEvents, event)
}

func ListChangeEvents(targetIP string, since time.Time, limit int) []model.ChangeEvent {
	var result []model.ChangeEvent
	for i := len(changeEvents) - 1; i >= 0 && len(result) < limit; i-- {
		e := changeEvents[i]
		if targetIP != "" && e.TargetIP != targetIP {
			continue
		}
		if !since.IsZero() && e.StartedAt.Before(since) {
			continue
		}
		result = append(result, *e)
	}
	return result
}

func ListChangeEventsByTargets(ips []string, start, end time.Time) []model.ChangeEvent {
	ipSet := make(map[string]bool, len(ips))
	for _, ip := range ips {
		ipSet[ip] = true
	}
	var result []model.ChangeEvent
	for _, e := range changeEvents {
		if !ipSet[e.TargetIP] {
			continue
		}
		if e.StartedAt.Before(start) || e.StartedAt.After(end) {
			continue
		}
		result = append(result, *e)
	}
	return result
}
