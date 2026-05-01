package handler

import (
	"net/http"
	"time"

	"ai-workbench-api/internal/correlator"

	"github.com/gin-gonic/gin"
)

func CorrelateHandler(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip required"})
		return
	}
	windowStr := c.DefaultQuery("window", "1h")
	window, err := time.ParseDuration(windowStr)
	if err != nil {
		window = time.Hour
	}
	result := correlator.Correlate(ip, window)
	c.JSON(http.StatusOK, result)
}
