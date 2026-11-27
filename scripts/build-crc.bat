@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo    VALHALLA CRC SERVER COMPILATION (WINDOWS + LINUX)
echo ========================================================

set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

if not exist "dist\bin" mkdir "dist\bin"

set "OLD_GOOS=%GOOS%"
set "OLD_GOARCH=%GOARCH%"
set "OLD_CGO=%CGO_ENABLED%"

echo.
echo [1/2] Compiling Windows CRC binary (dist\bin\valhalla-crc.exe)...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o "dist\bin\valhalla-crc.exe" ./cmd/valhalla-crc
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to compile valhalla-crc.exe
    goto :restore_fail
)

echo.
echo [2/2] Cross-compiling Linux CRC binary (dist\bin\valhalla-crc)...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o "dist\bin\valhalla-crc" ./cmd/valhalla-crc
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to cross-compile valhalla-crc for Linux
    goto :restore_fail
)

set "GOOS=%OLD_GOOS%"
set "GOARCH=%OLD_GOARCH%"
set "CGO_ENABLED=%OLD_CGO%"

echo.
echo ========================================================
echo [SUCCESS] Valhalla CRC binaries built successfully!
echo Binary outputs:
echo   - Windows CRC: dist\bin\valhalla-crc.exe
echo   - Linux CRC:   dist\bin\valhalla-crc
echo ========================================================
endlocal
exit /b 0

:restore_fail
set "GOOS=%OLD_GOOS%"
set "GOARCH=%OLD_GOARCH%"
set "CGO_ENABLED=%OLD_CGO%"
endlocal
exit /b 1
