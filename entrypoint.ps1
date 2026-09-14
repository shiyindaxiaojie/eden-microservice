param(
  [Parameter(Mandatory = $true, Position = 0)]
  [ValidateSet("build", "restart", "start", "stop")]
  [string]$Action,
  [string]$NodeExe = "",
  [switch]$SkipInstall
)

$ErrorActionPreference = "Stop"

$repoRoot = $PSScriptRoot
$runtimeRoot = Join-Path $repoRoot ".tmp\eden-microservice"
$binaryDir = Join-Path $runtimeRoot "bin"
$binaryPath = Join-Path $binaryDir "eden-microservice.exe"
$logDir = Join-Path $runtimeRoot "logs"
$serverStdoutLog = Join-Path $logDir "server.out.log"
$serverStderrLog = Join-Path $logDir "server.err.log"
$uiStdoutLog = Join-Path $logDir "ui.out.log"
$uiStderrLog = Join-Path $logDir "ui.err.log"
$serverPidPath = Join-Path $runtimeRoot "server.pid"
$uiPidPath = Join-Path $runtimeRoot "ui.pid"

$webRoot = Join-Path $repoRoot "apps\ui"
$viteCli = Join-Path $webRoot "node_modules\vite\bin\vite.js"

function Remove-PidFile {
  param([string]$Path)

  if (Test-Path -LiteralPath $Path -PathType Leaf) {
    Remove-Item -LiteralPath $Path -Force
  }
}

