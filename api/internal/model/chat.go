package model

import "time"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatSession struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Model     string        `json:"model"`
	TargetIP  string        `json:"target_ip,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Messages  []ChatMessage `json:"messages,omitempty"`
}

type ChatMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Model     string    `json:"model,omitempty"`
	TargetIP  string    `json:"target_ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatRequest struct {
	SessionID string    `json:"session_id"`
	Model     string    `json:"model" binding:"required"`
	Messages  []Message `json:"messages" binding:"required"`
	Stream    bool      `json:"stream"`
}

type ModelConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
}
