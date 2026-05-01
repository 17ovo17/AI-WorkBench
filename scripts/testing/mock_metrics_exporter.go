// Mock Metrics Exporter for AI WorkBench end-to-end testing.
//
// 独立的 main 包，零外部依赖。
// 监听 :9101，对外暴露：
//   GET  /metrics          —— Prometheus 兼容文本格式
//   POST /scenario/{name}  —— 切换当前场景
//
// 使用方式：
//   go run mock_metrics_exporter.go
//
// 设计要点：
//   1. 每个场景下都暴露所有指标，只是数值不同；这样 Prometheus 抓取规则保持稳定。
//   2. instance="172.20.32.65:9100" 显示异常值，instance="172.20.32.66:9100" 始终为正常值，
//      用于模拟"172.20.32.65 主机异常"的场景。
//   3. 场景切换通过 sync.RWMutex 保护，写少读多。
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

// 监听端口（避免与 API 8080 / Prometheus 9090 / node_exporter 9100 / web 3000 冲突）。
const listenAddr = ":9101"

// 模拟的两台主机：65 用于异常，66 用于正常对照。
const (
	abnormalInstance = "172.20.32.65:9100"
	normalInstance   = "172.20.32.66:9100"
)

// metricKey 是指标名 + 可选的额外标签（不含 instance）。
// 例如 disk_used_percent{device="/dev/sda1",fstype="ext4",path="/"}
// 这里把额外标签放进 key，方便每个场景独立配置不同维度的取值。
type metricKey struct {
	Name        string
	ExtraLabels string // 形如 `device="sda",mode="user"`，已渲染好
	HelpText    string
	MetricType  string // "gauge" | "counter"
}

// scenarioValues 保存某个场景下，每个 metricKey 对应的两台主机的取值：
//   [0] = 异常主机 65 的取值
//   [1] = 正常主机 66 的取值
type scenarioValues map[metricKey][2]float64

// ========== 工具：构造 metricKey 的便捷函数 ==========

func gauge(name, help string) metricKey {
	return metricKey{Name: name, HelpText: help, MetricType: "gauge"}
}

func gaugeL(name, help, labels string) metricKey {
	return metricKey{Name: name, ExtraLabels: labels, HelpText: help, MetricType: "gauge"}
}

func counter(name, help string) metricKey {
	return metricKey{Name: name, HelpText: help, MetricType: "counter"}
}

func counterL(name, help, labels string) metricKey {
	return metricKey{Name: name, ExtraLabels: labels, HelpText: help, MetricType: "counter"}
}

// ========== 指标定义（所有场景共享的元数据） ==========
//
// 所有指标都在 baseline() 里给出"正常基线"。
// 各异常场景在 baseline 上覆写少量字段。

