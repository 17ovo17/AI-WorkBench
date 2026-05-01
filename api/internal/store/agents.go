package store

import (
	"context"
	"encoding/json"
	"time"

	"ai-workbench-api/internal/model"
)

// UpsertAgent inserts or updates a catpaw agent record.
func UpsertAgent(a *model.CatpawAgent) {
	mu.Lock()
	agents[a.IP] = a
	mu.Unlock()
	if redisOK {
		b, _ := json.Marshal(a)
		_ = redisClient.Set(context.Background(), "agent:"+a.IP, b, 5*time.Minute).Err()
	}
	if mysqlOK {
		_, _ = db.Exec(`REPLACE INTO catpaw_agents (ip,hostname,version,last_seen) VALUES (?,?,?,?)`, a.IP, a.Hostname, a.Version, a.LastSeen)
	}
}

// ListAgents returns all catpaw agents with online status.
func ListAgents() []*model.CatpawAgent {
	out := []*model.CatpawAgent{}
	if mysqlOK {
		rows, err := db.Query(`SELECT ip,hostname,version,last_seen FROM catpaw_agents ORDER BY last_seen DESC`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				a := model.CatpawAgent{}
				_ = rows.Scan(&a.IP, &a.Hostname, &a.Version, &a.LastSeen)
				a.Online = time.Since(a.LastSeen) < 5*time.Minute
				out = append(out, &a)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, a := range agents {
		cp := *a
		cp.Online = time.Since(cp.LastSeen) < 5*time.Minute
		out = append(out, &cp)
	}
	return out
}

// DeleteAgent removes a catpaw agent by IP.
func DeleteAgent(ip string) {
	mu.Lock()
	delete(agents, ip)
	mu.Unlock()
	if redisOK {
		_ = redisClient.Del(context.Background(), "agent:"+ip).Err()
	}
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM catpaw_agents WHERE ip=?`, ip)
	}
}

// HasOnlineAgent checks if there is an online agent for the given IP.
func HasOnlineAgent(ip string) bool {
	for _, a := range ListAgents() {
		if a.IP == ip && a.Online {
			return true
		}
	}
	return false
}
