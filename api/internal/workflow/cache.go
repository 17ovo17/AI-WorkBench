package workflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"ai-workbench-api/internal/store"
	"ai-workbench-api/internal/workflow/engine"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ---------- 常量 ----------

const (
	defaultWorkflowTTL    = 300 // 秒
	nodeTTLHTTPRequest    = 60  // 秒
	nodeTTLKnowledgeRetrieval = 300 // 秒

	keyPrefixWorkflow = "wf:"
	keyPrefixNode     = "wfnode:"
)

// ---------- 内存缓存 fallback ----------

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

type memCache struct {
	mu    sync.RWMutex
	items map[string]*cacheItem
}

var fallbackCache = &memCache{items: make(map[string]*cacheItem)}

func init() {
	go fallbackCache.cleanupLoop()
}

// cleanupLoop 定期清理过期的内存缓存条目。
func (mc *memCache) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		mc.evictExpired()
	}
}

func (mc *memCache) evictExpired() {
	now := time.Now()
	mc.mu.Lock()
	defer mc.mu.Unlock()
	for k, item := range mc.items {
		if now.After(item.expiresAt) {
			delete(mc.items, k)
		}
	}
}

func (mc *memCache) get(key string) ([]byte, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	item, ok := mc.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.data, true
}

func (mc *memCache) set(key string, data []byte, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.items[key] = &cacheItem{data: data, expiresAt: time.Now().Add(ttl)}
}

// ---------- 配置读取 ----------

// CacheEnabled 检查缓存是否启用。
func CacheEnabled() bool {
	if viper.IsSet("workflow.cache.enabled") {
		return viper.GetBool("workflow.cache.enabled")
	}
	return true
}

func workflowTTL() time.Duration {
	sec := viper.GetInt("workflow.cache.ttl_seconds")
	if sec <= 0 {
		sec = defaultWorkflowTTL
	}
	return time.Duration(sec) * time.Second
}

// ---------- 公开 API ----------

// HashInputs 计算任意数据的 SHA256 哈希。
func HashInputs(data any) string {
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// CacheGet 获取工作流级缓存。
func CacheGet(workflowName string, inputs map[string]any) (*engine.WorkflowResult, bool) {
	key := keyPrefixWorkflow + workflowName + ":" + HashInputs(inputs)
	data, ok := cacheRead(key)
	if !ok {
		return nil, false
	}
	var result engine.WorkflowResult
	if err := json.Unmarshal(data, &result); err != nil {
		logrus.Warnf("workflow cache unmarshal error: %v", err)
		return nil, false
	}
	return &result, true
}

// CacheSet 设置工作流级缓存。
func CacheSet(workflowName string, inputs map[string]any, result *engine.WorkflowResult) {
	key := keyPrefixWorkflow + workflowName + ":" + HashInputs(inputs)
	data, err := json.Marshal(result)
	if err != nil {
		logrus.Warnf("workflow cache marshal error: %v", err)
		return
	}
	cacheWrite(key, data, workflowTTL())
}

// NodeCacheGet 获取节点级缓存。
func NodeCacheGet(nodeType, configHash string) (map[string]any, bool) {
	key := keyPrefixNode + nodeType + ":" + configHash
	data, ok := cacheRead(key)
	if !ok {
		return nil, false
	}
	var outputs map[string]any
	if err := json.Unmarshal(data, &outputs); err != nil {
		logrus.Warnf("node cache unmarshal error: %v", err)
		return nil, false
	}
	return outputs, true
}

// NodeCacheSet 设置节点级缓存。
func NodeCacheSet(nodeType, configHash string, outputs map[string]any) {
	key := keyPrefixNode + nodeType + ":" + configHash
	data, err := json.Marshal(outputs)
	if err != nil {
		logrus.Warnf("node cache marshal error: %v", err)
		return
	}
	ttl := nodeTTL(nodeType)
	if ttl == 0 {
		return // LLM 等不缓存的节点类型
	}
	cacheWrite(key, data, ttl)
}

// ---------- 内部读写（Redis 优先，内存 fallback） ----------

func cacheRead(key string) ([]byte, bool) {
	rc, ok := store.RedisClient()
	if ok && rc != nil {
		val, err := rc.Get(context.Background(), key).Bytes()
		if err == nil {
			return val, true
		}
		return nil, false
	}
	return fallbackCache.get(key)
}

func cacheWrite(key string, data []byte, ttl time.Duration) {
	rc, ok := store.RedisClient()
	if ok && rc != nil {
		if err := rc.Set(context.Background(), key, data, ttl).Err(); err != nil {
			logrus.Warnf("redis cache set error: %v", err)
		}
		return
	}
	fallbackCache.set(key, data, ttl)
}

func nodeTTL(nodeType string) time.Duration {
	switch nodeType {
	case "http_request":
		return time.Duration(nodeTTLHTTPRequest) * time.Second
	case "knowledge_retrieval":
		return time.Duration(nodeTTLKnowledgeRetrieval) * time.Second
	default:
		// LLM、agent 等节点不缓存
		return 0
	}
}
