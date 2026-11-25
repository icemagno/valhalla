@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo    VALHALLA SDK COMPILATION AND PACKAGING (WINDOWS)
echo ========================================================

set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

if not exist "dist\bin" mkdir "dist\bin"
if not exist "dist\java" mkdir "dist\java"
if not exist "sdk\c\lib" mkdir "sdk\c\lib"
if not exist "sdk\c\include" mkdir "sdk\c\include"
if not exist "sdk\java\src\main\resources\libs" mkdir "sdk\java\src\main\resources\libs"

echo.
echo [1/3] Compiling native C-ABI LRC libraries (DLL and SO into dist\bin)...

:: 1. Compile Windows DLL
echo  -^> Compiling Windows DLL...
go build -buildmode=c-shared -o "dist\bin\valhalla_lrc.dll" ./pkg/cabi
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to compile valhalla_lrc.dll
    exit /b %ERRORLEVEL%
)

:: 2. Compile Linux SO (using zig cc cross-compiler if available)
where zig >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo  -^> Compiling Linux SO via zig cc...
    set "OLD_GOOS=%GOOS%"
    set "OLD_GOARCH=%GOARCH%"
    set "OLD_CGO=%CGO_ENABLED%"
    set "OLD_CC=%CC%"

    set GOOS=linux
    set GOARCH=amd64
    set CGO_ENABLED=1
    set CC=zig cc -target x86_64-linux-gnu
    go build -buildmode=c-shared -o "dist\bin\libvalhalla_lrc.so" ./pkg/cabi

    set "GOOS=!OLD_GOOS!"
    set "GOARCH=!OLD_GOARCH!"
    set "CGO_ENABLED=!OLD_CGO!"
    set "CC=!OLD_CC!"
) else (
    echo  -^> [NOTICE] zig not found; using existing dist\bin\libvalhalla_lrc.so
)

echo.
echo [2/3] Distributing compiled libraries to SDK folders...
copy /Y "pkg\cabi\valhalla_lrc.h" "sdk\c\include\valhalla_lrc.h" >nul
copy /Y "dist\bin\valhalla_lrc.dll" "sdk\c\lib\valhalla_lrc.dll" >nul
if exist "dist\bin\libvalhalla_lrc.so" copy /Y "dist\bin\libvalhalla_lrc.so" "sdk\c\lib\libvalhalla_lrc.so" >nul

echo  -^> Injecting DLL and SO into Java SDK resources...
copy /Y "dist\bin\valhalla_lrc.dll" "sdk\java\src\main\resources\libs\valhalla_lrc.dll" >nul
if exist "dist\bin\libvalhalla_lrc.so" copy /Y "dist\bin\libvalhalla_lrc.so" "sdk\java\src\main\resources\libs\libvalhalla_lrc.so" >nul

echo.
echo [3/3] Compiling and packaging Java Client SDK (valhalla-lrc.jar)...
cd sdk\java
call mvn clean package
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to package Java SDK
    cd /d "%PROJECT_ROOT%"
    exit /b %ERRORLEVEL%
)

cd /d "%PROJECT_ROOT%"
echo.
echo ========================================================
echo [SUCCESS] Valhalla SDKs built successfully!
echo Binary outputs:
echo   - LRC Windows DLL: dist\bin\valhalla_lrc.dll
echo   - LRC Linux SO:    dist\bin\libvalhalla_lrc.so
echo   - Java SDK JAR:    dist\java\valhalla-lrc.jar (with embedded DLL + SO)
echo   - C SDK Headers:   sdk\c\include\valhalla_lrc.h
echo ========================================================
