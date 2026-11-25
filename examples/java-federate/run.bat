@echo off
setlocal
cd /d "%~dp0"

set CRC_ADDR=%~1
if "%CRC_ADDR%"=="" set CRC_ADDR=127.0.0.1:52295

echo [*] Executing Valhalla Java Homologation Federate against CRC at %CRC_ADDR%...
call mvn compile exec:java -Dexec.args="%CRC_ADDR%"
set TEST_STATUS=%ERRORLEVEL%

if %TEST_STATUS% neq 0 (
    echo [-] Federate failed with status code %TEST_STATUS%.
    exit /b %TEST_STATUS%
)

echo [+] Homologation run completed successfully!
endlocal
