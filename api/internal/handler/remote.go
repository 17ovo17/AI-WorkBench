package handler

import (
	"ai-workbench-api/internal/security"
	"ai-workbench-api/internal/store"

	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

type RemoteCredential struct {
	IP       string `json:"ip"`
	Protocol string `json:"protocol"` // ssh | winrm
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSHKey   string `json:"ssh_key"`
}

type RemoteExecRequest struct {
	RemoteCredential
	CredentialID  string `json:"credential_id"`
	Command       string `json:"command" binding:"required"`
	SafetyConfirm string `json:"safety_confirm"`
	TestBatchID   string `json:"test_batch_id"`
}

func CheckRemotePort(c *gin.Context) {
	var req struct {
		IP   string `json:"ip" binding:"required"`
		Port int    `json:"port" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if decision := security.ValidateNetworkProbeTarget(req.IP); !decision.Allowed {
		auditEvent(c, "remote.check_port", req.IP, decision.Level, "reject", decision.Reason, c.GetHeader("X-Test-Batch-Id"))
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}
	address := fmt.Sprintf("%s:%d", req.IP, req.Port)
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"reachable": false, "address": address, "error": err.Error()})
		return
	}
	conn.Close()
	c.JSON(http.StatusOK, gin.H{"reachable": true, "address": address})
}

func hasAsset(name string) bool {
	_, err := os.Stat("./assets/" + name)
	return err == nil
}

func isLocalRemoteTarget(host string) bool {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return false
	}
	if strings.EqualFold(trimmed, "localhost") {
		return true
	}
	ip := net.ParseIP(trimmed)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
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

func execLocalShell(script string) (string, error) {
	cmd := exec.Command("bash", "-lc", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("??????????: %v", err)
	}
	return string(out), nil
}

func applySavedCredential(cred *RemoteCredential, credentialID string) bool {
	if strings.TrimSpace(credentialID) == "" {
		return false
	}
	saved, ok := store.GetCredential(credentialID)
	if !ok {
		return false
	}
	if cred.Protocol == "" {
		cred.Protocol = saved.Protocol
	}
	if cred.Port == 0 {
		cred.Port = saved.Port
	}
	if cred.Username == "" {
		cred.Username = saved.Username
	}
	if cred.Password == "" || cred.Password == "******" {
		cred.Password = saved.Password
	}
	if cred.SSHKey == "" || cred.SSHKey == "******" {
		cred.SSHKey = saved.SSHKey
	}
	return true
}

func requireSavedCredential(c *gin.Context, cred *RemoteCredential, credentialID string) bool {
	if strings.TrimSpace(credentialID) == "" {
		return true
	}
	if !applySavedCredential(cred, credentialID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "credential_id not found"})
		return false
	}
	return true
}

func RemoteExec(c *gin.Context) {
	var req RemoteExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !requireSavedCredential(c, &req.RemoteCredential, req.CredentialID) {
		return
	}
	if strings.TrimSpace(req.IP) == "" || strings.TrimSpace(req.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip and username are required"})
		return
	}
	if decision := security.ValidateRemoteHost(req.IP); !decision.Allowed {
		auditEvent(c, "remote.exec", req.IP, decision.Level, "reject", decision.Reason, req.TestBatchID)
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}
	commandDecision := security.ValidateConfirm(security.ClassifyCommand(req.Command), req.SafetyConfirm)
	if !commandDecision.Allowed {
		status := http.StatusPreconditionRequired
		if commandDecision.Level == "L4" {
			status = http.StatusForbidden
		}
		auditEvent(c, "remote.exec", req.IP, commandDecision.Level, "reject", commandDecision.Reason, req.TestBatchID)
		c.JSON(status, gin.H{"error": commandDecision.Reason, "safety": commandDecision})
		return
	}
	auditEvent(c, "remote.exec", req.IP, commandDecision.Level, "allow", commandDecision.Reason, req.TestBatchID)
	switch req.Protocol {
	case "wmi":
		out, err := execWMI(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": out})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": out})
	case "winrm":
		out, err := execWinRM(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": out})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": out})
	default:
		out, err := execSSH(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"output": out})
	}
}

func encodePowerShell(script string) string {
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, len(encoded)*2)
	for i, value := range encoded {
		bytes[i*2] = byte(value)
		bytes[i*2+1] = byte(value >> 8)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func cleanPowerShellOutput(out []byte) string {
	text := string(out)
	text = strings.ReplaceAll(text, "#< CLIXML\r\n", "")
	text = strings.ReplaceAll(text, "#< CLIXML\n", "")
	if idx := strings.Index(text, "<Objs Version="); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

func execWMI(req RemoteExecRequest) (string, error) {
	port := req.Port
	if port == 0 {
		port = 135
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", req.IP, port), 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("WMI 连接失败: %s:%d 不可达: %v", req.IP, port, err)
	}
	conn.Close()
	localTarget := req.IP == "127.0.0.1" || strings.EqualFold(req.IP, "localhost")
	remoteCommand := fmt.Sprintf("powershell.exe -NoProfile -ExecutionPolicy Bypass -EncodedCommand %s", encodePowerShell(req.Command))
	script := ""
	if localTarget && strings.TrimSpace(req.Password) == "" {
		script = fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$result = Invoke-WmiMethod -ComputerName %q -Class Win32_Process -Name Create -ArgumentList %q -ErrorAction Stop
if ($result.ReturnValue -ne 0) { throw "WMI process create failed: ReturnValue=$($result.ReturnValue)" }
Write-Output "WMI process started: ProcessId=$($result.ProcessId)"
`, req.IP, remoteCommand)
	} else {
		if strings.TrimSpace(req.Password) == "" {
			return "", fmt.Errorf("WMI 连接失败: password is required")
		}
		script = fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$secure = ConvertTo-SecureString %q -AsPlainText -Force
$cred = New-Object System.Management.Automation.PSCredential(%q, $secure)
$result = Invoke-WmiMethod -ComputerName %q -Credential $cred -Class Win32_Process -Name Create -ArgumentList %q -ErrorAction Stop
if ($result.ReturnValue -ne 0) { throw "WMI process create failed: ReturnValue=$($result.ReturnValue)" }
Write-Output "WMI process started: ProcessId=$($result.ProcessId)"
`, req.Password, req.Username, req.IP, remoteCommand)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("WMI 连接失败: %v；请确认目标已开放 WMI/DCOM、445/135 与动态 RPC 端口、防火墙允许远程管理，且凭据属于本地管理员", err)
	}
	if strings.Contains(string(out), "拒绝访问") || strings.Contains(strings.ToLower(string(out)), "access is denied") || strings.Contains(string(out), "UnauthorizedAccessException") {
		return string(out), fmt.Errorf("WMI 连接失败: 目标拒绝访问；请确认用户名格式、管理员权限、远程 UAC 本地账号限制与 WMI/DCOM 权限")
	}
	return cleanPowerShellOutput(out), nil
}

func execSSH(req RemoteExecRequest) (string, error) {
	port := req.Port
	if port == 0 {
		port = 22
	}
	var auth []ssh.AuthMethod
	if req.SSHKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(req.SSHKey))
		if err != nil {
			return "", fmt.Errorf("解析 SSH 密钥失败: %v", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if req.Password != "" {
		auth = append(auth, ssh.Password(req.Password))
	}
	cfg := &ssh.ClientConfig{
		User:            req.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", req.IP, port), cfg)
	if err != nil {
		return "", fmt.Errorf("SSH 连接失败: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.CombinedOutput(req.Command)
	return string(out), err
}

func execWinRM(req RemoteExecRequest) (string, error) {
	port := req.Port
	if port == 0 {
		port = 5985
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", req.IP, port), 5*time.Second)
	if err != nil {
		return "", fmt.Errorf("WinRM 连接失败: %s:%d 不可达: %v", req.IP, port, err)
	}
	conn.Close()
	if strings.TrimSpace(req.Password) == "" {
		return "", fmt.Errorf("WinRM 连接失败: password is required")
	}
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$secure = ConvertTo-SecureString %q -AsPlainText -Force
$cred = New-Object System.Management.Automation.PSCredential(%q, $secure)
$sessionOption = New-PSSessionOption -SkipCACheck -SkipCNCheck -SkipRevocationCheck
$session = $null
$lastError = $null
foreach ($auth in @('Negotiate','Basic')) {
  try {
    $session = New-PSSession -ComputerName %q -Port %d -Credential $cred -Authentication $auth -SessionOption $sessionOption
    break
  } catch {
    $lastError = $_
  }
}
if (-not $session) { throw $lastError }
try {
  Invoke-Command -Session $session -ScriptBlock { %s }
} finally {
  if ($session) { Remove-PSSession $session }
}
`, req.Password, req.Username, req.IP, port, req.Command)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodePowerShell(script))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("WinRM 连接失败: %v；如目标为 Windows/IP 直连，请确认目标端已执行 RDP 引导，且平台侧已以管理员执行 WinRM 客户端配置：TrustedHosts、AllowUnencrypted", err)
	}
	return cleanPowerShellOutput(out), nil
}

