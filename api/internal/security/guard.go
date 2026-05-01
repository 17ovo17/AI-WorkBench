package security

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type Decision struct {
	Allowed bool   `json:"allowed"`
	Level   string `json:"level"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

var forbiddenSnippets = []string{
	"rm -rf /",
	"rm -rf /*",
	"del c:\\",
	"rd /s /q c:\\",
	"format c:",
	"mkfs",
	"drop database ai_workbench",
	"flushall",
	"netsh advfirewall set allprofiles state off",
	"set-netfirewallprofile -enabled false",
}

var dangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rm\s+-rf`),
	regexp.MustCompile(`(?i)remove-item\s+.*-recurse`),
	regexp.MustCompile(`(?i)\bdel\s+`),
	regexp.MustCompile(`(?i)\brd\s+/s`),
	regexp.MustCompile(`(?i)drop\s+database`),
	regexp.MustCompile(`(?i)truncate\s+table`),
	regexp.MustCompile(`(?i)flushall`),
	regexp.MustCompile(`(?i)\bformat\s+`),
	regexp.MustCompile(`(?i)\bmkfs`),
	regexp.MustCompile(`(?i)stop-process`),
	regexp.MustCompile(`(?i)\bpkill\b`),
	regexp.MustCompile(`(?i)schtasks\s+/delete`),
}

var allowedRemoteHosts = map[string]bool{
	"192.168.1.7": true,
	"127.0.0.1":   true,
	"localhost":   true,
}

func ClassifyCommand(command string) Decision {
	variants := commandVariants(command)
	for _, variant := range variants {
		lower := strings.ToLower(variant)
		for _, snippet := range forbiddenSnippets {
			if strings.Contains(lower, snippet) {
				return Decision{Allowed: false, Level: "L4", Action: "block", Reason: "forbidden command matched: " + snippet}
			}
		}
	}
	for _, variant := range variants {
		for _, pattern := range dangerPatterns {
			if pattern.MatchString(variant) {
				return Decision{Allowed: false, Level: "L3", Action: "confirm_required", Reason: "danger pattern matched: " + pattern.String()}
			}
		}
	}
	if len(command) > 4096 {
		return Decision{Allowed: false, Level: "L2", Action: "confirm_required", Reason: "command length exceeds 4096"}
	}
	return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "no dangerous pattern matched"}
}

func commandVariants(command string) []string {
	variants := []string{command, normalizeCommand(command)}
	for _, decoded := range decodePowerShellEncodedCommands(command) {
		variants = append(variants, decoded, normalizeCommand(decoded))
	}
	return variants
}

func normalizeCommand(command string) string {
	replacer := strings.NewReplacer("`", "", "^", "", "\\\n", "", "\\\r", "", "\"", "", "'", "")
	normalized := replacer.Replace(command)
	return strings.Join(strings.Fields(normalized), " ")
}

func decodePowerShellEncodedCommands(command string) []string {
	pattern := regexp.MustCompile(`(?i)(?:-|/)(?:enc|encodedcommand)\s+([A-Za-z0-9+/=]+)`)
	matches := pattern.FindAllStringSubmatch(command, -1)
	decoded := make([]string, 0, len(matches))
	for _, match := range matches {
		raw, err := base64.StdEncoding.DecodeString(match[1])
		if err != nil || len(raw) == 0 {
			continue
		}
		if text := decodeUTF16LE(raw); strings.TrimSpace(text) != "" {
			decoded = append(decoded, text)
		}
		if text := string(raw); strings.TrimSpace(text) != "" {
			decoded = append(decoded, text)
		}
	}
	return decoded
}

func decodeUTF16LE(raw []byte) string {
	if len(raw)%2 != 0 {
		return ""
	}
	runes := make([]rune, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		r := rune(raw[i]) | rune(raw[i+1])<<8
		if r == 0 {
			continue
		}
		runes = append(runes, r)
	}
	return string(runes)
}

