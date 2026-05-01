package embedding

import (
	"strconv"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/spf13/viper"
)

// EmbedConfig embedding 配置
type EmbedConfig struct {
	Provider   string // "builtin" | "api" | "hybrid"
	APIURL     string
	APIKey     string
	Model      string
	Dimensions int
	BatchSize  int
}

// RerankerConfig reranker 配置
type RerankerConfig struct {
	Enabled  bool
	Provider string // "llm" | "api"
	APIURL   string
	APIKey   string
	Model    string
	TopK     int
}

// LoadConfig 从 ai_settings 表读取 embedding 配置，fallback 到 viper
func LoadConfig() EmbedConfig {
	return EmbedConfig{
		Provider:   getSettingOrViper("embedding.provider", "embedding.provider", "builtin"),
		APIURL:     getSettingOrViper("embedding.api.url", "embedding.api.url", ""),
		APIKey:     getSettingOrViper("embedding.api.key", "embedding.api.key", ""),
		Model:      getSettingOrViper("embedding.api.model", "embedding.api.model", ""),
		Dimensions: getSettingOrViperInt("embedding.api.dimensions", "embedding.api.dimensions", 0),
		BatchSize:  getSettingOrViperInt("embedding.api.batch_size", "embedding.api.batch_size", defaultBatchSize),
	}
}

// LoadRerankerConfig 从 ai_settings 表读取 reranker 配置，fallback 到 viper
func LoadRerankerConfig() RerankerConfig {
	return RerankerConfig{
		Enabled:  getSettingOrViperBool("reranker.enabled", "reranker.enabled", false),
		Provider: getSettingOrViper("reranker.provider", "reranker.provider", "llm"),
		APIURL:   getSettingOrViper("reranker.api.url", "reranker.api.url", ""),
		APIKey:   getSettingOrViper("reranker.api.key", "reranker.api.key", ""),
		Model:    getSettingOrViper("reranker.api.model", "reranker.api.model", ""),
		TopK:     getSettingOrViperInt("reranker.top_k", "reranker.top_k", 0),
	}
}

// SaveEmbedConfig 将 embedding 配置写入 ai_settings 表
func SaveEmbedConfig(cfg EmbedConfig) {
	saveSetting("embedding.provider", cfg.Provider)
	saveSetting("embedding.api.url", cfg.APIURL)
	saveSetting("embedding.api.key", cfg.APIKey)
	saveSetting("embedding.api.model", cfg.Model)
	saveSetting("embedding.api.dimensions", strconv.Itoa(cfg.Dimensions))
	saveSetting("embedding.api.batch_size", strconv.Itoa(cfg.BatchSize))
}

// SaveRerankerConfig 将 reranker 配置写入 ai_settings 表
func SaveRerankerConfig(cfg RerankerConfig) {
	saveSetting("reranker.enabled", strconv.FormatBool(cfg.Enabled))
	saveSetting("reranker.provider", cfg.Provider)
	saveSetting("reranker.api.url", cfg.APIURL)
	saveSetting("reranker.api.key", cfg.APIKey)
	saveSetting("reranker.api.model", cfg.Model)
	saveSetting("reranker.top_k", strconv.Itoa(cfg.TopK))
}

// --- 内部辅助函数 ---

// getSettingOrViper 先从 ai_settings 读取，fallback 到 viper，再 fallback 到默认值
func getSettingOrViper(settingKey, viperKey, defaultVal string) string {
	if s, ok := store.GetAISetting(settingKey); ok && s.SettingValue != "" {
		return s.SettingValue
	}
	if v := viper.GetString(viperKey); v != "" {
		return v
	}
	return defaultVal
}

// getSettingOrViperInt 整数版本的 getSettingOrViper
func getSettingOrViperInt(settingKey, viperKey string, defaultVal int) int {
	if s, ok := store.GetAISetting(settingKey); ok && s.SettingValue != "" {
		if v, err := strconv.Atoi(s.SettingValue); err == nil {
			return v
		}
	}
	if v := viper.GetInt(viperKey); v != 0 {
		return v
	}
	return defaultVal
}

// getSettingOrViperBool 布尔版本的 getSettingOrViper
func getSettingOrViperBool(settingKey, viperKey string, defaultVal bool) bool {
	if s, ok := store.GetAISetting(settingKey); ok && s.SettingValue != "" {
		if v, err := strconv.ParseBool(s.SettingValue); err == nil {
			return v
		}
	}
	if viper.IsSet(viperKey) {
		return viper.GetBool(viperKey)
	}
	return defaultVal
}

// saveSetting 保存单个配置项到 ai_settings 表
func saveSetting(key, value string) {
	store.SaveAISetting(&model.AISetting{
		SettingKey:   key,
		SettingValue: value,
	})
}
