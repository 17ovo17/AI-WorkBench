package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const auditHMACKey = "aiw-audit-integrity-2026"

// computeAuditHash 计算审计事件的 HMAC 签名，用于防篡改校验。
func computeAuditHash(e *AuditEvent) string {
	content := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		e.Action, e.Target, e.Risk, e.Decision,
		e.Detail, e.Operator, e.Description, e.CreatedAt.Format(time.RFC3339))
	mac := hmac.New(sha256.New, []byte(auditHMACKey))
	mac.Write([]byte(content))
	return hex.EncodeToString(mac.Sum(nil))
}

// AddAuditEvent persists an audit event to memory and MySQL.
func AddAuditEvent(e AuditEvent) {
	fillAuditDerivedFields(&e)
	e.IntegrityHash = computeAuditHash(&e)
	mu.Lock()
	auditEvents = append([]AuditEvent{e}, auditEvents...)
	if len(auditEvents) > 1000 {
		auditEvents = auditEvents[:1000]
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO audit_events (id,action,target,risk,decision,detail,operator,description,test_batch_id,client_ip,created_at,integrity_hash) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			e.ID, e.Action, e.Target, e.Risk, e.Decision, e.Detail, e.Operator, e.Description, e.TestBatchID, e.ClientIP, e.CreatedAt, e.IntegrityHash)
	}
}

// ListAuditEvents returns audit events ordered by created_at desc.
func ListAuditEvents(limit int) []AuditEvent {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	if mysqlOK {
		rows, err := db.Query(`SELECT id,action,target,risk,decision,detail,COALESCE(operator,''),COALESCE(description,''),test_batch_id,client_ip,created_at,COALESCE(integrity_hash,'') FROM audit_events ORDER BY created_at DESC LIMIT ?`, limit)
		if err == nil {
			defer rows.Close()
			out := []AuditEvent{}
			for rows.Next() {
				var e AuditEvent
				_ = rows.Scan(&e.ID, &e.Action, &e.Target, &e.Risk, &e.Decision, &e.Detail, &e.Operator, &e.Description, &e.TestBatchID, &e.ClientIP, &e.CreatedAt, &e.IntegrityHash)
				fillAuditDerivedFields(&e)
				out = append(out, e)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	if len(auditEvents) < limit {
		limit = len(auditEvents)
	}
	out := make([]AuditEvent, limit)
	copy(out, auditEvents[:limit])
	for i := range out {
		fillAuditDerivedFields(&out[i])
	}
	return out
}

func fillAuditDerivedFields(e *AuditEvent) {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.Timestamp == "" {
		e.Timestamp = e.CreatedAt.Format(time.RFC3339)
	}
	if e.Operator == "" {
		e.Operator = "anonymous"
	}
	if e.Description == "" {
		e.Description = fmt.Sprintf("%s 操作对象 %s，结果 %s", e.Action, e.Target, e.Decision)
		if e.Detail != "" {
			e.Description += "，说明：" + e.Detail
		}
	}
}
