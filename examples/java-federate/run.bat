@echo off
setlocal
cd /d "%~dp0"

set CRC_ADDR=%~1
if "%CRC_ADDR%"=="" set CRC_ADDR=127.0.0.1:52295

set LOCAL_CRC=0
set CRC_EXE=..\..\dist\bin\valhalla-crc.exe

:: Check if CRC is already running on port 52295
netstat -ano | findstr /R ":52295 " >nul 2>&1
if %ERRORLEVEL% neq 0 (
    if exist "%CRC_EXE%" (
        echo [*] Starting local Valhalla CRC in background on :52295...
        start "Valhalla CRC" /B "%CRC_EXE%" -port :52295
        set LOCAL_CRC=1
        ping -n 3 127.0.0.1 >nul
    )
)

echo [*] Executing Valhalla Java Homologation Federate...
call mvn compile exec:java -Dexec.args="%CRC_ADDR%"
set TEST_STATUS=%ERRORLEVEL%

if %LOCAL_CRC% equ 1 (
    echo [*] Stopping local Valhalla CRC...
    taskkill /F /IM valhalla-crc.exe >nul 2>&1
)

if %TEST_STATUS% neq 0 (
    echo [-] Federate failed with status code %TEST_STATUS%.
    exit /b %TEST_STATUS%
)

echo [+] Homologation run completed successfully!
endlocal
