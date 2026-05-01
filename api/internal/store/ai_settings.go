package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

// SaveAISetting 保存或更新 AI 设置（key-value）。
func SaveAISetting(s *model.AISetting) {
	s.UpdatedAt = time.Now()
	if s.ID == "" {
		s.ID = NewID()
	}
	mu.Lock()
	aiSettings[s.SettingKey] = s
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(
			`INSERT INTO ai_settings (id,setting_key,setting_value,updated_at) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE setting_value=VALUES(setting_value),updated_at=VALUES(updated_at)`,
			s.ID, s.SettingKey, s.SettingValue, s.UpdatedAt,
		)
	}
}

// GetAISetting 通过 key 查询单个设置。
func GetAISetting(key string) (*model.AISetting, bool) {
	if mysqlOK {
		row := db.QueryRow(`SELECT id,setting_key,COALESCE(setting_value,''),updated_at FROM ai_settings WHERE setting_key=?`, key)
		var s model.AISetting
		if err := row.Scan(&s.ID, &s.SettingKey, &s.SettingValue, &s.UpdatedAt); err == nil {
			return &s, true
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	s, ok := aiSettings[key]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// ListAISettings 列出所有 AI 设置。
func ListAISettings() []model.AISetting {
	if mysqlOK {
		rows, err := db.Query(`SELECT id,setting_key,COALESCE(setting_value,''),updated_at FROM ai_settings ORDER BY setting_key`)
		if err == nil {
			defer rows.Close()
			out := []model.AISetting{}
			for rows.Next() {
				var s model.AISetting
				_ = rows.Scan(&s.ID, &s.SettingKey, &s.SettingValue, &s.UpdatedAt)
				out = append(out, s)
			}
			return out
		}
	}
	mu.RLock()
	defer mu.RUnlock()
	out := make([]model.AISetting, 0, len(aiSettings))
	for _, s := range aiSettings {
		out = append(out, *s)
	}
	return out
}

// DeleteAISetting 通过 key 删除设置。
func DeleteAISetting(key string) {
	mu.Lock()
	delete(aiSettings, key)
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`DELETE FROM ai_settings WHERE setting_key=?`, key)
	}
}