// InstallCatpaw 通过 SSH/WinRM 一键安装 catpaw 到目标机器
func InstallCatpaw(c *gin.Context) {
	var req struct {
		RemoteCredential
		CredentialID string `json:"credential_id"`
		Mode         string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !requireSavedCredential(c, &req.RemoteCredential, req.CredentialID) {
		return
	}
	if strings.TrimSpace(req.IP) == "" || strings.TrimSpace(req.Username) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ip and username are required"})
		return
	}
	if decision := security.ValidateRemoteHost(req.IP); !decision.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}
	if req.Mode == "" {
		req.Mode = "run"
	}

	reportURL := c.GetHeader("X-Platform-URL")
	if reportURL == "" {
		reportURL = "http://your-ai-workbench:8080"
	}
	if decision := security.ValidatePlatformURL(reportURL); !decision.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}

	var script string
	var out string
	var err error
	if req.Protocol == "winrm" {
		if !hasAsset("catpaw_windows_amd64.exe") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Windows 探针二进制: ./assets/catpaw_windows_amd64.exe"})
			return
		}
		script = buildWinRMInstallCmd(reportURL)
		out, err = execWinRM(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
	} else if req.Protocol == "wmi" {
		if !hasAsset("catpaw_windows_amd64.exe") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Windows 探针二进制: ./assets/catpaw_windows_amd64.exe"})
			return
		}
		script = buildWMIInstallScript(reportURL)
		out, err = execWMI(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
	} else {
		script = buildInstallScript(req.IP, reportURL, req.Mode)
		if req.Protocol == "local" || isLocalRemoteTarget(req.IP) {
			out, err = execLocalShell(script)
		} else {
			out, err = execSSH(RemoteExecRequest{RemoteCredential: req.RemoteCredential, Command: script})
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": out})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": out})
}