func ValidateRemoteHost(host string) Decision {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	if trimmed == "" {
		return Decision{Allowed: false, Level: "L2", Action: "reject", Reason: "remote host is required"}
	}
	if allowedRemoteHosts[trimmed] {
		return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "remote host is in test whitelist"}
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		if ip.IsLoopback() {
			return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "loopback host is allowed for local test"}
		}
		if isLocalInterfaceIP(ip) {
			return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "local WSL/Linux interface is allowed"}
		}
		if isPrivateLabIP(ip) {
			return Decision{Allowed: true, Level: "L1", Action: "allow", Reason: "private lab host is allowed for Catpaw onboarding"}
		}
	}
	return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "remote host is outside safety whitelist"}
}

func ValidateConfirm(decision Decision, confirm string) Decision {
	if decision.Level == "L0" || decision.Allowed {
		return decision
	}
	if decision.Level == "L4" {
		return decision
	}
	if strings.EqualFold(strings.TrimSpace(confirm), "ALLOW-"+decision.Level) || strings.EqualFold(strings.TrimSpace(confirm), "CONFIRM") {
		decision.Allowed = true
		decision.Action = "confirmed"
		decision.Reason += "; confirmation accepted"
		return decision
	}
	return decision
}

func ValidatePlatformURL(raw string) Decision {
	if strings.TrimSpace(raw) == "" {
		return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "empty platform URL uses default"}
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return Decision{Allowed: false, Level: "L2", Action: "reject", Reason: "invalid platform URL"}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "only http/https platform URLs are allowed"}
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "local platform URL accepted"}
	}
	if strings.EqualFold(host, "metadata.google.internal") {
		return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "metadata/SSRF platform URL is forbidden"}
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedMetadataIP(ip) {
			return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "metadata/link-local platform URL is forbidden"}
		}
		if ip.IsLoopback() || isLocalInterfaceIP(ip) {
			return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "local platform URL accepted"}
		}
		if isBlockedSSRFIP(ip) {
			return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "non-local private platform URL is forbidden"}
		}
	}
	return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "platform URL accepted"}
}

func ValidateNetworkProbeTarget(host string) Decision {
	if decision := ValidateRemoteHost(host); decision.Allowed {
		return decision
	}
	return Decision{Allowed: false, Level: "L3", Action: "reject", Reason: "network probe target is outside safety whitelist"}
}

func ValidateTestBatch(batchID string) Decision {
	trimmed := strings.TrimSpace(batchID)
	if trimmed == "" {
		return Decision{Allowed: false, Level: "L2", Action: "reject", Reason: "test_batch_id is required for test mutations"}
	}
	if !regexp.MustCompile(`^aiw-[A-Za-z0-9._:-]+$`).MatchString(trimmed) {
		return Decision{Allowed: false, Level: "L2", Action: "reject", Reason: "test_batch_id must start with aiw-"}
	}
	return Decision{Allowed: true, Level: "L0", Action: "allow", Reason: "test_batch_id accepted"}
}

func isLocalInterfaceIP(ip net.IP) bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var candidate net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				candidate = value.IP
			case *net.IPAddr:
				candidate = value.IP
			}
			if candidate != nil && candidate.Equal(ip) {
				return true
			}
		}
	}
	return false
}

func isPrivateLabIP(ip net.IP) bool {
	text := ip.String()
	return ip.IsPrivate() || strings.HasPrefix(text, "198.18.") || strings.HasPrefix(text, "198.19.")
}

func isBlockedMetadataIP(ip net.IP) bool {
	text := ip.String()
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || strings.HasPrefix(text, "169.254.")
}

func isBlockedSSRFIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	privateCIDRs := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "fc00::/7", "fe80::/10"}
	for _, cidr := range privateCIDRs {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// ClassifyCommandForWorkflow 工作流模式下的命令安全分级。
// L0/L1 放行，L2 降级为 L1（只读），L3/L4 直接拒绝。
func ClassifyCommandForWorkflow(cmd string) Decision {
	d := ClassifyCommand(cmd)
	switch d.Level {
	case "L3", "L4":
		d.Allowed = false
		d.Action = "block"
		d.Reason = fmt.Sprintf("workflow mode: blocked (%s: %s)", d.Level, d.Reason)
	case "L2":
		d.Allowed = true
		d.Level = "L1"
		d.Action = "allow"
		d.Reason = fmt.Sprintf("workflow mode: downgraded L2->L1 (%s)", d.Reason)
	}
	return d
}
