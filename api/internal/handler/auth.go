package handler

import (
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret []byte

func InitAuth() {
	jwtSecret = make([]byte, 32)
	rand.Read(jwtSecret)
	if store.UserCount() == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		store.CreateUser(&model.User{
			ID: store.NewID(), Username: "admin",
			PasswordHash: string(hash), Role: "admin", MustChangePwd: true,
		})
	}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	user := store.GetUserByUsername(strings.TrimSpace(req.Username))
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	token := generateToken()
	tokenStore.Store(token, tokenEntry{userID: user.ID, username: user.Username, role: user.Role, expiresAt: time.Now().Add(24 * time.Hour)})
	c.JSON(http.StatusOK, loginResp{Token: token, User: user})
}

func Logout(c *gin.Context) {
	token := extractToken(c)
	if token != "" {
		tokenStore.Delete(token)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已退出"})
}

func GetMe(c *gin.Context) {
	username, _ := c.Get("username")
	user := store.GetUserByUsername(username.(string))
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码至少 6 位"})
		return
	}
	username, _ := c.Get("username")
	user := store.GetUserByUsername(username.(string))
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "旧密码错误"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	store.UpdateUserPassword(user.ID, string(hash))
	c.JSON(http.StatusOK, gin.H{"message": "密码已更新"})
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func extractToken(c *gin.Context) string {
	if t := c.GetHeader("Authorization"); strings.HasPrefix(t, "Bearer ") {
		return strings.TrimPrefix(t, "Bearer ")
	}
	return c.Query("token")
}
