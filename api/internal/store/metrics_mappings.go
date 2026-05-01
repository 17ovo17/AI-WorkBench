package store

import (
	"strings"
	"time"

	"ai-workbench-api/internal/model"
)

// SaveMetricsMapping persists a metrics mapping to memory and MySQL.
func SaveMetricsMapping(m *model.MetricsMapping) {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	mu.Lock()
	metricsMappings[m.ID] = m
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO metrics_mappings (id,datasource_id,raw_name,standard_name,exporter,description,transform,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			m.ID, m.DatasourceID, m.RawName, m.StandardName, m.Exporter,
			m.Description, m.Transform, m.Status, m.CreatedAt, m.UpdatedAt,
		)
	}
}

// GetMetricsMapping retrieves a single metrics mapping by ID.
func GetMetricsMapping(id string) (*model.MetricsMapping, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,datasource_id,raw_name,standard_name,exporter,description,transform,status,created_at,updated_at FROM metrics_mappings WHERE id=?`, id)
		var m model.MetricsMapping
		if err := row.Scan(&m.ID, &m.DatasourceID, &m.RawName, &m.StandardName, &m.Exporter, &m.Description, &m.Transform, &m.Status, &m.CreatedAt, &m.UpdatedAt); err == nil {
			return &m, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	m, ok := metricsMappings[id]
	if !ok {
		return nil, false
	}
	cp := *m
	return &cp, true
}

// UpdateMetricsMapping updates an existing mapping.
func UpdateMetricsMapping(m *model.MetricsMapping) {
	m.UpdatedAt = time.Now()
	mu.Lock()
	metricsMappings[m.ID] = m
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`UPDATE metrics_mappings SET standard_name=?,exporter=?,description=?,transform=?,status=?,updated_at=? WHERE id=?`,
			m.StandardName, m.Exporter, m.Description, m.Transform, m.Status, m.UpdatedAt, m.ID,
		)
	}
}

// ListMetricsMappings returns paginated mappings filtered by datasource and status.
func ListMetricsMappings(datasourceID, status string, page, limit int) ([]model.MetricsMapping, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if mysqlOK {
		return listMappingsMySQL(datasourceID, status, page, limit)
	}
	return listMappingsMemory(datasourceID, status, page, limit)
}

func listMappingsMySQL(datasourceID, status string, page, limit int) ([]model.MetricsMapping, int) {
	clauses := []string{}
	args := []any{}
	if datasourceID != "" {
		clauses = append(clauses, "datasource_id=?")
		args = append(args, datasourceID)
	}
	if status != "" {
		clauses = append(clauses, "status=?")
		args = append(args, status)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := db.QueryRow("SELECT COUNT(*) FROM metrics_mappings"+where, args...).Scan(&total); err != nil {
		return []model.MetricsMapping{}, 0
	}
	offset := (page - 1) * limit
	args = append(args, limit, offset)
	rows, err := db.Query("SELECT id,datasource_id,raw_name,standard_name,exporter,description,transform,status,created_at,updated_at FROM metrics_mappings"+where+" ORDER BY raw_name ASC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return []model.MetricsMapping{}, 0
	}
	defer rows.Close()
	out := []model.MetricsMapping{}
	for rows.Next() {
		var m model.MetricsMapping
		_ = rows.Scan(&m.ID, &m.DatasourceID, &m.RawName, &m.StandardName, &m.Exporter, &m.Description, &m.Transform, &m.Status, &m.CreatedAt, &m.UpdatedAt)
		out = append(out, m)
	}
	return out, total
}

func listMappingsMemory(datasourceID, status string, page, limit int) ([]model.MetricsMapping, int) {
	mu.RLock()
	defer mu.RUnlock()
	filtered := make([]model.MetricsMapping, 0, len(metricsMappings))
	for _, m := range metricsMappings {
		if datasourceID != "" && m.DatasourceID != datasourceID {
			continue
		}
		if status != "" && m.Status != status {
			continue
		}
		filtered = append(filtered, *m)
	}
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.MetricsMapping{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// FindMappingByRawName looks up a mapping by datasource_id + raw_name.
func FindMappingByRawName(datasourceID, rawName string) (*model.MetricsMapping, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,datasource_id,raw_name,standard_name,exporter,description,transform,status,created_at,updated_at FROM metrics_mappings WHERE datasource_id=? AND raw_name=?`, datasourceID, rawName)
		var m model.MetricsMapping
		if err := row.Scan(&m.ID, &m.DatasourceID, &m.RawName, &m.StandardName, &m.Exporter, &m.Description, &m.Transform, &m.Status, &m.CreatedAt, &m.UpdatedAt); err == nil {
			return &m, true
		}
		return nil, false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, m := range metricsMappings {
		if m.DatasourceID == datasourceID && m.RawName == rawName {
			cp := *m
			return &cp, true
		}
	}
	return nil, false
}

