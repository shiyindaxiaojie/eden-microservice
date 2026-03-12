@echo off
echo Running Service Discovery Example...

set WORKDIR=%~dp0..\..

cd %WORKDIR%\examples\service-discovery
go run main.go

echo.
pause
