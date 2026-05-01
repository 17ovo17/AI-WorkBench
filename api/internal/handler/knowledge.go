package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// caseListResponse is the paginated list response shape.
type caseListResponse struct {
	Items []model.DiagnosisCase `json:"items"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// ListCases GET /api/v1/knowledge/cases
func ListCases(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	keyword := c.Query("keyword")
	category := c.Query("category")
	items, total := store.ListCases(page, limit, keyword, category)
	c.JSON(http.StatusOK, caseListResponse{Items: items, Total: total, Page: page, Limit: limit})
}

// GetCase GET /api/v1/knowledge/cases/:id
func GetCase(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	item, ok := store.GetCase(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// CreateCase POST /api/v1/knowledge/cases
func CreateCase(c *gin.Context) {
	var item model.DiagnosisCase
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if item.RootCauseCategory == "" || item.RootCauseDescription == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "root_cause_category and root_cause_description required"})
		return
	}
	if hasUnrenderedTemplate(item.RootCauseCategory, item.RootCauseDescription, item.Keywords, item.TreatmentSteps, string(item.MetricSnapshot)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "case contains unrendered template variables"})
		return
	}
	if item.ID == "" {
		item.ID = store.NewID()
	}
	item.CreatedAt = time.Now()
	store.SaveCase(&item)
	auditEvent(c, "knowledge.case.create", item.ID, "low", "ok",
		"category="+item.RootCauseCategory, c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, item)
}

// UpdateCase PUT /api/v1/knowledge/cases/:id
func UpdateCase(c *gin.Context) {
	id := c.Param("id")
	existing, ok := store.GetCase(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
		return
	}
	var input model.DiagnosisCase
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if hasUnrenderedTemplate(input.RootCauseCategory, input.RootCauseDescription, input.Keywords, input.TreatmentSteps, string(input.MetricSnapshot)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "case contains unrendered template variables"})
		return
	}
	input.ID = existing.ID
	input.CreatedAt = existing.CreatedAt
	store.UpdateCase(&input)
	auditEvent(c, "knowledge.case.update", id, "low", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, input)
}

// DeleteCase DELETE /api/v1/knowledge/cases/:id
func DeleteCase(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if _, ok := store.GetCase(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "case not found"})
		return
	}
	store.DeleteCase(id)
	auditEvent(c, "knowledge.case.delete", id, "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ImportCases POST /api/v1/knowledge/cases/import
func ImportCases(c *gin.Context) {
	var items []model.DiagnosisCase
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	imported := 0
	for i := range items {
		if items[i].RootCauseCategory == "" || items[i].RootCauseDescription == "" {
			continue
		}
		if hasUnrenderedTemplate(items[i].RootCauseCategory, items[i].RootCauseDescription, items[i].Keywords, items[i].TreatmentSteps, string(items[i].MetricSnapshot)) {
			continue
		}
		if items[i].ID == "" {
			items[i].ID = store.NewID()
		}
		if items[i].CreatedAt.IsZero() {
			items[i].CreatedAt = time.Now()
		}
		store.SaveCase(&items[i])
		imported++
	}
	auditEvent(c, "knowledge.case.import", "batch", "medium", "ok",
		"count="+strconv.Itoa(imported), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"imported": imported, "total": len(items)})
}

func hasUnrenderedTemplate(values ...string) bool {
	for _, value := range values {
		text := strings.TrimSpace(value)
		if strings.Contains(text, "{{") || strings.Contains(text, "}}") {
			return true
		}
		if strings.Contains(text, "parameter_extractor.") || strings.Contains(text, "llm_diagnosis.") {
			return true
		}
	}
	return false
}

// ExportCases GET /api/v1/knowledge/cases/export
// 支持 ?category=cpu_high 按分类筛选导出。
func ExportCases(c *gin.Context) {
	category := c.Query("category")
	items, _ := store.ListCases(1, 10000, "", category)
	filename := "diagnosis_cases.json"
	if category != "" {
		filename = "diagnosis_cases_" + category + ".json"
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.JSON(http.StatusOK, items)
}
