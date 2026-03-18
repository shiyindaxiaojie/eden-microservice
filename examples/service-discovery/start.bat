@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Eden Registry - Service Discovery Demo
echo ======================================================
echo.

set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

echo [1/4] Starting User Center (port 9001)...
start "User-Center" cmd /c "go run ./examples/service-discovery/cmd/user-center/main.go"
timeout /t 2 /nobreak >nul

echo [2/4] Starting Auth Center (port 9002)...
start "Auth-Center" cmd /c "go run ./examples/service-discovery/cmd/auth-center/main.go"
timeout /t 2 /nobreak >nul

echo [3/4] Starting Order Center (port 9003)...
start "Order-Center" cmd /c "go run ./examples/service-discovery/cmd/order-center/main.go"
timeout /t 2 /nobreak >nul

echo.
echo All services started!
echo ------------------------------------------------------
echo  User Center:  http://localhost:9001/api/users
echo  Auth Center:  http://localhost:9002/api/auth/token
echo  Order Center: http://localhost:9003/api/order/demo
echo ------------------------------------------------------
echo.
echo Try: curl http://localhost:9003/api/order/demo
echo.
echo Press any key to stop all services...
pause >nul

taskkill /fi "windowtitle eq User-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Auth-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Order-Center" /im cmd.exe /t /f >nul 2>&1

echo Done.
pause
