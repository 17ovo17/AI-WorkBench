package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

// SaveRunbook 持久化运维手册到内存和 MySQL。
func SaveRunbook(r *model.Runbook) {
	now := time.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	if r.Version == 0 {
		r.Version = 1
	}
	if r.Severity == "" {
		r.Severity = "medium"
	}
	mu.Lock()
	runbooks[r.ID] = r
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO runbooks (id,title,category,trigger_conditions,steps,auto_executable,created_at,updated_at,version,severity,estimated_time,prerequisites,rollback_steps,variables,last_executed_at,execution_count,success_rate) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			r.ID, r.Title, r.Category, r.TriggerConditions, r.Steps,
			boolToInt(r.AutoExecutable), r.CreatedAt, r.UpdatedAt,
			r.Version, r.Severity, r.EstimatedTime, r.Prerequisites,
			r.RollbackSteps, r.Variables, nullableTime(r.LastExecutedAt),
			r.ExecutionCount, r.SuccessRate,
		)
	}
}

// GetRunbook 通过 ID 查询单条 Runbook。
func GetRunbook(id string) (*model.Runbook, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,title,category,COALESCE(trigger_conditions,'{}'),steps,auto_executable,created_at,updated_at,COALESCE(version,1),COALESCE(severity,'medium'),COALESCE(estimated_time,''),COALESCE(prerequisites,''),COALESCE(rollback_steps,''),COALESCE(variables,'{}'),last_executed_at,COALESCE(execution_count,0),COALESCE(success_rate,0) FROM runbooks WHERE id=?`, id)
		var r model.Runbook
		var auto int
		if err := row.Scan(&r.ID, &r.Title, &r.Category, &r.TriggerConditions, &r.Steps, &auto, &r.CreatedAt, &r.UpdatedAt, &r.Version, &r.Severity, &r.EstimatedTime, &r.Prerequisites, &r.RollbackSteps, &r.Variables, &r.LastExecutedAt, &r.ExecutionCount, &r.SuccessRate); err == nil {
			r.AutoExecutable = auto != 0
			return &r, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	r, ok := runbooks[id]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// UpdateRunbook 更新现有 Runbook。
func UpdateRunbook(r *model.Runbook) {
	r.UpdatedAt = time.Now()
	r.Version++
	mu.Lock()
	runbooks[r.ID] = r
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`UPDATE runbooks SET title=?,category=?,trigger_conditions=?,steps=?,auto_executable=?,updated_at=?,version=?,severity=?,estimated_time=?,prerequisites=?,rollback_steps=?,variables=? WHERE id=?`,
			r.Title, r.Category, r.TriggerConditions, r.Steps,
			boolToInt(r.AutoExecutable), r.UpdatedAt,
			r.Version, r.Severity, r.EstimatedTime, r.Prerequisites,
			r.RollbackSteps, r.Variables, r.ID,
		)
	}
}

// DeleteRunbook 通过 ID 删除 Runbook。
func DeleteRunbook(id string) {
	mu.Lock()
	delete(runbooks, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM runbooks WHERE id=?`, id)
	}
}

// ListRunbooks 分页列出 Runbook，可按 category 筛选。
func ListRunbooks(category string, page, limit int) ([]model.Runbook, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listRunbooksMySQL(category, page, limit)
	}
	return listRunbooksMemory(category, page, limit)
}

func listRunbooksMySQL(category string, page, limit int) ([]model.Runbook, int) {
	where := ""
	args := []any{}
	if category != "" {
		where = " WHERE category=?"
		args = append(args, category)
	}
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM runbooks"+where, args...).Scan(&total); err != nil {
		return []model.Runbook{}, 0
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := db.Query("SELECT id,title,category,COALESCE(trigger_conditions,'{}'),steps,auto_executable,created_at,updated_at,COALESCE(version,1),COALESCE(severity,'medium'),COALESCE(estimated_time,''),COALESCE(prerequisites,''),COALESCE(rollback_steps,''),COALESCE(variables,'{}'),last_executed_at,COALESCE(execution_count,0),COALESCE(success_rate,0) FROM runbooks"+where+" ORDER BY updated_at DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return []model.Runbook{}, 0
	}
	defer rows.Close()
	out := []model.Runbook{}
	for rows.Next() {
		var r model.Runbook
		var auto int
		_ = rows.Scan(&r.ID, &r.Title, &r.Category, &r.TriggerConditions, &r.Steps, &auto, &r.CreatedAt, &r.UpdatedAt, &r.Version, &r.Severity, &r.EstimatedTime, &r.Prerequisites, &r.RollbackSteps, &r.Variables, &r.LastExecutedAt, &r.ExecutionCount, &r.SuccessRate)
		r.AutoExecutable = auto != 0
		out = append(out, r)
	}
	return out, total
}

func listRunbooksMemory(category string, page, limit int) ([]model.Runbook, int) {
	mu.RLock()
	defer mu.RUnlock()
	filtered := make([]model.Runbook, 0, len(runbooks))
	for _, r := range runbooks {
		if category != "" && r.Category != category {
			continue
		}
		filtered = append(filtered, *r)
	}
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.Runbook{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// IncrementRunbookExecCount 递增执行计数并更新成功率。
func IncrementRunbookExecCount(id string, success bool) {
	now := time.Now()
	mu.Lock()
	if r, ok := runbooks[id]; ok {
		r.ExecutionCount++
		r.LastExecutedAt = &now
		if success {
			total := float64(r.ExecutionCount)
			r.SuccessRate = (r.SuccessRate*float64(r.ExecutionCount-1) + 100) / total
		} else {
			total := float64(r.ExecutionCount)
			r.SuccessRate = (r.SuccessRate * float64(r.ExecutionCount-1)) / total
		}
	}
	mu.Unlock()
	if mysqlOK {
		if success {
			_, _ = db.Exec(
				`UPDATE runbooks SET execution_count=execution_count+1, last_executed_at=?, success_rate=((COALESCE(success_rate,0)*COALESCE(execution_count,0)+100)/(execution_count+1)) WHERE id=?`,
				now, id,
			)
		} else {
			_, _ = db.Exec(
				`UPDATE runbooks SET execution_count=execution_count+1, last_executed_at=?, success_rate=((COALESCE(success_rate,0)*COALESCE(execution_count,0))/(execution_count+1)) WHERE id=?`,
				now, id,
			)
		}
	}
}