// GenerateInstallCmd 生成安装命令（不直接执行，供用户复制）
func GenerateInstallCmd(c *gin.Context) {
	var req struct {
		IP          string `json:"ip"`
		OS          string `json:"os"`
		ReportURL   string `json:"report_url"`
		PlatformURL string `json:"platform_url"`
		Protocol    string `json:"protocol"` // ssh | winrm | curl
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ReportURL == "" {
		req.ReportURL = req.PlatformURL
	}
	if req.ReportURL == "" {
		req.ReportURL = "http://your-ai-workbench:8080"
	}
	if decision := security.ValidatePlatformURL(req.ReportURL); !decision.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": decision.Reason, "safety": decision})
		return
	}

	var cmd string
	protocol := req.Protocol
	if protocol == "" && req.OS == "windows" {
		protocol = "winrm"
	}
	switch protocol {
	case "rdp-winrm", "winrm-bootstrap":
		cmd = buildRDPWinRMBootstrapCmd(req.ReportURL)
	case "wmi":
		cmd = buildWMIInstallScript(req.ReportURL)
	case "winrm":
		cmd = buildWinRMInstallCmd(req.ReportURL)
	default:
		cmd = buildCurlInstallCmd(req.ReportURL)
	}
	c.JSON(http.StatusOK, gin.H{"command": cmd})
}

