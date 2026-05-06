@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Official Consul API Service Discovery Demo
echo ======================================================
echo.

if "%CONSUL_ADDR%"=="" set CONSUL_ADDR=127.0.0.1:8500

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Focalors address: %CONSUL_ADDR%
echo.
echo [1/3] Starting auth-center instances...
start "consul-auth-1" cmd /c "set SERVICE_PORT=22002&& set SERVICE_ID=consul-auth-center-1&& go run ./examples/service-discovery/consul/cmd/auth-center"
start "consul-auth-2" cmd /c "set SERVICE_PORT=22012&& set SERVICE_ID=consul-auth-center-2&& go run ./examples/service-discovery/consul/cmd/auth-center"
start "consul-auth-3" cmd /c "set SERVICE_PORT=22022&& set SERVICE_ID=consul-auth-center-3&& go run ./examples/service-discovery/consul/cmd/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "consul-user-1" cmd /c "set SERVICE_PORT=22001&& set SERVICE_ID=consul-user-center-1&& go run ./examples/service-discovery/consul/cmd/user-center"
start "consul-user-2" cmd /c "set SERVICE_PORT=22011&& set SERVICE_ID=consul-user-center-2&& go run ./examples/service-discovery/consul/cmd/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "consul-order-1" cmd /c "set SERVICE_PORT=22003&& set SERVICE_ID=consul-order-center-1&& go run ./examples/service-discovery/consul/cmd/order-center"
start "consul-order-2" cmd /c "set SERVICE_PORT=22013&& set SERVICE_ID=consul-order-center-2&& go run ./examples/service-discovery/consul/cmd/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:22002/api/auth/token?user_id=1
echo   http://127.0.0.1:22001/api/users/1/profile
echo   http://127.0.0.1:22003/api/orders/create?user_id=1
echo   http://127.0.0.1:22003/api/orders/demo?user_id=1
echo.
echo Press any key to stop all demo processes...
pause >nul

taskkill /fi "windowtitle eq consul-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq consul-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.

