@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo    VALHALLA SDK COMPILATION AND PACKAGING (WINDOWS)
echo ========================================================

set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

echo.
echo [1/3] Compiling native C-ABI dynamic library (valhalla_lrc.dll)...
if not exist "dist\bin" mkdir "dist\bin"
if not exist "dist\c\lib" mkdir "dist\c\lib"
if not exist "dist\c\include" mkdir "dist\c\include"
if not exist "sdk\java\src\main\resources\libs" mkdir "sdk\java\src\main\resources\libs"

go build -buildmode=c-shared -o "dist\bin\valhalla_lrc.dll" ./pkg/cabi
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Failed to compile valhalla_lrc.dll
    exit /b %ERRORLEVEL%
)

echo.
echo [2/3] Distributing compiled libraries and C headers...
copy /Y "dist\bin\valhalla_lrc.dll" "dist\c\lib\valhalla_lrc.dll" >nul
copy /Y "dist\bin\valhalla_lrc.h" "dist\c\include\valhalla_lrc.h" >nul
copy /Y "dist\bin\valhalla_lrc.dll" "sdk\java\src\main\resources\libs\valhalla_lrc.dll" >nul

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
echo [SUCCESS] Valhalla SDK built successfully!
echo Binary outputs:
echo   - DLL:  dist\bin\valhalla_lrc.dll
echo   - C:    dist\c\lib\valhalla_lrc.dll
echo   - Java: dist\java\valhalla-lrc.jar
echo ========================================================
