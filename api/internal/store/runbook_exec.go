package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

// SaveRunbookExecution 保存一条 Runbook 执行记录。
func SaveRunbookExecution(exec *model.RunbookExecution) {
	if exec.StartedAt.IsZero() {
		exec.StartedAt = time.Now()
	}
	mu.Lock()
	runbookExecs[exec.ID] = exec
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`INSERT INTO runbook_executions (id,runbook_id,target_ip,executor,status,variables,output,error_message,started_at,finished_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			exec.ID, exec.RunbookID, exec.TargetIP, exec.Executor,
			exec.Status, exec.Variables, exec.Output, exec.ErrorMessage,
			exec.StartedAt, nullableTime(exec.FinishedAt),
		)
	}
}

// GetRunbookExecution 通过 ID 查询单条执行记录。
func GetRunbookExecution(id string) (*model.RunbookExecution, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,runbook_id,COALESCE(target_ip,''),COALESCE(executor,''),status,COALESCE(variables,''),COALESCE(output,''),COALESCE(error_message,''),started_at,finished_at FROM runbook_executions WHERE id=?`, id,
		)
		var e model.RunbookExecution
		if err := row.Scan(&e.ID, &e.RunbookID, &e.TargetIP, &e.Executor, &e.Status, &e.Variables, &e.Output, &e.ErrorMessage, &e.StartedAt, &e.FinishedAt); err == nil {
			return &e, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	e, ok := runbookExecs[id]
	if !ok {
		return nil, false
	}
	cp := *e
	return &cp, true
}

// ListRunbookExecutions 分页列出指定 Runbook 的执行记录。
func ListRunbookExecutions(runbookID string, page, limit int) ([]model.RunbookExecution, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listRunbookExecsMySQL(runbookID, page, limit)
	}
	return listRunbookExecsMemory(runbookID, page, limit)
}

func listRunbookExecsMySQL(runbookID string, page, limit int) ([]model.RunbookExecution, int) {
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM runbook_executions WHERE runbook_id=?", runbookID).Scan(&total); err != nil {
		return []model.RunbookExecution{}, 0
	}
	offset := (page - 1) * limit
	rows, err := db.Query(
		`SELECT id,runbook_id,COALESCE(target_ip,''),COALESCE(executor,''),status,COALESCE(variables,''),COALESCE(output,''),COALESCE(error_message,''),started_at,finished_at FROM runbook_executions WHERE runbook_id=? ORDER BY started_at DESC LIMIT ? OFFSET ?`,
		runbookID, limit, offset,
	)
	if err != nil {
		return []model.RunbookExecution{}, 0
	}
	defer rows.Close()
	out := []model.RunbookExecution{}
	for rows.Next() {
		var e model.RunbookExecution
		_ = rows.Scan(&e.ID, &e.RunbookID, &e.TargetIP, &e.Executor, &e.Status, &e.Variables, &e.Output, &e.ErrorMessage, &e.StartedAt, &e.FinishedAt)
		out = append(out, e)
	}
	return out, total
}

func listRunbookExecsMemory(runbookID string, page, limit int) ([]model.RunbookExecution, int) {
	mu.RLock()
	defer mu.RUnlock()
	filtered := make([]model.RunbookExecution, 0)
	for _, e := range runbookExecs {
		if e.RunbookID == runbookID {
			filtered = append(filtered, *e)
		}
	}
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.RunbookExecution{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// UpdateRunbookExecution 通过回调函数更新执行记录。
func UpdateRunbookExecution(id string, fn func(*model.RunbookExecution)) {
	mu.Lock()
	if e, ok := runbookExecs[id]; ok {
		fn(e)
	}
	mu.Unlock()
	if mysqlOK {
		e, ok := GetRunbookExecution(id)
		if !ok {
			return
		}
		fn(e)
		_, _ = db.Exec(
			`UPDATE runbook_executions SET status=?,output=?,error_message=?,finished_at=? WHERE id=?`,
			e.Status, e.Output, e.ErrorMessage, nullableTime(e.FinishedAt), id,
		)
	}
}
