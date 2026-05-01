$ErrorActionPreference = 'Stop'
$secure = ConvertTo-SecureString 'lzt131456' -AsPlainText -Force
$cred = New-Object System.Management.Automation.PSCredential('DESKTOP-7QO0TFK\admin', $secure)
$sessionOption = New-PSSessionOption -SkipCACheck -SkipCNCheck -SkipRevocationCheck
$session = $null
$lastError = $null
foreach ($auth in @('Negotiate','Basic')) {
  try {
    Write-Output "try $auth"
    $session = New-PSSession -ComputerName '192.168.1.7' -Port 5985 -Credential $cred -Authentication $auth -SessionOption $sessionOption
    Write-Output "got $($session.Id)"
    break
  } catch {
    Write-Output "err $auth $($_.Exception.Message)"
    $lastError = $_
  }
}
if (-not $session) { throw $lastError }
try {
  $r=Invoke-Command -Session $session -ScriptBlock { hostname; whoami }
  Write-Output "count=$($r.Count)"
  $r | % { Write-Output "OUT: $_" }
} finally {
  if ($session) { Remove-PSSession $session }
}
