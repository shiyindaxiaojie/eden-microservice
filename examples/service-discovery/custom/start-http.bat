@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Custom Service Discovery Demo - HTTP
echo ======================================================
echo.

if "%CUSTOM_HTTP_ADDRS%"=="" set CUSTOM_HTTP_ADDRS=http://127.0.0.1:8500

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Focalors HTTP addresses: %CUSTOM_HTTP_ADDRS%
echo.
echo [1/3] Starting auth-center instances...
start "custom-http-auth-1" cmd /c "set SERVICE_PORT=24102&& set SERVICE_ID=custom-http-auth-center-1&& go run ./examples/service-discovery/custom/cmd/http/auth-center"
start "custom-http-auth-2" cmd /c "set SERVICE_PORT=24112&& set SERVICE_ID=custom-http-auth-center-2&& go run ./examples/service-discovery/custom/cmd/http/auth-center"
start "custom-http-auth-3" cmd /c "set SERVICE_PORT=24122&& set SERVICE_ID=custom-http-auth-center-3&& go run ./examples/service-discovery/custom/cmd/http/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "custom-http-user-1" cmd /c "set SERVICE_PORT=24101&& set SERVICE_ID=custom-http-user-center-1&& go run ./examples/service-discovery/custom/cmd/http/user-center"
start "custom-http-user-2" cmd /c "set SERVICE_PORT=24111&& set SERVICE_ID=custom-http-user-center-2&& go run ./examples/service-discovery/custom/cmd/http/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "custom-http-order-1" cmd /c "set SERVICE_PORT=24103&& set SERVICE_ID=custom-http-order-center-1&& go run ./examples/service-discovery/custom/cmd/http/order-center"
start "custom-http-order-2" cmd /c "set SERVICE_PORT=24113&& set SERVICE_ID=custom-http-order-center-2&& go run ./examples/service-discovery/custom/cmd/http/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:24102/api/auth/token?user_id=1
echo   http://127.0.0.1:24101/api/users/1/profile
echo   http://127.0.0.1:24103/api/orders/create?user_id=1
echo   http://127.0.0.1:24103/api/orders/demo?user_id=1
echo.
echo Press any key to stop all HTTP demo processes...
pause >nul

taskkill /fi "windowtitle eq custom-http-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-http-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.

