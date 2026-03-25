@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  pkg/eden Service Discovery Demo - gRPC
echo ======================================================
echo.

if "%EDEN_GRPC_ADDRS%"=="" set EDEN_GRPC_ADDRS=127.0.0.1:9000

set WORKDIR=%~dp0..\..\..
cd /d %WORKDIR%

echo Registry gRPC addresses: %EDEN_GRPC_ADDRS%
echo.
echo [1/3] Starting auth-center instances...
start "eden-grpc-auth-1" cmd /c "set SERVICE_PORT=21002&& set SERVICE_ID=eden-grpc-auth-center-1&& go run ./examples/service-discovery/eden/cmd/grpc/auth-center"
start "eden-grpc-auth-2" cmd /c "set SERVICE_PORT=21012&& set SERVICE_ID=eden-grpc-auth-center-2&& go run ./examples/service-discovery/eden/cmd/grpc/auth-center"
start "eden-grpc-auth-3" cmd /c "set SERVICE_PORT=21022&& set SERVICE_ID=eden-grpc-auth-center-3&& go run ./examples/service-discovery/eden/cmd/grpc/auth-center"
timeout /t 2 /nobreak >nul

echo [2/3] Starting user-center instances...
start "eden-grpc-user-1" cmd /c "set SERVICE_PORT=21001&& set SERVICE_ID=eden-grpc-user-center-1&& go run ./examples/service-discovery/eden/cmd/grpc/user-center"
start "eden-grpc-user-2" cmd /c "set SERVICE_PORT=21011&& set SERVICE_ID=eden-grpc-user-center-2&& go run ./examples/service-discovery/eden/cmd/grpc/user-center"
timeout /t 2 /nobreak >nul

echo [3/3] Starting order-center instances...
start "eden-grpc-order-1" cmd /c "set SERVICE_PORT=21003&& set SERVICE_ID=eden-grpc-order-center-1&& go run ./examples/service-discovery/eden/cmd/grpc/order-center"
start "eden-grpc-order-2" cmd /c "set SERVICE_PORT=21013&& set SERVICE_ID=eden-grpc-order-center-2&& go run ./examples/service-discovery/eden/cmd/grpc/order-center"
timeout /t 2 /nobreak >nul

echo.
echo Test URLs:
echo   http://127.0.0.1:21002/api/auth/token?user_id=1
echo   http://127.0.0.1:21001/api/users/1/profile
echo   http://127.0.0.1:21003/api/orders/create?user_id=1
echo   http://127.0.0.1:21003/api/orders/demo?user_id=1
echo.
echo Press any key to stop all gRPC demo processes...
pause >nul

taskkill /fi "windowtitle eq eden-grpc-auth-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-auth-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-auth-3" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-user-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-user-2" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-order-1" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq eden-grpc-order-2" /im cmd.exe /t /f >nul 2>&1

echo Done.
