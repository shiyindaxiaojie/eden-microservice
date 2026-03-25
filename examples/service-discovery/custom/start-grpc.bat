@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Custom Service Discovery Demo - gRPC
echo ======================================================
echo.

if "%CUSTOM_GRPC_ADDRS%"=="" set CUSTOM_GRPC_ADDRS=127.0.0.1:9000

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Registry gRPC addresses: %CUSTOM_GRPC_ADDRS%
echo.
echo [1/3] Starting auth-center instances...
start "custom-grpc-auth-1" cmd /c "set SERVICE_PORT=24002&& set SERVICE_ID=custom-grpc-auth-center-1&& go run ./examples/service-discovery/custom/cmd/grpc/auth-center"
start "custom-grpc-auth-2" cmd /c "set SERVICE_PORT=24012&& set SERVICE_ID=custom-grpc-auth-center-2&& go run ./examples/service-discovery/custom/cmd/grpc/auth-center"
start "custom-grpc-auth-3" cmd /c "set SERVICE_PORT=24022&& set SERVICE_ID=custom-grpc-auth-center-3&& go run ./examples/service-discovery/custom/cmd/grpc/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "custom-grpc-user-1" cmd /c "set SERVICE_PORT=24001&& set SERVICE_ID=custom-grpc-user-center-1&& go run ./examples/service-discovery/custom/cmd/grpc/user-center"
start "custom-grpc-user-2" cmd /c "set SERVICE_PORT=24011&& set SERVICE_ID=custom-grpc-user-center-2&& go run ./examples/service-discovery/custom/cmd/grpc/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "custom-grpc-order-1" cmd /c "set SERVICE_PORT=24003&& set SERVICE_ID=custom-grpc-order-center-1&& go run ./examples/service-discovery/custom/cmd/grpc/order-center"
start "custom-grpc-order-2" cmd /c "set SERVICE_PORT=24013&& set SERVICE_ID=custom-grpc-order-center-2&& go run ./examples/service-discovery/custom/cmd/grpc/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:24002/api/auth/token?user_id=1
echo   http://127.0.0.1:24001/api/users/1/profile
echo   http://127.0.0.1:24003/api/orders/create?user_id=1
echo   http://127.0.0.1:24003/api/orders/demo?user_id=1
echo.
echo Press any key to stop all gRPC demo processes...
pause >nul

taskkill /fi "windowtitle eq custom-grpc-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq custom-grpc-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.
