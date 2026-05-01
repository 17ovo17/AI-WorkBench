param(
  [string]$Backend = "http://localhost:8080",
  [string]$EvidenceDir = ".test-evidence\manual\sensitive-api",
  [string]$CasesPath = "$PSScriptRoot\api_sensitive_cases.json",
  [string]$PolicyPath = "$PSScriptRoot\safety_policy.json"
)

$ErrorActionPreference = "Stop"
Import-Module "$PSScriptRoot\aiw_test_safety.psm1" -Force
$policy = Read-AiwSafetyPolicy -PolicyPath $PolicyPath
$cases = Get-Content -LiteralPath $CasesPath -Raw -Encoding UTF8 | ConvertFrom-Json
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null

Add-Type -AssemblyName System.Net.Http
$client = [System.Net.Http.HttpClient]::new()
$client.Timeout = [TimeSpan]::FromSeconds(15)

function Invoke-Case($case) {
  $url = "$Backend$($case.path)"
  $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::new($case.method), $url)
  foreach ($name in $cases.default_headers.PSObject.Properties.Name) { $request.Headers.TryAddWithoutValidation($name, [string]$cases.default_headers.$name) | Out-Null }
  if ($case.PSObject.Properties.Name -contains 'headers') {
    foreach ($name in $case.headers.PSObject.Properties.Name) { $request.Headers.TryAddWithoutValidation($name, [string]$case.headers.$name) | Out-Null }
  }
  $json = $case.body | ConvertTo-Json -Depth 20
  $request.Content = [System.Net.Http.StringContent]::new($json, [Text.Encoding]::UTF8, "application/json")
  try {
    $resp = $client.SendAsync($request).GetAwaiter().GetResult()
    $body = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
    return [ordered]@{ status = [int]$resp.StatusCode; body = $body; error = $null }
  } catch {
    return [ordered]@{ status = $null; body = ""; error = $_.Exception.Message }
  }
}

$results = @()
foreach ($case in $cases.cases) {
  $command = ""
  if ($case.body.PSObject.Properties.Name -contains 'command') { $command = [string]$case.body.command }
  $risk = if ($command) { Get-AiwCommandRisk -Command $command -Policy $policy } else { [pscustomobject]@{ level = $case.risk; action = "endpoint-case"; reason = "declared case risk" } }
  $hostOk = $true
  if ($case.body.PSObject.Properties.Name -contains 'ip') { $hostOk = Test-AiwAllowedHost -Host ([string]$case.body.ip) -Policy $policy }
  $response = Invoke-Case $case
  $bodyText = [string]$response.body
  $status = $response.status
  $serverCrashed = ($status -eq $null -or $status -ge 500)
  $rejected = ($status -in @(400,401,403,409,422,428))
  $secretEcho = $bodyText.Contains("SHOULD_NOT_ECHO")
  $passed = $false
  switch ([string]$case.expected) {
    "reject" { $passed = $rejected -and -not $serverCrashed }
    "reject_or_confirm_required" { $passed = ($rejected -or $status -eq 428) -and -not $serverCrashed }
    "reject_or_no_remote_exec" { $passed = -not $serverCrashed }
    "no_secret_echo" { $passed = -not $serverCrashed -and -not $secretEcho }
    default { $passed = -not $serverCrashed }
  }
  $record = [ordered]@{
    id = $case.id
    path = $case.path
    expected = $case.expected
    declared_risk = $case.risk
    classified_risk = $risk.level
    host_allowed = $hostOk
    status = $status
    rejected = $rejected
    secret_echo = $secretEcho
    passed = $passed
    response = $bodyText
    error = $response.error
  }
  ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "$($case.id).json") -InputObject $record
  $results += $record
}

$failures = @($results | Where-Object { -not $_.passed })
$summary = [ordered]@{
  generated_at = (Get-Date).ToString("o")
  backend = $Backend
  total = $results.Count
  failed = $failures.Count
  passed = $results.Count - $failures.Count
  note = "This script intentionally uses dry-run and unsafe payloads to verify rejection. It must never be used to execute destructive commands."
  results = $results
}
ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "sensitive-api-summary.json") -InputObject $summary
if ($failures.Count -gt 0) { exit 1 }
