@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Focalors - Cluster Restarting
echo ======================================================
echo.

set SCRIPT_DIR=%~dp0
set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

:: 0. Stop all processes
echo [1/3] Stopping all cluster nodes...
taskkill /fi "windowtitle eq Focalors-Node*" /im cmd.exe /t /f >nul 2>&1
timeout /t 2 /nobreak >nul

:: 1. Optional: No data cleaning in restart by default to preserve state
:: echo [2/4] Cleaning old cluster data...
:: if exist "data\node1" rd /s /q "data\node1"
:: if exist "data\node2" rd /s /q "data\node2"
:: if exist "data\node3" rd /s /q "data\node3"

:: 2. Re-building
echo [2/3] Re-building server binary...
go build -o registry-server.exe ./cmd/server/main.go
if errorlevel 1 (
    echo ERROR: Build failed!
    pause
    exit /b 1
)
echo       Build successful.

:: 3. Start back up
echo [3/3] Restarting cluster...
call "%SCRIPT_DIR%start.bat"

echo.
echo Restart complete.


