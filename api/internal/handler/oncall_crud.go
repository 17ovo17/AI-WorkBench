package handler

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

type OnCallGroup struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Members  []string `json:"members"`
	Schedule string   `json:"schedule"`
	Role     string   `json:"role"`
	Enabled  bool     `json:"enabled"`
}

type OnCallChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Receiver    string `json:"receiver"`
	Endpoint    string `json:"endpoint"`
	Webhook     string `json:"webhook"`
	Secret      string `json:"secret,omitempty"`
	RetryPolicy string `json:"retry_policy"`
	Enabled     bool   `json:"enabled"`
}

type OnCallSchedule struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	GroupID  string `json:"group_id"`
	Window   string `json:"window"`
	Timezone string `json:"timezone"`
	Enabled  bool   `json:"enabled"`
}

type OnCallEscalationStep struct {
	ID        string `json:"id"`
	DelayMin  int    `json:"delay_min"`
	Target    string `json:"target"`
	Action    string `json:"action"`
	Condition string `json:"condition"`
	Enabled   bool   `json:"enabled"`
}

var (
	onCallMu         sync.RWMutex
	onCallGroups     []OnCallGroup
	onCallChannels   []OnCallChannel
	onCallSchedules  []OnCallSchedule
	onCallEscalation []OnCallEscalationStep
	onCallRecords    []onCallTestResponse
)

func GetOnCallConfig(c *gin.Context) {
	onCallMu.RLock()
	defer onCallMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"groups": onCallGroups, "escalation": onCallEscalation, "schedules": onCallSchedules})
}

