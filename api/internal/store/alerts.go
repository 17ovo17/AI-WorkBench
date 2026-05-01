package store

import (
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
)

// AddAlert normalizes, deduplicates, and persists an alert record.
func AddAlert(a *model.AlertRecord) bool {
	normalizeAlert(a)
	if mysqlOK {
		for _, existing := range ListAlerts() {
			if existing.Fingerprint == a.Fingerprint && existing.DeletedAt == nil {
				mergeAlert(existing, a)
				return false
			}
		}
	}
	mu.Lock()
	merged := false
	for _, existing := range alerts {
		if existing.Fingerprint == a.Fingerprint && existing.DeletedAt == nil {
			mergeAlert(existing, a)
			merged = true
			break
		}
	}
	if !merged {
		alerts = append([]*model.AlertRecord{a}, alerts...)
	}
	if len(alerts) > 500 {
		alerts = alerts[:500]
	}
	mu.Unlock()
	if mysqlOK && !merged {
		persistAlert(a)
	}
	return !merged
}

func normalizeAlert(a *model.AlertRecord) {
	now := time.Now()
	if a.ID == "" {
		a.ID = NewID()
	}
	if a.CreateTime.IsZero() {
		a.CreateTime = now
	}
	if a.FirstSeen.IsZero() {
		a.FirstSeen = a.CreateTime
	}
	if a.LastSeen.IsZero() {
		a.LastSeen = a.CreateTime
	}
	if a.Status == "" {
		a.Status = "firing"
	}
	if a.Count <= 0 {
		a.Count = 1
	}
	if a.Labels == nil {
		a.Labels = map[string]string{}
	}
	if a.BusinessID == "" {
		a.BusinessID = a.Labels["business_id"]
	}
	if a.TestBatchID == "" {
		a.TestBatchID = a.Labels["test_batch_id"]
	}
	if a.RunbookURL == "" {
		a.RunbookURL = a.Labels["runbook_url"]
	}
	if a.Fingerprint == "" {
		key := strings.Join([]string{a.Title, a.TargetIP, a.Severity, a.BusinessID, a.Source}, "|")
		a.Fingerprint = fmt.Sprintf("%x", sha1.Sum([]byte(key)))
	}
	if len(a.ActionLog) == 0 {
		a.ActionLog = []model.AlertAction{{Action: "created", From: "", To: a.Status, TraceID: a.AuditTraceID, CreatedAt: a.CreateTime}}
	}
	if len(a.NotificationTrail) == 0 && a.Status == "firing" {
		a.NotificationTrail = []model.AlertNotification{{ID: "notify-" + a.ID, Channel: "console", Receiver: "oncall-primary", Status: "queued", Detail: "waiting for on-call acknowledgement", Retry: 0, CreatedAt: a.CreateTime}}
	}
}

func mergeAlert(existing, incoming *model.AlertRecord) {
	previous := existing.Status
	existing.Count += max(1, incoming.Count)
	existing.LastSeen = incoming.LastSeen
	existing.Labels = mergeStringMap(existing.Labels, incoming.Labels)
	existing.Annotations = mergeStringMap(existing.Annotations, incoming.Annotations)
	if incoming.Status == "resolved" {
		existing.Status = "resolved"
		existing.ResolvedAt = incoming.ResolvedAt
		if existing.ResolvedAt == nil {
			now := incoming.LastSeen
			existing.ResolvedAt = &now
		}
	} else if previous == "resolved" || previous == "archived" {
		existing.Status = "firing"
	}
	existing.ActionLog = append(existing.ActionLog, model.AlertAction{Action: "deduplicated", From: previous, To: existing.Status, Reason: fmt.Sprintf("merged duplicate event %s", incoming.ID), CreatedAt: incoming.LastSeen})
	existing.NotificationTrail = append(existing.NotificationTrail, model.AlertNotification{ID: "notify-" + incoming.ID, Channel: "console", Receiver: "oncall-primary", Status: "suppressed", Detail: "duplicate event folded into incident", CreatedAt: incoming.LastSeen})
	if mysqlOK {
		persistAlert(existing)
	}
}