function Get-ManagedProcess {
  param(
    [string]$Name,
    [string]$PidFile,
    [string]$ExpectedPath,
    [string]$ExpectedCommandFragment = "",
    [switch]$Quiet
  )

  if (-not (Test-Path -LiteralPath $PidFile -PathType Leaf)) {
    return $null
  }

  $rawProcessID = (Get-Content -LiteralPath $PidFile -Raw).Trim()
  $processID = 0
  if (-not [int]::TryParse($rawProcessID, [ref]$processID)) {
    if (-not $Quiet) {
      Write-Warning "$Name PID 文件无效，已清理: $PidFile"
    }
    Remove-PidFile -Path $PidFile
    return $null
  }

  $managed = Get-Process -Id $processID -ErrorAction SilentlyContinue
  if (-not $managed) {
    Remove-PidFile -Path $PidFile
    return $null
  }

  $actualPath = ""
  try {
    $actualPath = [System.IO.Path]::GetFullPath($managed.Path)
  }
  catch {
    $actualPath = ""
  }
  $normalizedExpectedPath = [System.IO.Path]::GetFullPath($ExpectedPath)
  if (-not $actualPath.Equals($normalizedExpectedPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    if (-not $Quiet) {
      Write-Warning "$Name PID $processID 已被其他进程复用，拒绝停止该进程并清理陈旧 PID 文件。"
    }
    Remove-PidFile -Path $PidFile
    return $null
  }

  if ($ExpectedCommandFragment) {
    try {
      $processInfo = Get-CimInstance -ClassName Win32_Process -Filter "ProcessId = $processID" -ErrorAction Stop
    }
    catch {
      throw "无法验证 $Name 进程命令行，拒绝管理 PID $processID。"
    }
    if (-not $processInfo -or -not $processInfo.CommandLine -or $processInfo.CommandLine.IndexOf($ExpectedCommandFragment, [System.StringComparison]::OrdinalIgnoreCase) -lt 0) {
      if (-not $Quiet) {
        Write-Warning "$Name PID $processID 不是本仓库启动的进程，拒绝停止并清理陈旧 PID 文件。"
      }
      Remove-PidFile -Path $PidFile
      return $null
    }
  }

  return $managed
}

function Show-RecentLogs {
  param([string[]]$Paths)

  foreach ($path in $Paths) {
    if (Test-Path -LiteralPath $path -PathType Leaf) {
      Write-Host "--- $path"
      Get-Content -LiteralPath $path -Tail 20
    }
  }
}

function Get-ListeningEndpoint {
  param(
    [System.Diagnostics.Process]$Process,
    [int]$FallbackPort,
    [string]$HealthPath = "/health",
    [int]$TimeoutSeconds = 180
  )

  $timer = [System.Diagnostics.Stopwatch]::StartNew()
  $nextProgress = 0
  do {
    $Process.Refresh()
    if ($Process.HasExited) {
      throw "进程 PID $($Process.Id) 在启动完成前退出，退出码: $($Process.ExitCode)"
    }
    $connections = @()
    try {
      $connections = @(Get-NetTCPConnection -OwningProcess $Process.Id -State Listen -ErrorAction Stop)
    }
    catch { $connections = @() }
    foreach ($connection in $connections | Sort-Object LocalPort, LocalAddress) {
      $address = $connection.LocalAddress
      if ($address -eq '0.0.0.0') { $address = '127.0.0.1' }
      if ($address -eq '::') { $address = '::1' }
      if ($address.Contains(':')) { $address = "[$address]" }
      $url = "http://${address}:$($connection.LocalPort)"
      try {
        $response = Invoke-WebRequest -Uri "$url$HealthPath" -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
          $listenAddress = $connection.LocalAddress
          if ($listenAddress.Contains(':')) { $listenAddress = "[$listenAddress]" }
          return [PSCustomObject]@{
            Listen = "${listenAddress}:$($connection.LocalPort)"
            Port = $connection.LocalPort
            Url = $url
          }
        }
      }
      catch { }
    }
    if ($timer.Elapsed.TotalSeconds -ge $TimeoutSeconds) { break }
    if ($timer.Elapsed.TotalSeconds -ge $nextProgress) {
      Write-Host "正在等待 PID $($Process.Id) 启动就绪（预期端口 $FallbackPort，已等待 $([int]$timer.Elapsed.TotalSeconds) 秒）..."
      $nextProgress = $timer.Elapsed.TotalSeconds + 10
    }
    Start-Sleep -Milliseconds 500
  } while ($timer.Elapsed.TotalSeconds -lt $TimeoutSeconds)
  throw "启动超时：PID $($Process.Id) 在 $TimeoutSeconds 秒内未通过监听及 HTTP 健康检查（预期端口 $FallbackPort）。请查看日志。"
}
function Show-RuntimeStatus {
  param(
    [System.Diagnostics.Process]$Server,
    [System.Diagnostics.Process]$UI
  )

  try {
    $serverEndpoint = Get-ListeningEndpoint -Process $Server -FallbackPort 8500
    $uiEndpoint = Get-ListeningEndpoint -Process $UI -FallbackPort 2019 -HealthPath "/"
  }
  catch {
    Show-RecentLogs -Paths @($serverStdoutLog, $serverStderrLog, $uiStdoutLog, $uiStderrLog)
    throw
  }

  Write-Host ""
  Write-Host "Eden Microservice 前后端已运行"
  Write-Host "后端 (server)"
  Write-Host "  PID:      $($Server.Id)"
  Write-Host "  监听地址: $($serverEndpoint.Listen)"
  Write-Host "  API 地址: $($serverEndpoint.Url)"
  Write-Host "  stdout:   $serverStdoutLog"
  Write-Host "  stderr:   $serverStderrLog"
  Write-Host "前端 (ui)"
  Write-Host "  PID:      $($UI.Id)"
  Write-Host "  监听地址: $($uiEndpoint.Listen)"
  Write-Host "  访问地址: $($uiEndpoint.Url)"
  Write-Host "  stdout:   $uiStdoutLog"
  Write-Host "  stderr:   $uiStderrLog"
}

function Resolve-NodeExe {
  param([string]$Preferred)

  $candidates = @()
  if ($Preferred) {
    $candidates += $Preferred
  }
  $command = Get-Command node -ErrorAction SilentlyContinue
  if ($command -and $command.Source) {
    $candidates += $command.Source
  }

  $candidates += "C:\Program Files\nodejs\node.exe"

  foreach ($candidate in $candidates | Select-Object -Unique) {
    if ($candidate -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
      return (Resolve-Path -LiteralPath $candidate).Path
    }
  }
  throw "未找到可用的 node.exe，请通过 -NodeExe 显式传入。"
}

