@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo Starting Eden Go Registry Cluster with Frontends...
echo ======================================================

set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

:: 1. Cleanup old frontend processes (optional but recommended)
taskkill /fi "windowtitle eq Eden-Node*" /im cmd.exe /t /f >nul 2>&1

:: --- Node 1 ---
echo [1/3] Starting Node 1 (Backend: 8500, Frontend: 2019)...
start "Eden-Node1-Backend" cmd /c "go run ./cmd/server/main.go -config examples/cluster/configs/node1.yaml"
timeout /t 2 /nobreak >nul
cd web
start "Eden-Node1-Frontend" cmd /c "set VITE_PORT=2019&& set VITE_PROXY_TARGET=http://127.0.0.1:8500&& npx vite"
cd ..

:: --- Node 2 ---
echo [2/3] Starting Node 2 (Backend: 8501, Frontend: 2020)...
start "Eden-Node2-Backend" cmd /c "go run ./cmd/server/main.go -config examples/cluster/configs/node2.yaml"
timeout /t 2 /nobreak >nul
cd web
start "Eden-Node2-Frontend" cmd /c "set VITE_PORT=2020&& set VITE_PROXY_TARGET=http://127.0.0.1:8501&& npx vite"
cd ..

:: --- Node 3 ---
echo [3/3] Starting Node 3 (Backend: 8502, Frontend: 2021)...
start "Eden-Node3-Backend" cmd /c "go run ./cmd/server/main.go -config examples/cluster/configs/node3.yaml"
timeout /t 2 /nobreak >nul
cd web
start "Eden-Node3-Frontend" cmd /c "set VITE_PORT=2021&& set VITE_PROXY_TARGET=http://127.0.0.1:8502&& npx vite"
cd ..

echo.
echo Cluster and Frontends started!
echo ------------------------------------------------------
echo Node 1 Control: http://localhost:2019 (Backend: 8500)
echo Node 2 Control: http://localhost:2020 (Backend: 8501)
echo Node 3 Control: http://localhost:2021 (Backend: 8502)
echo ------------------------------------------------------
echo.
echo Press any key to stop all nodes...
pause >nul

:: Stop all processes started in new windows with the specified titles
echo Shutting down...
taskkill /fi "windowtitle eq Eden-Node1-*" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Eden-Node2-*" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Eden-Node3-*" /im cmd.exe /t /f >nul 2>&1

echo Done.
pause
