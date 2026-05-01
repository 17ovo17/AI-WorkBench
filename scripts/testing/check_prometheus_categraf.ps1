param(
  [string]$Backend = "http://localhost:8080",
  [string]$Prometheus = "http://localhost:9090",
  [string]$EvidenceDir = ".test-evidence\manual"
)

$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

$hosts = @("198.18.20.11", "198.18.20.12", "198.18.20.20", "198.18.22.11", "198.18.22.12", "198.18.22.13")
$required = @("cpu_usage_active", "mem_used_percent", "disk_used_percent", "net_bits_recv", "net_bits_sent", "net_drop_in", "net_drop_out", "net_err_in", "net_err_out", "netstat_tcp_established", "netstat_tcp_time_wait", "system_load1")

function Query-Prom($query, $suffix) {
  $encoded = [uri]::EscapeDataString($query)
  $url = "$Prometheus/api/v1/query?query=$encoded"
  try {
    $resp = Invoke-RestMethod -Uri $url -Method Get -TimeoutSec 20
    $safe = $suffix -replace '[^a-zA-Z0-9_.-]', '_'
    $resp | ConvertTo-Json -Depth 30 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "prom-$safe.json")
    return $resp
  } catch {
    @{ status = "error"; error = $_.Exception.Message } | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "prom-$suffix.error.json")
    return $null
  }
}

$summary = [ordered]@{
  generated_at = (Get-Date).ToString("o")
  hosts = @{}
  platform_hosts = $null
}

try {
  $summary.platform_hosts = Invoke-RestMethod -Uri "$Backend/api/v1/prometheus/hosts" -Method Get -TimeoutSec 20
  $summary.platform_hosts | ConvertTo-Json -Depth 30 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "platform-prometheus-hosts.json")
} catch {
  $summary.platform_hosts = @{ error = $_.Exception.Message }
}

$countResp = Query-Prom 'count(count by(instance) (cpu_usage_active{job="categraf"}))' "host-count"

foreach ($hostIp in $hosts) {
  $hostResult = [ordered]@{ metrics = @{}; platform_metric_count = 0; missing = @() }
  foreach ($metric in $required) {
    $query = 'count_over_time({0}{{instance="{1}"}}[5m])' -f $metric, $hostIp
    $resp = Query-Prom $query "$hostIp-$metric"
    $value = $null
    if ($resp -and $resp.data -and $resp.data.result -and $resp.data.result.Count -gt 0) { $value = $resp.data.result[0].value[1] }
    $hostResult.metrics[$metric] = $value
    if (-not $value -or [double]$value -le 0) { $hostResult.missing += $metric }
  }
  try {
    $platformMetrics = Invoke-RestMethod -Uri "$Backend/api/v1/prometheus/metrics?ip=$hostIp" -Method Get -TimeoutSec 20
    $platformMetrics | ConvertTo-Json -Depth 30 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "platform-metrics-$hostIp.json")
    if ($platformMetrics.metrics) { $hostResult.platform_metric_count = $platformMetrics.metrics.Count }
    elseif ($platformMetrics.samples) { $hostResult.platform_metric_count = $platformMetrics.samples.Count }
    elseif ($platformMetrics.data) { $hostResult.platform_metric_count = $platformMetrics.data.Count }
  } catch {
    $hostResult.platform_error = $_.Exception.Message
  }
  $summary.hosts[$hostIp] = $hostResult
}

$summary | ConvertTo-Json -Depth 30 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "prometheus-categraf-summary.json")
$failed = $false
foreach ($hostIp in $hosts) { if ($summary.hosts[$hostIp].missing.Count -gt 0) { $failed = $true } }
if ($failed) { exit 1 }
