package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type promResult struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type MetricQuery struct {
	Name  string `mapstructure:"name"`
	Query string `mapstructure:"query"`
}

type promTarget struct {
	LabelKey   string              `json:"label_key"`
	LabelVal   string              `json:"label_val"`
	Series     []map[string]string `json:"-"`
	Metrics    []string            `json:"metrics"`
	Categories []metricCategory    `json:"categories"`
	TargetOnly bool                `json:"target_only"`
}

type metricCategory struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Metrics     []string `json:"metrics"`
}

type metricSample struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Query    string `json:"query"`
	Value    string `json:"value"`
}

type promHostSummary struct {
	IP         string   `json:"ip"`
	Labels     []string `json:"labels"`
	MetricCnt  int      `json:"metric_count"`
	TargetOnly bool     `json:"target_only"`
}

var ipRe = regexp.MustCompile(`\b(\d{1,3}(?:\.\d{1,3}){3})\b`)

var targetOnlyMetrics = map[string]bool{
	"up":                                    true,
	"scrape_duration_seconds":               true,
	"scrape_samples_scraped":                true,
	"scrape_samples_post_metric_relabeling": true,
	"scrape_series_added":                   true,
}

var categoryRules = []struct {
	Key         string
	Name        string
	Description string
	Prefixes    []string
	Exact       []string
}{
	{Key: "cpu", Name: "CPU", Description: "Categraf CPU 使用率、分态占比、核心数", Prefixes: []string{"cpu_"}},
	{Key: "memory", Name: "内存", Description: "Categraf 内存、Swap、可用率", Prefixes: []string{"mem_", "swap_"}},
	{Key: "disk", Name: "磁盘容量", Description: "Categraf 文件系统容量、inode、挂载点", Prefixes: []string{"disk_"}},
	{Key: "diskio", Name: "磁盘 IO", Description: "Categraf 磁盘吞吐、IOPS、等待、利用率", Prefixes: []string{"diskio_"}},
	{Key: "network", Name: "网络", Description: "Categraf 网卡带宽、包量、丢包、错误包、协议栈", Prefixes: []string{"net_", "netstat_", "sockstat_", "conntrack_", "ethtool_", "net_response_", "ping_"}},
	{Key: "system", Name: "系统", Description: "Categraf load、内核、进程总览、文件句柄", Prefixes: []string{"system_", "kernel_", "kernel_vmstat_", "linux_sysctl_fs_", "processes", "procstat_"}},
	{Key: "container", Name: "容器", Description: "Docker、Kubernetes、cAdvisor、Kubelet 等容器指标", Prefixes: []string{"docker_", "container_", "cadvisor_", "kubernetes_", "kube_", "kubelet_"}},
	{Key: "mysql", Name: "MySQL", Description: "MySQL 状态、InnoDB、复制、慢查询", Prefixes: []string{"mysql_"}},
	{Key: "redis", Name: "Redis", Description: "Redis 连接、内存、命中率、命令、慢日志", Prefixes: []string{"redis_"}},
	{Key: "nginx", Name: "Nginx", Description: "Nginx stub_status、VTS、upstream", Prefixes: []string{"nginx_", "nginx_vts_", "nginx_upstream_", "tengine_"}},
	{Key: "database", Name: "数据库", Description: "PostgreSQL、MongoDB、Oracle、SQLServer、ClickHouse、Greenplum", Prefixes: []string{"postgresql_", "postgres_", "mongodb_", "mongo_", "oracle_", "sqlserver_", "clickhouse_", "greenplum_"}},
	{Key: "middleware", Name: "中间件", Description: "RabbitMQ、Kafka、ZooKeeper、NATS、NSQ、RocketMQ、Elasticsearch", Prefixes: []string{"rabbitmq_", "kafka_", "zookeeper_", "nats_", "nsq_", "rocketmq_", "elasticsearch_", "logstash_"}},
	{Key: "web", Name: "Web/应用", Description: "Apache、HAProxy、Tomcat、JVM、HTTP 探测、PHP-FPM", Prefixes: []string{"apache_", "haproxy_", "tomcat_", "jvm_", "http_", "phpfpm_", "spring_"}},
	{Key: "storage", Name: "存储", Description: "NFS、SMART、Ceph/对象存储、文件计数", Prefixes: []string{"nfs_", "nfsclient_", "smart_", "filecount_", "xskyapi_"}},
	{Key: "network_device", Name: "网络设备/SNMP", Description: "SNMP、IPMI、Redfish、交换机与设备侧指标", Prefixes: []string{"snmp_", "ipmi_", "redfish_", "switch_"}},
	{Key: "cloud", Name: "云与虚拟化", Description: "CloudWatch、阿里云、Google Cloud、vSphere", Prefixes: []string{"cloudwatch_", "aliyun_", "googlecloud_", "vsphere_"}},
	{Key: "prometheus", Name: "Prometheus 抓取", Description: "Prometheus target、scrape、自监控数据", Prefixes: []string{"scrape_", "prometheus_"}, Exact: []string{"up"}},
}

