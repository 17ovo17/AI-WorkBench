package store

import (
	"sort"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
)

// SaveCase persists a diagnosis case to memory and MySQL.
func SaveCase(c *model.DiagnosisCase) {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	mu.Lock()
	diagnosisCases[c.ID] = c
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO diagnosis_cases (id,metric_snapshot,root_cause_category,root_cause_description,treatment_steps,keywords,source_diagnosis_id,created_at,created_by,evaluation_avg,avg_feedback_rating) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			c.ID, c.MetricSnapshot, c.RootCauseCategory, c.RootCauseDescription,
			c.TreatmentSteps, c.Keywords, c.SourceDiagnosisID,
			c.CreatedAt, c.CreatedBy, c.EvaluationAvg, c.AvgFeedbackRating,
		)
	}
}

// GetCase retrieves a single diagnosis case by ID.
func GetCase(id string) (*model.DiagnosisCase, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,metric_snapshot,root_cause_category,root_cause_description,treatment_steps,keywords,source_diagnosis_id,created_at,created_by,evaluation_avg,COALESCE(avg_feedback_rating,0) FROM diagnosis_cases WHERE id=?`, id)
		var c model.DiagnosisCase
		if err := row.Scan(&c.ID, &c.MetricSnapshot, &c.RootCauseCategory, &c.RootCauseDescription, &c.TreatmentSteps, &c.Keywords, &c.SourceDiagnosisID, &c.CreatedAt, &c.CreatedBy, &c.EvaluationAvg, &c.AvgFeedbackRating); err == nil {
			if caseHasUnrenderedTemplate(c) {
				return nil, false
			}
			return &c, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	c, ok := diagnosisCases[id]
	if !ok {
		return nil, false
	}
	cp := *c
	if caseHasUnrenderedTemplate(cp) {
		return nil, false
	}
	return &cp, true
}

// UpdateCase updates an existing diagnosis case.
func UpdateCase(c *model.DiagnosisCase) {
	mu.Lock()
	diagnosisCases[c.ID] = c
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`UPDATE diagnosis_cases SET metric_snapshot=?,root_cause_category=?,root_cause_description=?,treatment_steps=?,keywords=?,source_diagnosis_id=?,created_by=?,evaluation_avg=?,avg_feedback_rating=? WHERE id=?`,
			c.MetricSnapshot, c.RootCauseCategory, c.RootCauseDescription,
			c.TreatmentSteps, c.Keywords, c.SourceDiagnosisID,
			c.CreatedBy, c.EvaluationAvg, c.AvgFeedbackRating, c.ID,
		)
	}
}

// DeleteCase removes a diagnosis case by ID.
func DeleteCase(id string) {
	mu.Lock()
	delete(diagnosisCases, id)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM diagnosis_cases WHERE id=?`, id)
	}
}

// ListCases returns paginated diagnosis cases with optional keyword and category filter.
func ListCases(page, limit int, keyword, category string) ([]model.DiagnosisCase, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listCasesMySQL(page, limit, keyword, category)
	}
	return listCasesMemory(page, limit, keyword, category)
}

