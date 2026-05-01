package model

import "time"

type AIProvider struct {
	ID      string   `json:"id" mapstructure:"id"`
	Name    string   `json:"name" mapstructure:"name"`
	BaseURL string   `json:"base_url" mapstructure:"base_url"`
	APIKey  string   `json:"api_key" mapstructure:"api_key"`
	Models  []string `json:"models" mapstructure:"models"`
	Default bool     `json:"default" mapstructure:"default"`
}

type DataSource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Database string `json:"database"`
}

type UserProfile struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Hosts       []ProfileHost    `json:"hosts"`
	Endpoints   []ProfileEndpoint `json:"endpoints"`
	Description string           `json:"description"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type ProfileHost struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	Label    string `json:"label"`
}

type ProfileEndpoint struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Service string `json:"service"`
	Label   string `json:"label"`
}

type User struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	PasswordHash  string    `json:"-"`
	Role          string    `json:"role"`
	MustChangePwd bool      `json:"must_change_pwd"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