func buildRDPWinRMBootstrapCmd(reportURL string) string {
	return strings.TrimSpace(fmt.Sprintf(`# RDP 引导命令：请在目标 Windows 主机的“管理员 PowerShell”中执行
# 作用：启用 WinRM、开放远程管理防火墙、解除本地管理员远程 UAC 令牌过滤。
# 完成后回到平台选择 Windows + WinRM 安装 Catpaw。

$ErrorActionPreference = "Stop"
Write-Host "[1/6] Enable WinRM service"
Enable-PSRemoting -Force

Write-Host "[2/6] Configure WinRM for local administrator remote management"
winrm quickconfig -quiet
winrm set winrm/config/service '@{AllowUnencrypted="true"}'
winrm set winrm/config/service/auth '@{Basic="true"}'

Write-Host "[3/6] Open Windows firewall rules"
Enable-NetFirewallRule -DisplayGroup "Windows Remote Management" -ErrorAction SilentlyContinue
Enable-NetFirewallRule -DisplayGroup "Windows Management Instrumentation (WMI)" -ErrorAction SilentlyContinue
Enable-NetFirewallRule -DisplayGroup "Remote Service Management" -ErrorAction SilentlyContinue

Write-Host "[4/6] Disable remote UAC token filtering for local admin accounts"
New-Item -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" -Force | Out-Null
Set-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" -Name LocalAccountTokenFilterPolicy -Type DWord -Value 1

Write-Host "[5/6] Verify WinRM listener"
winrm enumerate winrm/config/listener

Write-Host "[6/6] Platform URL for Catpaw callback: %s"
Write-Host "RDP bootstrap completed. Now install from AI WorkBench with protocol: WinRM, port: 5985."

# 平台侧也需要以管理员 PowerShell 执行一次（把 <TARGET_IP> 改成目标 IP，例如 192.168.1.7）：
# Set-Item -Path WSMan:\localhost\Client\TrustedHosts -Value "<TARGET_IP>" -Force
# Set-Item -Path WSMan:\localhost\Client\AllowUnencrypted -Value $true -Force
`, reportURL))
}

func buildInstallScript(ip, reportURL, mode string) string {
	_ = ip
	return strings.TrimSpace(fmt.Sprintf(`
set -e
ARCH=$(uname -m)
[ "$ARCH" = "x86_64" ] && ARCH=amd64 || ARCH=arm64
# 获取本机 IP
HOST_IP=$(hostname -I | awk '{print $1}')
pkill -x catpaw 2>/dev/null || true
sleep 1
curl -kfsSL "%s/download/catpaw_linux_${ARCH}" -o /tmp/catpaw.$$ 
chmod +x /tmp/catpaw.$$
mv /tmp/catpaw.$$ /usr/local/bin/catpaw
mkdir -p /etc/catpaw/conf.d
cat > /etc/catpaw/conf.d/config.toml << EOF
[global.labels]
from_hostip = "${HOST_IP}"

[notify.webapi]
enabled = true
url = "%s/api/v1/catpaw/report"
method = "POST"

[notify.heartbeat]
enabled = true
url = "%s/api/v1/catpaw/heartbeat"
interval = "60s"

[ai]
enabled = true
model_priority = []
max_rounds = 10
request_timeout = "120s"
max_retries = 1
retry_backoff = "2s"
tool_timeout = "20s"
queue_full_policy = "wait"
language = "zh"

[ai.gateway]
enabled = true
base_url = "%s/api/v1/agent/llm"
max_retries = 1
request_timeout = "120s"
fallback_to_direct = false
EOF
nohup catpaw --configs /etc/catpaw/conf.d %s > /var/log/catpaw.log 2>&1 &
echo "catpaw started (ip=${HOST_IP}) in %s mode"
`, reportURL, reportURL, reportURL, reportURL, mode, mode))
}

func buildCurlInstallCmd(reportURL string) string {
	return fmt.Sprintf(`ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64 || ARCH=arm64
HOST_IP=$(hostname -I | awk '{print $1}')
pkill -x catpaw 2>/dev/null || true
sleep 1
curl -kfsSL "%s/download/catpaw_linux_${ARCH}" -o /tmp/catpaw.$$ && chmod +x /tmp/catpaw.$$ && mv /tmp/catpaw.$$ /usr/local/bin/catpaw
mkdir -p /etc/catpaw/conf.d
cat > /etc/catpaw/conf.d/config.toml << EOF
[global.labels]
from_hostip = "${HOST_IP}"
[notify.webapi]
enabled = true
url = "%s/api/v1/catpaw/report"
method = "POST"
[notify.heartbeat]
enabled = true
url = "%s/api/v1/catpaw/heartbeat"
interval = "60s"

[ai]
enabled = true
model_priority = []
max_rounds = 10
request_timeout = "120s"
max_retries = 1
retry_backoff = "2s"
tool_timeout = "20s"
queue_full_policy = "wait"
language = "zh"

[ai.gateway]
enabled = true
base_url = "%s/api/v1/agent/llm"
max_retries = 1
request_timeout = "120s"
fallback_to_direct = false
EOF
nohup catpaw --configs /etc/catpaw/conf.d run > /var/log/catpaw.log 2>&1 &
echo "catpaw started (ip=${HOST_IP})"`, reportURL, reportURL, reportURL, reportURL)
}

