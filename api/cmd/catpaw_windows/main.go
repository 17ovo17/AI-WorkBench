package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type config struct {
	IP           string
	HeartbeatURL string
	ReportURL    string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("catpaw windows agent: run | inspect | selftest | diagnose")
		return
	}
	switch os.Args[1] {
	case "run":
		cfg := loadConfig(parseConfigDir(os.Args[2:]))
		sendHeartbeat(cfg)
		sendReport(cfg, "Catpaw Windows 启动巡检", collectReport(cfg.IP))
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			sendHeartbeat(cfg)
		}
	case "inspect":
		cfg := loadConfig(parseConfigDir(os.Args[2:]))
		report := collectReport(cfg.IP)
		fmt.Println(report)
		if cfg.ReportURL != "" {
			sendReport(cfg, "Catpaw Windows 即时巡检", report)
		}
	case "selftest":
		cfg := loadConfig(parseConfigDir(os.Args[2:]))
		fmt.Printf("catpaw selftest ok ip=%s heartbeat=%s report=%s\n", cfg.IP, cfg.HeartbeatURL, cfg.ReportURL)
	case "diagnose":
		fmt.Println("catpaw diagnose list: local lightweight agent has no local history store")
	default:
		fmt.Printf("unknown command: %s\n", os.Args[1])
	}
}

func parseConfigDir(args []string) string {
	fs := flag.NewFlagSet("catpaw", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	configs := fs.String("configs", `C:\catpaw\conf.d`, "config directory")
	_ = fs.Parse(args)
	return *configs
}

func loadConfig(dir string) config {
	cfg := config{IP: firstIPv4()}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".toml") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(b)
		if v := tomlString(text, "from_hostip"); v != "" && !strings.Contains(v, "$") {
			cfg.IP = v
		}
		if strings.Contains(text, "[notify.heartbeat]") {
			if v := sectionString(text, "notify.heartbeat", "url"); v != "" {
				cfg.HeartbeatURL = v
			}
		}
		if strings.Contains(text, "[notify.webapi]") {
			if v := sectionString(text, "notify.webapi", "url"); v != "" {
				cfg.ReportURL = v
			}
		}
		return nil
	})
	return cfg
}

func tomlString(text, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*=\s*"([^"]*)"`)
	m := re.FindStringSubmatch(text)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func sectionString(text, section, key string) string {
	start := strings.Index(text, "["+section+"]")
	if start < 0 {
		return ""
	}
	rest := text[start+len(section)+2:]
	if next := strings.Index(rest, "\n["); next >= 0 {
		rest = rest[:next]
	}
	return tomlString(rest, key)
}

