package store

import (
	"fmt"
	"time"

	"ai-workbench-api/internal/model"
)

// --- in-memory fallback ---

var (
	workflows        = map[string]*model.Workflow{}
	workflowRuns     = map[string][]*model.WorkflowRun{} // keyed by workflow_id
	workflowVersions = map[string]map[int]*model.WorkflowVersion{}
)

// SaveWorkflow persists a workflow definition.
func SaveWorkflow(w *model.Workflow) {
	if w.Version <= 0 {
		w.Version = 1
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = time.Now()
	}
	w.UpdatedAt = time.Now()
	mu.Lock()
	workflows[w.ID] = w
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO workflows (id,name,description,dsl,builtin,version,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`,
			w.ID, w.Name, w.Description, w.DSL, w.Builtin, w.Version, w.CreatedAt, w.UpdatedAt,
		)
	}
	SaveWorkflowVersion(workflowVersionFromWorkflow(w))
}

func workflowVersionFromWorkflow(w *model.Workflow) *model.WorkflowVersion {
	return &model.WorkflowVersion{
		ID:          workflowVersionID(w.ID, w.Version),
		WorkflowID:  w.ID,
		Version:     w.Version,
		Name:        w.Name,
		Description: w.Description,
		DSL:         w.DSL,
		CreatedAt:   w.UpdatedAt,
	}
}

func workflowVersionID(workflowID string, version int) string {
	return fmt.Sprintf("%s:v%d", workflowID, version)
}

// SaveWorkflowVersion persists an immutable workflow DSL snapshot.
func SaveWorkflowVersion(v *model.WorkflowVersion) {
	if v == nil || v.WorkflowID == "" || v.Version <= 0 {
		return
	}
	if v.ID == "" {
		v.ID = workflowVersionID(v.WorkflowID, v.Version)
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	mu.Lock()
	if workflowVersions[v.WorkflowID] == nil {
		workflowVersions[v.WorkflowID] = map[int]*model.WorkflowVersion{}
	}
	cp := *v
	workflowVersions[v.WorkflowID][v.Version] = &cp
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO workflow_versions (id,workflow_id,version,name,description,dsl,created_at) VALUES (?,?,?,?,?,?,?)`,
			v.ID, v.WorkflowID, v.Version, v.Name, v.Description, v.DSL, v.CreatedAt,
		)
	}
}

// GetWorkflowVersion retrieves a workflow DSL snapshot by workflow ID and version.
func GetWorkflowVersion(workflowID string, version int) (*model.WorkflowVersion, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,workflow_id,version,name,description,dsl,created_at FROM workflow_versions WHERE workflow_id=? AND version=?`,
			workflowID, version,
		)
		var v model.WorkflowVersion
		if err := row.Scan(&v.ID, &v.WorkflowID, &v.Version, &v.Name, &v.Description, &v.DSL, &v.CreatedAt); err == nil {
			return &v, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	versions := workflowVersions[workflowID]
	if versions == nil {
		return nil, false
	}
	v, ok := versions[version]
	if !ok {
		return nil, false
	}
	cp := *v
	return &cp, true
}

// GetWorkflow retrieves a workflow by ID.
func GetWorkflow(id string) (*model.Workflow, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,name,description,dsl,builtin,version,created_at,updated_at FROM workflows WHERE id=?`, id)
		var w model.Workflow
		if err := row.Scan(&w.ID, &w.Name, &w.Description, &w.DSL, &w.Builtin, &w.Version, &w.CreatedAt, &w.UpdatedAt); err == nil {
			return &w, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	w, ok := workflows[id]
	if !ok {
		return nil, false
	}
	cp := *w
	return &cp, true
}

// ListWorkflows returns all custom workflows from storage.
func ListWorkflows() []model.Workflow {
	if mysqlOK {
		rows, err := db.Query(
			`SELECT id,name,description,dsl,builtin,version,created_at,updated_at FROM workflows ORDER BY created_at DESC`)
		if err == nil {
			defer rows.Close()
			out := []model.Workflow{}
			for rows.Next() {
				var w model.Workflow
				_ = rows.Scan(&w.ID, &w.Name, &w.Description, &w.DSL, &w.Builtin, &w.Version, &w.CreatedAt, &w.UpdatedAt)
				out = append(out, w)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]model.Workflow, 0, len(workflows))
	for _, w := range workflows {
		out = append(out, *w)
	}
	return out
}

// DeleteWorkflow removes a workflow by ID.
func DeleteWorkflow(id string) {
	mu.Lock()
	delete(workflows, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM workflows WHERE id=?`, id)
	}
}

// SaveWorkflowRun persists a workflow execution record.
func SaveWorkflowRun(r *model.WorkflowRun) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	mu.Lock()
	workflowRuns[r.WorkflowID] = append(workflowRuns[r.WorkflowID], r)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO workflow_runs (id,workflow_id,workflow_version,status,inputs,outputs,error_message,elapsed_ms,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			r.ID, r.WorkflowID, r.WorkflowVersion, r.Status, r.Inputs, r.Outputs, r.ErrorMessage, r.ElapsedMs, r.CreatedAt,
		)
	}
}

// ListWorkflowRuns returns execution history for a workflow.
func ListWorkflowRuns(workflowID string, page, limit int) ([]model.WorkflowRun, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listWorkflowRunsMySQL(workflowID, page, limit)
	}
	return listWorkflowRunsMemory(workflowID, page, limit)
}

func listWorkflowRunsMySQL(wfID string, page, limit int) ([]model.WorkflowRun, int) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM workflow_runs WHERE workflow_id=?`, wfID).Scan(&total); err != nil {
		return []model.WorkflowRun{}, 0
	}
	offset := (page - 1) * limit
	rows, err := db.Query(
		`SELECT id,workflow_id,workflow_version,status,inputs,outputs,error_message,elapsed_ms,created_at FROM workflow_runs WHERE workflow_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		wfID, limit, offset)
	if err != nil {
		return []model.WorkflowRun{}, 0
	}
	defer rows.Close()
	out := []model.WorkflowRun{}
	for rows.Next() {
		var r model.WorkflowRun
		_ = rows.Scan(&r.ID, &r.WorkflowID, &r.WorkflowVersion, &r.Status, &r.Inputs, &r.Outputs, &r.ErrorMessage, &r.ElapsedMs, &r.CreatedAt)
		out = append(out, r)
	}
	return out, total
}

func listWorkflowRunsMemory(wfID string, page, limit int) ([]model.WorkflowRun, int) {
	mu.RLock()
	defer mu.RUnlock()
	runs := workflowRuns[wfID]
	total := len(runs)
	start := (page - 1) * limit
	if start >= total {
		return []model.WorkflowRun{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	// return newest first
	out := make([]model.WorkflowRun, 0, end-start)
	for i := total - 1 - start; i >= total-end && i >= 0; i-- {
		out = append(out, *runs[i])
	}
	return out, total
}
