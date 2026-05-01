package store

import (
	"sort"
	"time"

	"ai-workbench-api/internal/model"
)

// --- in-memory fallback ---

var workflowRunSteps = map[string][]*model.WorkflowRunStep{} // keyed by run_id

// SaveRunStep 保存节点执行记录
func SaveRunStep(step *model.WorkflowRunStep) {
	if step.StartedAt == nil {
		now := time.Now()
		step.StartedAt = &now
	}
	mu.Lock()
	workflowRunSteps[step.RunID] = append(workflowRunSteps[step.RunID], step)
	mu.Unlock()
	if mysqlOK {
		saveRunStepMySQL(step)
	}
}

// GetRunStep 获取指定 run 中某个节点的执行记录
func GetRunStep(runID, nodeID string) (*model.WorkflowRunStep, bool) {
	if mysqlOK {
		return getRunStepMySQL(runID, nodeID)
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, s := range workflowRunSteps[runID] {
		if s.NodeID == nodeID {
			cp := *s
			return &cp, true
		}
	}
	return nil, false
}

// ListRunSteps 列出某次执行的所有节点记录
func ListRunSteps(runID string) []model.WorkflowRunStep {
	if mysqlOK {
		return listRunStepsMySQL(runID)
	}
	mu.RLock()
	defer mu.RUnlock()
	steps := workflowRunSteps[runID]
	out := make([]model.WorkflowRunStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// --- MySQL 实现 ---

func saveRunStepMySQL(s *model.WorkflowRunStep) {
	_, _ = db.Exec(
		`INSERT INTO workflow_run_steps (run_id,node_id,node_type,status,inputs,outputs,error,started_at,finished_at,elapsed_ms,retry_count) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		s.RunID, s.NodeID, s.NodeType, s.Status,
		s.Inputs, s.Outputs, s.Error,
		nullableTime(s.StartedAt), nullableTime(s.FinishedAt),
		s.ElapsedMs, s.RetryCount,
	)
}

func getRunStepMySQL(runID, nodeID string) (*model.WorkflowRunStep, bool) {
	row := db.QueryRow(
		`SELECT id,run_id,node_id,node_type,status,inputs,outputs,`+"`error`"+`,started_at,finished_at,elapsed_ms,retry_count FROM workflow_run_steps WHERE run_id=? AND node_id=? ORDER BY id DESC LIMIT 1`,
		runID, nodeID,
	)
	var s model.WorkflowRunStep
	if err := row.Scan(&s.ID, &s.RunID, &s.NodeID, &s.NodeType, &s.Status, &s.Inputs, &s.Outputs, &s.Error, &s.StartedAt, &s.FinishedAt, &s.ElapsedMs, &s.RetryCount); err != nil {
		return nil, false
	}
	return &s, true
}

func listRunStepsMySQL(runID string) []model.WorkflowRunStep {
	rows, err := db.Query(
		`SELECT id,run_id,node_id,node_type,status,inputs,outputs,`+"`error`"+`,started_at,finished_at,elapsed_ms,retry_count FROM workflow_run_steps WHERE run_id=? ORDER BY id ASC`,
		runID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []model.WorkflowRunStep
	for rows.Next() {
		var s model.WorkflowRunStep
		_ = rows.Scan(&s.ID, &s.RunID, &s.NodeID, &s.NodeType, &s.Status, &s.Inputs, &s.Outputs, &s.Error, &s.StartedAt, &s.FinishedAt, &s.ElapsedMs, &s.RetryCount)
		out = append(out, s)
	}
	return out
}