func queryProm(query string) (string, error) {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return "", fmt.Errorf("prometheus.url is empty")
	}
	if val := queryPromInstant(base, query, 0); val != "" {
		return val, nil
	}
	return queryPromRange(base, query), nil
}

func queryPromInstant(base, query string, offsetSec int64) string {
	endpoint := base + "/api/v1/query?query=" + url.QueryEscape(query)
	if offsetSec != 0 {
		endpoint += "&time=" + fmt.Sprintf("%d", time.Now().Unix()-offsetSec)
	}
	resp, err := http.Get(endpoint)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result promResult
	if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" || len(result.Data.Result) == 0 {
		return ""
	}
	return formatPromResult(result.Data.Result)
}

func queryPromRange(base, query string) string {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	step := int64(math.Ceil(now.Sub(start).Seconds() / 10000))
	if step < 60 {
		step = 60
	}
	endpoint := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%d", base, url.QueryEscape(query), start.Unix(), now.Unix(), step)
	resp, err := http.Get(endpoint)
	if err != nil {
		return "无数据"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" {
		return "无数据"
	}
	parts := []string{}
	for _, item := range result.Data.Result {
		if len(item.Values) == 0 {
			continue
		}
		last := item.Values[len(item.Values)-1]
		if len(last) < 2 {
			continue
		}
		value, ok := last[1].(string)
		if !ok || value == "" {
			continue
		}
		parts = append(parts, formatValueWithLabels(item.Metric, value))
	}
	if len(parts) == 0 {
		return "无数据"
	}
	return strings.Join(parts, "; ")
}

func formatPromResult(results []struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}) string {
	parts := []string{}
	for _, item := range results {
		if len(item.Value) < 2 {
			continue
		}
		value, ok := item.Value[1].(string)
		if !ok || value == "" {
			continue
		}
		parts = append(parts, formatValueWithLabels(item.Metric, value))
	}
	return strings.Join(parts, "; ")
}

func formatValueWithLabels(labels map[string]string, value string) string {
	keys := []string{"device", "iface", "interface", "path", "mountpoint", "state", "proto", "service", "name", "container", "pod"}
	shown := []string{}
	for _, key := range keys {
		if val := labels[key]; val != "" {
			shown = append(shown, key+"="+val)
		}
	}
	if len(shown) > 0 {
		return strings.Join(shown, ",") + ": " + value
	}
	return value
}

func resolveIdent(ip string) string {
	target := discoverPromTarget(ip)
	if target.LabelKey == "ident" {
		return target.LabelVal
	}
	return ""
}

func extractIP(text string) string {
	return ipRe.FindString(text)
}

func discoverPromTarget(ip string) promTarget {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" || ip == "" {
		return promTarget{}
	}
	labelPriority := []string{"ident", "instance", "ip", "host", "hostname", "agent_hostname", "target", "node", "nodename", "exported_instance", "address"}
	best := promTarget{}
	bestScore := -1
	seenMatcher := map[string]bool{}
	for _, key := range labelPriority {
		for _, value := range promLabelValues(base, key) {
			if !labelContainsExactIP(value, ip) {
				continue
			}
			matcherKey := key + "=" + value
			if seenMatcher[matcherKey] {
				continue
			}
			seenMatcher[matcherKey] = true
			series := promSeriesByMatcher(base, fmt.Sprintf(`{%s="%s"}`, key, value))
			if len(series) == 0 {
				continue
			}
			metrics := metricNamesFromSeries(series)
			score := scorePromTarget(key, value, ip, metrics, len(series))
			if score > bestScore {
				bestScore = score
				best = promTarget{LabelKey: key, LabelVal: value, Series: series, Metrics: metrics, TargetOnly: isTargetOnlyMetrics(metrics)}
			}
		}
	}
	if best.LabelKey == "" {
		matched := promSeriesContainingIP(base, ip)
		if len(matched) == 0 {
			return promTarget{}
		}
		key, value := mostCommonIPLabel(matched, ip)
		metrics := metricNamesFromSeries(matched)
		best = promTarget{LabelKey: key, LabelVal: value, Series: matched, Metrics: metrics, TargetOnly: isTargetOnlyMetrics(metrics)}
	}
	best.Categories = categorizeMetrics(best.Metrics)
	return best
}