func firstIPv4() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip != nil && !strings.HasPrefix(ip.String(), "169.254.") {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func sendHeartbeat(cfg config) {
	if cfg.HeartbeatURL == "" {
		return
	}
	hostname, _ := os.Hostname()
	postJSON(cfg.HeartbeatURL, map[string]any{
		"ip": cfg.IP, "hostname": hostname, "version": "windows-local-compat-1.0",
	})
}

func sendReport(cfg config, title, report string) {
	if cfg.ReportURL == "" {
		return
	}
	postJSON(cfg.ReportURL, map[string]any{"ip": cfg.IP, "title": title, "report": report})
}

func postJSON(url string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(b))
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

func collectReport(ip string) string {
	hostname, _ := os.Hostname()
	plugins := []struct {
		name        string
		description string
		script      string
	}{
		{"windows.cpu", "Windows CPU 负载、核心数、队列长度与进程级 CPU 热点。", `
$cpuAvg=(Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
$cpuInfo=Get-CimInstance Win32_Processor | Select-Object Name,NumberOfCores,NumberOfLogicalProcessors,LoadPercentage
$queue=(Get-Counter '\System\Processor Queue Length' -ErrorAction SilentlyContinue).CounterSamples.CookedValue
$top=Get-Process | Sort-Object CPU -Descending | Select-Object -First 8 ProcessName,Id,CPU,@{n='ThreadCount';e={$_.Threads.Count}}
[pscustomobject]@{CPUUsedPercent=$cpuAvg;ProcessorQueueLength=[math]::Round($queue,2);Processors=$cpuInfo;TopCPUProcesses=$top} | ConvertTo-Json -Depth 5`},
		{"windows.mem", "Windows 物理内存、页面文件、提交量与高内存进程。", `
$os=Get-CimInstance Win32_OperatingSystem
$memUsed=[math]::Round((($os.TotalVisibleMemorySize-$os.FreePhysicalMemory)/$os.TotalVisibleMemorySize)*100,2)
$top=Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 8 ProcessName,Id,@{n='WorkingSetMB';e={[math]::Round($_.WorkingSet64/1MB,2)}}
[pscustomobject]@{TotalMemoryMB=[math]::Round($os.TotalVisibleMemorySize/1024,2);FreeMemoryMB=[math]::Round($os.FreePhysicalMemory/1024,2);MemUsedPercent=$memUsed;TotalVirtualMemoryMB=[math]::Round($os.TotalVirtualMemorySize/1024,2);FreeVirtualMemoryMB=[math]::Round($os.FreeVirtualMemory/1024,2);TopMemoryProcesses=$top} | ConvertTo-Json -Depth 5`},
		{"windows.disk", "Windows 逻辑磁盘容量、剩余空间、使用率与文件系统类型。", `
Get-CimInstance Win32_LogicalDisk -Filter "DriveType=3" | Select-Object DeviceID,VolumeName,FileSystem,@{n='SizeGB';e={[math]::Round($_.Size/1GB,2)}},@{n='FreeGB';e={[math]::Round($_.FreeSpace/1GB,2)}},@{n='UsedPercent';e={[math]::Round((($_.Size-$_.FreeSpace)/$_.Size)*100,2)}} | ConvertTo-Json -Depth 5`},
		{"windows.diskio", "Windows 物理磁盘吞吐、IOPS、队列长度与延迟计数器。", `
$counters='\PhysicalDisk(*)\Disk Reads/sec','\PhysicalDisk(*)\Disk Writes/sec','\PhysicalDisk(*)\Disk Read Bytes/sec','\PhysicalDisk(*)\Disk Write Bytes/sec','\PhysicalDisk(*)\Avg. Disk Queue Length'
Get-Counter $counters -ErrorAction SilentlyContinue | Select-Object -ExpandProperty CounterSamples | Where-Object {$_.InstanceName -ne '_total'} | Select-Object InstanceName,Path,@{n='Value';e={[math]::Round($_.CookedValue,2)}} | ConvertTo-Json -Depth 5`},
		{"windows.net", "Windows 网卡状态、链路速率、收发字节、丢弃包与错误包。", `
$adapters=Get-NetAdapter | Select-Object Name,InterfaceDescription,Status,MacAddress,LinkSpeed
$stats=Get-NetAdapterStatistics | Select-Object Name,ReceivedBytes,SentBytes,ReceivedUnicastPackets,SentUnicastPackets,ReceivedDiscardedPackets,OutboundDiscardedPackets,ReceivedPacketErrors,OutboundPacketErrors
[pscustomobject]@{Adapters=$adapters;Statistics=$stats} | ConvertTo-Json -Depth 5`},
		{"windows.tcpstate", "Windows TCP/UDP 连接状态、监听端口、TIME_WAIT 与远端连接分布。", `
$tcp=Get-NetTCPConnection -ErrorAction SilentlyContinue
$states=$tcp | Group-Object State | Select-Object Name,Count
$listen=$tcp | Where-Object {$_.State -eq 'Listen'} | Select-Object -First 30 LocalAddress,LocalPort,OwningProcess
$udp=Get-NetUDPEndpoint -ErrorAction SilentlyContinue | Select-Object -First 30 LocalAddress,LocalPort,OwningProcess
[pscustomobject]@{TCPStateSummary=$states;ListeningTCP=$listen;UDPEndpoints=$udp} | ConvertTo-Json -Depth 5`},
		{"windows.process", "Windows 进程总量、线程数、句柄数与资源占用 Top 进程。", `
$procs=Get-Process
$topHandles=$procs | Sort-Object Handles -Descending | Select-Object -First 8 ProcessName,Id,Handles,@{n='ThreadCount';e={$_.Threads.Count}}
$topThreads=$procs | Sort-Object {$_.Threads.Count} -Descending | Select-Object -First 8 ProcessName,Id,Handles,@{n='ThreadCount';e={$_.Threads.Count}}
[pscustomobject]@{ProcessCount=$procs.Count;ThreadCount=($procs | Measure-Object Threads -Sum).Sum;HandleCount=($procs | Measure-Object Handles -Sum).Sum;TopHandles=$topHandles;TopThreads=$topThreads} | ConvertTo-Json -Depth 5`},
		{"windows.service", "Windows 服务状态、自动启动但未运行服务与关键服务检查。", `
$autoStopped=Get-CimInstance Win32_Service | Where-Object {$_.StartMode -eq 'Auto' -and $_.State -ne 'Running'} | Select-Object -First 30 Name,@{n='DisplayName';e={[string]$_.DisplayName}},State,StartMode
$critical=Get-Service | Where-Object {$_.Name -in 'Winmgmt','EventLog','LanmanServer','LanmanWorkstation','TermService','WinRM'} | Select-Object Name,DisplayName,Status,StartType
[pscustomobject]@{AutoServicesStopped=$autoStopped;CriticalServices=$critical} | ConvertTo-Json -Depth 5`},
		{"windows.eventlog", "Windows 系统/应用事件日志最近错误与严重事件。", `
$events=Get-WinEvent -FilterHashtable @{LogName='System','Application'; Level=1,2; StartTime=(Get-Date).AddHours(-24)} -MaxEvents 30 -ErrorAction SilentlyContinue | ForEach-Object { [pscustomobject]@{ TimeCreated=$_.TimeCreated.ToString('yyyy-MM-dd HH:mm:ss'); LogName=$_.LogName; ProviderName=$_.ProviderName; Id=$_.Id; LevelDisplayName=if ($_.LevelDisplayName) { $_.LevelDisplayName } elseif ($_.Level -eq 1) { 'Critical' } else { 'Error' }; Message=([string]$_.Message) } }
$events | ConvertTo-Json -Depth 5`},
		{"windows.firewall", "Windows 防火墙配置、远程管理相关规则与当前网络配置。", `
$profiles=Get-NetFirewallProfile | Select-Object Name,Enabled,DefaultInboundAction,DefaultOutboundAction
$remoteRules=Get-NetFirewallRule | Where-Object {$_.DisplayName -match 'Windows Management Instrumentation|Remote Service Management|Windows Remote Management|Remote Desktop'} | Select-Object DisplayName,Enabled,Direction,Action,Profile
$ip=Get-NetIPConfiguration | Select-Object InterfaceAlias,IPv4Address,IPv4DefaultGateway,DNSServer
[pscustomobject]@{FirewallProfiles=$profiles;RemoteManagementRules=$remoteRules;IPConfiguration=$ip} | ConvertTo-Json -Depth 5`},
		{"windows.update", "Windows 更新、系统启动时间、补丁与安全中心状态。", `
$os=Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,BuildNumber,LastBootUpTime
$hotfix=Get-HotFix | Sort-Object InstalledOn -Descending | Select-Object -First 10 HotFixID,Description,InstalledOn
[pscustomobject]@{OS=$os;RecentHotfix=$hotfix} | ConvertTo-Json -Depth 5`},
	}
	var sb strings.Builder
	for _, plugin := range plugins {
		sb.WriteString(fmt.Sprintf("\n## 插件：%s\n\n%s\n\n```json\n%s\n```\n", plugin.name, plugin.description, strings.TrimSpace(runPS(plugin.script))))
	}
	return fmt.Sprintf(`# Catpaw Windows 健康巡检

- 数据源: catpaw_windows_native
- 主机: %s
- IP: %s
- OS/Arch: %s/%s
- 时间: %s
- 插件范围: CPU、内存、磁盘、磁盘 IO、网卡、TCP/UDP、进程、服务、事件日志、防火墙、更新

%s
`, hostname, ip, runtime.GOOS, runtime.GOARCH, time.Now().Format(time.RFC3339), sb.String())
}

func runPS(script string) string {
	prefix := `[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false); $OutputEncoding = [System.Text.UTF8Encoding]::new($false);`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", prefix+"\n"+script)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out) + "\n" + err.Error()
	}
	return string(out)
}
