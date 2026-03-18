@echo off
setlocal enabledelayedexpansion

echo ======================================================
echo  Eden Registry - Service Discovery Restarting
echo ======================================================
echo.

set SCRIPT_DIR=%~dp0
set WORKDIR=%~dp0..\..
cd /d %WORKDIR%

:: 0. Stop all processes
echo [1/2] Stopping all services...
taskkill /fi "windowtitle eq User-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Auth-Center" /im cmd.exe /t /f >nul 2>&1
taskkill /fi "windowtitle eq Order-Center" /im cmd.exe /t /f >nul 2>&1
timeout /t 2 /nobreak >nul

:: 1. Start back up
echo [2/2] Restarting service discovery demo...
call "%SCRIPT_DIR%start.bat"

echo.
echo Restart complete.
