package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

func cleanIP(value string) (string, bool) {
	ip := strings.TrimSpace(value)
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.String() != ip {
		return "", false
	}
	if parsed.IsUnspecified() || parsed.IsMulticast() || parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() {
		return "", false
	}
	return ip, true
}

func CatpawHeartbeat(c *gin.Context) {
	var a model.CatpawAgent
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ip, ok := cleanIP(a.IP); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid ip is required"})
		return
	} else {
		a.IP = ip
	}
	a.LastSeen = time.Now()
	store.UpsertAgent(&a)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func CatpawReport(c *gin.Context) {
	var body struct {
		IP     string `json:"ip"`
		Report string `json:"report"`
		Title  string `json:"title"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ip, ok := cleanIP(body.IP); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid ip is required"})
		return
	} else {
		body.IP = ip
	}
	summary := summarizeCatpawReport(body.Report)
	now := time.Now()
	rec := &model.DiagnoseRecord{
		ID:            fmt.Sprintf("%d", now.UnixNano()),
		TargetIP:      body.IP,
		Trigger:       "catpaw",
		Source:        "catpaw",
		Status:        model.StatusDone,
		Report:        summary,
		SummaryReport: summary,
		RawReport:     body.Report,
		AlertTitle:    body.Title,
		CreateTime:    now,
		EndTime:       &now,
	}
	store.AddRecord(rec)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func summarizeCatpawReport(report string) string {
	if strings.TrimSpace(report) == "" {
		return "# Catpaw Inspection Summary\n\n- No valid inspection content was received."
	}
	if !strings.Contains(report, "Catpaw Windows") {
		if len([]rune(report)) > 4000 {
			return string([]rune(report)[:4000]) + "\n\n> Raw report was truncated here. The full content is stored in raw_report."
		}
		return report
	}
	return summarizeWindowsCatpawReport(report)
}

func summarizeWindowsCatpawReport(report string) string {
	cpu := asMap(pluginPayload(report, "windows.cpu"))
	mem := asMap(pluginPayload(report, "windows.mem"))
	disks := asList(pluginPayload(report, "windows.disk"))
	diskIO := asList(pluginPayload(report, "windows.diskio"))
	netInfo := asMap(pluginPayload(report, "windows.net"))
	tcpInfo := asMap(pluginPayload(report, "windows.tcpstate"))
	processInfo := asMap(pluginPayload(report, "windows.process"))
	serviceInfo := asMap(pluginPayload(report, "windows.service"))
	events := normalizeWindowsEvents(asList(pluginPayload(report, "windows.eventlog")))
	firewallInfo := asMap(pluginPayload(report, "windows.firewall"))
	updateInfo := asMap(pluginPayload(report, "windows.update"))
	plugins := discoveredPluginNames(report)

	cpuUsed := numberField(cpu, "CPUUsedPercent")
	cpuQueue := numberField(cpu, "ProcessorQueueLength")
	memUsed := numberField(mem, "MemUsedPercent")
	diskMax := maxNumberField(disks, "UsedPercent")
	autoStopped := normalizeServiceRows(asList(serviceInfo["AutoServicesStopped"]))
	timeWait := stateCount(tcpInfo, "TimeWait")
	established := stateCount(tcpInfo, "Established")
	listen := len(asList(tcpInfo["ListeningTCP"]))
	eventErrors := len(events)

	risks := make([]string, 0, 6)
	if cpuUsed >= 80 {
		risks = append(risks, fmt.Sprintf("CPU usage %.2f%% is high. Check Top CPU processes.", cpuUsed))
	}
	if cpuQueue >= 4 {
		risks = append(risks, fmt.Sprintf("CPU queue %.2f is high. There may be runnable queue pressure.", cpuQueue))
	}
	if memUsed >= 85 {
		risks = append(risks, fmt.Sprintf("Memory usage %.2f%% is high. Check Top memory processes and page file usage.", memUsed))
	}
	if diskMax >= 85 {
		risks = append(risks, fmt.Sprintf("Maximum disk usage %.2f%% is high. Check the affected drive free space.", diskMax))
	}
	if len(autoStopped) > 0 {
		risks = append(risks, fmt.Sprintf("Found %d auto-start services that are not running. Verify whether they affect business services.", len(autoStopped)))
	}
	if eventErrors > 0 {
		risks = append(risks, fmt.Sprintf("Found %d critical/error events in the last 24 hours.", eventErrors))
	}
	if len(risks) == 0 {
		risks = append(risks, "No obvious high-risk pressure was found. Continue monitoring trends and business port availability.")
	}

	var sb strings.Builder
	sb.WriteString("# Catpaw Windows Inspection Summary\n\n")
	sb.WriteString("- Data source: Catpaw Windows native inspection plus platform-side structured denoising.\n")
	sb.WriteString("- Display policy: key conclusions, Top-N details, and risks are shown by default; full raw plugin JSON is kept in raw_report for folding or download.\n")
	sb.WriteString(fmt.Sprintf("- Collected plugins: %s\n\n", strings.Join(plugins, ", ")))

	sb.WriteString("## Overall Conclusion\n")
	for _, risk := range risks {
		sb.WriteString("- " + risk + "\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Base Resources\n")
	sb.WriteString(fmt.Sprintf("- CPU: usage %s, queue %s, processor groups %d.\n", percentText(cpuUsed), numberText(cpuQueue), len(asList(cpu["Processors"]))))
	sb.WriteString(fmt.Sprintf("- Memory: usage %s, total %s MB, free %s MB, free virtual %s MB.\n", percentText(memUsed), anyText(mem["TotalMemoryMB"]), anyText(mem["FreeMemoryMB"]), anyText(mem["FreeVirtualMemoryMB"])))
	sb.WriteString(fmt.Sprintf("- Disk: %d local disks, highest usage %s.\n", len(disks), percentText(diskMax)))
	sb.WriteString(fmt.Sprintf("- TCP: Established=%d, TIME_WAIT=%d, listening port samples=%d.\n\n", established, timeWait, listen))

	sb.WriteString("## CPU / Memory Top Details\n")
	sb.WriteString(markdownTable("Top CPU processes", asList(cpu["TopCPUProcesses"]), []string{"ProcessName", "Id", "CPU", "ThreadCount"}, 8))
	sb.WriteString(markdownTable("Top memory processes", asList(mem["TopMemoryProcesses"]), []string{"ProcessName", "Id", "WorkingSetMB", "PagedMemoryMB"}, 8))
	sb.WriteString(markdownTable("Top handle processes", asList(processInfo["TopHandles"]), []string{"ProcessName", "Id", "Handles", "ThreadCount"}, 6))

	sb.WriteString("## Disk / IO\n")
	sb.WriteString(markdownTable("Logical disks", disks, []string{"DeviceID", "VolumeName", "FileSystem", "SizeGB", "FreeGB", "UsedPercent"}, 8))
	sb.WriteString(markdownTable("Physical disk IO samples", diskIO, []string{"InstanceName", "Path", "Value"}, 10))

	sb.WriteString("## Network / Ports\n")
	sb.WriteString(markdownTable("Network adapters", asList(netInfo["Adapters"]), []string{"Name", "Status", "LinkSpeed", "MacAddress"}, 8))
	sb.WriteString(markdownTable("Network traffic and errors", asList(netInfo["Statistics"]), []string{"Name", "ReceivedBytes", "SentBytes", "ReceivedDiscardedPackets", "OutboundDiscardedPackets", "ReceivedPacketErrors", "OutboundPacketErrors"}, 8))
	sb.WriteString(markdownTable("TCP state summary", asList(tcpInfo["TCPStateSummary"]), []string{"Name", "Count"}, 12))
	sb.WriteString(markdownTable("Listening TCP port samples", asList(tcpInfo["ListeningTCP"]), []string{"LocalAddress", "LocalPort", "OwningProcess"}, 12))

	sb.WriteString("## Services / Events / Security\n")
	sb.WriteString(markdownTable("Critical service status", normalizeServiceRows(asList(serviceInfo["CriticalServices"])), []string{"Name", "DisplayName", "Status", "StartType"}, 10))
	sb.WriteString(markdownTable("Auto-start services not running", autoStopped, []string{"Name", "DisplayName", "State", "StartMode"}, 10))
	sb.WriteString(markdownTable("Recent critical/error events", events, []string{"TimeCreated", "LogName", "ProviderName", "Id", "LevelDisplayName", "Message"}, 8))
	sb.WriteString(markdownTable("Firewall profiles", asList(firewallInfo["FirewallProfiles"]), []string{"Name", "Enabled", "DefaultInboundAction", "DefaultOutboundAction"}, 6))
	sb.WriteString(markdownTable("Remote management rules", asList(firewallInfo["RemoteManagementRules"]), []string{"DisplayName", "Enabled", "Direction", "Action", "Profile"}, 10))

	sb.WriteString("## OS / Patches\n")
	sb.WriteString(markdownTable("Operating system", asList(updateInfo["OS"]), []string{"Caption", "Version", "BuildNumber", "LastBootUpTime"}, 2))
	sb.WriteString(markdownTable("Recent hotfixes", asList(updateInfo["RecentHotfix"]), []string{"HotFixID", "Description", "InstalledOn"}, 10))

	sb.WriteString("## Recommendations\n")
	sb.WriteString("- Keep Top-N and key fields in the summary; expand or download raw_report for deep investigation.\n")
	sb.WriteString("- Windows license, activation, storage optimization, or service-control events must be judged by Provider, Id, error code, and business impact.\n")
	sb.WriteString("- When Prometheus/Categraf metrics are available, prefer trend-based pressure analysis over one-shot event interpretation.\n")
	return sb.String()
}

func pluginPayload(report, plugin string) any {
	pluginIndex := strings.Index(report, plugin)
	if pluginIndex < 0 {
		return nil
	}
	jsonStart := strings.Index(report[pluginIndex:], "```json")
	if jsonStart < 0 {
		return nil
	}
	payloadStart := pluginIndex + jsonStart + len("```json")
	jsonEnd := strings.Index(report[payloadStart:], "```")
	if jsonEnd < 0 {
		return nil
	}
	jsonText := report[payloadStart : payloadStart+jsonEnd]
	var payload any
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(jsonText)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil
	}
	return payload
}

