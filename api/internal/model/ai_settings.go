package model

import "time"

// AISetting represents a key-value AI configuration entry.
type AISetting struct {
	ID           string    `json:"id"`
	SettingKey   string    `json:"setting_key"`
	SettingValue string    `json:"setting_value"`
	UpdatedAt    time.Time `json:"updated_at"`
}
