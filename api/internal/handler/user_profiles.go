package handler

import (
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListUserProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListUserProfiles())
}

func SaveUserProfile(c *gin.Context) {
	var p model.UserProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.ID == "" {
		p.ID = store.NewID()
	}
	if p.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	store.SaveUserProfile(&p)
	c.JSON(http.StatusOK, p)
}

func DeleteUserProfile(c *gin.Context) {
	id := c.Param("id")
	store.DeleteUserProfile(id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
