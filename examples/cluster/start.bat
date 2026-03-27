@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Eden Go Registry - 3 Node Cluster Startup
echo ======================================================
echo.

set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

:: 0. Cleanup old processes
echo [0/5] Cleaning up old processes...
taskkill /fi "windowtitle eq Eden-Node*" /im cmd.exe /t /f >nul 2>&1
timeout /t 1 /nobreak >nul

:: 1. Clean old cluster data to avoid stale settings
echo [1/5] Cleaning old cluster data...
if exist "data\node1" rd /s /q "data\node1"
if exist "data\node2" rd /s /q "data\node2"
if exist "data\node3" rd /s /q "data\node3"

:: 2. Pre-build the server binary (so go run doesn't slow down startup)
echo [2/5] Building server binary...
go build -o eden-server.exe ./cmd/server/main.go
if errorlevel 1 (
    echo ERROR: Build failed!
    pause
    exit /b 1
)
echo       Build successful.
echo.

:: --- Node 1 ---
echo [3/5] Starting backend nodes...
echo        Node 1 (HTTP: 8500)...
start "Eden-Node1-Backend" cmd /c "eden-server.exe -config examples/cluster/configs/node1.yaml"
timeout /t 3 /nobreak >nul

:: --- Node 2 ---
echo        Node 2 (HTTP: 8501)...
start "Eden-Node2-Backend" cmd /c "eden-server.exe -config examples/cluster/configs/node2.yaml"
timeout /t 2 /nobreak >nul

:: --- Node 3 ---
echo        Node 3 (HTTP: 8502)...
start "Eden-Node3-Backend" cmd /c "eden-server.exe -config examples/cluster/configs/node3.yaml"
timeout /t 2 /nobreak >nul

:: --- Cluster membership ---
echo [4/5] Registering cluster members through API...
powershell -NoProfile -Command "$login = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8500/v1/auth/login' -ContentType 'application/json' -Body '{\"username\":\"admin\",\"password\":\"8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918\"}'; $headers = @{ Authorization = 'Bearer ' + $login.token }; Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:8500/v1/cluster/member' -Headers $headers -ContentType 'application/json' -Body '{\"addresses\":[\"http://127.0.0.1:8501\",\"http://127.0.0.1:8502\"]}' | Out-Null"
if errorlevel 1 (
    echo ERROR: Failed to register cluster members!
    pause
    exit /b 1
)
echo        Cluster membership configured.
echo.

:: --- Frontends ---
echo [5/5] Starting frontend dev servers...
cd web
start "Eden-Node1-Frontend" cmd /c "set VITE_PORT=2019&& set VITE_PROXY_TARGET=http://127.0.0.1:8500&& npx vite"
timeout /t 1 /nobreak >nul
start "Eden-Node2-Frontend" cmd /c "set VITE_PORT=2020&& set VITE_PROXY_TARGET=http://127.0.0.1:8501&& npx vite"
timeout /t 1 /nobreak >nul
start "Eden-Node3-Frontend" cmd /c "set VITE_PORT=2021&& set VITE_PROXY_TARGET=http://127.0.0.1:8502&& npx vite"
cd ..

echo.
echo ======================================================
echo  Cluster started successfully!
echo ======================================================
echo.
echo  Node 1: http://localhost:2019 (Backend: :8500)
echo  Node 2: http://localhost:2020 (Backend: :8501)
echo  Node 3: http://localhost:2021 (Backend: :8502)
echo.
echo  Press any key to stop all nodes...
pause >nul

:: Stop all processes
echo.
echo Shutting down...
taskkill /fi "windowtitle eq Eden-Node*" /im cmd.exe /t /f >nul 2>&1

:: Cleanup binary
del /q eden-server.exe >nul 2>&1

echo Done.
pause