// FindMetricMappingByStandard returns the best mapped raw metric for a standard name.
func FindMetricMappingByStandard(datasourceID string, standards, available []string) (*model.MetricsMapping, bool) {
	standardSet := normalizedSet(standards)
	availableSet := normalizedSet(available)
	if len(standardSet) == 0 || len(availableSet) == 0 {
		return nil, false
	}
	if mysqlOK {
		if m, ok := findMetricMappingByStandardMySQL(datasourceID, standardSet, availableSet); ok {
			return m, true
		}
		if datasourceID != "" {
			return findMetricMappingByStandardMySQL("", standardSet, availableSet)
		}
	}
	return findMetricMappingByStandardMemory(datasourceID, standardSet, availableSet)
}

func findMetricMappingByStandardMySQL(datasourceID string, standards, available map[string]bool) (*model.MetricsMapping, bool) {
	clauses := []string{"status <> 'unmapped'", "standard_name <> ''"}
	args := []any{}
	if datasourceID != "" {
		clauses = append(clauses, "datasource_id=?")
		args = append(args, datasourceID)
	}
	placeholders := make([]string, 0, len(standards))
	for name := range standards {
		placeholders = append(placeholders, "?")
		args = append(args, name)
	}
	clauses = append(clauses, "LOWER(standard_name) IN ("+strings.Join(placeholders, ",")+")")
	query := `SELECT id,datasource_id,raw_name,standard_name,exporter,description,transform,status,created_at,updated_at FROM metrics_mappings WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY CASE status WHEN 'confirmed' THEN 0 WHEN 'custom' THEN 1 WHEN 'auto' THEN 2 ELSE 3 END, updated_at DESC LIMIT 100`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, false
	}
	defer rows.Close()
	for rows.Next() {
		var m model.MetricsMapping
		_ = rows.Scan(&m.ID, &m.DatasourceID, &m.RawName, &m.StandardName, &m.Exporter, &m.Description, &m.Transform, &m.Status, &m.CreatedAt, &m.UpdatedAt)
		if available[strings.ToLower(strings.TrimSpace(m.RawName))] {
			return &m, true
		}
	}
	return nil, false
}

func findMetricMappingByStandardMemory(datasourceID string, standards, available map[string]bool) (*model.MetricsMapping, bool) {
	mu.RLock()
	defer mu.RUnlock()
	var best *model.MetricsMapping
	for _, m := range metricsMappings {
		if datasourceID != "" && m.DatasourceID != datasourceID {
			continue
		}
		if m.Status == "unmapped" || !standards[strings.ToLower(strings.TrimSpace(m.StandardName))] {
			continue
		}
		if !available[strings.ToLower(strings.TrimSpace(m.RawName))] {
			continue
		}
		candidate := *m
		if best == nil || mappingStatusRank(candidate.Status) < mappingStatusRank(best.Status) || candidate.UpdatedAt.After(best.UpdatedAt) {
			best = &candidate
		}
	}
	return best, best != nil
}

func normalizedSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out[item] = true
		}
	}
	return out
}

func mappingStatusRank(status string) int {
	switch status {
	case "confirmed":
		return 0
	case "custom":
		return 1
	case "auto":
		return 2
	default:
		return 3
	}
}

// BulkSaveMetricsMappings persists multiple mappings, returning count of new entries.
func BulkSaveMetricsMappings(mappings []*model.MetricsMapping) int {
	added := 0
	for _, m := range mappings {
		if _, exists := FindMappingByRawName(m.DatasourceID, m.RawName); exists {
			continue
		}
		SaveMetricsMapping(m)
		added++
	}
	return added
}

// ConfirmAutoMappings batch-updates auto-status mappings to confirmed for a datasource.
func ConfirmAutoMappings(datasourceID string) int {
	updated := 0
	mu.Lock()
	for _, m := range metricsMappings {
		if m.DatasourceID == datasourceID && m.Status == "auto" {
			m.Status = "confirmed"
			m.UpdatedAt = time.Now()
			updated++
		}
	}
	mu.Unlock()
	if mysqlOK {
		res, err := db.Exec(`UPDATE metrics_mappings SET status='confirmed', updated_at=? WHERE datasource_id=? AND status='auto'`, time.Now(), datasourceID)
		if err == nil {
			if affected, err := res.RowsAffected(); err == nil {
				updated = int(affected)
			}
		}
	}
	return updated
}
