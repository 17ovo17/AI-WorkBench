package model

import "time"

// MetricsMapping maps a raw metric name to a standardized name.
type MetricsMapping struct {
	ID           string    `json:"id"`
	DatasourceID string    `json:"datasource_id"`
	RawName      string    `json:"raw_name"`
	StandardName string    `json:"standard_name"`
	Exporter     string    `json:"exporter"`
	Description  string    `json:"description"`
	Transform    string    `json:"transform"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