// baseline 返回一个全部为正常值的场景值表。
// 每条值对应 [异常主机, 正常主机]，正常场景下两台都是健康值。
func baseline() scenarioValues {
	v := scenarioValues{}

	// ---------- categraf 风格 CPU/内存/负载 ----------
	v[gauge("cpu_usage_active", "CPU active usage percentage")] = [2]float64{32.5, 28.7}
	v[gauge("cpu_usage_user", "CPU user usage percentage")] = [2]float64{18.3, 16.1}
	v[gauge("cpu_usage_system", "CPU system usage percentage")] = [2]float64{9.8, 8.4}
	v[gauge("cpu_usage_iowait", "CPU iowait percentage")] = [2]float64{1.2, 0.9}

	v[gauge("mem_used_percent", "Memory used percentage")] = [2]float64{52.3, 48.1}
	v[gauge("mem_available_percent", "Memory available percentage")] = [2]float64{47.7, 51.9}
	v[gauge("swap_used_percent", "Swap used percentage")] = [2]float64{2.1, 1.8}

	v[gauge("load1", "Load average 1 minute")] = [2]float64{1.8, 1.5}
	v[gauge("load5", "Load average 5 minutes")] = [2]float64{2.1, 1.7}
	v[gauge("load15", "Load average 15 minutes")] = [2]float64{2.0, 1.6}

	v[gauge("processes_total", "Total number of processes")] = [2]float64{245, 230}
	v[gauge("processes_running", "Running processes")] = [2]float64{3, 2}
	v[gauge("processes_zombie", "Zombie processes")] = [2]float64{0, 0}

	v[gauge("system_uptime", "System uptime in seconds")] = [2]float64{864000, 950000}

	// ---------- 磁盘 ----------
	v[gaugeL("disk_used_percent", "Disk used percentage",
		`device="sda1",fstype="ext4",path="/"`)] = [2]float64{42.5, 38.7}
	v[gaugeL("disk_used_percent", "Disk used percentage",
		`device="sda2",fstype="ext4",path="/data"`)] = [2]float64{55.3, 50.1}
	v[gauge("disk_inodes_used_percent", "Disk inodes used percentage")] = [2]float64{12.8, 10.5}
	v[gauge("disk_io_util", "Disk IO utilization percentage")] = [2]float64{15.2, 12.8}
	v[gauge("disk_io_await", "Disk IO await ms")] = [2]float64{8.5, 6.7}

	// ---------- 网络 ----------
	v[gauge("net_drop_in", "Network packets dropped on receive")] = [2]float64{0, 0}
	v[gauge("net_drop_out", "Network packets dropped on send")] = [2]float64{0, 0}
	v[gauge("net_err_in", "Network receive errors")] = [2]float64{0, 0}
	v[gauge("net_err_out", "Network send errors")] = [2]float64{0, 0}
	v[counter("net_bytes_recv", "Network bytes received")] = [2]float64{1.523e9, 1.412e9}
	v[counter("net_bytes_sent", "Network bytes sent")] = [2]float64{8.34e8, 7.91e8}

	v[gauge("tcp_time_wait", "TCP TIME_WAIT connections")] = [2]float64{1850, 1620}
	v[gauge("tcp_estab", "TCP established connections")] = [2]float64{420, 380}
	v[gauge("tcp_close_wait", "TCP CLOSE_WAIT connections")] = [2]float64{12, 8}

	// ---------- node_exporter 风格 ----------
	v[counterL("node_cpu_seconds_total", "Seconds the cpus spent in each mode",
		`cpu="0",mode="user"`)] = [2]float64{12345.67, 11234.56}
	v[counterL("node_cpu_seconds_total", "Seconds the cpus spent in each mode",
		`cpu="0",mode="system"`)] = [2]float64{4521.32, 4102.45}
	v[counterL("node_cpu_seconds_total", "Seconds the cpus spent in each mode",
		`cpu="0",mode="idle"`)] = [2]float64{98765.43, 102345.21}

	v[gauge("node_memory_MemAvailable_bytes", "Memory available bytes")] = [2]float64{8.5e9, 9.2e9}
	v[gauge("node_memory_MemTotal_bytes", "Total memory bytes")] = [2]float64{1.6e10, 1.6e10}

	v[gaugeL("node_filesystem_avail_bytes", "Filesystem available bytes",
		`mountpoint="/",fstype="ext4"`)] = [2]float64{2.3e10, 2.5e10}
	v[gaugeL("node_filesystem_avail_bytes", "Filesystem available bytes",
		`mountpoint="/data",fstype="ext4"`)] = [2]float64{4.5e11, 4.8e11}

	v[gauge("node_load1", "Node load average 1m")] = [2]float64{1.8, 1.5}
	v[gauge("node_load5", "Node load average 5m")] = [2]float64{2.1, 1.7}
	v[gauge("node_load15", "Node load average 15m")] = [2]float64{2.0, 1.6}

	v[counterL("node_disk_io_time_seconds_total", "Disk IO time seconds",
		`device="sda"`)] = [2]float64{4521.3, 4123.7}

	v[counterL("node_network_receive_bytes_total", "Network receive bytes",
		`device="eth0"`)] = [2]float64{1.523e9, 1.412e9}

	// ---------- 业务指标：MySQL ----------
	v[counter("mysql_slow_queries", "MySQL slow query count")] = [2]float64{12, 8}
	v[gauge("mysql_threads_running", "MySQL running threads")] = [2]float64{18, 15}
	v[gauge("mysql_threads_connected", "MySQL connected threads")] = [2]float64{85, 72}
	v[counter("mysql_innodb_row_lock_waits", "InnoDB row lock waits")] = [2]float64{3, 1}

	// ---------- 业务指标：Redis ----------
	v[gauge("redis_memory_used_bytes", "Redis memory used bytes")] = [2]float64{5.2e8, 4.8e8}
	v[gauge("redis_memory_max_bytes", "Redis memory max bytes")] = [2]float64{4.3e9, 4.3e9}
	v[gauge("redis_connected_clients", "Redis connected clients")] = [2]float64{42, 38}
	v[counter("redis_keyspace_hits_total", "Redis keyspace hits")] = [2]float64{1.234e7, 1.156e7}
	v[counter("redis_keyspace_misses_total", "Redis keyspace misses")] = [2]float64{2.34e5, 2.12e5}
	v[counter("redis_evicted_keys_total", "Redis evicted keys")] = [2]float64{120, 85}

	// ---------- 业务指标：JVM ----------
	v[gaugeL("jvm_memory_used_bytes", "JVM memory used bytes",
		`area="heap"`)] = [2]float64{1.2e9, 1.1e9}
	v[gaugeL("jvm_memory_used_bytes", "JVM memory used bytes",
		`area="nonheap"`)] = [2]float64{2.5e8, 2.3e8}
	v[gauge("jvm_gc_pause_seconds_max", "JVM GC pause seconds max")] = [2]float64{0.18, 0.15}
	v[counterL("jvm_gc_collection_seconds_count", "JVM GC collection count",
		`gc="G1 Young Generation"`)] = [2]float64{12, 10}
	v[counterL("jvm_gc_collection_seconds_count", "JVM GC collection count",
		`gc="G1 Old Generation"`)] = [2]float64{2, 1}

	// ---------- 业务指标：容器 / Nginx ----------
	v[counter("container_oom_events_total", "Container OOM events")] = [2]float64{0, 0}
	v[gauge("container_memory_usage_bytes", "Container memory usage bytes")] = [2]float64{5.2e8, 4.8e8}

	v[counterL("nginx_http_requests_total", "Nginx HTTP requests total",
		`status="200",path="/api/v1/health"`)] = [2]float64{125000, 118000}
	v[counterL("nginx_http_requests_total", "Nginx HTTP requests total",
		`status="404",path="/api/v1/health"`)] = [2]float64{12, 8}
	v[counterL("nginx_http_requests_total", "Nginx HTTP requests total",
		`status="502",path="/api/v1/health"`)] = [2]float64{0, 0}

	return v
}

