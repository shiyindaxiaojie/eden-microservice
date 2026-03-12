@echo off
echo Starting Eden Go Registry AP Cluster (3 Nodes)...

set WORKDIR=%~dp0..\..

cd %WORKDIR%

echo Starting Node 1 (Port 8500)...
start "Eden-Node1" cmd /c "go run ./cmd/server/main.go -config configs/node1.yaml"

timeout /t 2 /nobreak >nul

echo Starting Node 2 (Port 8501)...
start "Eden-Node2" cmd /c "go run ./cmd/server/main.go -config configs/node2.yaml"

timeout /t 2 /nobreak >nul

echo Starting Node 3 (Port 8502)...
start "Eden-Node3" cmd /c "go run ./cmd/server/main.go -config configs/node3.yaml"

echo.
echo AP Cluster started successfully!
echo Node 1: http://localhost:8500
echo Node 2: http://localhost:8501
echo Node 3: http://localhost:8502
echo.
echo Press any key to stop all nodes...
pause >nul

taskkill /fi "windowtitle eq Eden-Node1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Eden-Node2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Eden-Node3" /im cmd.exe /t /f >nul 2>&1

echo Cluster stopped.
