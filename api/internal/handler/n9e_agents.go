package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type N9eHostMeta struct {
	AgentVersion string            `json:"agent_version"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	Hostname     string            `json:"hostname"`
	CpuNum       int               `json:"cpu_num"`
	CpuUtil      float64           `json:"cpu_util"`
	MemUtil      float64           `json:"mem_util"`
	HostIP       string            `json:"host_ip"`
	UnixTime     int64             `json:"unixtime"`
	RemoteAddr   string            `json:"remote_addr"`
	GlobalLabels map[string]string `json:"global_labels"`
}

type N9eAgent struct {
	Ident        string  `json:"ident"`
	IP           string  `json:"ip"`
	Hostname     string  `json:"hostname"`
	OS           string  `json:"os"`
	Version      string  `json:"version"`
	CpuUtil      float64 `json:"cpu_util"`
	MemUtil      float64 `json:"mem_util"`
	Status       string  `json:"status"`
	LastSeen     int64   `json:"last_seen"`
	LastSeenText string  `json:"last_seen_text"`
}

func n9eRedisClient() *redis.Client {
	addr := viper.GetString("n9e.redis_addr")
	if addr == "" {
		addr = viper.GetString("redis.addr")
	}
	if addr == "" {
		return nil
	}
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: viper.GetString("n9e.redis_password"),
		DB:       viper.GetInt("n9e.redis_db"),
	})
}

func ListN9eAgents(c *gin.Context) {
	rdb := n9eRedisClient()
	if rdb == nil {
		c.JSON(http.StatusOK, []N9eAgent{})
		return
	}
	defer rdb.Close()
	ctx := context.Background()

	agents := []N9eAgent{}
	now := time.Now().Unix()

	keys, err := rdb.Keys(ctx, "n9e_meta_*").Result()
	if err != nil {
		logrus.Warnf("n9e redis scan: %v", err)
		c.JSON(http.StatusOK, agents)
		return
	}

	for _, key := range keys {
		if strings.Contains(key, "update_time") || strings.Contains(key, "extend") {
			continue
		}
		val, err := rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var meta N9eHostMeta
		if err := json.Unmarshal([]byte(val), &meta); err != nil {
			continue
		}
		ident := strings.TrimPrefix(key, "n9e_meta_")

		status := "offline"
		lastSeen := meta.UnixTime / 1000
		if lastSeen == 0 {
			lastSeen = meta.UnixTime
		}
		delta := now - lastSeen
		if delta < 60 {
			status = "online"
		} else if delta < 180 {
			status = "warning"
		}

		agents = append(agents, N9eAgent{
			Ident:        ident,
			IP:           meta.HostIP,
			Hostname:     meta.Hostname,
			OS:           meta.OS,
			Version:      meta.AgentVersion,
			CpuUtil:      meta.CpuUtil,
			MemUtil:      meta.MemUtil,
			Status:       status,
			LastSeen:     lastSeen,
			LastSeenText: time.Unix(lastSeen, 0).Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, agents)
}

func N9eAgentStatus(ip string) string {
	rdb := n9eRedisClient()
	if rdb == nil {
		return "unknown"
	}
	defer rdb.Close()
	ctx := context.Background()

	keys, _ := rdb.Keys(ctx, "n9e_meta_*"+strings.ReplaceAll(ip, ".", "*")+"*").Result()
	now := time.Now().Unix()
	for _, key := range keys {
		if strings.Contains(key, "update_time") || strings.Contains(key, "extend") {
			continue
		}
		val, err := rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var meta N9eHostMeta
		if err := json.Unmarshal([]byte(val), &meta); err != nil {
			continue
		}
		lastSeen := meta.UnixTime / 1000
		if lastSeen == 0 {
			lastSeen = meta.UnixTime
		}
		delta := now - lastSeen
		if delta < 60 {
			return "online"
		} else if delta < 180 {
			return "warning"
		}
	}
	return "offline"
}
