package handler

import (
	"net/http"
	"strings"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// 敏感字段判定关键字（不区分大小写）
var sensitiveKeywords = []string{"api_key", "password", "secret", "token"}

// maskValue 对敏感字段做掩码：长度 > 6 时显示前 6 位 + "..."。
func maskValue(value string) string {
	if len(value) <= 6 {
		return strings.Repeat("*", len(value))
	}
	return value[:6] + "..."
}

// isSensitive 判断 key 是否为敏感字段。
func isSensitive(key string) bool {
	lower := strings.ToLower(key)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// maskIfSensitive 如果 key 是敏感字段则掩码 value。
func maskIfSensitive(key, value string) string {
	if isSensitive(key) {
		return maskValue(value)
	}
	return value
}

// GetAISettings GET /api/v1/settings/ai
// 返回所有设置的 key-value map（敏感字段已掩码）。
func GetAISettings(c *gin.Context) {
	settings := store.ListAISettings()
	out := make(map[string]string, len(settings))
	for _, s := range settings {
		out[s.SettingKey] = maskIfSensitive(s.SettingKey, s.SettingValue)
	}
	c.JSON(http.StatusOK, out)
}

// UpdateAISettings PUT /api/v1/settings/ai
// 批量更新设置，请求体为 map[string]string。
// 如果传入 value 是掩码字符串（含 "..."），跳过该字段（避免把掩码写回去）。
func UpdateAISettings(c *gin.Context) {
	var input map[string]string
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated := 0
	for key, value := range input {
		if isSensitive(key) && strings.HasSuffix(value, "...") {
			continue // 跳过掩码值
		}
		store.SaveAISetting(&model.AISetting{
			SettingKey:   key,
			SettingValue: value,
		})
		updated++
	}
	auditEvent(c, "settings.ai.update", "batch", "low", "ok",
		"keys="+strings.Join(mapKeys(input), ","), c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true, "updated": updated})
}

// GetAISetting GET /api/v1/settings/ai/:key
func GetAISetting(c *gin.Context) {
	key := c.Param("key")
	s, ok := store.GetAISetting(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "setting not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"key":   s.SettingKey,
		"value": maskIfSensitive(s.SettingKey, s.SettingValue),
	})
}

// UpdateAISettingHandler PUT /api/v1/settings/ai/:key
func UpdateAISettingHandler(c *gin.Context) {
	key := c.Param("key")
	var input struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if isSensitive(key) && strings.HasSuffix(input.Value, "...") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "masked value rejected, please provide real value"})
		return
	}
	store.SaveAISetting(&model.AISetting{
		SettingKey:   key,
		SettingValue: input.Value,
	})
	auditEvent(c, "settings.ai.update", key, "low", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeleteAISetting DELETE /api/v1/settings/ai/:key
func DeleteAISetting(c *gin.Context) {
	key := c.Param("key")
	if _, ok := store.GetAISetting(key); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "setting not found"})
		return
	}
	store.DeleteAISetting(key)
	auditEvent(c, "settings.ai.delete", key, "medium", "ok", "", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
