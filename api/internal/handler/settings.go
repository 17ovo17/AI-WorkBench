package handler

import (
	"net/http"
	"strings"
	"sync"

	"ai-workbench-api/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var settingsMu sync.RWMutex

func GetAIProviders(c *gin.Context) {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	providers := loadAIProviders()
	for i := range providers {
		key := strings.TrimSpace(providers[i].APIKey)
		if key == "" || strings.Contains(key, "${") || key == "******" {
			providers[i].APIKey = ""
		} else {
			providers[i].APIKey = "******"
		}
	}
	c.JSON(http.StatusOK, providers)
}
func SaveAIProviders(c *gin.Context) {
	var providers []model.AIProvider
	if err := c.ShouldBindJSON(&providers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	existing := loadAIProviders()
	for i := range providers {
		if providers[i].APIKey == "******" {
			for _, e := range existing {
				if e.ID == providers[i].ID {
					providers[i].APIKey = e.APIKey
					break
				}
			}
		}
	}
	viper.Set("ai_providers", providers)
	viper.WriteConfig()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func GetDataSources(c *gin.Context) {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	ds := loadDataSources()
	for i := range ds {
		if ds[i].Password != "" {
			ds[i].Password = "******"
		}
	}
	c.JSON(http.StatusOK, ds)
}

func SaveDataSources(c *gin.Context) {
	var ds []model.DataSource
	if err := c.ShouldBindJSON(&ds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	settingsMu.Lock()
	defer settingsMu.Unlock()
	existing := loadDataSources()
	cleaned := make([]model.DataSource, 0, len(ds))
	for i := range ds {
		if ds[i].ID == "platform-mysql" {
			continue
		}
		if ds[i].Password == "******" {
			for _, e := range existing {
				if e.ID == ds[i].ID {
					ds[i].Password = e.Password
					break
				}
			}
		}
		cleaned = append(cleaned, ds[i])
	}
	viper.Set("data_sources", cleaned)
	if err := viper.WriteConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func loadAIProviders() []model.AIProvider {
	providers := []model.AIProvider{}
	var raw []struct {
		ID         string   `mapstructure:"id"`
		Name       string   `mapstructure:"name"`
		BaseURL    string   `mapstructure:"base_url"`
		BaseURLAlt string   `mapstructure:"baseurl"`
		APIKey     string   `mapstructure:"api_key"`
		APIKeyAlt  string   `mapstructure:"apikey"`
		Models     []string `mapstructure:"models"`
		Default    bool     `mapstructure:"default"`
	}
	_ = viper.UnmarshalKey("ai_providers", &raw)
	for _, item := range raw {
		baseURL := strings.TrimSpace(item.BaseURL)
		if baseURL == "" {
			baseURL = strings.TrimSpace(item.BaseURLAlt)
		}
		apiKey := strings.TrimSpace(item.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(item.APIKeyAlt)
		}
		providers = append(providers, model.AIProvider{
			ID: item.ID, Name: item.Name,
			BaseURL: strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/chat/completions"),
			APIKey:  apiKey, Models: item.Models, Default: item.Default,
		})
	}
	if len(providers) == 0 || allAIProvidersEmpty(providers) {
		baseURL := strings.TrimSpace(viper.GetString("ai.base_url"))
		apiKey := strings.TrimSpace(viper.GetString("ai.api_key"))
		modelName := resolveDefaultModel()
		if baseURL != "" || apiKey != "" || modelName != "" {
			models := []string{}
			if modelName != "" {
				models = append(models, modelName)
			}
			providers = []model.AIProvider{{ID: "legacy-ai", Name: "\u5e73\u53f0 AI \u914d\u7f6e", BaseURL: strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/chat/completions"), APIKey: apiKey, Models: models, Default: true}}
		}
	}
	hasDefault := false
	for _, provider := range providers {
		if provider.Default {
			hasDefault = true
			break
		}
	}
	if !hasDefault && len(providers) > 0 {
		providers[0].Default = true
	}
	return providers
}

func allAIProvidersEmpty(providers []model.AIProvider) bool {
	for _, provider := range providers {
		if strings.TrimSpace(provider.BaseURL) != "" || strings.TrimSpace(provider.APIKey) != "" || len(provider.Models) > 0 {
			return false
		}
	}
	return true
}

func loadDataSources() []model.DataSource {
	var ds []model.DataSource
	_ = viper.UnmarshalKey("data_sources", &ds)
	promURL := strings.TrimSpace(viper.GetString("prometheus.url"))
	mysqlDSNValue := strings.TrimSpace(viper.GetString("mysql.dsn"))
	hasPrometheus := false
	hasPlatformMySQL := false
	for i := range ds {
		if ds[i].Type == "prometheus" {
			hasPrometheus = true
			if strings.TrimSpace(ds[i].URL) == "" && promURL != "" {
				ds[i].URL = promURL
			}
		}
		if ds[i].ID == "platform-mysql" {
			hasPlatformMySQL = true
		}
		if ds[i].Type == "mysql" && strings.TrimSpace(ds[i].URL) == "" {
			ds[i].URL = mysqlDisplayEndpointFromDSN(mysqlDSNValue)
		}
	}
	if !hasPrometheus && promURL != "" {
		ds = append([]model.DataSource{{ID: "prometheus-default", Name: "Prometheus", Type: "prometheus", URL: promURL}}, ds...)
	}
	if !hasPlatformMySQL && mysqlDSNValue != "" {
		platform := model.DataSource{
			ID:       "platform-mysql",
			Name:     "\u5e73\u53f0 MySQL\uff08AI WorkBench\uff09",
			Type:     "mysql",
			URL:      mysqlDisplayEndpointFromDSN(mysqlDSNValue),
			Username: mysqlUserFromDSN(mysqlDSNValue),
			Database: mysqlDatabaseFromDSN(mysqlDSNValue),
		}
		ds = append(ds, platform)
	}
	return ds
}

func mysqlDisplayEndpointFromDSN(dsn string) string {
	if host := mysqlHostFromDSN(dsn); host != "" {
		return host
	}
	start := strings.Index(dsn, "@unix(")
	if start >= 0 {
		start += len("@unix(")
		end := strings.Index(dsn[start:], ")")
		if end >= 0 {
			return "unix:" + dsn[start:start+end]
		}
	}
	return ""
}

func mysqlUserFromDSN(dsn string) string {
	idx := strings.Index(dsn, "@")
	if idx <= 0 {
		return ""
	}
	cred := dsn[:idx]
	if colon := strings.Index(cred, ":"); colon >= 0 {
		return cred[:colon]
	}
	return cred
}

func mysqlDatabaseFromDSN(dsn string) string {
	end := strings.Index(dsn, "?")
	if end < 0 {
		end = len(dsn)
	}
	slash := strings.LastIndex(dsn[:end], "/")
	if slash < 0 || slash+1 >= end {
		return "ai_workbench"
	}
	return dsn[slash+1 : end]
}
