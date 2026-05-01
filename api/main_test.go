package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestRequireAdminTokenPermissiveWhenNotConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Unsetenv("AIW_ADMIN_TOKEN")
	viper.Set("security.admin_token", "")
	r := gin.New()
	r.POST("/protected", requireAdminToken(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Header().Get("X-Security-Mode") != "permissive-admin-token-not-configured" {
		t.Fatalf("expected permissive mode, code=%d header=%q", w.Code, w.Header().Get("X-Security-Mode"))
	}
}

func TestRequireAdminTokenEnforcesConfiguredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Setenv("AIW_ADMIN_TOKEN", "unit-secret")
	defer os.Unsetenv("AIW_ADMIN_TOKEN")
	r := gin.New()
	r.POST("/protected", requireAdminToken(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/protected", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing token should be unauthorized, got %d", missing.Code)
	}
	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("X-Admin-Token", "unit-secret")
	r.ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK || allowed.Header().Get("X-Security-Mode") != "admin-token-enforced" {
		t.Fatalf("valid token should pass, code=%d header=%q", allowed.Code, allowed.Header().Get("X-Security-Mode"))
	}
}

func TestRequireAdminTokenAcceptsBearerLoginToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	os.Setenv("AIW_ADMIN_TOKEN", "unit-secret")
	defer os.Unsetenv("AIW_ADMIN_TOKEN")
	authRequired := func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer login-token" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid login token"})
			return
		}
		c.Set("username", "admin")
		c.Next()
	}
	r := gin.New()
	r.POST("/protected", requireAdminTokenWithAuth(authRequired), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	allowed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("Authorization", "Bearer login-token")
	r.ServeHTTP(allowed, req)
	if allowed.Code != http.StatusOK || allowed.Header().Get("X-Security-Mode") != "login-token-enforced" {
		t.Fatalf("valid login bearer should pass, code=%d header=%q", allowed.Code, allowed.Header().Get("X-Security-Mode"))
	}

	denied := httptest.NewRecorder()
	badReq := httptest.NewRequest(http.MethodPost, "/protected", nil)
	badReq.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(denied, badReq)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login bearer should be unauthorized, got %d", denied.Code)
	}
}

func TestCORSDefaultsAreLocalhostOnly(t *testing.T) {
	viper.Set("server.allowed_origins", []string{})
	cfg := corsConfig()
	if len(cfg.AllowOrigins) == 0 {
		t.Fatal("expected default localhost origins")
	}
	for _, origin := range cfg.AllowOrigins {
		if origin == "*" {
			t.Fatal("default CORS must not allow wildcard origin")
		}
	}
}
