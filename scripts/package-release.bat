@echo off
setlocal enabledelayedexpansion

set "VERSION=%~1"
if "%VERSION%"=="" set "VERSION=1.0.0"

set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

echo ====================================================================
echo    PROJECT VALHALLA - OFFICIAL RELEASE PACKAGER
echo    Target Version: v%VERSION%
echo ====================================================================

if not exist "releases" mkdir "releases"

set "ZIP_NAME=valhalla-v%VERSION%.zip"
set "ZIP_PATH=releases\%ZIP_NAME%"

if exist "%ZIP_PATH%" (
    echo [*] Removing existing release archive: %ZIP_PATH%
    del /f /q "%ZIP_PATH%"
)

echo.
echo [1/4] Cleaning build artifacts in examples...
if exist "examples\java-federate\target" (
    echo  -^> Cleaning examples\java-federate\target...
    cd examples\java-federate
    call mvn clean >nul 2>&1
    cd /d "%PROJECT_ROOT%"
)

echo.
echo [2/4] Verifying required distribution artifacts...
set "MISSING=0"
if not exist "dist\bin\valhalla-crc.exe" set "MISSING=1" & echo  [-] Missing dist\bin\valhalla-crc.exe
if not exist "dist\bin\valhalla_lrc.dll" set "MISSING=1" & echo  [-] Missing dist\bin\valhalla_lrc.dll
if not exist "dist\bin\libvalhalla_lrc.so" set "MISSING=1" & echo  [-] Missing dist\bin\libvalhalla_lrc.so
if not exist "dist\lib\librti1516e.a" set "MISSING=1" & echo  [-] Missing dist\lib\librti1516e.a
if not exist "dist\java\valhalla-lrc.jar" set "MISSING=1" & echo  [-] Missing dist\java\valhalla-lrc.jar
if not exist "dist\web\valhalla-hla.js" set "MISSING=1" & echo  [-] Missing dist\web\valhalla-hla.js

if "%MISSING%"=="1" (
    echo.
    echo [-] Error: Core distribution artifacts are missing. Run scripts\build-sdk.bat first.
    exit /b 1
)
echo  [+] All core distribution artifacts verified!

echo.
echo [3/4] Creating self-contained release archive: %ZIP_PATH%...
powershell -NoProfile -Command ^
    "Compress-Archive -Path 'dist', 'examples', 'LICENSE', 'README.md' -DestinationPath '%ZIP_PATH%' -Force"

if not exist "%ZIP_PATH%" (
    echo [-] Failed to create release archive!
    exit /b 1
)

echo.
echo [4/4] Computing SHA-256 Checksum...
powershell -NoProfile -Command ^
    "$hash = (Get-FileHash -Path '%ZIP_PATH%' -Algorithm SHA256).Hash; Write-Host (' [+] SHA256: ' + $hash); [IO.File]::WriteAllText('releases\%ZIP_NAME%.sha256', $hash + '  %ZIP_NAME%`n')"

echo.
echo ====================================================================
echo [+] Release bundle successfully generated:
echo     Archive:  releases\%ZIP_NAME%
echo     Checksum: releases\%ZIP_NAME%.sha256
echo ====================================================================
endlocal
