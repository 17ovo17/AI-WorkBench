package handler

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type tokenEntry struct {
	userID    string
	username  string
	role      string
	expiresAt time.Time
}

var tokenStore sync.Map

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "未登录"})
			return
		}
		val, ok := tokenStore.Load(token)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "登录已过期"})
			return
		}
		entry := val.(tokenEntry)
		if time.Now().After(entry.expiresAt) {
			tokenStore.Delete(token)
			c.AbortWithStatusJSON(401, gin.H{"error": "登录已过期"})
			return
		}
		c.Set("userID", entry.userID)
		c.Set("username", entry.username)
		c.Set("role", entry.role)
		c.Next()
	}
}