func scorePromTarget(key, value, ip string, metrics []string, seriesCount int) int {
	score := seriesCount
	if labelIsExactIPTarget(value, ip) {
		score += 100000
	}
	if !isTargetOnlyMetrics(metrics) {
		score += 50000
	}
	switch key {
	case "ident":
		score += 20000
	case "instance":
		score += 10000
	}
	return score
}

func labelContainsExactIP(value, ip string) bool {
	if value == "" || ip == "" {
		return false
	}
	for _, match := range ipRe.FindAllString(value, -1) {
		if match == ip {
			return true
		}
	}
	return false
}

func labelIsExactIPTarget(value, ip string) bool {
	return value == ip || strings.HasPrefix(value, ip+":") || strings.HasSuffix(value, "-"+ip) || strings.HasSuffix(value, "_"+ip)
}

func isTargetOnlyMetrics(metrics []string) bool {
	if len(metrics) == 0 {
		return false
	}
	for _, metric := range metrics {
		if targetOnlyMetrics[metric] || strings.HasPrefix(metric, "scrape_") {
			continue
		}
		return false
	}
	return true
}

func promLabelValues(base, label string) []string {
	if base == "" {
		return nil
	}
	resp, err := http.Get(base + "/api/v1/label/" + url.PathEscape(label) + "/values")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string   `json:"status"`
		Data   []string `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Status != "success" {
		return nil
	}
	return result.Data
}

func promSeriesByMatcher(base, matcher string) []map[string]string {
	if base == "" || matcher == "" {
		return nil
	}
	resp, err := http.Get(base + "/api/v1/series?match[]=" + url.QueryEscape(matcher))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Status != "success" {
		return nil
	}
	return result.Data
}

func promSeriesContainingIP(base, ip string) []map[string]string {
	all := promSeriesByMatcher(base, `{__name__=~".+"}`)
	matched := []map[string]string{}
	seen := map[string]bool{}
	for _, item := range all {
		for key, value := range item {
			if key == "__name__" || !labelContainsExactIP(value, ip) {
				continue
			}
			sig := seriesSignature(item)
			if !seen[sig] {
				seen[sig] = true
				matched = append(matched, item)
			}
			break
		}
	}
	return matched
}

func seriesSignature(series map[string]string) string {
	keys := make([]string, 0, len(series))
	for key := range series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+series[key])
	}
	return strings.Join(parts, "|")
}

func mostCommonIPLabel(series []map[string]string, ip string) (string, string) {
	counts := map[string]map[string]int{}
	for _, item := range series {
		for key, value := range item {
			if key == "__name__" || !labelContainsExactIP(value, ip) {
				continue
			}
			if counts[key] == nil {
				counts[key] = map[string]int{}
			}
			counts[key][value]++
		}
	}
	bestKey, bestVal, bestCount := "", "", 0
	for key, values := range counts {
		for value, count := range values {
			if count > bestCount || (count == bestCount && labelIsExactIPTarget(value, ip)) {
				bestKey, bestVal, bestCount = key, value, count
			}
		}
	}
	return bestKey, bestVal
}

func metricNamesFromSeries(series []map[string]string) []string {
	seen := map[string]bool{}
	names := []string{}
	for _, item := range series {
		name := item["__name__"]
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func discoveredPromHosts(limit int) []string {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return nil
	}
	summary := map[string]*promHostSummary{}
	for _, label := range []string{"ident", "instance", "ip", "host", "hostname", "target", "address"} {
		for _, value := range promLabelValues(base, label) {
			for _, ip := range ipRe.FindAllString(value, -1) {
				if summary[ip] == nil {
					summary[ip] = &promHostSummary{IP: ip}
				}
				summary[ip].Labels = appendUnique(summary[ip].Labels, label+"="+value)
			}
		}
	}
	ips := make([]string, 0, len(summary))
	for ip := range summary {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	if limit > 0 && len(ips) > limit {
		ips = ips[:limit]
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip+" ("+strings.Join(summary[ip].Labels, "; ")+")")
	}
	return out
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func availableMetrics(ip string) string {
	target := discoverPromTarget(ip)
	if target.LabelKey == "" {
		hosts := discoveredPromHosts(50)
		if len(hosts) == 0 {
			return "Prometheus 已连接，但没有发现任何主机标签或指标序列。"
		}
		return fmt.Sprintf("Prometheus 已全量检索 label values 与 series，未精确匹配到 IP %s。当前发现主机：%s。精确 IP 匹配不会把 198.18.20.12 误配到 198.18.20.122/123。", ip, strings.Join(hosts, ", "))
	}
	categoryText := []string{}
	for _, category := range target.Categories {
		categoryText = append(categoryText, fmt.Sprintf("%s:%d", category.Name, len(category.Metrics)))
	}
	if target.TargetOnly {
		return fmt.Sprintf("Prometheus 已发现测试目标：%s=\"%s\"，IP=%s。当前只有 up/scrape/target 序列；测试环境中 up=0/offline 仍代表目标已被发现，但不能判断主机资源压力。序列数=%d，指标=%s", target.LabelKey, target.LabelVal, ip, len(target.Series), strings.Join(target.Metrics, ", "))
	}
	return fmt.Sprintf("Prometheus 已发现 Categraf/Prometheus 性能指标：%s=\"%s\"，IP=%s。序列数=%d，指标数=%d，分类=%s，指标=%s", target.LabelKey, target.LabelVal, ip, len(target.Series), len(target.Metrics), strings.Join(categoryText, ", "), strings.Join(target.Metrics, ", "))
}

func buildMonitorContext(ip, userMsg string) string {
	_ = userMsg
	if ip == "" {
		return ""
	}
	target := discoverPromTarget(ip)
	if target.LabelKey == "" {
		return fmt.Sprintf("\n\n[Prometheus 全量发现结果 - 主机: %s]\n- 数据源: prometheus\n- 解析结果: 未精确匹配到该 IP 的 ident/instance/ip/host/hostname/target/address 标签，也未在全量 series 标签值中匹配到该 IP。\n- 已发现主机: %s\n- 说明: 已使用精确 IP 匹配，不会把 198.18.20.12 误配到 198.18.20.122/123。\n[数据结束]\n", ip, strings.Join(discoveredPromHosts(50), ", "))
	}
	queries := buildCategrafQueries(target.LabelKey, target.LabelVal, target.Metrics)
	samples := runMetricQueries(queries)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n[Prometheus 全量发现数据 - 主机: %s，匹配标签: %s=%s]\n", ip, target.LabelKey, target.LabelVal))
	sb.WriteString("- 数据源: prometheus\n")
	if target.TargetOnly {
		sb.WriteString("- 发现类型: target/scrape 数据。测试环境中 up=0/offline 仍代表该目标已被发现；在线状态只作为健康字段，不作为发现失败标准。\n")
	} else {
		sb.WriteString("- 发现类型: Categraf/Prometheus 性能指标。已动态适配 Categraf 的主机、网络、磁盘、容器、中间件与应用指标前缀。\n")
	}
	sb.WriteString(fmt.Sprintf("- 目标解析: 发现 %d 条序列、%d 个唯一指标。\n", len(target.Series), len(target.Metrics)))
	for _, category := range target.Categories {
		sb.WriteString(fmt.Sprintf("- 指标分类: %s，数量=%d，说明=%s\n", category.Name, len(category.Metrics), category.Description))
		sb.WriteString(fmt.Sprintf("  指标: %s\n", strings.Join(category.Metrics, ", ")))
	}
	if len(samples) == 0 {
		sb.WriteString("- 实际数值: 未查询到可用最新值或历史值。\n")
	} else {
		for _, sample := range samples {
			sb.WriteString(fmt.Sprintf("- %s/%s: %s\n", sample.Category, sample.Name, sample.Value))
		}
	}
	sb.WriteString("[数据结束。若只有 target/scrape 数据，只能判断目标被发现和抓取健康，不能判断主机资源压力；不要把 up=0 当作测试失败。]\n")
	return sb.String()
}

func buildCategrafQueries(labelKey, labelVal string, metrics []string) []MetricQuery {
	selector := fmt.Sprintf(`%s="%s"`, labelKey, labelVal)
	queries := []MetricQuery{}
	seen := map[string]bool{}
	add := func(title, query string) {
		if seen[query] {
			return
		}
		seen[query] = true
		queries = append(queries, MetricQuery{Name: title, Query: query})
	}
	metricSet := map[string]bool{}
	for _, metric := range metrics {
		metricSet[metric] = true
	}
	for _, spec := range priorityMetricSpecs() {
		if metricSet[spec.Metric] {
			add(spec.Title, fmt.Sprintf(spec.Query, selector))
		}
	}
	for _, metric := range metrics {
		if seen[metric] {
			continue
		}
		add(friendlyMetricName(metric), fmt.Sprintf(`%s{%s}`, metric, selector))
	}
	return queries
}

func runMetricQueries(queries []MetricQuery) []metricSample {
	samples := []metricSample{}
	for _, query := range queries {
		value, err := queryProm(query.Query)
		if err != nil || strings.TrimSpace(value) == "" || value == "无数据" {
			continue
		}
		samples = append(samples, metricSample{Name: query.Name, Category: categoryNameForMetric(metricNameFromQuery(query.Query)), Query: query.Query, Value: value})
	}
	return samples
}

func metricNameFromQuery(query string) string {
	cleaned := strings.TrimSpace(query)
	for _, fn := range []string{"rate", "irate", "sum", "avg", "max", "min", "increase"} {
		cleaned = strings.TrimPrefix(cleaned, fn+"(")
	}
	match := regexp.MustCompile(`[a-zA-Z_:][a-zA-Z0-9_:]*`).FindString(cleaned)
	return match
}

func friendlyMetricName(name string) string {
	return categoryNameForMetric(name) + "/" + name
}

func categoryNameForMetric(metric string) string {
	for _, rule := range categoryRules {
		for _, exact := range rule.Exact {
			if metric == exact {
				return rule.Name
			}
		}
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(metric, prefix) {
				return rule.Name
			}
		}
	}
	return "其他"
}

func categorizeMetrics(metrics []string) []metricCategory {
	byKey := map[string]*metricCategory{}
	order := []string{}
	for _, metric := range metrics {
		key, name, desc := categoryForMetric(metric)
		if byKey[key] == nil {
			byKey[key] = &metricCategory{Key: key, Name: name, Description: desc}
			order = append(order, key)
		}
		byKey[key].Metrics = append(byKey[key].Metrics, metric)
	}
	for _, category := range byKey {
		sort.Strings(category.Metrics)
	}
	out := make([]metricCategory, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	return out
}

func categoryForMetric(metric string) (string, string, string) {
	for _, rule := range categoryRules {
		for _, exact := range rule.Exact {
			if metric == exact {
				return rule.Key, rule.Name, rule.Description
			}
		}
		for _, prefix := range rule.Prefixes {
			if strings.HasPrefix(metric, prefix) {
				return rule.Key, rule.Name, rule.Description
			}
		}
	}
	return "other", "其他", "未识别前缀，仍按原始 Prometheus 指标纳入适配"
}

type priorityMetricSpec struct {
	Title  string
	Metric string
	Query  string
}

func priorityMetricSpecs() []priorityMetricSpec {
	return []priorityMetricSpec{
		{Title: "CPU 使用率(%)", Metric: "cpu_usage_active", Query: `cpu_usage_active{%s}`},
		{Title: "CPU 空闲率(%)", Metric: "cpu_usage_idle", Query: `cpu_usage_idle{%s}`},
		{Title: "CPU iowait(%)", Metric: "cpu_usage_iowait", Query: `cpu_usage_iowait{%s}`},
		{Title: "CPU steal(%)", Metric: "cpu_usage_steal", Query: `cpu_usage_steal{%s}`},
		{Title: "内存使用率(%)", Metric: "mem_used_percent", Query: `mem_used_percent{%s}`},
		{Title: "内存可用率(%)", Metric: "mem_available_percent", Query: `mem_available_percent{%s}`},
		{Title: "Swap 使用率(%)", Metric: "swap_used_percent", Query: `swap_used_percent{%s}`},
		{Title: "磁盘使用率(%)", Metric: "disk_used_percent", Query: `disk_used_percent{%s}`},
		{Title: "磁盘读吞吐(bytes/s)", Metric: "diskio_read_bytes", Query: `rate(diskio_read_bytes{%s}[5m])`},
		{Title: "磁盘写吞吐(bytes/s)", Metric: "diskio_write_bytes", Query: `rate(diskio_write_bytes{%s}[5m])`},
		{Title: "磁盘 IO 利用率(%)", Metric: "diskio_io_util", Query: `diskio_io_util{%s}`},
		{Title: "磁盘 IO 等待(ms)", Metric: "diskio_io_await", Query: `diskio_io_await{%s}`},
		{Title: "网络入带宽(bits/s)", Metric: "net_bits_recv", Query: `net_bits_recv{%s}`},
		{Title: "网络出带宽(bits/s)", Metric: "net_bits_sent", Query: `net_bits_sent{%s}`},
		{Title: "网络入包量", Metric: "net_packets_recv", Query: `net_packets_recv{%s}`},
		{Title: "网络出包量", Metric: "net_packets_sent", Query: `net_packets_sent{%s}`},
		{Title: "网络入丢包", Metric: "net_drop_in", Query: `net_drop_in{%s}`},
		{Title: "网络出丢包", Metric: "net_drop_out", Query: `net_drop_out{%s}`},
		{Title: "网络入错误包", Metric: "net_err_in", Query: `net_err_in{%s}`},
		{Title: "网络出错误包", Metric: "net_err_out", Query: `net_err_out{%s}`},
		{Title: "TCP inuse", Metric: "netstat_tcp_inuse", Query: `netstat_tcp_inuse{%s}`},
		{Title: "TCP TIME_WAIT", Metric: "netstat_tcp_tw", Query: `netstat_tcp_tw{%s}`},
		{Title: "Socket 已用", Metric: "netstat_sockets_used", Query: `netstat_sockets_used{%s}`},
		{Title: "系统负载 1m", Metric: "system_load1", Query: `system_load1{%s}`},
		{Title: "系统负载 5m", Metric: "system_load5", Query: `system_load5{%s}`},
		{Title: "系统负载 15m", Metric: "system_load15", Query: `system_load15{%s}`},
		{Title: "CPU 核数", Metric: "system_n_cpus", Query: `system_n_cpus{%s}`},
		{Title: "进程总数", Metric: "processes", Query: `processes{%s}`},
		{Title: "僵尸进程", Metric: "processes_zombies", Query: `processes_zombies{%s}`},
		{Title: "MySQL 在线", Metric: "mysql_up", Query: `mysql_up{%s}`},
		{Title: "MySQL 连接数", Metric: "mysql_global_status_threads_connected", Query: `mysql_global_status_threads_connected{%s}`},
		{Title: "MySQL QPS", Metric: "mysql_global_status_queries", Query: `rate(mysql_global_status_queries{%s}[5m])`},
		{Title: "Redis 在线", Metric: "redis_up", Query: `redis_up{%s}`},
		{Title: "Redis 客户端连接", Metric: "redis_connected_clients", Query: `redis_connected_clients{%s}`},
		{Title: "Redis OPS", Metric: "redis_instantaneous_ops_per_sec", Query: `redis_instantaneous_ops_per_sec{%s}`},
		{Title: "Nginx 在线", Metric: "nginx_up", Query: `nginx_up{%s}`},
		{Title: "Nginx 活跃连接", Metric: "nginx_active", Query: `nginx_active{%s}`},
		{Title: "Nginx 请求速率", Metric: "nginx_requests", Query: `rate(nginx_requests{%s}[5m])`},
		{Title: "Docker 容器数", Metric: "docker_n_containers", Query: `docker_n_containers{%s}`},
		{Title: "JVM GC 停顿最大(s)", Metric: "jvm_gc_pause_seconds_max", Query: `jvm_gc_pause_seconds_max{%s}`},
		{Title: "JVM GC 停顿总计(s)", Metric: "jvm_gc_pause_seconds_sum", Query: `jvm_gc_pause_seconds_sum{%s}`},
		{Title: "JVM GC 次数", Metric: "jvm_gc_pause_seconds_count", Query: `jvm_gc_pause_seconds_count{%s}`},
		{Title: "JVM 堆内存已用(bytes)", Metric: "jvm_memory_used_bytes", Query: `jvm_memory_used_bytes{%s}`},
		{Title: "JVM 堆内存最大(bytes)", Metric: "jvm_memory_max_bytes", Query: `jvm_memory_max_bytes{%s}`},
		{Title: "JVM 线程数", Metric: "jvm_threads_live_threads", Query: `jvm_threads_live_threads{%s}`},
		{Title: "JVM 已加载类", Metric: "jvm_classes_loaded_classes", Query: `jvm_classes_loaded_classes{%s}`},
		{Title: "进程 CPU 使用率", Metric: "process_cpu_usage", Query: `process_cpu_usage{%s}`},
	}
}

func PrometheusHosts(c *gin.Context) {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "prometheus.url is empty"})
		return
	}
	hostMap := map[string]*promHostSummary{}
	for _, label := range []string{"ident", "instance", "ip", "host", "hostname", "target", "address"} {
		for _, value := range promLabelValues(base, label) {
			for _, ip := range ipRe.FindAllString(value, -1) {
				if hostMap[ip] == nil {
					hostMap[ip] = &promHostSummary{IP: ip}
				}
				hostMap[ip].Labels = appendUnique(hostMap[ip].Labels, label+"="+value)
			}
		}
	}
	ips := make([]string, 0, len(hostMap))
	for ip := range hostMap {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	ensureDefaultMonitoringBusiness(ips)
	items := []promHostSummary{}
	for _, ip := range ips {
		target := discoverPromTarget(ip)
		item := *hostMap[ip]
		item.MetricCnt = len(target.Metrics)
		item.TargetOnly = target.TargetOnly
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"hosts": items})
}

func ensureDefaultMonitoringBusiness(hosts []string) {
	unassigned := unassignedPrometheusHosts(hosts)
	if len(unassigned) == 0 {
		return
	}
	const defaultBusinessName = "默认监控业务"
	business, ok := defaultMonitoringBusiness(defaultBusinessName)
	if !ok {
		business = model.TopologyBusiness{Name: defaultBusinessName, Attributes: map[string]string{"source": "prometheus_auto_discovery"}}
	}
	business.Hosts = mergeHosts(business.Hosts, unassigned)
	if business.Attributes == nil {
		business.Attributes = map[string]string{}
	}
	business.Attributes["source"] = "prometheus_auto_discovery"
	syncDefaultMonitoringBusinessGraph(&business, unassigned)
	store.SaveTopologyBusiness(business)
}

func unassignedPrometheusHosts(hosts []string) []string {
	assigned := map[string]bool{}
	for _, business := range store.ListTopologyBusinesses() {
		for _, host := range business.Hosts {
			assigned[strings.TrimSpace(host)] = true
		}
	}
	out := []string{}
	for _, host := range normalizeHosts(hosts) {
		if !assigned[host] {
			out = append(out, host)
		}
	}
	return out
}

func defaultMonitoringBusiness(name string) (model.TopologyBusiness, bool) {
	for _, business := range store.ListTopologyBusinesses() {
		if strings.TrimSpace(business.Name) == name {
			return business, true
		}
	}
	return model.TopologyBusiness{}, false
}

func businessGraphForHosts(hosts []string) model.TopologyGraph {
	now := time.Now()
	graph := model.TopologyGraph{Nodes: []model.TopologyNode{}, Edges: []model.TopologyEdge{}}
	hosts = normalizeHosts(hosts)
	for index, host := range hosts {
		addHostDiscovery(&graph, host, index, now, nil)
	}
	graph.Discovery = buildTopologyDiscoveryPlan(hosts, nil, &graph, false)
	layoutBusinessTree(&graph)
	return graph
}

func syncDefaultMonitoringBusinessGraph(business *model.TopologyBusiness, newHosts []string) {
	if len(business.Graph.Nodes) == 0 {
		business.Graph = businessGraphForHosts(business.Hosts)
		return
	}
	now := time.Now()
	for _, host := range normalizeHosts(newHosts) {
		if !hasNode(&business.Graph, "host-"+sanitizeID(host)) {
			addHostDiscovery(&business.Graph, host, len(business.Graph.Nodes), now, nil)
		}
	}
	business.Graph.Discovery = buildTopologyDiscoveryPlan(business.Hosts, business.Endpoints, &business.Graph, false)
	layoutBusinessTree(&business.Graph)
}

func PrometheusMetrics(c *gin.Context) {
	ip := strings.TrimSpace(c.Query("ip"))
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip is required"})
		return
	}
	target := discoverPromTarget(ip)
	if target.LabelKey == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "ip not found in prometheus", "hosts": discoveredPromHosts(50)})
		return
	}
	queries := buildCategrafQueries(target.LabelKey, target.LabelVal, target.Metrics)
	samples := runMetricQueries(queries)
	c.JSON(http.StatusOK, gin.H{"ip": ip, "target": target, "samples": samples})
}

func parseFloatValue(text string) float64 {
	fields := strings.Fields(strings.ReplaceAll(text, ";", " "))
	for _, field := range fields {
		field = strings.Trim(field, ",")
		if v, err := strconv.ParseFloat(field, 64); err == nil {
			return v
		}
		if i := strings.LastIndex(field, ":"); i >= 0 {
			if v, err := strconv.ParseFloat(field[i+1:], 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func hasRecentPrometheusData(ip string) bool {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return false
	}
	query := fmt.Sprintf(`cpu_usage_active{ident=~".*%s.*"}`, ip)
	val := queryPromInstant(base, query, 0)
	return val != "" && !strings.Contains(val, "无数据")
}

func queryPromMetricValue(ip, metricName string) float64 {
	base := strings.TrimRight(viper.GetString("prometheus.url"), "/")
	if base == "" {
		return 0
	}
	target := discoverPromTarget(ip)
	if target.LabelKey == "" {
		return 0
	}
	query := fmt.Sprintf(`%s{%s="%s"}`, metricName, target.LabelKey, target.LabelVal)
	val := queryPromInstant(base, query, 0)
	if val == "" {
		return 0
	}
	return parseFloatValue(val)
}

var metricDisplayNames = map[string]string{
	"cpu_usage_active":               "CPU 使用率",
	"cpu_usage_idle":                 "CPU 空闲率",
	"cpu_usage_system":               "CPU 系统态",
	"cpu_usage_user":                 "CPU 用户态",
	"cpu_usage_iowait":               "CPU IO等待",
	"cpu_usage_softirq":              "CPU 软中断",
	"cpu_usage_steal":                "CPU 窃取",
	"mem_used_percent":               "内存使用率",
	"mem_available_percent":          "内存可用率",
	"mem_used":                       "已用内存",
	"mem_available":                  "可用内存",
	"mem_cached":                     "内存缓存",
	"swap_used_percent":              "Swap 使用率",
	"disk_used_percent":              "磁盘使用率",
	"disk_total":                     "磁盘总量",
	"diskio_io_util":                 "磁盘 IO 繁忙度",
	"diskio_io_await":                "磁盘 IO 等待时间",
	"diskio_read_bytes":              "磁盘读取速率",
	"diskio_write_bytes":             "磁盘写入速率",
	"net_bits_recv":                  "网络入流量",
	"net_bits_sent":                  "网络出流量",
	"net_drop_in":                    "网络入丢包",
	"net_drop_out":                   "网络出丢包",
	"net_err_in":                     "网络入错误",
	"net_err_out":                    "网络出错误",
	"netstat_tcp_inuse":              "TCP 活跃连接数",
	"netstat_tcp_tw":                 "TCP TIME_WAIT 数",
	"netstat_sockets_used":           "Socket 使用数",
	"system_load1":                   "系统负载(1分钟)",
	"system_load5":                   "系统负载(5分钟)",
	"system_load15":                  "系统负载(15分钟)",
	"system_n_cpus":                  "CPU 核心数",
	"kernel_context_switches":        "内核上下文切换",
	"kernel_vmstat_oom_kill":         "OOM 终止次数",
	"processes":                      "进程总数",
	"processes_zombies":              "僵尸进程数",
	"processes_blocked":              "阻塞进程数",
	"linux_sysctl_fs_file_max":       "文件句柄上限",
	"oracle_up":                      "Oracle 在线状态",
	"oracle_buffer_cache_hit_ratio":  "Oracle 缓存命中率",
	"oracle_process_count":           "Oracle 进程数",
	"oracle_sessions":                "Oracle 会话数",
	"oracle_tablespace_used_percent": "Oracle 表空间使用率",
}

func MetricDisplayName(name string) string {
	if cn, ok := metricDisplayNames[name]; ok {
		return cn
	}
	return name
}