func listCasesMySQL(page, limit int, keyword, category string) ([]model.DiagnosisCase, int) {
	where, args := buildCaseWhere(keyword, category)
	querySQL := "SELECT id,metric_snapshot,root_cause_category,root_cause_description,treatment_steps,keywords,source_diagnosis_id,created_at,created_by,evaluation_avg,COALESCE(avg_feedback_rating,0) FROM diagnosis_cases" + where + " ORDER BY avg_feedback_rating DESC,created_at DESC LIMIT 10000"
	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return []model.DiagnosisCase{}, 0
	}
	defer rows.Close()
	out := []model.DiagnosisCase{}
	for rows.Next() {
		var c model.DiagnosisCase
		_ = rows.Scan(&c.ID, &c.MetricSnapshot, &c.RootCauseCategory, &c.RootCauseDescription, &c.TreatmentSteps, &c.Keywords, &c.SourceDiagnosisID, &c.CreatedAt, &c.CreatedBy, &c.EvaluationAvg, &c.AvgFeedbackRating)
		if !caseHasUnrenderedTemplate(c) {
			out = append(out, c)
		}
	}
	out = dedupeCases(out)
	total := len(out)
	start := (page - 1) * limit
	if start >= total {
		return []model.DiagnosisCase{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return out[start:end], total
}

func buildCaseWhere(keyword, category string) (string, []any) {
	clauses := []string{}
	args := []any{}
	if keyword != "" {
		clauses = append(clauses, "MATCH(keywords,root_cause_description) AGAINST(? IN BOOLEAN MODE)")
		args = append(args, keyword)
	}
	if category != "" {
		clauses = append(clauses, "root_cause_category=?")
		args = append(args, category)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func listCasesMemory(page, limit int, keyword, category string) ([]model.DiagnosisCase, int) {
	mu.RLock()
	defer mu.RUnlock()
	filtered := make([]model.DiagnosisCase, 0, len(diagnosisCases))
	kw := strings.ToLower(keyword)
	for _, c := range diagnosisCases {
		if caseHasUnrenderedTemplate(*c) {
			continue
		}
		if category != "" && c.RootCauseCategory != category {
			continue
		}
		if kw != "" && !caseMatchesKeyword(c, kw) {
			continue
		}
		filtered = append(filtered, *c)
	}
	sort.SliceStable(filtered, func(i, j int) bool { return caseSortLess(filtered[i], filtered[j]) })
	filtered = dedupeCases(filtered)
	total := len(filtered)
	start := (page - 1) * limit
	if start >= total {
		return []model.DiagnosisCase{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

func caseHasUnrenderedTemplate(c model.DiagnosisCase) bool {
	return containsTemplateMarker(c.RootCauseCategory) ||
		containsTemplateMarker(c.RootCauseDescription) ||
		containsTemplateMarker(c.Keywords) ||
		containsTemplateMarker(c.TreatmentSteps) ||
		containsTemplateMarker(string(c.MetricSnapshot))
}

func containsTemplateMarker(value string) bool {
	text := strings.TrimSpace(value)
	return strings.Contains(text, "{{") ||
		strings.Contains(text, "}}") ||
		strings.Contains(text, "parameter_extractor.") ||
		strings.Contains(text, "llm_diagnosis.")
}

func dedupeCases(items []model.DiagnosisCase) []model.DiagnosisCase {
	sort.SliceStable(items, func(i, j int) bool { return caseSortLess(items[i], items[j]) })
	seen := map[string]bool{}
	out := make([]model.DiagnosisCase, 0, len(items))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.SourceDiagnosisID))
		if key == "" {
			key = strings.ToLower(strings.Join([]string{
				strings.TrimSpace(item.RootCauseCategory),
				strings.TrimSpace(item.RootCauseDescription),
				strings.TrimSpace(item.Keywords),
			}, "|"))
		}
		if key != "" && seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func caseSortLess(left, right model.DiagnosisCase) bool {
	if left.AvgFeedbackRating != right.AvgFeedbackRating {
		return left.AvgFeedbackRating > right.AvgFeedbackRating
	}
	return left.CreatedAt.After(right.CreatedAt)
}

func RefreshCaseFeedbackRating(diagnosisID string) {
	if diagnosisID == "" {
		return
	}
	avg, ok := averageFeedbackScore(diagnosisID)
	if !ok {
		return
	}
	mu.Lock()
	for _, c := range diagnosisCases {
		if c.SourceDiagnosisID == diagnosisID {
			c.AvgFeedbackRating = avg
		}
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`UPDATE diagnosis_cases SET avg_feedback_rating=? WHERE source_diagnosis_id=?`, avg, diagnosisID)
	}
}

func averageFeedbackScore(diagnosisID string) (float64, bool) {
	feedbacks := ListFeedbacksByDiagnosis(diagnosisID)
	if len(feedbacks) == 0 {
		return 0, false
	}
	total := 0.0
	for _, f := range feedbacks {
		total += feedbackScore(f.Rating)
	}
	return total / float64(len(feedbacks)), true
}

func feedbackScore(rating string) float64 {
	switch rating {
	case "accurate":
		return 1
	case "partial":
		return 0.5
	default:
		return 0
	}
}

func caseMatchesKeyword(c *model.DiagnosisCase, kw string) bool {
	return strings.Contains(strings.ToLower(c.Keywords), kw) ||
		strings.Contains(strings.ToLower(c.RootCauseDescription), kw)
}

// GetCaseByDiagnosisID 通过 source_diagnosis_id 查找案例。
func GetCaseByDiagnosisID(diagnosisID string) (*model.DiagnosisCase, bool) {
	if diagnosisID == "" {
		return nil, false
	}
	if mysqlOK {
		row := db.QueryRow(`SELECT id,metric_snapshot,root_cause_category,root_cause_description,treatment_steps,keywords,source_diagnosis_id,created_at,created_by,evaluation_avg,COALESCE(avg_feedback_rating,0) FROM diagnosis_cases WHERE source_diagnosis_id=? LIMIT 1`, diagnosisID)
		var c model.DiagnosisCase
		if err := row.Scan(&c.ID, &c.MetricSnapshot, &c.RootCauseCategory, &c.RootCauseDescription, &c.TreatmentSteps, &c.Keywords, &c.SourceDiagnosisID, &c.CreatedAt, &c.CreatedBy, &c.EvaluationAvg, &c.AvgFeedbackRating); err == nil {
			if caseHasUnrenderedTemplate(c) {
				return nil, false
			}
			return &c, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, c := range diagnosisCases {
		if c.SourceDiagnosisID == diagnosisID {
			cp := *c
			if caseHasUnrenderedTemplate(cp) {
				return nil, false
			}
			return &cp, true
		}
	}
	return nil, false
}
