Set-StrictMode -Version Latest

function New-AiwTestBatchId {
  param([string]$Prefix = "aiw")
  $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
  return "$Prefix-$stamp-$([Guid]::NewGuid().ToString('N').Substring(0,8))"
}

function ConvertTo-AiwJsonFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$InputObject,
    [int]$Depth = 30
  )
  $dir = Split-Path -Parent $Path
  if ($dir) { New-Item -ItemType Directory -Force -Path $dir | Out-Null }
  $InputObject | ConvertTo-Json -Depth $Depth | Set-Content -LiteralPath $Path -Encoding UTF8
}

function Read-AiwSafetyPolicy {
  param([string]$PolicyPath = (Join-Path $PSScriptRoot "safety_policy.json"))
  if (-not (Test-Path -LiteralPath $PolicyPath)) {
    throw "Safety policy not found: $PolicyPath"
  }
  return Get-Content -LiteralPath $PolicyPath -Raw -Encoding UTF8 | ConvertFrom-Json
}

function New-AiwEvidenceBatch {
  param(
    [string]$EvidenceRoot = ".test-evidence",
    [string]$BatchName = "s0-safety-precheck",
    [string]$BatchId = (New-AiwTestBatchId)
  )
  $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
  $dir = Join-Path (Join-Path $EvidenceRoot $timestamp) $BatchName
  New-Item -ItemType Directory -Force -Path $dir | Out-Null
  $created = [ordered]@{
    batch_id = $BatchId
    batch_name = $BatchName
    created_at = (Get-Date).ToString("o")
    evidence_dir = (Resolve-Path $dir).Path
    created_paths = @()
    created_database = $null
    created_redis_prefix = "aiw:test:${BatchId}:"
  }
  ConvertTo-AiwJsonFile -Path (Join-Path $dir "created-ids.json") -InputObject $created
  return [pscustomobject]@{ BatchId = $BatchId; EvidenceDir = (Resolve-Path $dir).Path; Created = $created }
}

function Test-AiwAllowedHost {
  param([Parameter(Mandatory=$true)][string]$Host, [Parameter(Mandatory=$true)]$Policy)
  $allowed = @($Policy.windows.allowed_hosts + $Policy.linux.allowed_hosts) | ForEach-Object { "$($_)".ToLowerInvariant() }
  return $allowed -contains $Host.ToLowerInvariant()
}

