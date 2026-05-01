package handler

import (
	"net/http"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// ListPromptTemplates GET /api/v1/prompts
func ListPromptTemplates(c *gin.Context) {
	category := c.Query("category")
	items := store.ListPromptTemplates(category)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}

// GetPromptTemplate GET /api/v1/prompts/:name
func GetPromptTemplate(c *gin.Context) {
	name := c.Param("name")
	t, ok := store.GetPromptTemplate(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "prompt template not found"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// CreatePromptTemplate POST /api/v1/prompts
func CreatePromptTemplate(c *gin.Context) {
	var input model.PromptTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if input.Name == "" || input.Template == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and template required"})
		return
	}
	if _, exists := store.GetPromptTemplate(input.Name); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "prompt template already exists"})
		return
	}
	input.IsActive = true
	store.SavePromptTemplate(&input)
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": input.Name})
}

// UpdatePromptTemplate PUT /api/v1/prompts/:name
func UpdatePromptTemplate(c *gin.Context) {
	name := c.Param("name")
	existing, ok := store.GetPromptTemplate(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "prompt template not found"})
		return
	}
	var input model.PromptTemplate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	existing.Template = input.Template
	existing.Category = input.Category
	existing.Variables = input.Variables
	existing.Description = input.Description
	existing.IsActive = input.IsActive
	existing.Version++
	existing.UpdatedAt = time.Now()
	store.SavePromptTemplate(existing)
	c.JSON(http.StatusOK, gin.H{"ok": true, "version": existing.Version})
}

// DeletePromptTemplate DELETE /api/v1/prompts/:name
func DeletePromptTemplate(c *gin.Context) {
	name := c.Param("name")
	if _, ok := store.GetPromptTemplate(name); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "prompt template not found"})
		return
	}
	store.DeletePromptTemplate(name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
