$ErrorActionPreference = 'Stop'

$demoDir = $PSScriptRoot
$repoRoot = (Resolve-Path (Join-Path $demoDir '..\..\..')).Path
$serverBinary = Join-Path $demoDir '.demo-server.exe'
$clientBinary = Join-Path $demoDir '.demo-client.exe'
$dataDir = Join-Path $demoDir '.demo-data'

Push-Location $repoRoot
try {
    & go build -o $serverBinary ./cmd/server/main.go
    if ($LASTEXITCODE -ne 0) { throw 'Failed to build Eden server' }
    & go build -o $clientBinary ./examples/config/nacos/cmd/listener
    if ($LASTEXITCODE -ne 0) { throw 'Failed to build Nacos Config client' }

    $server = Start-Process -FilePath $serverBinary -ArgumentList @(
        '-http-addr', ':8858',
        '-data-dir', $dataDir,
        '-mode', 'standalone',
        '-consistency', 'ap',
        '-grpc', 'off',
        '-quic', 'off',
        '-raft', 'off'
    ) -WindowStyle Hidden -PassThru
    try {
        $healthy = $false
        for ($attempt = 0; $attempt -lt 60; $attempt++) {
            try {
                Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:8858/health' -TimeoutSec 2 | Out-Null
                $healthy = $true
                break
            } catch {
                Start-Sleep -Seconds 1
            }
        }
        if (-not $healthy) { throw 'Eden server did not become healthy' }

        & $clientBinary -server 127.0.0.1:8858
        if ($LASTEXITCODE -ne 0) { throw 'Nacos Config example failed' }
    } finally {
        Stop-Process -Id $server.Id -ErrorAction SilentlyContinue
    }
} finally {
    Pop-Location
}
