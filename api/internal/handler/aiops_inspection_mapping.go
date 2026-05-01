package handler

import (
	"fmt"
	"strings"

	"ai-workbench-api/internal/store"

	"github.com/spf13/viper"
)

var inspectionStandardNames = map[string][]string{
	"cpu_usage_active":        {"host.cpu.usage", "host.cpu.usage_active"},
	"mem_used_percent":        {"host.memory.usage", "host.memory.used_percent"},
	"disk_used_percent":       {"host.disk.usage", "host.disk.used_percent"},
	"system_load1":            {"host.system.load1", "host.load.1m"},
	"netstat_tcp_established": {"host.tcp.connections", "host.tcp.established"},
	"network_error_rate":      {"host.network.error_rate"},
	"processes":               {"host.process.count"},
}

func mappedInspectionMetricPromQL(spec inspectionMetricSpec, target promTarget, selector string) string {
	standards := inspectionStandardCandidates(spec)
	mapping, ok := store.FindMetricMappingByStandard(defaultPrometheusDatasourceID(), standards, target.Metrics)
	if !ok {
		return ""
	}
	return mappedPromQL(mapping.RawName, mapping.Transform, selector)
}

func inspectionStandardCandidates(spec inspectionMetricSpec) []string {
	out := []string{spec.Metric, spec.Name}
	out = append(out, inspectionStandardNames[spec.Metric]...)
	out = append(out, spec.Aliases...)
	return inspectionUniqueStrings(out)
}

func mappedPromQL(rawName, transform, selector string) string {
	transform = strings.TrimSpace(transform)
	if transform == "" || transform == "{}" || strings.EqualFold(transform, "none") {
		return defaultMappedPromQL(rawName, selector)
	}
	if strings.Contains(transform, "{ip}") || strings.Contains(transform, "{instance}") {
		return normalizeMappedPromQL(rawName, applyMappedPromQLPlaceholders(transform, selector), selector)
	}
	if strings.Contains(transform, "{}") {
		query := strings.ReplaceAll(transform, "{}", fmt.Sprintf(`%s{%s}`, rawName, selector))
		return normalizeMappedPromQL(rawName, query, selector)
	}
	if strings.Contains(transform, rawName) {
		return normalizeMappedPromQL(rawName, ensureMappedSelector(transform, rawName, selector), selector)
	}
	return defaultMappedPromQL(rawName, selector)
}

func normalizeMappedPromQL(rawName, query, selector string) string {
	if !mappedMetricNeedsRate(rawName) || strings.Contains(strings.ToLower(query), "rate(") {
		return query
	}
	return defaultMappedPromQL(rawName, selector)
}

func mappedMetricNeedsRate(rawName string) bool {
	return rawName == "node_cpu_seconds_total" || strings.HasSuffix(rawName, "_total") || strings.HasSuffix(rawName, "_seconds_total")
}

func defaultMappedPromQL(rawName, selector string) string {
	switch rawName {
	case "node_cpu_seconds_total":
		return fmt.Sprintf(`100 * (1 - avg(rate(node_cpu_seconds_total{%s,mode="idle"}[5m])))`, selector)
	case "node_memory_MemAvailable_bytes":
		return fmt.Sprintf(`100 * (1 - max(node_memory_MemAvailable_bytes{%s}) / clamp_min(max(node_memory_MemTotal_bytes{%s}), 1))`, selector, selector)
	case "node_filesystem_avail_bytes":
		fsSelector := selector + `,fstype!~"tmpfs|overlay|squashfs",mountpoint!~"/run.*|/var/lib/docker.*"`
		return fmt.Sprintf(`100 * (1 - max(node_filesystem_avail_bytes{%s}) / clamp_min(max(node_filesystem_size_bytes{%s}), 1))`, fsSelector, fsSelector)
	case "node_load1":
		return fmt.Sprintf(`max(node_load1{%s})`, selector)
	case "node_procs_running":
		return fmt.Sprintf(`max(node_procs_running{%s})`, selector)
	}
	if strings.HasSuffix(rawName, "_total") || strings.HasSuffix(rawName, "_seconds_total") {
		return fmt.Sprintf(`sum(rate(%s{%s}[5m]))`, rawName, selector)
	}
	return fmt.Sprintf(`max(%s{%s})`, rawName, selector)
}

func applyMappedPromQLPlaceholders(query, selector string) string {
	key, value := splitPromSelector(selector)
	instance := value
	if key != "instance" && value != "" {
		instance = value + ":9100"
	}
	query = strings.ReplaceAll(query, "{ip}", value)
	query = strings.ReplaceAll(query, "{instance}", instance)
	return query
}

func ensureMappedSelector(query, rawName, selector string) string {
	metricSelector := rawName + "{"
	if strings.Contains(query, metricSelector) {
		return query
	}
	return strings.ReplaceAll(query, rawName, fmt.Sprintf(`%s{%s}`, rawName, selector))
}

func splitPromSelector(selector string) (string, string) {
	parts := strings.SplitN(selector, "=", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), `"`)
}

func defaultPrometheusDatasourceID() string {
	var sources []struct {
		ID   string `mapstructure:"id"`
		Type string `mapstructure:"type"`
	}
	_ = viper.UnmarshalKey("data_sources", &sources)
	for _, source := range sources {
		if strings.EqualFold(source.Type, "prometheus") && strings.TrimSpace(source.ID) != "" {
			return strings.TrimSpace(source.ID)
		}
	}
	return "prometheus-default"
}
