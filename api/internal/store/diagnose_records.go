package store

import (
	"database/sql"

	"ai-workbench-api/internal/model"
)

// AddRecord inserts a diagnose record at the front of the in-memory list
// and persists it to MySQL if available.
func AddRecord(r *model.DiagnoseRecord) {
	mu.Lock()
	records = append([]*model.DiagnoseRecord{r}, records...)
	if len(records) > maxRecs {
		records = records[:maxRecs]
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO diagnose_records (id,target_ip,`+"`trigger`"+`,source,data_source,status,report,summary_report,raw_report,alert_title,create_time,end_time) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.TargetIP, r.Trigger, r.Source, r.DataSource, string(r.Status), r.Report, r.SummaryReport, r.RawReport, r.AlertTitle, r.CreateTime, nullableTime(r.EndTime))
	}
}

// UpdateRecord applies fn to the record with the given id.
func UpdateRecord(id string, fn func(*model.DiagnoseRecord)) {
	var updated *model.DiagnoseRecord
	mu.Lock()
	for _, r := range records {
		if r.ID == id {
			fn(r)
			cp := *r
			updated = &cp
			break
		}
	}
	mu.Unlock()
	if updated != nil && mysqlOK {
		AddRecord(updated)
	}
	if updated == nil && mysqlOK {
		list := ListRecords()
		for _, r := range list {
			if r.ID == id {
				fn(r)
				AddRecord(r)
				return
			}
		}
	}
}

// DeleteRecord removes a single diagnose record by id.
func DeleteRecord(id string) {
	mu.Lock()
	for i, r := range records {
		if r.ID == id {
			records = append(records[:i], records[i+1:]...)
			break
		}
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM diagnose_records WHERE id=?`, id)
	}
}

// DeleteRecordsByFilter removes records matching the filter and returns the count.
func DeleteRecordsByFilter(filter func(*model.DiagnoseRecord) bool) int {
	deleted := 0
	ids := []string{}
	mu.Lock()
	kept := records[:0]
	for _, r := range records {
		if filter(r) {
			deleted++
			ids = append(ids, r.ID)
			continue
		}
		kept = append(kept, r)
	}
	records = kept
	mu.Unlock()
	if mysqlOK {
		for _, r := range ListRecords() {
			if filter(r) {
				_, _ = db.Exec(`DELETE FROM diagnose_records WHERE id=?`, r.ID)
				deleted++
			}
		}
		for _, id := range ids {
			_, _ = db.Exec(`DELETE FROM diagnose_records WHERE id=?`, id)
		}
	}
	return deleted
}

// ListRecords returns diagnose records ordered by create_time desc.
func ListRecords() []*model.DiagnoseRecord {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,target_ip,` + "`trigger`" + `,source,data_source,status,report,summary_report,raw_report,alert_title,create_time,end_time FROM diagnose_records ORDER BY create_time DESC LIMIT 500`)
		if err == nil {
			defer rows.Close()
			out := []*model.DiagnoseRecord{}
			for rows.Next() {
				var r model.DiagnoseRecord
				var status string
				var end sql.NullTime
				_ = rows.Scan(&r.ID, &r.TargetIP, &r.Trigger, &r.Source, &r.DataSource, &status, &r.Report, &r.SummaryReport, &r.RawReport, &r.AlertTitle, &r.CreateTime, &end)
				r.Status = model.DiagnoseStatus(status)
				if end.Valid {
					r.EndTime = &end.Time
				}
				out = append(out, &r)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]*model.DiagnoseRecord, len(records))
	copy(out, records)
	return out
}

// LatestCatpawReport returns the most recent catpaw report for the given IP.
func LatestCatpawReport(ip string) (*model.DiagnoseRecord, bool) {
	for _, r := range ListRecords() {
		if r.TargetIP == ip && r.Source == "catpaw" && r.Report != "" {
			cp := *r
			return &cp, true
		}
	}
	return nil, false
}
