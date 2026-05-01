package model

import "time"

// PromptTemplate represents a reusable prompt template with versioning.
type PromptTemplate struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Template    string    `json:"template"`
	Version     int       `json:"version"`
	Variables   string    `json:"variables"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
