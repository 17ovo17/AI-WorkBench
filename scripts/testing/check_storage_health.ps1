param(
  [string]$Backend = "http://localhost:8080",
  [string]$EvidenceDir = ".test-evidence\manual"
)

$ErrorActionPreference = "Continue"
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

function Invoke-Json($name, $url) {
  Write-Host "GET $url"
  try {
    $resp = Invoke-RestMethod -Uri $url -Method Get -TimeoutSec 15
    $resp | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir $name)
    return @{ ok = $true; data = $resp }
  } catch {
    @{ ok = $false; error = $_.Exception.Message } | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir $name)
    return @{ ok = $false; error = $_.Exception.Message }
  }
}

$results = [ordered]@{}
$results.storage = Invoke-Json "health-storage.json" "$Backend/api/v1/health/storage"
$results.datasources = Invoke-Json "health-datasources.json" "$Backend/api/v1/health/datasources"
$results.aiProviders = Invoke-Json "health-ai-providers.json" "$Backend/api/v1/health/ai-providers"
$results.platformIP = Invoke-Json "platform-ip.json" "$Backend/api/v1/platform/ip"
$results.aiConfig = Invoke-Json "ai-providers.json" "$Backend/api/v1/ai-providers"
$results.dataSources = Invoke-Json "data-sources.json" "$Backend/api/v1/data-sources"

$results | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 (Join-Path $EvidenceDir "storage-health-summary.json")
if (-not $results.storage.ok -or -not $results.datasources.ok) { exit 1 }