// override 在 baseline 上覆写若干 metricKey 的"异常主机取值"。
// 正常主机 66 的值保持不变（健康基线），只有异常主机 65 显示异常值。
func override(base scenarioValues, abnormal map[metricKey]float64) scenarioValues {
	out := make(scenarioValues, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range abnormal {
		// 找到原 baseline 的"正常主机取值"作为 66 的值
		if existing, ok := out[k]; ok {
			out[k] = [2]float64{v, existing[1]}
		} else {
			// baseline 里没有的 key（边界场景），66 取一个保守正常值 0
			out[k] = [2]float64{v, 0}
		}
	}
	return out
}

// ========== 场景表 ==========

func buildScenarios() map[string]scenarioValues {
	scenarios := map[string]scenarioValues{}

	// normal：全部基线
	scenarios["normal"] = baseline()

	// cpu_high
	scenarios["cpu_high"] = override(baseline(), map[metricKey]float64{
		gauge("cpu_usage_active", "CPU active usage percentage"): 92.5,
		gauge("load1", "Load average 1 minute"):                  12.3,
		gauge("processes_total", "Total number of processes"):    480,
	})

	// memory_leak
	scenarios["memory_leak"] = override(baseline(), map[metricKey]float64{
		gauge("mem_used_percent", "Memory used percentage"):                   95.8,
		gauge("swap_used_percent", "Swap used percentage"):                    78.2,
		gaugeL("jvm_memory_used_bytes", "JVM memory used bytes", `area="heap"`): 4.2e9,
	})

	// disk_full
	scenarios["disk_full"] = override(baseline(), map[metricKey]float64{
		gaugeL("disk_used_percent", "Disk used percentage",
			`device="sda1",fstype="ext4",path="/"`): 96.5,
		gauge("disk_inodes_used_percent", "Disk inodes used percentage"): 45.2,
	})

	// oom
	scenarios["oom"] = override(baseline(), map[metricKey]float64{
		counter("container_oom_events_total", "Container OOM events"):            3,
		gauge("container_memory_usage_bytes", "Container memory usage bytes"):    2147483648,
	})

	// slow_sql
	scenarios["slow_sql"] = override(baseline(), map[metricKey]float64{
		counter("mysql_slow_queries", "MySQL slow query count"):  1250,
		gauge("mysql_threads_running", "MySQL running threads"):  180,
	})

	// network_drop
	scenarios["network_drop"] = override(baseline(), map[metricKey]float64{
		gauge("net_drop_in", "Network packets dropped on receive"): 1523,
		gauge("net_drop_out", "Network packets dropped on send"):   890,
		gauge("net_err_in", "Network receive errors"):              42,
	})

	// connection_pool
	scenarios["connection_pool"] = override(baseline(), map[metricKey]float64{
		gauge("tcp_time_wait", "TCP TIME_WAIT connections"):              28500,
		gauge("tcp_estab", "TCP established connections"):                12000,
		gauge("mysql_threads_connected", "MySQL connected threads"):      498,
	})

	// io_saturation
	scenarios["io_saturation"] = override(baseline(), map[metricKey]float64{
		gauge("disk_io_util", "Disk IO utilization percentage"): 98,
		gauge("disk_io_await", "Disk IO await ms"):              185,
	})

	// redis_memory
	scenarios["redis_memory"] = override(baseline(), map[metricKey]float64{
		gauge("redis_memory_used_bytes", "Redis memory used bytes"): 4.2e9,
		counter("redis_evicted_keys_total", "Redis evicted keys"):   520000,
	})

	// nginx_502
	scenarios["nginx_502"] = override(baseline(), map[metricKey]float64{
		counterL("nginx_http_requests_total", "Nginx HTTP requests total",
			`status="502",path="/api/v1/health"`): 8500,
	})

	// jvm_gc
	scenarios["jvm_gc"] = override(baseline(), map[metricKey]float64{
		gauge("jvm_gc_pause_seconds_max", "JVM GC pause seconds max"): 2.3,
		counterL("jvm_gc_collection_seconds_count", "JVM GC collection count",
			`gc="G1 Young Generation"`): 42,
	})

	return scenarios
}

// ========== 全局状态 ==========

var (
	mu              sync.RWMutex
	currentScenario = "normal"
	scenarios       = buildScenarios()
)

// ========== HTTP Handler ==========

// formatValue 用 Prometheus 推荐的格式输出 float64：
// 整数走 %d，浮点保留 6 位有效精度避免科学计数法歧义。
func formatValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// renderLabels 将 instance + extraLabels 拼接成 Prometheus 标签段：{instance="...",foo="bar"}
func renderLabels(instance, extra string) string {
	if extra == "" {
		return fmt.Sprintf(`{instance=%q}`, instance)
	}
	return fmt.Sprintf(`{instance=%q,%s}`, instance, extra)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	scName := currentScenario
	values := scenarios[scName]
	mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// 按指标名分组：同名指标合并 HELP/TYPE 行，再依次输出每个 (extraLabels, instance) 组合。
	type group struct {
		help    string
		mtype   string
		entries []struct {
			extra string
			vals  [2]float64
		}
	}
	groups := map[string]*group{}
	// 保持稳定顺序：用 names slice 记录首次出现顺序。
	var names []string

	for k, v := range values {
		g, ok := groups[k.Name]
		if !ok {
			g = &group{help: k.HelpText, mtype: k.MetricType}
			groups[k.Name] = g
			names = append(names, k.Name)
		}
		g.entries = append(g.entries, struct {
			extra string
			vals  [2]float64
		}{extra: k.ExtraLabels, vals: v})
	}

	// 按指标名字典序输出，保证多次抓取顺序一致。
	sortStrings(names)

	for _, name := range names {
		g := groups[name]
		fmt.Fprintf(w, "# HELP %s %s\n", name, g.help)
		fmt.Fprintf(w, "# TYPE %s %s\n", name, g.mtype)

		// 同一 metric 下，先按 extraLabels 字典序，再按 instance 顺序输出。
		entries := g.entries
		sortEntries(entries)

		for _, e := range entries {
			// 异常主机
			fmt.Fprintf(w, "%s%s %s\n", name,
				renderLabels(abnormalInstance, e.extra), formatValue(e.vals[0]))
			// 正常主机
			fmt.Fprintf(w, "%s%s %s\n", name,
				renderLabels(normalInstance, e.extra), formatValue(e.vals[1]))
		}
	}

	// 末尾输出当前场景元数据，便于排查。
	fmt.Fprintf(w, "# HELP mock_exporter_current_scenario_info Current mock scenario name (info metric)\n")
	fmt.Fprintf(w, "# TYPE mock_exporter_current_scenario_info gauge\n")
	fmt.Fprintf(w, "mock_exporter_current_scenario_info{scenario=%q} 1\n", scName)
}

func scenarioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/scenario/")
	if name == "" {
		http.Error(w, "scenario name required", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	if _, ok := scenarios[name]; !ok {
		http.Error(w, "scenario not found", http.StatusNotFound)
		return
	}
	currentScenario = name
	fmt.Fprintf(w, "switched to %s\n", name)
}

// rootHandler：访问 / 时给个简短帮助。
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	mu.RLock()
	cur := currentScenario
	mu.RUnlock()

	fmt.Fprintln(w, "Mock Metrics Exporter")
	fmt.Fprintf(w, "Current scenario: %s\n", cur)
	fmt.Fprintln(w, "Endpoints:")
	fmt.Fprintln(w, "  GET  /metrics")
	fmt.Fprintln(w, "  POST /scenario/{name}")
	fmt.Fprintln(w, "Available scenarios:")
	mu.RLock()
	defer mu.RUnlock()
	var names []string
	for k := range scenarios {
		names = append(names, k)
	}
	sortStrings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  - %s\n", n)
	}
}

// ========== 简单的排序工具（避免引入 sort 包外的依赖；其实 sort 是标准库，直接用即可） ==========

func sortStrings(s []string) {
	// 标准库 sort 也是可用的零外部依赖，使用之。
	// 这里只是为了把所有排序逻辑集中。
	// （保留独立函数便于将来切换实现）
	sortByLess(len(s), func(i, j int) bool { return s[i] < s[j] }, func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

type entry = struct {
	extra string
	vals  [2]float64
}

func sortEntries(es []entry) {
	sortByLess(len(es), func(i, j int) bool { return es[i].extra < es[j].extra }, func(i, j int) {
		es[i], es[j] = es[j], es[i]
	})
}

// 极简插入排序，n 都很小（指标条目数 < 200），无需引入 sort.Slice。
// 这样保持文件零外部依赖、对标准库的使用也最小化。
func sortByLess(n int, less func(i, j int) bool, swap func(i, j int)) {
	for i := 1; i < n; i++ {
		for j := i; j > 0 && less(j, j-1); j-- {
			swap(j, j-1)
		}
	}
}

// ========== main ==========

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/scenario/", scenarioHandler)

	log.Printf("Mock metrics exporter listening on %s (scenario=%s)", listenAddr, currentScenario)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}
