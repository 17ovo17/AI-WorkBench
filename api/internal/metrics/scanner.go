package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
)

const (
	scanTimeout    = 30 * time.Second
	labelValuePath = "/api/v1/label/__name__/values"
)

// promLabelResponse models the Prometheus label values API response.
type promLabelResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// ScanPrometheusMetrics fetches all metric names from Prometheus
// and persists new entries into metrics_mappings (status=unmapped).
// Returns the number of newly added mappings.
func ScanPrometheusMetrics(prometheusURL, datasourceID string) (int, error) {
	if strings.TrimSpace(prometheusURL) == "" {
		return 0, fmt.Errorf("prometheus URL is empty")
	}
	if strings.TrimSpace(datasourceID) == "" {
		return 0, fmt.Errorf("datasource id is empty")
	}
	names, err := fetchMetricNames(prometheusURL)
	if err != nil {
		return 0, err
	}
	mappings := buildNewMappings(names, datasourceID)
	if len(mappings) == 0 {
		return 0, nil
	}
	return store.BulkSaveMetricsMappings(mappings), nil
}

func fetchMetricNames(prometheusURL string) ([]string, error) {
	url := strings.TrimRight(prometheusURL, "/") + labelValuePath
	client := &http.Client{Timeout: scanTimeout}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("prometheus request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned %d", resp.StatusCode)
	}
	var parsed promLabelResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	if parsed.Status != "success" {
		return nil, fmt.Errorf("prometheus response status %q", parsed.Status)
	}
	return parsed.Data, nil
}

func buildNewMappings(names []string, datasourceID string) []*model.MetricsMapping {
	now := time.Now()
	out := make([]*model.MetricsMapping, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, &model.MetricsMapping{
			ID:           store.NewID(),
			DatasourceID: datasourceID,
			RawName:      name,
			Exporter:     guessExporter(name),
			Status:       "unmapped",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	return out
}

// guessExporter infers the exporter name from a metric name prefix.
func guessExporter(name string) string {
	switch {
	case strings.HasPrefix(name, "node_"):
		return "node_exporter"
	case strings.HasPrefix(name, "process_"):
		return "process_exporter"
	case strings.HasPrefix(name, "mysql_"):
		return "mysqld_exporter"
	case strings.HasPrefix(name, "redis_"):
		return "redis_exporter"
	case strings.HasPrefix(name, "container_") || strings.HasPrefix(name, "kube_"):
		return "kubernetes"
	case strings.HasPrefix(name, "go_"):
		return "go_runtime"
	default:
		return ""
	}
}
