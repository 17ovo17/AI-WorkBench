package store

import (
	"sync/atomic"
	"time"

	"ai-workbench-api/internal/model"
)

var (
	promptTemplates = map[string]*model.PromptTemplate{}
	promptIDSeq     int64
)

// SavePromptTemplate 保存 Prompt 模板到内存和 MySQL。
func SavePromptTemplate(t *model.PromptTemplate) {
	now := time.Now()
	if t.ID == 0 {
		t.ID = atomic.AddInt64(&promptIDSeq, 1)
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.Version == 0 {
		t.Version = 1
	}
	t.UpdatedAt = now
	mu.Lock()
	promptTemplates[t.Name] = t
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO prompt_templates (id,name,category,template,version,variables,description,is_active,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			t.ID, t.Name, t.Category, t.Template, t.Version,
			t.Variables, t.Description, t.IsActive, t.CreatedAt, t.UpdatedAt,
		)
	}
}

// GetPromptTemplate 通过 name 查询单条 Prompt 模板。
func GetPromptTemplate(name string) (*model.PromptTemplate, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,name,COALESCE(category,''),template,version,COALESCE(variables,''),COALESCE(description,''),is_active,created_at,updated_at FROM prompt_templates WHERE name=?`, name)
		var t model.PromptTemplate
		if err := row.Scan(&t.ID, &t.Name, &t.Category, &t.Template, &t.Version, &t.Variables, &t.Description, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err == nil {
			return &t, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	t, ok := promptTemplates[name]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// ListPromptTemplates 列出 Prompt 模板，可按 category 筛选。
func ListPromptTemplates(category string) []model.PromptTemplate {
	if mysqlOK {
		return listPromptTemplatesMySQL(category)
	}
	return listPromptTemplatesMemory(category)
}

func listPromptTemplatesMySQL(category string) []model.PromptTemplate {
	query := `SELECT id,name,COALESCE(category,''),template,version,COALESCE(variables,''),COALESCE(description,''),is_active,created_at,updated_at FROM prompt_templates`
	args := []any{}
	if category != "" {
		query += " WHERE category=?"
		args = append(args, category)
	}
	query += " ORDER BY updated_at DESC"
	rows, err := db.Query(query, args...)
	if err != nil {
		return []model.PromptTemplate{}
	}
	defer rows.Close()
	out := []model.PromptTemplate{}
	for rows.Next() {
		var t model.PromptTemplate
		_ = rows.Scan(&t.ID, &t.Name, &t.Category, &t.Template, &t.Version, &t.Variables, &t.Description, &t.IsActive, &t.CreatedAt, &t.UpdatedAt)
		out = append(out, t)
	}
	return out
}

func listPromptTemplatesMemory(category string) []model.PromptTemplate {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]model.PromptTemplate, 0, len(promptTemplates))
	for _, t := range promptTemplates {
		if category != "" && t.Category != category {
			continue
		}
		out = append(out, *t)
	}
	return out
}

// DeletePromptTemplate 通过 name 删除 Prompt 模板。
func DeletePromptTemplate(name string) {
	mu.Lock()
	delete(promptTemplates, name)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM prompt_templates WHERE name=?`, name)
	}
}
