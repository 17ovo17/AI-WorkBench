param(
  [string]$Backend = "http://localhost:8080",
  [string]$EvidenceDir = ".test-evidence/s1-danger-sandbox"
)
$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Import-Module (Join-Path $scriptDir "aiw_test_safety.psm1") -Force
$policy = Read-AiwSafetyPolicy
$batchId = New-AiwTestBatchId -Prefix "aiw-s1"
New-Item -ItemType Directory -Force -Path $EvidenceDir | Out-Null
$EvidenceDir = (Resolve-Path $EvidenceDir).Path
New-AiwSnapshot -BatchId $batchId -EvidenceDir $EvidenceDir | Out-Null
$results = @()
function Invoke-Case {
  param([string]$Id,[string]$Method="POST",[string]$Path,[object]$Body,[int[]]$ExpectedStatus,[string]$ExpectedLevel)
  $uri = "$Backend$Path"
  $json = $null
  try {
    if ($Body -ne $null) { $json = $Body | ConvertTo-Json -Depth 20 }
    $resp = Invoke-WebRequest -UseBasicParsing -Method $Method -Uri $uri -ContentType 'application/json' -Body $json -Headers @{ 'X-Test-Batch-Id'=$batchId } -TimeoutSec 10
    $status = [int]$resp.StatusCode; $content = $resp.Content
  } catch {
    $status = 0
    $content = $_.Exception.Message
    if ($_.Exception.Response) {
      try { $status = [int]$_.Exception.Response.StatusCode.value__ } catch {}
    }
    if ($_.ErrorDetails -and $_.ErrorDetails.Message) {
      $content = $_.ErrorDetails.Message
    } elseif ($_.Exception.Response) {
      try { $reader = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); $content = $reader.ReadToEnd() } catch {}
    }
  }
  $parsed = $null; try { $parsed = $content | ConvertFrom-Json } catch {}
  $level = if($parsed -and $parsed.safety){ $parsed.safety.level } else { $null }
  $pass = ($ExpectedStatus -contains $status) -and ([string]::IsNullOrWhiteSpace($ExpectedLevel) -or $level -eq $ExpectedLevel)
  $item = [ordered]@{ id=$Id; method=$Method; path=$Path; body=$Body; status=$status; expected_status=$ExpectedStatus; safety_level=$level; expected_level=$ExpectedLevel; pass=$pass; response=$content }
  $script:results += [pscustomobject]$item
}
$base = @{ ip='192.168.1.7'; protocol='winrm'; port=5985; username='aiw-test'; password='redacted'; test_batch_id=$batchId }
Invoke-Case -Id 's1-remote-l4-rm-root' -Path '/api/v1/remote/exec' -Body ($base + @{ command='rm -rf /' }) -ExpectedStatus @(403) -ExpectedLevel 'L4'
Invoke-Case -Id 's1-remote-l4-encoded' -Path '/api/v1/remote/exec' -Body ($base + @{ command='powershell -EncodedCommand cgBtACAALQByAGYAIAAvAA==' }) -ExpectedStatus @(403) -ExpectedLevel 'L4'
Invoke-Case -Id 's1-remote-l3-no-confirm' -Path '/api/v1/remote/exec' -Body ($base + @{ command='Remove-Item -Recurse -Force C:\catpaw' }) -ExpectedStatus @(428) -ExpectedLevel 'L3'
Invoke-Case -Id 's1-remote-host-block' -Path '/api/v1/remote/exec' -Body @{ ip='8.8.8.8'; protocol='ssh'; port=22; username='x'; password='redacted'; command='whoami'; test_batch_id=$batchId } -ExpectedStatus @(403) -ExpectedLevel 'L3'
Invoke-Case -Id 's1-check-port-ssrf' -Path '/api/v1/remote/check-port' -Body @{ ip='169.254.169.254'; port=80 } -ExpectedStatus @(403) -ExpectedLevel 'L3'
Invoke-Case -Id 's1-install-url-ssrf' -Path '/api/v1/remote/install-cmd' -Body @{ os='linux'; mode='run'; platform_url='http://169.254.169.254/latest/meta-data' } -ExpectedStatus @(403,400) -ExpectedLevel 'L3'
try {
  $audit = Invoke-WebRequest -UseBasicParsing -Uri "$Backend/api/v1/audit/events" -TimeoutSec 10
  $audit.Content | Set-Content -LiteralPath (Join-Path $EvidenceDir 'audit-events-after.json') -Encoding UTF8
} catch { $_.Exception.Message | Set-Content -LiteralPath (Join-Path $EvidenceDir 'audit-events-after.error.txt') -Encoding UTF8 }
ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir 'api-results.json') -InputObject $results
$failed = @($results | Where-Object { -not $_.pass })
$score = if($failed.Count -eq 0){100}else{[Math]::Max(0,100-($failed.Count*20))}
$findings = @($failed | ForEach-Object { "P1 $($_.id) expected status=$($_.expected_status -join '/') level=$($_.expected_level), got status=$($_.status) level=$($_.safety_level)" })
Write-AiwBatchSkeleton -EvidenceDir $EvidenceDir -BatchId $batchId -BatchName 's1-danger-command-sandbox' -Score $score -Findings $findings
"test_id,module,case,priority,result,evidence`n$($results | ForEach-Object { "$($_.id),safety,$($_.path),P0,$(if($_.pass){'pass'}else{'fail'}),api-results.json" } | Out-String)" | Set-Content -LiteralPath (Join-Path $EvidenceDir 'coverage-matrix.csv') -Encoding UTF8
if($failed.Count -gt 0){ exit 1 }