func discoveredPluginNames(report string) []string {
	re := regexp.MustCompile(`windows\.[a-z0-9]+`)
	matches := re.FindAllString(report, -1)
	seen := map[string]bool{}
	plugins := make([]string, 0, len(matches))
	for _, match := range matches {
		if !seen[match] {
			seen[match] = true
			plugins = append(plugins, match)
		}
	}
	sort.Strings(plugins)
	return plugins
}

func asMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}
func asList(value any) []map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return []map[string]any{typed}
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if row, ok := item.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}
func numberField(row map[string]any, key string) float64 {
	if row == nil {
		return 0
	}
	return toFloat(row[key])
}
func maxNumberField(rows []map[string]any, key string) float64 {
	maxValue := 0.0
	for _, row := range rows {
		if value := numberField(row, key); value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}
func toFloat(value any) float64 {
	switch typed := value.(type) {
	case json.Number:
		v, _ := typed.Float64()
		return v
	case float64:
		return typed
	case int:
		return float64(typed)
	case string:
		v, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return v
	default:
		return 0
	}
}
func stateCount(tcpInfo map[string]any, state string) int {
	for _, row := range asList(tcpInfo["TCPStateSummary"]) {
		if strings.EqualFold(anyText(row["Name"]), state) {
			return int(toFloat(row["Count"]))
		}
	}
	return 0
}

func markdownTable(title string, rows []map[string]any, columns []string, limit int) string {
	if len(rows) == 0 {
		return fmt.Sprintf("### %s\n\n- Not collected or no result.\n\n", title)
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	var sb strings.Builder
	sb.WriteString("### " + title + "\n\n")
	sb.WriteString("| " + strings.Join(columns, " | ") + " |\n")
	separators := make([]string, len(columns))
	for i := range separators {
		separators[i] = "---"
	}
	sb.WriteString("| " + strings.Join(separators, " | ") + " |\n")
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, tableText(row[column]))
		}
		sb.WriteString("| " + strings.Join(values, " | ") + " |\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func tableText(value any) string {
	text := strings.ReplaceAll(anyText(value), "|", "\\|")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = normalizeWindowsDate(text)
	if isMojibake(text) {
		text = "Unreadable encoded text; raw value is kept in raw_report. Use Provider/Id/error-code for investigation."
	}
	if len([]rune(text)) > 160 {
		return string([]rune(text)[:160]) + "..."
	}
	if strings.TrimSpace(text) == "" {
		return "-"
	}
	return text
}

func anyText(value any) string {
	switch typed := value.(type) {
	case nil:
		return "not collected"
	case json.Number:
		return typed.String()
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case []any, map[string]any:
		b, _ := json.Marshal(typed)
		return string(b)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
func percentText(value float64) string {
	if value == 0 {
		return "not collected"
	}
	return fmt.Sprintf("%.2f%%", value)
}
func numberText(value float64) string {
	if value == 0 {
		return "not collected"
	}
	return fmt.Sprintf("%.2f", value)
}

func normalizeServiceRows(rows []map[string]any) []map[string]any {
	for _, row := range rows {
		if name := anyText(row["Name"]); name == "edgeupdate" && isMojibake(anyText(row["DisplayName"])) {
			row["DisplayName"] = "Microsoft Edge Update Service (edgeupdate)"
		}
	}
	return rows
}
func normalizeWindowsEvents(rows []map[string]any) []map[string]any {
	for _, row := range rows {
		row["TimeCreated"] = normalizeWindowsDate(anyText(row["TimeCreated"]))
		if isMojibake(anyText(row["LevelDisplayName"])) {
			row["LevelDisplayName"] = levelNameFromID(row)
		}
		if message := normalizeWindowsEventMessage(row); message != "" {
			row["Message"] = message
		}
	}
	return rows
}
func normalizeWindowsDate(text string) string {
	re := regexp.MustCompile(`/Date\((\d+)\)/`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		ms, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return match
		}
		return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
	})
}
func isMojibake(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	bad := strings.Count(text, "?")
	return strings.Contains(text, "????") || bad >= 1 || strings.Contains(text, "\ufffd")
}
func levelNameFromID(row map[string]any) string {
	text := strings.ToLower(anyText(row["LevelDisplayName"]))
	if strings.Contains(text, "critical") {
		return "Critical"
	}
	return "Error"
}
func normalizeWindowsEventMessage(row map[string]any) string {
	provider := anyText(row["ProviderName"])
	id := int(toFloat(row["Id"]))
	message := normalizeWindowsDate(anyText(row["Message"]))
	if !isMojibake(message) && strings.TrimSpace(message) != "" {
		return message
	}
	switch provider {
	case "Microsoft-Windows-Security-SPP":
		switch id {
		case 1014:
			return "Security-SPP license acquisition failed. Commonly related to Windows activation/licensing; check hr=0xC004C060 and business impact."
		case 8200:
			return "Security-SPP detailed license acquisition failure event. Check activation state, licensing service, and recent system changes."
		case 8198:
			return "Security-SPP activation-related action failed. Check slui.exe, licensing service, and error code."
		}
	case "Microsoft-Windows-Defrag":
		return "Storage optimization event. The volume or virtual disk may not support this operation; correlate with disk health and IO metrics."
	}
	return "Event message encoding is unreadable. The original value is kept in raw_report; investigate by Provider, Id, and error code."
}

func ListAgents(c *gin.Context) {
	c.JSON(http.StatusOK, store.ListAgents())
}

func DeleteAgent(c *gin.Context) {
	ip, ok := cleanIP(c.Param("ip"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid ip is required"})
		return
	}
	store.DeleteAgent(ip)
	auditEvent(c, "catpaw.agent.delete", ip, "L2", "allow", "agent record removed by user confirmation", c.Query("test_batch_id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DeleteDiagnose(c *gin.Context) {
	store.DeleteRecord(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func CleanupDiagnose(c *gin.Context) {
	scope := strings.TrimSpace(c.Query("scope"))
	batchID := strings.TrimSpace(c.Query("test_batch_id"))
	businessID := strings.TrimSpace(c.Query("business_id"))
	if scope == "" && batchID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope or test_batch_id is required"})
		return
	}
	matchesBusiness := func(r *model.DiagnoseRecord) bool {
		if businessID == "" {
			return true
		}
		joined := strings.Join([]string{r.TargetIP, r.AlertTitle, r.RawReport, r.Report, r.SummaryReport}, " ")
		return strings.Contains(joined, businessID)
	}
	deleted := store.DeleteRecordsByFilter(func(r *model.DiagnoseRecord) bool {
		if batchID != "" {
			joined := strings.Join([]string{r.ID, r.TargetIP, r.Trigger, r.Source, r.DataSource, r.AlertTitle, r.RawReport, r.Report}, " ")
			return strings.Contains(joined, batchID) && matchesBusiness(r)
		}
		switch scope {
		case "business_inspection":
			isInspection := r.Trigger == "business_inspection" || r.Source == "business_inspection" || r.DataSource == "business_inspection"
			return isInspection && matchesBusiness(r)
		case "test":
			joined := strings.ToLower(strings.Join([]string{r.ID, r.TargetIP, r.Trigger, r.Source, r.DataSource, r.AlertTitle}, " "))
			return (strings.Contains(joined, "test") || strings.Contains(joined, "whitebox") || strings.Contains(joined, "aiw-")) && matchesBusiness(r)
		default:
			return false
		}
	})
	scopeLabel := scope
	if businessID != "" {
		scopeLabel = scopeLabel + ":" + businessID
	}
	auditEvent(c, "diagnose.cleanup", scopeLabel, "L3", "allow", fmt.Sprintf("deleted %d diagnose records", deleted), batchID)
	c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": deleted, "scope": scope, "business_id": businessID})
}

func ListDiagnose(c *gin.Context) {
	records := store.ListRecords()
	if businessID := strings.TrimSpace(c.Query("business_id")); businessID != "" {
		filtered := []*model.DiagnoseRecord{}
		for _, r := range records {
			if strings.Contains(r.TargetIP, businessID) || strings.Contains(r.AlertTitle, businessID) || strings.Contains(r.RawReport, businessID) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		filtered := []*model.DiagnoseRecord{}
		for _, r := range records {
			if r.Source == source || r.DataSource == source || r.Trigger == source {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}
	c.JSON(http.StatusOK, records)
}

func StartDiagnose(c *gin.Context) {
	var req struct {
		IP           string           `json:"ip" binding:"required"`
		Prompt       string           `json:"prompt"`
		CredentialID string           `json:"credential_id"`
		Credential   RemoteCredential `json:"credential"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ip, ok := cleanIP(req.IP); !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid ip is required"})
		return
	} else {
		req.IP = ip
	}
	now := time.Now()
	source := "prometheus"
	if store.HasOnlineAgent(req.IP) {
		source = "catpaw"
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = fmt.Sprintf("请对主机 %s 进行全面的健康诊断，分析 CPU、内存、磁盘、网络等关键指标，给出根因分析和处置建议。", req.IP)
	}
	rec := &model.DiagnoseRecord{
		ID:         fmt.Sprintf("%d", now.UnixNano()),
		TargetIP:   req.IP,
		Trigger:    "manual",
		Source:     source,
		DataSource: source,
		Status:     model.StatusPending,
		CreateTime: now,
	}
	store.AddRecord(rec)
	go RunDiagnoseWithOptions(rec, DiagnoseOptions{Prompt: prompt, CredentialID: req.CredentialID, Credential: req.Credential})
	c.JSON(http.StatusOK, gin.H{"id": rec.ID, "source": source})
}
