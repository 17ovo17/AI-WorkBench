package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

// AddFeedback 添加诊断反馈到内存和 MySQL。
func AddFeedback(f *model.DiagnosisFeedback) {
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	mu.Lock()
	diagnosisFeedbacks = append([]*model.DiagnosisFeedback{f}, diagnosisFeedbacks...)
	if len(diagnosisFeedbacks) > 1000 {
		diagnosisFeedbacks = diagnosisFeedbacks[:1000]
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`INSERT INTO diagnosis_feedback (id,diagnosis_id,user,rating,comment,created_at) VALUES (?,?,?,?,?,?)`,
			f.ID, f.DiagnosisID, f.User, f.Rating, f.Comment, f.CreatedAt,
		)
	}
	RefreshCaseFeedbackRating(f.DiagnosisID)
}

// ListFeedbacksByDiagnosis 列出某条诊断的所有反馈。
func ListFeedbacksByDiagnosis(diagnosisID string) []model.DiagnosisFeedback {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,diagnosis_id,user,rating,COALESCE(comment,''),created_at FROM diagnosis_feedback WHERE diagnosis_id=? ORDER BY created_at DESC`, diagnosisID)
		if err == nil {
			defer rows.Close()
			out := []model.DiagnosisFeedback{}
			for rows.Next() {
				var f model.DiagnosisFeedback
				_ = rows.Scan(&f.ID, &f.DiagnosisID, &f.User, &f.Rating, &f.Comment, &f.CreatedAt)
				out = append(out, f)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := []model.DiagnosisFeedback{}
	for _, f := range diagnosisFeedbacks {
		if f.DiagnosisID == diagnosisID {
			out = append(out, *f)
		}
	}
	return out
}

// ListAllFeedbacks 分页列出所有反馈。
func ListAllFeedbacks(page, limit int) ([]model.DiagnosisFeedback, int) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		var total int
		_ = db.QueryRow(`SELECT COUNT(*) FROM diagnosis_feedback`).Scan(&total)
		offset := (page - 1) * limit
		rows, err := db.Query(`SELECT id,diagnosis_id,user,rating,COALESCE(comment,''),created_at FROM diagnosis_feedback ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
		if err == nil {
			defer rows.Close()
			out := []model.DiagnosisFeedback{}
			for rows.Next() {
				var f model.DiagnosisFeedback
				_ = rows.Scan(&f.ID, &f.DiagnosisID, &f.User, &f.Rating, &f.Comment, &f.CreatedAt)
				out = append(out, f)
			}
			return out, total
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	total := len(diagnosisFeedbacks)
	start := (page - 1) * limit
	if start >= total {
		return []model.DiagnosisFeedback{}, total
	}
	end := start + limit
	if end > total {
		end = total
	}
	out := make([]model.DiagnosisFeedback, end-start)
	for i := start; i < end; i++ {
		out[i-start] = *diagnosisFeedbacks[i]
	}
	return out, total
}
