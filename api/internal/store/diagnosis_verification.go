package store

import (
	"sync/atomic"
	"time"

	"ai-workbench-api/internal/model"
)

var (
	diagVerifications = map[string]*model.DiagnosisVerification{} // keyed by diagnosis_id
	verifIDSeq        int64
)

// SaveVerification 保存诊断验证结果到内存和 MySQL。
func SaveVerification(v *model.DiagnosisVerification) {
	if v.ID == 0 {
		v.ID = atomic.AddInt64(&verifIDSeq, 1)
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	mu.Lock()
	diagVerifications[v.DiagnosisID] = v
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`REPLACE INTO diagnosis_verifications (id,diagnosis_id,alert_resolution_rate,recurrence_count,runbook_executed,runbook_success,score,verified_at,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			v.ID, v.DiagnosisID, v.AlertResolutionRate, v.RecurrenceCount,
			v.RunbookExecuted, v.RunbookSuccess, v.Score, v.VerifiedAt, v.CreatedAt,
		)
	}
}

// GetVerification 通过 diagnosis_id 查询验证结果。
func GetVerification(diagnosisID string) (*model.DiagnosisVerification, bool) {
	if mysqlOK {
		row := db.QueryRow(
			`SELECT id,diagnosis_id,COALESCE(alert_resolution_rate,0),COALESCE(recurrence_count,0),COALESCE(runbook_executed,0),COALESCE(runbook_success,0),COALESCE(score,0),verified_at,created_at FROM diagnosis_verifications WHERE diagnosis_id=?`, diagnosisID)
		var v model.DiagnosisVerification
		if err := row.Scan(&v.ID, &v.DiagnosisID, &v.AlertResolutionRate, &v.RecurrenceCount, &v.RunbookExecuted, &v.RunbookSuccess, &v.Score, &v.VerifiedAt, &v.CreatedAt); err == nil {
			return &v, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	v, ok := diagVerifications[diagnosisID]
	if !ok {
		return nil, false
	}
	cp := *v
	return &cp, true
}

// ListVerifications 列出最近的验证结果，按创建时间倒序。
func ListVerifications(limit int) []model.DiagnosisVerification {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if mysqlOK {
		return listVerificationsMySQL(limit)
	}
	return listVerificationsMemory(limit)
}

func listVerificationsMySQL(limit int) []model.DiagnosisVerification {
	rows, err := db.Query(
		`SELECT id,diagnosis_id,COALESCE(alert_resolution_rate,0),COALESCE(recurrence_count,0),COALESCE(runbook_executed,0),COALESCE(runbook_success,0),COALESCE(score,0),verified_at,created_at FROM diagnosis_verifications ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return []model.DiagnosisVerification{}
	}
	defer rows.Close()
	out := []model.DiagnosisVerification{}
	for rows.Next() {
		var v model.DiagnosisVerification
		_ = rows.Scan(&v.ID, &v.DiagnosisID, &v.AlertResolutionRate, &v.RecurrenceCount, &v.RunbookExecuted, &v.RunbookSuccess, &v.Score, &v.VerifiedAt, &v.CreatedAt)
		out = append(out, v)
	}
	return out
}

func listVerificationsMemory(limit int) []model.DiagnosisVerification {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]model.DiagnosisVerification, 0, len(diagVerifications))
	for _, v := range diagVerifications {
		out = append(out, *v)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
