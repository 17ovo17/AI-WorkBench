package scheduler

import (
	"context"
	"sync"
	"time"

	"ai-workbench-api/internal/workflow"

	log "github.com/sirupsen/logrus"
)

// WorkflowSchedule 定义一个工作流定时调度任务。
type WorkflowSchedule struct {
	ID           string         `json:"id"`
	WorkflowName string         `json:"workflow_name"`
	CronExpr     string         `json:"cron_expr"`
	Inputs       map[string]any `json:"inputs"`
	Enabled      bool           `json:"enabled"`
	LastRun      *time.Time     `json:"last_run"`
}

var (
	schedules   []WorkflowSchedule
	schedulesMu sync.RWMutex
)

// AddSchedule 添加一个定时调度。
func AddSchedule(s WorkflowSchedule) {
	schedulesMu.Lock()
	defer schedulesMu.Unlock()
	schedules = append(schedules, s)
}

// ListSchedules 返回所有定时调度。
func ListSchedules() []WorkflowSchedule {
	schedulesMu.RLock()
	defer schedulesMu.RUnlock()
	out := make([]WorkflowSchedule, len(schedules))
	copy(out, schedules)
	return out
}

// RemoveSchedule 按 ID 删除一个定时调度。
func RemoveSchedule(id string) bool {
	schedulesMu.Lock()
	defer schedulesMu.Unlock()
	for i, s := range schedules {
		if s.ID == id {
			schedules = append(schedules[:i], schedules[i+1:]...)
			return true
		}
	}
	return false
}

// StartWorkflowScheduler 启动工作流定时调度器（60 秒轮询）。
func StartWorkflowScheduler() {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			checkAndRunSchedules()
		}
	}()
	log.Info("scheduler: workflow scheduler started (60s interval)")
}

const minRunInterval = 59 * time.Second

func checkAndRunSchedules() {
	now := time.Now()
	schedulesMu.Lock()
	defer schedulesMu.Unlock()

	for i := range schedules {
		s := &schedules[i]
		if !s.Enabled {
			continue
		}
		expr, err := ParseCron(s.CronExpr)
		if err != nil {
			log.WithError(err).Warnf("scheduler: invalid cron for %s", s.WorkflowName)
			continue
		}
		if !expr.Matches(now) {
			continue
		}
		if s.LastRun != nil && now.Sub(*s.LastRun) < minRunInterval {
			continue
		}
		t := now
		s.LastRun = &t
		go runScheduledWorkflow(s.WorkflowName, s.Inputs)
	}
}

const scheduledWorkflowTimeout = 10 * time.Minute

func runScheduledWorkflow(name string, inputs map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), scheduledWorkflowTimeout)
	defer cancel()
	_, err := workflow.RunWorkflow(ctx, name, inputs)
	if err != nil {
		log.WithError(err).Errorf("scheduler: workflow %s failed", name)
	} else {
		log.Infof("scheduler: workflow %s completed", name)
	}
}