func SaveOnCallConfig(c *gin.Context) {
	var req struct {
		Groups     []OnCallGroup          `json:"groups"`
		Escalation []OnCallEscalationStep `json:"escalation"`
		Schedules  []OnCallSchedule       `json:"schedules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	onCallMu.Lock()
	onCallGroups = normalizeOnCallGroups(req.Groups)
	onCallEscalation = normalizeOnCallEscalation(req.Escalation)
	if req.Schedules != nil {
		onCallSchedules = normalizeOnCallSchedules(req.Schedules)
	}
	onCallMu.Unlock()
	auditEvent(c, "oncall.config.save", "oncall", "low", "ok", "groups saved", c.GetHeader("X-Test-Batch-Id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func ListOnCallGroups(c *gin.Context) {
	onCallMu.RLock()
	defer onCallMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"items": onCallGroups})
}

func SaveOnCallGroup(c *gin.Context) {
	var item OnCallGroup
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group name required"})
		return
	}
	onCallMu.Lock()
	item = upsertOnCallGroupLocked(item)
	onCallMu.Unlock()
	c.JSON(http.StatusOK, item)
}

func DeleteOnCallGroup(c *gin.Context) {
	if deleteOnCallItem(c.Param("id"), &onCallGroups, func(item OnCallGroup) string { return item.ID }) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func ListOnCallChannels(c *gin.Context) {
	onCallMu.RLock()
	defer onCallMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"items": sanitizedOnCallChannels(onCallChannels)})
}

func SaveOnCallChannel(c *gin.Context) {
	var item OnCallChannel
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isAllowedOnCallChannel(firstNonEmpty(item.Type, item.ID)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported notification channel"})
		return
	}
	onCallMu.Lock()
	item = upsertOnCallChannelLocked(item)
	onCallMu.Unlock()
	c.JSON(http.StatusOK, sanitizeOnCallChannel(item))
}

func DeleteOnCallChannel(c *gin.Context) {
	if deleteOnCallItem(c.Param("id"), &onCallChannels, func(item OnCallChannel) string { return item.ID }) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func ListOnCallSchedules(c *gin.Context) {
	onCallMu.RLock()
	defer onCallMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"items": onCallSchedules})
}

func SaveOnCallSchedule(c *gin.Context) {
	var item OnCallSchedule
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "schedule name required"})
		return
	}
	onCallMu.Lock()
	item = upsertOnCallScheduleLocked(item)
	onCallMu.Unlock()
	c.JSON(http.StatusOK, item)
}

func DeleteOnCallSchedule(c *gin.Context) {
	if deleteOnCallItem(c.Param("id"), &onCallSchedules, func(item OnCallSchedule) string { return item.ID }) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
}

func ListOnCallRecords(c *gin.Context) {
	onCallMu.RLock()
	defer onCallMu.RUnlock()
	items := make([]onCallTestResponse, len(onCallRecords))
	copy(items, onCallRecords)
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func appendOnCallRecord(record onCallTestResponse) {
	onCallMu.Lock()
	defer onCallMu.Unlock()
	onCallRecords = append([]onCallTestResponse{record}, onCallRecords...)
	if len(onCallRecords) > 100 {
		onCallRecords = onCallRecords[:100]
	}
}

func normalizeOnCallGroups(items []OnCallGroup) []OnCallGroup {
	out := make([]OnCallGroup, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if item.ID == "" {
			item.ID = "group_" + store.NewID()
		}
		out = append(out, item)
	}
	return out
}

func normalizeOnCallEscalation(items []OnCallEscalationStep) []OnCallEscalationStep {
	out := make([]OnCallEscalationStep, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Target) == "" || strings.TrimSpace(item.Action) == "" {
			continue
		}
		if item.ID == "" {
			item.ID = "esc_" + store.NewID()
		}
		out = append(out, item)
	}
	return out
}

func normalizeOnCallSchedules(items []OnCallSchedule) []OnCallSchedule {
	out := make([]OnCallSchedule, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		if item.ID == "" {
			item.ID = "schedule_" + store.NewID()
		}
		out = append(out, item)
	}
	return out
}

func upsertOnCallGroupLocked(item OnCallGroup) OnCallGroup {
	if item.ID == "" {
		item.ID = "group_" + store.NewID()
	}
	for i := range onCallGroups {
		if onCallGroups[i].ID == item.ID {
			onCallGroups[i] = item
			return item
		}
	}
	onCallGroups = append(onCallGroups, item)
	return item
}

func upsertOnCallChannelLocked(item OnCallChannel) OnCallChannel {
	item.Type = firstNonEmpty(strings.TrimSpace(item.Type), "console")
	if item.ID == "" {
		item.ID = item.Type + "_" + store.NewID()
	}
	if item.Webhook == "" {
		item.Webhook = item.Endpoint
	}
	for i := range onCallChannels {
		if onCallChannels[i].ID == item.ID {
			onCallChannels[i] = item
			return item
		}
	}
	onCallChannels = append(onCallChannels, item)
	return item
}

func upsertOnCallScheduleLocked(item OnCallSchedule) OnCallSchedule {
	if item.ID == "" {
		item.ID = "schedule_" + store.NewID()
	}
	if item.Timezone == "" {
		item.Timezone = "Asia/Shanghai"
	}
	for i := range onCallSchedules {
		if onCallSchedules[i].ID == item.ID {
			onCallSchedules[i] = item
			return item
		}
	}
	onCallSchedules = append(onCallSchedules, item)
	return item
}

func deleteOnCallItem[T any](id string, items *[]T, idFn func(T) string) bool {
	onCallMu.Lock()
	defer onCallMu.Unlock()
	for i, item := range *items {
		if idFn(item) == id {
			*items = append((*items)[:i], (*items)[i+1:]...)
			return true
		}
	}
	return false
}

func sanitizedOnCallChannels(items []OnCallChannel) []OnCallChannel {
	out := make([]OnCallChannel, len(items))
	for i, item := range items {
		out[i] = sanitizeOnCallChannel(item)
	}
	return out
}

func sanitizeOnCallChannel(item OnCallChannel) OnCallChannel {
	item.Secret = ""
	return item
}

func init() {
	onCallChannels = []OnCallChannel{{ID: "console", Name: "Console", Type: "console", Receiver: "平台值班人员", Enabled: true}}
	onCallGroups = []OnCallGroup{{ID: "primary", Name: "SRE 主值", Members: []string{"admin"}, Schedule: "7x24", Role: "primary", Enabled: true}}
	onCallSchedules = []OnCallSchedule{{ID: "default", Name: "默认 7x24", GroupID: "primary", Window: "7x24", Timezone: "Asia/Shanghai", Enabled: true}}
	onCallEscalation = []OnCallEscalationStep{{ID: "esc_default", DelayMin: 15, Target: "SRE 主值", Action: "升级通知", Condition: "P0/P1 未确认", Enabled: true}}
	onCallRecords = []onCallTestResponse{{ID: "notify_seed", Channel: "console", Receiver: "SRE 主值", Status: "success", Detail: "默认值班通知记录", TraceID: "notify_seed", CreatedAt: time.Now()}}
}
