@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Eden Registry - Service Discovery Demo (Multi-Instance)
echo ======================================================
echo.

set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

echo [1/3] Starting Auth Center (3 instances: 9002, 9012, 9022)...
start "Auth-Center-1" cmd /c "set SERVICE_PORT=9002&& set SERVICE_ID=auth-center-1&& go run ./examples/service-discovery/cmd/auth-center/main.go"
start "Auth-Center-2" cmd /c "set SERVICE_PORT=9012&& set SERVICE_ID=auth-center-2&& go run ./examples/service-discovery/cmd/auth-center/main.go"
start "Auth-Center-3" cmd /c "set SERVICE_PORT=9022&& set SERVICE_ID=auth-center-3&& go run ./examples/service-discovery/cmd/auth-center/main.go"
timeout /t 2 /nobreak >nul

echo [2/3] Starting User Center (2 instances: 9001, 9011)...
start "User-Center-1" cmd /c "set SERVICE_PORT=9001&& set SERVICE_ID=user-center-1&& go run ./examples/service-discovery/cmd/user-center/main.go"
start "User-Center-2" cmd /c "set SERVICE_PORT=9011&& set SERVICE_ID=user-center-2&& go run ./examples/service-discovery/cmd/user-center/main.go"
timeout /t 2 /nobreak >nul

echo [3/3] Starting Order Center (2 instances: 9003, 9013)...
start "Order-Center-1" cmd /c "set SERVICE_PORT=9003&& set SERVICE_ID=order-center-1&& go run ./examples/service-discovery/cmd/order-center/main.go"
start "Order-Center-2" cmd /c "set SERVICE_PORT=9013&& set SERVICE_ID=order-center-2&& go run ./examples/service-discovery/cmd/order-center/main.go"
timeout /t 2 /nobreak >nul

echo.
echo All services started!
echo ------------------------------------------------------
echo  Auth Center:  :9002, :9012, :9022
echo  User Center:  :9001, :9011
echo  Order Center: :9003, :9013
echo ------------------------------------------------------
echo.
echo Try: curl http://localhost:9003/api/order/demo
echo.
echo Press any key to stop all services...
pause >nul

taskkill /fi "windowtitle eq Auth-Center-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Auth-Center-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Auth-Center-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq User-Center-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq User-Center-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Order-Center-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Order-Center-2" /im cmd.exe /t /f >nul 2>&1

:: Also kill the old single instances if they happen to be running
taskkill /fi "windowtitle eq Auth-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq User-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Order-Center" /im cmd.exe /t /f >nul 2>&1

echo Done.
pause