func buildWMIInstallScript(reportURL string) string {
	return fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
New-Item -ItemType Directory -Force -Path C:\catpaw\conf.d | Out-Null
$hostIP = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.InterfaceAlias -notlike '*Loopback*' -and $_.IPAddress -notlike '169.254.*'} | Select-Object -First 1).IPAddress
certutil -urlcache -f "%s/download/catpaw_windows_amd64.exe" C:\catpaw\catpaw.exe | Out-Null
$cfg = @"
[global.labels]
from_hostip = "$hostIP"
[notify.webapi]
enabled = true
url = "%s/api/v1/catpaw/report"
method = "POST"
[notify.heartbeat]
enabled = true
url = "%s/api/v1/catpaw/heartbeat"
interval = "30s"
[ai.gateway]
enabled = true
base_url = "%s/api/v1/agent/llm"
fallback_to_direct = false
"@
[System.IO.File]::WriteAllText('C:\catpaw\conf.d\config.toml', $cfg, [System.Text.UTF8Encoding]::new($false))
$bat = @'
@echo off
cd /d C:\catpaw
C:\catpaw\catpaw.exe run --configs C:\catpaw\conf.d >> C:\catpaw\catpaw.stdout.log 2>> C:\catpaw\catpaw.stderr.log
'@
[System.IO.File]::WriteAllText('C:\catpaw\start-catpaw.bat', $bat, [System.Text.ASCIIEncoding]::new())
schtasks /End /TN Catpaw /F 2>$null
schtasks /Delete /TN Catpaw /F 2>$null
schtasks /Create /TN Catpaw /SC ONSTART /RL HIGHEST /F /TR 'C:\catpaw\start-catpaw.bat' | Out-Null
Start-Process -FilePath 'C:\catpaw\start-catpaw.bat' -WindowStyle Hidden
Start-Sleep -Seconds 3
if (-not (Get-Process catpaw -ErrorAction SilentlyContinue)) { throw "catpaw did not start" }
Write-Output "catpaw started"
`, reportURL, reportURL, reportURL, reportURL)
}

func buildWinRMInstallCmd(reportURL string) string {
	return fmt.Sprintf(`# PowerShell 安装（在目标机器上执行）
$hostIP = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object {$_.InterfaceAlias -notlike '*Loopback*'} | Select-Object -First 1).IPAddress
New-Item -ItemType Directory -Force -Path C:\catpaw\conf.d | Out-Null
certutil -urlcache -f "%s/download/catpaw_windows_amd64.exe" C:\catpaw\catpaw.exe
@"
[global.labels]
from_hostip = "$hostIP"

[notify.webapi]
enabled = true
url = "%s/api/v1/catpaw/report"
method = "POST"

[notify.heartbeat]
enabled = true
url = "%s/api/v1/catpaw/heartbeat"
interval = "60s"

[ai]
enabled = true
model_priority = []
max_rounds = 10
request_timeout = "120s"
max_retries = 1
retry_backoff = "2s"
tool_timeout = "20s"
queue_full_policy = "wait"
language = "zh"

[ai.gateway]
enabled = true
base_url = "%s/api/v1/agent/llm"
max_retries = 1
request_timeout = "120s"
fallback_to_direct = false
"@ | Out-File C:\catpaw\conf.d\config.toml -Encoding UTF8
schtasks /End /TN Catpaw /F 2>$null
schtasks /Delete /TN Catpaw /F 2>$null
schtasks /Create /TN Catpaw /SC ONSTART /RL HIGHEST /F /TR 'C:\catpaw\catpaw.exe run --configs C:\catpaw\conf.d' | Out-Null
schtasks /Run /TN Catpaw | Out-Null
Start-Sleep -Seconds 2
if (-not (Get-Process catpaw -ErrorAction SilentlyContinue)) { throw "catpaw scheduled task did not start" }
Write-Output "catpaw scheduled task started"`, reportURL, reportURL, reportURL, reportURL)
}
