package handler

import (
	"net/http"
	"strconv"
	"strings"

	"ai-workbench-api/internal/metrics"
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// scanMetricsRequest models the scan request body.
type scanMetricsRequest struct {
	DatasourceID string `json:"datasource_id"`
}

// metricsListResponse is the paginated mappings list response.
type metricsListResponse struct {
	Items []model.MetricsMapping `json:"items"`
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

// confirmMappingsRequest models the bulk confirm payload.
type confirmMappingsRequest struct {
	DatasourceID string `json:"datasource_id"`
}

// ScanMetrics POST /api/v1/metrics/scan
func ScanMetrics(c *gin.Context) {
	var req scanMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.DatasourceID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "datasource_id required"})
		return
	}
	prometheusURL := resolvePrometheusURL(req.DatasourceID)
	if prometheusURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prometheus URL not found for datasource"})
		return
	}
	added, err := metrics.ScanPrometheusMetrics(prometheusURL, req.DatasourceID)
	if err != nil {
		auditEvent(c, "metrics.scan", req.DatasourceID, "medium", "fail",
			err.Error(), c.GetHeader("X-Test-Batch-Id"))
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	auditEvent(c, "metrics.scan", req.DatasourceID, "low", "ok",
		"added="+strconv.Itoa(added), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"added": added, "datasource_id": req.DatasourceID})
}

// resolvePrometheusURL finds the Prometheus URL for the given datasource id,
// falling back to the global prometheus.url config when no match is found.
func resolvePrometheusURL(datasourceID string) string {
	var sources []model.DataSource
	_ = viper.UnmarshalKey("data_sources", &sources)
	for _, s := range sources {
		if s.ID == datasourceID && strings.EqualFold(s.Type, "prometheus") {
			if url := strings.TrimSpace(s.URL); url != "" {
				return url
			}
		}
	}
	return strings.TrimSpace(viper.GetString("prometheus.url"))
}

// ListMetricsMappings GET /api/v1/metrics/mappings
func ListMetricsMappings(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	datasourceID := c.Query("datasource_id")
	status := c.Query("status")
	items, total := store.ListMetricsMappings(datasourceID, status, page, limit)
	c.JSON(http.StatusOK, metricsListResponse{
		Items: items, Total: total, Page: page, Limit: limit,
	})
}

// UpdateMetricMapping PUT /api/v1/metrics/mappings/:id
func UpdateMetricMapping(c *gin.Context) {
	id := c.Param("id")
	existing, ok := store.GetMetricsMapping(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
		return
	}
	var input model.MetricsMapping
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.ID = existing.ID
	input.DatasourceID = existing.DatasourceID
	input.RawName = existing.RawName
	input.CreatedAt = existing.CreatedAt
	if input.Status == "" {
		input.Status = "custom"
	}
	store.UpdateMetricsMapping(&input)
	auditEvent(c, "metrics.mapping.update", id, "low", "ok",
		"status="+input.Status, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, input)
}

// ConfirmMappings POST /api/v1/metrics/mappings/confirm
func ConfirmMappings(c *gin.Context) {
	var req confirmMappingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.DatasourceID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "datasource_id required"})
		return
	}
	updated := store.ConfirmAutoMappings(req.DatasourceID)
	auditEvent(c, "metrics.mapping.confirm", req.DatasourceID, "low", "ok",
		"updated="+strconv.Itoa(updated), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}
