param(
  [string]$Backend = "http://localhost:8080",
  [string]$Prometheus = "http://localhost:9090",
  [string]$EvidenceRoot = ".test-evidence"
)

$ErrorActionPreference = "Continue"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$evidenceDir = Join-Path $EvidenceRoot $timestamp
New-Item -ItemType Directory -Force -Path $evidenceDir | Out-Null

function Write-Section([string]$name) {
  Write-Host "`n== $name ==" -ForegroundColor Cyan
}

function Save-Json($name, $data) {
  $path = Join-Path $evidenceDir $name
  $data | ConvertTo-Json -Depth 20 | Set-Content -Encoding UTF8 $path
}

Write-Section "AI WorkBench full smoke"
Write-Host "Evidence: $evidenceDir"

& "$PSScriptRoot\check_storage_health.ps1" -Backend $Backend -EvidenceDir $evidenceDir
& "$PSScriptRoot\check_prometheus_categraf.ps1" -Backend $Backend -Prometheus $Prometheus -EvidenceDir $evidenceDir
& "$PSScriptRoot\check_api_security_inputs.ps1" -Backend $Backend -EvidenceDir $evidenceDir

$summary = [ordered]@{
  generated_at = (Get-Date).ToString("o")
  backend = $Backend
  prometheus = $Prometheus
  evidence_dir = (Resolve-Path $evidenceDir).Path
  note = "UI exhaustive checks must be executed with Playwright MCP and recorded in docs/testing/AI_WorkBench_测试执行记录.md."
}
Save-Json "summary.json" $summary
Write-Host "`nSmoke completed. Evidence saved to $evidenceDir" -ForegroundColor Green