function Start-UIProcess {
  param([string]$ResolvedNode)

  $originalNoColor = [Environment]::GetEnvironmentVariable("NO_COLOR", "Process")
  $originalForceColor = [Environment]::GetEnvironmentVariable("FORCE_COLOR", "Process")
  try {
    [Environment]::SetEnvironmentVariable("NO_COLOR", "1", "Process")
    [Environment]::SetEnvironmentVariable("FORCE_COLOR", $null, "Process")
    return Start-Process `
      -FilePath $ResolvedNode `
      -ArgumentList @(('"{0}"' -f $viteCli), "--host", "127.0.0.1") `
      -WorkingDirectory $webRoot `
      -WindowStyle Hidden `
      -RedirectStandardOutput $uiStdoutLog `
      -RedirectStandardError $uiStderrLog `
      -PassThru
  }
  finally {
    [Environment]::SetEnvironmentVariable("NO_COLOR", $originalNoColor, "Process")
    [Environment]::SetEnvironmentVariable("FORCE_COLOR", $originalForceColor, "Process")
  }
}

function Install-UIWorkspaceIfNeeded {
  param([string]$ResolvedNode)

  if (Test-Path -LiteralPath $viteCli -PathType Leaf) {
    return
  }
  if ($SkipInstall) {
    throw "未找到 Vite: $viteCli。请移除 -SkipInstall 后重试，或先安装前端依赖。"
  }

  $nodeHome = Split-Path -Parent $ResolvedNode
  $npmCli = Join-Path $nodeHome "node_modules\npm\bin\npm-cli.js"
  if (-not (Test-Path -LiteralPath $npmCli -PathType Leaf)) {
    throw "未找到 npm-cli.js: $npmCli"
  }

  Write-Host "安装前端依赖..."
  Push-Location $webRoot
  $originalPath = $env:PATH
  try {
    $env:PATH = $nodeHome + [IO.Path]::PathSeparator + $env:PATH
    & $ResolvedNode $npmCli ci
    if ($LASTEXITCODE -ne 0) {
      throw "前端依赖安装失败，退出码: $LASTEXITCODE"
    }
  }
  finally {
    $env:PATH = $originalPath
    Pop-Location
  }
  if (-not (Test-Path -LiteralPath $viteCli -PathType Leaf)) {
    throw "安装完成后仍未找到 Vite: $viteCli"
  }
}

function Invoke-Build {
  $resolvedNode = Resolve-NodeExe -Preferred $NodeExe
  $server = Get-ManagedProcess -Name "后端" -PidFile $serverPidPath -ExpectedPath $binaryPath -Quiet
  $ui = Get-ManagedProcess -Name "前端" -PidFile $uiPidPath -ExpectedPath $resolvedNode -ExpectedCommandFragment $viteCli -Quiet
  if ($server -or $ui) {
    throw "前端或后端正在运行。请先执行 ./entrypoint.ps1 stop，再执行 build。"
  }

  if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "未找到 go 命令，请先安装 Go 并加入 PATH。"
  }
  New-Item -ItemType Directory -Path $binaryDir -Force | Out-Null
  Install-UIWorkspaceIfNeeded -ResolvedNode $resolvedNode
  $npmCli = Join-Path (Split-Path -Parent $resolvedNode) "node_modules\npm\bin\npm-cli.js"
  if (-not (Test-Path -LiteralPath $npmCli -PathType Leaf)) {
    throw "未找到 npm-cli.js: $npmCli"
  }
  Write-Host "[1/2] 构建前端..."
  Push-Location $webRoot
  $originalPath = $env:PATH
  try {
    $env:PATH = (Split-Path -Parent $resolvedNode) + [IO.Path]::PathSeparator + $env:PATH
    & $resolvedNode $npmCli run build
    if ($LASTEXITCODE -ne 0) { throw "前端构建失败，退出码: $LASTEXITCODE" }
  }
  finally {
    $env:PATH = $originalPath
    Pop-Location
  }
  Write-Host "[2/2] 构建后端..."
  Push-Location $repoRoot
  try {
    & go build -o $binaryPath ./apps/server/cmd/server
    if ($LASTEXITCODE -ne 0) {
      throw "后端构建失败，退出码: $LASTEXITCODE"
    }
  }
  finally {
    Pop-Location
  }

  Write-Host "构建完成: $binaryPath"
}

function Invoke-Start {
  $resolvedNode = Resolve-NodeExe -Preferred $NodeExe
  $server = Get-ManagedProcess -Name "后端" -PidFile $serverPidPath -ExpectedPath $binaryPath
  $ui = Get-ManagedProcess -Name "前端" -PidFile $uiPidPath -ExpectedPath $resolvedNode -ExpectedCommandFragment $viteCli
  if ($server -and $ui) {
    Show-RuntimeStatus -Server $server -UI $ui
    return
  }

  if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
    Write-Host "未找到构建产物，开始首次构建。"
    Invoke-Build
  }
  Install-UIWorkspaceIfNeeded -ResolvedNode $resolvedNode

  New-Item -ItemType Directory -Path $logDir -Force | Out-Null
  $serverStartedHere = $false
  $uiStartedHere = $false
  try {
    if (-not $server) {
      Set-Content -LiteralPath $serverStdoutLog -Value "" -Encoding utf8
      Set-Content -LiteralPath $serverStderrLog -Value "" -Encoding utf8
      $server = Start-Process `
        -FilePath $binaryPath `
        -WorkingDirectory $repoRoot `
        -WindowStyle Hidden `
        -RedirectStandardOutput $serverStdoutLog `
        -RedirectStandardError $serverStderrLog `
        -PassThru
      Set-Content -LiteralPath $serverPidPath -Value $server.Id -Encoding ascii
      $serverStartedHere = $true

      Start-Sleep -Milliseconds 1200
      $server.Refresh()
      if ($server.HasExited) {
        Remove-PidFile -Path $serverPidPath
        Show-RecentLogs -Paths @($serverStdoutLog, $serverStderrLog)
        throw "后端启动失败，退出码: $($server.ExitCode)"
      }
    }

    if (-not $ui) {
      Set-Content -LiteralPath $uiStdoutLog -Value "" -Encoding utf8
      Set-Content -LiteralPath $uiStderrLog -Value "" -Encoding utf8
      $ui = Start-UIProcess -ResolvedNode $resolvedNode
      $uiStartedHere = $true
      Set-Content -LiteralPath $uiPidPath -Value $ui.Id -Encoding ascii

      Start-Sleep -Milliseconds 1200
      $ui.Refresh()
      if ($ui.HasExited) {
        Remove-PidFile -Path $uiPidPath
        Show-RecentLogs -Paths @($uiStdoutLog, $uiStderrLog)
        throw "前端启动失败，退出码: $($ui.ExitCode)"
      }
    }
    Show-RuntimeStatus -Server $server -UI $ui
  }
  catch {
    if ($uiStartedHere -and $ui) {
      Stop-Process -Id $ui.Id -ErrorAction SilentlyContinue
      [void]$ui.WaitForExit(10000)
      Remove-PidFile -Path $uiPidPath
    }
    if ($serverStartedHere -and $server) {
      Stop-Process -Id $server.Id -ErrorAction SilentlyContinue
      [void]$server.WaitForExit(10000)
      Remove-PidFile -Path $serverPidPath
    }
    throw
  }

}

function Invoke-Stop {
  $resolvedNode = $null
  if (Test-Path -LiteralPath $uiPidPath -PathType Leaf) {
    $resolvedNode = Resolve-NodeExe -Preferred $NodeExe
  }
  $ui = $null
  if ($resolvedNode) {
    $ui = Get-ManagedProcess -Name "前端" -PidFile $uiPidPath -ExpectedPath $resolvedNode -ExpectedCommandFragment $viteCli
  }
  $server = Get-ManagedProcess -Name "后端" -PidFile $serverPidPath -ExpectedPath $binaryPath

  if ($ui) {
    $uiProcessID = $ui.Id
    Stop-Process -Id $uiProcessID -ErrorAction Stop
    [void]$ui.WaitForExit(10000)
    Remove-PidFile -Path $uiPidPath
    Write-Host "前端已停止，PID: $uiProcessID"
  }
  if ($server) {
    $serverProcessID = $server.Id
    Stop-Process -Id $serverProcessID -ErrorAction Stop
    [void]$server.WaitForExit(10000)
    Remove-PidFile -Path $serverPidPath
    Write-Host "后端已停止，PID: $serverProcessID"
  }
  if (-not $ui -and -not $server) {
    Write-Host "前后端均未运行。"
  }
}

switch ($Action.ToLowerInvariant()) {
  "build" {
    Invoke-Build
  }
  "start" {
    Invoke-Start
  }
  "stop" {
    Invoke-Stop
  }
  "restart" {
    Invoke-Stop
    Invoke-Start
  }
}
