param(
  [string]$EvidenceRoot = ".test-evidence",
  [string]$PolicyPath = "$PSScriptRoot\safety_policy.json"
)

$ErrorActionPreference = "Stop"
Import-Module "$PSScriptRoot\aiw_test_safety.psm1" -Force

$policy = Read-AiwSafetyPolicy -PolicyPath $PolicyPath
$batch = New-AiwEvidenceBatch -EvidenceRoot $EvidenceRoot -BatchName "s0-safety-precheck"

$findings = New-Object System.Collections.Generic.List[string]
$blockers = New-Object System.Collections.Generic.List[string]

New-AiwSnapshot -BatchId $batch.BatchId -EvidenceDir $batch.EvidenceDir | Out-Null

$sandboxRoot = "C:\catpaw-test-sandbox\$($batch.BatchId)"
New-Item -ItemType Directory -Force -Path $sandboxRoot | Out-Null
$batch.Created.created_paths += $sandboxRoot
ConvertTo-AiwJsonFile -Path (Join-Path $batch.EvidenceDir "created-ids.json") -InputObject $batch.Created

$checks = @(
  @{ name = "linux-forbidden-root-delete"; command = "rm -rf /"; expected = "L4" },
  @{ name = "windows-forbidden-root-delete"; command = "Remove-Item -Recurse -Force C:\"; expected = "L3" },
  @{ name = "mysql-forbidden-drop"; command = "DROP DATABASE ai_workbench"; expected = "L4" },
  @{ name = "redis-forbidden-flushall"; command = "redis-cli FLUSHALL"; expected = "L4" },
  @{ name = "windows-allowed-catapw-uninstall-scope"; command = "Stop-Process -Name catpaw -Force; Remove-Item -Recurse -Force C:\catpaw"; expected = "L3" },
  @{ name = "linux-readonly"; command = "cat /etc/os-release"; expected = "L0" },
  @{ name = "windows-readonly"; command = "Get-Process catpaw"; expected = "L0" },
  @{ name = "encoded-danger"; command = "powershell -EncodedCommand cm0gLXJmIC8="; expected = "L0" }
)

$classification = @()
foreach ($check in $checks) {
  $risk = Get-AiwCommandRisk -Command $check.command -Policy $policy
  $ok = $risk.level -eq $check.expected
  if (-not $ok) {
    $blockers.Add("P0 classifier mismatch for $($check.name): expected $($check.expected), got $($risk.level)")
  }
  $classification += [ordered]@{
    name = $check.name
    command = $check.command
    expected = $check.expected
    actual = $risk.level
    action = $risk.action
    reason = $risk.reason
    ok = $ok
  }
}
ConvertTo-AiwJsonFile -Path (Join-Path $batch.EvidenceDir "danger-command-classification.json") -InputObject $classification

$pathChecks = @(
  @{ path = "C:\catpaw"; expected = $true },
  @{ path = "C:\catpaw\conf.d\config.toml"; expected = $true },
  @{ path = "C:\Users"; expected = $false },
  @{ path = "D:\ai-workbench"; expected = $false },
  @{ path = "/etc/catpaw"; expected = $true },
  @{ path = "/home"; expected = $false }
)
$pathResults = @()
foreach ($pathCheck in $pathChecks) {
  $actual = Test-AiwAllowedPath -Path $pathCheck.path -Policy $policy
  if ($actual -ne $pathCheck.expected) {
    $blockers.Add("P0 path guard mismatch for $($pathCheck.path): expected $($pathCheck.expected), got $actual")
  }
  $pathResults += [ordered]@{ path = $pathCheck.path; expected = $pathCheck.expected; actual = $actual; ok = ($actual -eq $pathCheck.expected) }
}
ConvertTo-AiwJsonFile -Path (Join-Path $batch.EvidenceDir "path-guard-results.json") -InputObject $pathResults

$hostChecks = @(
  @{ host = "192.168.1.7"; expected = $true },
  @{ host = "127.0.0.1"; expected = $true },
  @{ host = "198.18.20.11"; expected = $false },
  @{ host = "8.8.8.8"; expected = $false }
)
$hostResults = @()
foreach ($hostCheck in $hostChecks) {
  $actual = Test-AiwAllowedHost -Host $hostCheck.host -Policy $policy
  if ($actual -ne $hostCheck.expected) {
    $blockers.Add("P0 host guard mismatch for $($hostCheck.host): expected $($hostCheck.expected), got $actual")
  }
  $hostResults += [ordered]@{ host = $hostCheck.host; expected = $hostCheck.expected; actual = $actual; ok = ($actual -eq $hostCheck.expected) }
}
ConvertTo-AiwJsonFile -Path (Join-Path $batch.EvidenceDir "host-guard-results.json") -InputObject $hostResults

$score = 100
if ($blockers.Count -gt 0) { $score = 0 }
elseif ($findings.Count -gt 0) { $score = 95 }

Write-AiwBatchSkeleton -EvidenceDir $batch.EvidenceDir -BatchId $batch.BatchId -BatchName "S0 Safety Precheck" -Score $score -Blockers $blockers.ToArray() -Findings $findings.ToArray()

Write-Host "S0 safety precheck evidence: $($batch.EvidenceDir)"
if ($blockers.Count -gt 0) { exit 1 }