func mergeStringMap(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// PLACEHOLDER_PERSIST_ALERT

func persistAlert(a *model.AlertRecord) {
	labels, _ := json.Marshal(a.Labels)
	annotations, _ := json.Marshal(a.Annotations)
	actions, _ := json.Marshal(a.ActionLog)
	notifications, _ := json.Marshal(a.NotificationTrail)
	_, _ = db.Exec(`REPLACE INTO alerts (id,title,target_ip,severity,status,labels,source,create_time,resolved_at,annotations,business_id,fingerprint,count,first_seen,last_seen,ack_by,assignee,muted_until,deleted_at,test_batch_id,audit_trace_id,diagnose_record_id,linked_business_id,resolution,runbook_url,action_log,notification_trail) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.Title, a.TargetIP, a.Severity, a.Status, string(labels), a.Source, a.CreateTime, nullableTime(a.ResolvedAt), string(annotations), a.BusinessID, a.Fingerprint, a.Count, a.FirstSeen, a.LastSeen, a.AckBy, a.Assignee, nullableTime(a.MutedUntil), nullableTime(a.DeletedAt), a.TestBatchID, a.AuditTraceID, a.DiagnoseRecordID, a.LinkedBusinessID, a.Resolution, a.RunbookURL, string(actions), string(notifications))
}

// ListAlerts returns all non-deleted alerts.
func ListAlerts() []*model.AlertRecord {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,title,target_ip,severity,status,labels,source,create_time,resolved_at,COALESCE(annotations,'{}'),COALESCE(business_id,''),COALESCE(fingerprint,''),COALESCE(count,1),COALESCE(first_seen,create_time),COALESCE(last_seen,create_time),COALESCE(ack_by,''),COALESCE(assignee,''),muted_until,deleted_at,COALESCE(test_batch_id,''),COALESCE(audit_trace_id,''),COALESCE(diagnose_record_id,''),COALESCE(linked_business_id,''),COALESCE(resolution,''),COALESCE(runbook_url,''),COALESCE(action_log,'[]'),COALESCE(notification_trail,'[]') FROM alerts WHERE deleted_at IS NULL ORDER BY last_seen DESC, create_time DESC LIMIT 500`)
		if err == nil {
			defer rows.Close()
			out := []*model.AlertRecord{}
			for rows.Next() {
				a := model.AlertRecord{}
				var labels, annotations, actions, notifications string
				var resolved, muted, deleted sql.NullTime
				_ = rows.Scan(&a.ID, &a.Title, &a.TargetIP, &a.Severity, &a.Status, &labels, &a.Source, &a.CreateTime, &resolved, &annotations, &a.BusinessID, &a.Fingerprint, &a.Count, &a.FirstSeen, &a.LastSeen, &a.AckBy, &a.Assignee, &muted, &deleted, &a.TestBatchID, &a.AuditTraceID, &a.DiagnoseRecordID, &a.LinkedBusinessID, &a.Resolution, &a.RunbookURL, &actions, &notifications)
				_ = json.Unmarshal([]byte(labels), &a.Labels)
				_ = json.Unmarshal([]byte(annotations), &a.Annotations)
				_ = json.Unmarshal([]byte(actions), &a.ActionLog)
				_ = json.Unmarshal([]byte(notifications), &a.NotificationTrail)
				if resolved.Valid {
					a.ResolvedAt = &resolved.Time
				}
				if muted.Valid {
					a.MutedUntil = &muted.Time
				}
				if deleted.Valid {
					a.DeletedAt = &deleted.Time
				}
				out = append(out, &a)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := []*model.AlertRecord{}
	for _, a := range alerts {
		if a.DeletedAt == nil {
			out = append(out, a)
		}
	}
	return out
}

// PLACEHOLDER_UPDATE_ALERT

// UpdateAlertAction applies an action and mutation to an alert.
func UpdateAlertAction(id string, action model.AlertAction, mutate func(*model.AlertRecord)) bool {
	now := time.Now()
	if action.CreatedAt.IsZero() {
		action.CreatedAt = now
	}
	found := false
	mu.Lock()
	for _, a := range alerts {
		if a.ID == id {
			if a.DeletedAt != nil {
				break
			}
			action.From = a.Status
			mutate(a)
			action.To = a.Status
			a.ActionLog = append(a.ActionLog, action)
			found = true
			if mysqlOK {
				persistAlert(a)
			}
			break
		}
	}
	mu.Unlock()
	if found || !mysqlOK {
		return found
	}
	for _, a := range ListAlerts() {
		if a.ID == id {
			action.From = a.Status
			mutate(a)
			action.To = a.Status
			a.ActionLog = append(a.ActionLog, action)
			persistAlert(a)
			return true
		}
	}
	return false
}

// ResolveAlert marks an alert as resolved.
func ResolveAlert(id string) bool {
	now := time.Now()
	return UpdateAlertAction(id, model.AlertAction{Action: "resolved", Reason: "marked resolved"}, func(a *model.AlertRecord) {
		a.Status = "resolved"
		a.ResolvedAt = &now
	})
}

// DeleteAlert soft-deletes an alert.
func DeleteAlert(id string) bool {
	now := time.Now()
	return UpdateAlertAction(id, model.AlertAction{Action: "deleted", Reason: "soft deleted by user confirmation"}, func(a *model.AlertRecord) {
		a.DeletedAt = &now
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
