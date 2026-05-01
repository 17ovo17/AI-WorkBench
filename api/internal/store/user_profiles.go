package store

import (
	"ai-workbench-api/internal/model"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
)

func SaveUserProfile(p *model.UserProfile) {
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	mu.Lock()
	defer mu.Unlock()

	if mysqlOK {
		hostsJSON, _ := json.Marshal(p.Hosts)
		endpointsJSON, _ := json.Marshal(p.Endpoints)
		_, err := db.Exec(`REPLACE INTO user_profiles (id,name,hosts,endpoints,description,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
			p.ID, p.Name, hostsJSON, endpointsJSON, p.Description, p.CreatedAt, p.UpdatedAt)
		if err != nil {
			logrus.Warnf("save user_profile: %v", err)
		}
	}
}

func ListUserProfiles() []model.UserProfile {
	var out []model.UserProfile
	if mysqlOK {
		rows, err := db.Query(`SELECT id,name,hosts,endpoints,description,created_at,updated_at FROM user_profiles ORDER BY updated_at DESC`)
		if err != nil {
			logrus.Warnf("list user_profiles: %v", err)
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var p model.UserProfile
			var hostsRaw, endpointsRaw []byte
			if err := rows.Scan(&p.ID, &p.Name, &hostsRaw, &endpointsRaw, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
				continue
			}
			_ = json.Unmarshal(hostsRaw, &p.Hosts)
			_ = json.Unmarshal(endpointsRaw, &p.Endpoints)
			out = append(out, p)
		}
	}
	return out
}

func DeleteUserProfile(id string) {
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM user_profiles WHERE id=?`, id)
	}
}
