@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Official Nacos SDK Service Discovery Demo
echo ======================================================
echo.

if "%NACOS_ADDR%"=="" set NACOS_ADDR=127.0.0.1:8500

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Registry address: %NACOS_ADDR%
echo.
echo [1/3] Starting auth-center instances...
start "nacos-auth-1" cmd /c "set SERVICE_PORT=23002&& set SERVICE_ID=nacos-auth-center-1&& go run ./examples/service-discovery/nacos/cmd/auth-center"
start "nacos-auth-2" cmd /c "set SERVICE_PORT=23012&& set SERVICE_ID=nacos-auth-center-2&& go run ./examples/service-discovery/nacos/cmd/auth-center"
start "nacos-auth-3" cmd /c "set SERVICE_PORT=23022&& set SERVICE_ID=nacos-auth-center-3&& go run ./examples/service-discovery/nacos/cmd/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "nacos-user-1" cmd /c "set SERVICE_PORT=23001&& set SERVICE_ID=nacos-user-center-1&& go run ./examples/service-discovery/nacos/cmd/user-center"
start "nacos-user-2" cmd /c "set SERVICE_PORT=23011&& set SERVICE_ID=nacos-user-center-2&& go run ./examples/service-discovery/nacos/cmd/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "nacos-order-1" cmd /c "set SERVICE_PORT=23003&& set SERVICE_ID=nacos-order-center-1&& go run ./examples/service-discovery/nacos/cmd/order-center"
start "nacos-order-2" cmd /c "set SERVICE_PORT=23013&& set SERVICE_ID=nacos-order-center-2&& go run ./examples/service-discovery/nacos/cmd/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:23002/api/auth/token?user_id=1
echo   http://127.0.0.1:23001/api/users/1/profile
echo   http://127.0.0.1:23003/api/orders/create?user_id=1
echo   http://127.0.0.1:23003/api/orders/demo?user_id=1
echo.
echo Press any key to stop all demo processes...
pause >nul

taskkill /fi "windowtitle eq nacos-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq nacos-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.
