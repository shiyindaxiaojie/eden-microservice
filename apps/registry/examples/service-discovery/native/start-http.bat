@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  pkg/sdk Service Discovery Demo - HTTP
echo ======================================================
echo.

if "%EDEN_HTTP_ADDRS%"=="" set EDEN_HTTP_ADDRS=http://127.0.0.1:8500

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Focalors HTTP addresses: %EDEN_HTTP_ADDRS%
echo.
echo [1/3] Starting auth-center instances...
start "native-http-auth-1" cmd /c "set SERVICE_PORT=21102&& set SERVICE_ID=native-http-auth-center-1&& go run ./examples/service-discovery/native/cmd/http/auth-center"
start "native-http-auth-2" cmd /c "set SERVICE_PORT=21112&& set SERVICE_ID=native-http-auth-center-2&& go run ./examples/service-discovery/native/cmd/http/auth-center"
start "native-http-auth-3" cmd /c "set SERVICE_PORT=21122&& set SERVICE_ID=native-http-auth-center-3&& go run ./examples/service-discovery/native/cmd/http/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "native-http-user-1" cmd /c "set SERVICE_PORT=21101&& set SERVICE_ID=native-http-user-center-1&& go run ./examples/service-discovery/native/cmd/http/user-center"
start "native-http-user-2" cmd /c "set SERVICE_PORT=21111&& set SERVICE_ID=native-http-user-center-2&& go run ./examples/service-discovery/native/cmd/http/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "native-http-order-1" cmd /c "set SERVICE_PORT=21103&& set SERVICE_ID=native-http-order-center-1&& go run ./examples/service-discovery/native/cmd/http/order-center"
start "native-http-order-2" cmd /c "set SERVICE_PORT=21113&& set SERVICE_ID=native-http-order-center-2&& go run ./examples/service-discovery/native/cmd/http/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:21102/api/auth/token?user_id=1
echo   http://127.0.0.1:21101/api/users/1/profile
echo   http://127.0.0.1:21103/api/orders/create?user_id=1
echo   http://127.0.0.1:21103/api/orders/demo?user_id=1
echo.
echo Press any key to stop all HTTP demo processes...
pause >nul

taskkill /fi "windowtitle eq native-http-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq native-http-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.


