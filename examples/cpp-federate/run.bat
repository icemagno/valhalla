@echo off
setlocal
cd /d "%~dp0"

if not exist cpp_federate.exe (
    echo [*] Building cpp_federate.exe...
    g++ -std=c++14 -Wno-deprecated src/main.cpp -Isrc -I../../sdk/cpp/include -I../../sdk/c/include ../../sdk/cpp/librti1516e.a ../../dist/bin/valhalla_lrc.dll -o cpp_federate.exe
    if errorlevel 1 (
        echo [-] Compilation failed!
        exit /b 1
    )
)

if not exist valhalla_lrc.dll (
    copy /y ..\..\dist\bin\valhalla_lrc.dll . >nul
)

echo [*] Launching C++ Homologation Federate...
cpp_federate.exe %*
