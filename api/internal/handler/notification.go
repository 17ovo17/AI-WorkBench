package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// NotificationChannel 通知渠道配置（钉钉/飞书/企业微信）。
type NotificationChannel struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // dingtalk / feishu / wecom
	Name     string `json:"name"`
	Endpoint string `json:"endpoint,omitempty"`
	Receiver string `json:"receiver,omitempty"`
	Secret   string `json:"secret,omitempty"`
	Webhook  string `json:"webhook"`
	Enabled  bool   `json:"enabled"`
}

var (
	notificationChannels   []NotificationChannel
	notificationChannelsMu sync.RWMutex
)

// ListNotificationChannels GET /api/v1/notifications/channels
func ListNotificationChannels(c *gin.Context) {
	notificationChannelsMu.RLock()
	defer notificationChannelsMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"items": notificationChannels})
}

// SaveNotificationChannel POST /api/v1/notifications/channels
func SaveNotificationChannel(c *gin.Context) {
	var ch NotificationChannel
	if err := c.ShouldBindJSON(&ch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ch.ID == "" {
		ch.ID = fmt.Sprintf("nc_%d", time.Now().UnixNano())
	}
	if ch.Webhook == "" {
		ch.Webhook = ch.Endpoint
	}
	if ch.Endpoint == "" {
		ch.Endpoint = ch.Webhook
	}

	notificationChannelsMu.Lock()
	found := false
	for i, existing := range notificationChannels {
		if existing.ID == ch.ID {
			notificationChannels[i] = ch
			found = true
			break
		}
	}
	if !found {
		notificationChannels = append(notificationChannels, ch)
	}
	notificationChannelsMu.Unlock()

	ch.Secret = ""
	c.JSON(http.StatusOK, ch)
}

// DeleteNotificationChannel DELETE /api/v1/notifications/channels/:id
func DeleteNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	notificationChannelsMu.Lock()
	defer notificationChannelsMu.Unlock()
	for i, ch := range notificationChannels {
		if ch.ID == id {
			notificationChannels = append(notificationChannels[:i], notificationChannels[i+1:]...)
			c.JSON(http.StatusOK, gin.H{"ok": true})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

// SendNotification 发送通知到所有启用的渠道。
func SendNotification(title, content, level string) {
	notificationChannelsMu.RLock()
	defer notificationChannelsMu.RUnlock()
	for _, ch := range notificationChannels {
		if !ch.Enabled {
			continue
		}
		go sendToChannel(ch, title, content, level)
	}
}

func sendToChannel(ch NotificationChannel, title, content, level string) {
	payload := buildChannelPayload(ch.Type, title, content, level)
	if payload == nil {
		return
	}
	resp, err := http.Post(ch.Webhook, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.WithError(err).Warnf("notification: send to %s(%s) failed", ch.Name, ch.Type)
		return
	}
	resp.Body.Close()
}

func buildChannelPayload(chType, title, content, level string) []byte {
	var payload []byte
	switch chType {
	case "dingtalk":
		payload, _ = json.Marshal(map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  fmt.Sprintf("### %s\n%s\n> 级别: %s", title, content, level),
			},
		})
	case "feishu":
		payload, _ = json.Marshal(map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": fmt.Sprintf("[%s] %s\n%s", level, title, content)},
		})
	case "wecom":
		payload, _ = json.Marshal(map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": fmt.Sprintf("### %s\n%s\n> 级别: %s", title, content, level),
			},
		})
	default:
		return nil
	}
	return payload
}
