param(
  [string]$Backend = "http://localhost:8080",
  [string]$EvidenceDir = ".test-evidence\manual"
)

$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

$payloads = @(
  @{ name = "empty-json"; body = @{} },
  @{ name = "xss"; body = @{ content = '<img src=x onerror=alert(1)>'; ip = '<script>alert(1)</script>' } },
  @{ name = "sql"; body = @{ content = "' OR '1'='1"; ip = "198.18.20.11' OR '1'='1" } },
  @{ name = "cmd-injection"; body = @{ content = '诊断 198.18.20.11; rm -rf /'; ip = '198.18.20.11;whoami' } },
  @{ name = "ssrf"; body = @{ content = '访问 http://169.254.169.254/latest/meta-data/'; ip = '169.254.169.254' } },
  @{ name = "path-traversal"; body = @{ content = '..\..\windows\win.ini'; ip = '..\..\etc\passwd' } },
  @{ name = "unicode"; body = @{ content = '压力测试😀中文العربية'; ip = '198.18.20.11' } },
  @{ name = "long-field"; body = @{ content = ('A' * 8192); ip = '198.18.20.11' } }
)

$targets = @(
  @{ method = "POST"; path = "/api/v1/diagnose" },
  @{ method = "POST"; path = "/api/v1/chat" },
  @{ method = "POST"; path = "/api/v1/alert/webhook" },
  @{ method = "POST"; path = "/api/v1/topology/discover" }
)

Add-Type -AssemblyName System.Net.Http
$httpClient = New-Object System.Net.Http.HttpClient
$httpClient.Timeout = [TimeSpan]::FromSeconds(30)

$summary = @()
foreach ($target in $targets) {
  foreach ($payload in $payloads) {
    $url = "$Backend$($target.path)"
    $safePath = ($target.path -replace '/', '_').Trim('_')
    $file = Join-Path $EvidenceDir ("api-security-{0}-{1}.json" -f $safePath, $payload.name)
    try {
      $json = $payload.body | ConvertTo-Json -Depth 10
      $content = New-Object System.Net.Http.StringContent($json, [Text.Encoding]::UTF8, "application/json")
      $resp = $httpClient.PostAsync($url, $content).GetAwaiter().GetResult()
      $body = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
      $status = [int]$resp.StatusCode
      $record = [ordered]@{ path = $target.path; payload = $payload.name; status = $status; ok = ($status -lt 500); body = $body }
    } catch {
      $status = $null
      if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
      $record = [ordered]@{ path = $target.path; payload = $payload.name; status = $status; ok = ($status -and $status -lt 500); error = $_.Exception.Message }
    }
    $record | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 $file
    $summary += $record
  }
}

$summary | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "api-security-summary.json")
$serverErrors = $summary | Where-Object { $_.status -ge 500 -or ($_.status -eq $null -and -not $_.ok) }
if ($serverErrors.Count -gt 0) { exit 1 }