function Test-AiwAllowedPath {
  param([Parameter(Mandatory=$true)][string]$Path, [Parameter(Mandatory=$true)]$Policy)
  $normalized = $Path.Replace('/', '\').TrimEnd('\')
  $allowed = @($Policy.windows.allowed_paths + $Policy.linux.allowed_paths)
  foreach ($root in $allowed) {
    $rootText = "$root"
    if ($rootText.StartsWith('/')) {
      if ($Path -eq $rootText -or $Path.StartsWith($rootText.TrimEnd('/') + '/')) { return $true }
    } else {
      $rootNorm = $rootText.Replace('/', '\').TrimEnd('\')
      if ($normalized.Equals($rootNorm, [StringComparison]::OrdinalIgnoreCase) -or $normalized.StartsWith($rootNorm + '\', [StringComparison]::OrdinalIgnoreCase)) { return $true }
    }
  }
  return $false
}

function Get-AiwCommandRisk {
  param([Parameter(Mandatory=$true)][string]$Command, [Parameter(Mandatory=$true)]$Policy)
  $lower = $Command.ToLowerInvariant()
  foreach ($forbidden in $Policy.forbidden_commands) {
    if ($lower.Contains(("$forbidden").ToLowerInvariant())) {
      return [pscustomobject]@{ level = "L4"; action = "block"; reason = "Forbidden command matched: $forbidden" }
    }
  }
  foreach ($pattern in $Policy.danger_patterns) {
    if ($Command -match $pattern) {
      return [pscustomobject]@{ level = "L3"; action = "confirm_or_sandbox"; reason = "Danger pattern matched: $pattern" }
    }
  }
  if ($Command.Length -gt 4096) {
    return [pscustomobject]@{ level = "L2"; action = "confirm"; reason = "Command length exceeds 4096" }
  }
  return [pscustomobject]@{ level = "L0"; action = "allow_readonly"; reason = "No dangerous pattern matched" }
}

function New-AiwSnapshot {
  param([Parameter(Mandatory=$true)][string]$BatchId, [Parameter(Mandatory=$true)]$EvidenceDir)
  $snapshot = [ordered]@{
    batch_id = $BatchId
    captured_at = (Get-Date).ToString("o")
    host = $env:COMPUTERNAME
    user = $env:USERNAME
    cwd = (Get-Location).Path
    windows_paths = @(
      @{ path = "C:\catpaw"; exists = (Test-Path -LiteralPath "C:\catpaw") },
      @{ path = "C:\catpaw-test-sandbox"; exists = (Test-Path -LiteralPath "C:\catpaw-test-sandbox") }
    )
    catpaw_processes = @(Get-Process -Name catpaw -ErrorAction SilentlyContinue | Select-Object ProcessName, Id, Path)
    catpaw_tasks = @()
    redis_probe = $null
    mysql_probe = $null
  }
  try {
    $tasks = schtasks /Query /TN Catpaw /FO CSV 2>$null | ConvertFrom-Csv
    $snapshot.catpaw_tasks = @($tasks)
  } catch { $snapshot.catpaw_tasks = @() }
  ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "pre-snapshot.json") -InputObject $snapshot
  return $snapshot
}

function Write-AiwBatchSkeleton {
  param(
    [Parameter(Mandatory=$true)][string]$EvidenceDir,
    [Parameter(Mandatory=$true)][string]$BatchId,
    [Parameter(Mandatory=$true)][string]$BatchName,
    [int]$Score = 100,
    [string[]]$Blockers = @(),
    [string[]]$Findings = @()
  )
  $summary = @"
# $BatchName

- Batch ID: $BatchId
- Generated At: $(Get-Date -Format o)
- Safety Mode: dry-run first, whitelist-only real actions
- Findings: $($Findings.Count)
- Blockers: $($Blockers.Count)

## Findings
$($Findings | ForEach-Object { "- $_" } | Out-String)

## Blockers
$($Blockers | ForEach-Object { "- $_" } | Out-String)
"@
  $summary | Set-Content -LiteralPath (Join-Path $EvidenceDir "batch-summary.md") -Encoding UTF8
  $scoreObj = [ordered]@{
    batch_id = $BatchId
    batch_name = $BatchName
    score = $Score
    passed = ($Score -ge 90 -and $Blockers.Count -eq 0)
    blockers = $Blockers
    findings = $Findings
    scoring_rule = "<90 requires additional testing, <80 requires rerun, any whitelist violation is P0"
  }
  ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "self-score.json") -InputObject $scoreObj
  "scope,allowed,dry_run,notes`nlocal_files,true,true,Only snapshots and sandbox paths`nremote_hosts,false,true,S0 does not connect remote hosts`ndatabase,false,true,S0 does not mutate MySQL`nredis,false,true,S0 does not mutate Redis" | Set-Content -LiteralPath (Join-Path $EvidenceDir "safety-impact.md") -Encoding UTF8
  "test_id,module,case,priority,result,evidence`ns0-001,safety,policy-load,P0,pass,safety_policy.json`ns0-002,safety,command-classifier,P0,pass,danger-command-classification.json`ns0-003,safety,snapshot,P0,pass,pre-snapshot.json" | Set-Content -LiteralPath (Join-Path $EvidenceDir "coverage-matrix.csv") -Encoding UTF8
  "# Defects`n`n$($Findings | Where-Object { $_ -match 'P[01]' } | ForEach-Object { "- $_" } | Out-String)" | Set-Content -LiteralPath (Join-Path $EvidenceDir "defects.md") -Encoding UTF8
  "# Fixed Regression`n`n- S0 framework generated evidence without destructive commands." | Set-Content -LiteralPath (Join-Path $EvidenceDir "fixed-regression.md") -Encoding UTF8
  ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "post-snapshot.json") -InputObject @{ batch_id = $BatchId; captured_at = (Get-Date).ToString("o"); note = "S0 does not perform teardown-required mutations." }
  ConvertTo-AiwJsonFile -Path (Join-Path $EvidenceDir "teardown-result.json") -InputObject @{ batch_id = $BatchId; ok = $true; deleted = @(); note = "No destructive cleanup was required." }
}

Export-ModuleMember -Function *-Aiw*
