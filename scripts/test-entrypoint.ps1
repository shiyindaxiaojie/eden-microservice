param([switch]$Smoke)
$ErrorActionPreference = "Stop"
$entrypoint = Join-Path (Split-Path $PSScriptRoot -Parent) "entrypoint.ps1"
if (-not (Test-Path -LiteralPath $entrypoint)) { throw "Missing entrypoint.ps1" }
$fixture = Join-Path (Split-Path $PSScriptRoot -Parent) (".tmp/entrypoint-test-" + [guid]::NewGuid())
New-Item -ItemType Directory -Force $fixture | Out-Null
Copy-Item -LiteralPath $entrypoint -Destination $fixture
$runtime = Join-Path $fixture ".tmp/eden-microservice"
New-Item -ItemType Directory -Force $runtime | Out-Null
$pidFile = Join-Path $runtime "server.pid"
$runner = (Get-Process -Id $PID).Path
foreach ($value in @("invalid", "$PID")) {
  Set-Content -LiteralPath $pidFile -Value $value
  & $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") stop
  if ($LASTEXITCODE -ne 0) { throw "stop failed for PID value $value" }
  if (Test-Path -LiteralPath $pidFile) { throw "Stale PID was not removed" }
  if (-not (Get-Process -Id $PID)) { throw "Unrelated process was stopped" }
}
& $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") stop
if ($LASTEXITCODE -ne 0) { throw "Repeated stop failed" }
Write-Host "PASS: invalid/stale PID cleanup, unrelated process protection, repeated stop"
if ($Smoke) {
  $repo = Split-Path $PSScriptRoot -Parent
  $binaryDir = Join-Path $runtime "bin"
  New-Item -ItemType Directory -Force $binaryDir | Out-Null
  Copy-Item -LiteralPath (Join-Path $repo ".tmp/eden-microservice/bin/eden-microservice.exe") -Destination $binaryDir
  $configDir = Join-Path $fixture "apps/server/config"
  New-Item -ItemType Directory -Force $configDir | Out-Null
  $testUI = Join-Path $fixture "apps/ui"
  New-Item -ItemType Directory -Force $testUI | Out-Null
  New-Item -ItemType Junction -Path (Join-Path $testUI "node_modules") -Target (Join-Path $repo "apps/ui/node_modules") | Out-Null
  Set-Content -LiteralPath (Join-Path $testUI "index.html") -Value '<!doctype html><title>Entrypoint smoke test</title>'
  Set-Content -LiteralPath (Join-Path $testUI "vite.config.mjs") -Value 'export default { cacheDir: ".vite-test", server: { host: "127.0.0.1", port: 12019 } }'
  Set-Content -LiteralPath (Join-Path $configDir "eden-microservice.yaml") -Value @"
mode: standalone
node_id: entrypoint-test
server:
  http: "127.0.0.1:0"
  grpc: "off"
  quic: "off"
  raft: "off"
data_dir: "./data"
"@
  $oldPort = $env:VITE_PORT
  $env:VITE_PORT = "12019"
  try {
    & $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") start -SkipInstall
    if ($LASTEXITCODE -ne 0) { throw "start failed" }
    $first = Get-Content -LiteralPath $pidFile
    & $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") start -SkipInstall
    if ($LASTEXITCODE -ne 0 -or (Get-Content -LiteralPath $pidFile) -ne $first) { throw "start was not idempotent" }
    & $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") restart -SkipInstall
    if ($LASTEXITCODE -ne 0) { throw "restart failed" }
    if (Get-Process -Id ([int]$first) -ErrorAction SilentlyContinue) { throw "restart left old server running" }
  }
  finally {
    & $runner -NoProfile -File (Join-Path $fixture "entrypoint.ps1") stop
    $env:VITE_PORT = $oldPort
  }
  if ($LASTEXITCODE -ne 0 -or (Test-Path -LiteralPath $pidFile) -or (Test-Path (Join-Path $runtime "ui.pid"))) { throw "stop did not clean PID files" }
  Write-Host "PASS: real server/UI start, repeated start, restart and stop"
}

