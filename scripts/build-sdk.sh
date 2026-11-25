#!/usr/bin/env bash
set -euo pipefail

echo "========================================================"
echo "    VALHALLA SDK COMPILATION AND PACKAGING (LINUX)      "
echo "========================================================"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

mkdir -p dist/bin dist/java sdk/c/lib sdk/c/include sdk/java/src/main/resources/libs

echo
echo "[1/3] Compiling native C-ABI LRC libraries (SO and DLL into dist/bin)..."

echo " -> Compiling Linux SO (dist/bin/libvalhalla_lrc.so)..."
go build -buildmode=c-shared -o "dist/bin/libvalhalla_lrc.so" ./pkg/cabi

if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
    echo " -> Cross-compiling Windows DLL via MinGW (dist/bin/valhalla_lrc.dll)..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -buildmode=c-shared -o "dist/bin/valhalla_lrc.dll" ./pkg/cabi
elif command -v zig >/dev/null 2>&1; then
    echo " -> Cross-compiling Windows DLL via zig cc (dist/bin/valhalla_lrc.dll)..."
    GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="zig cc -target x86_64-windows-gnu" go build -buildmode=c-shared -o "dist/bin/valhalla_lrc.dll" ./pkg/cabi
else
    echo " -> [NOTICE] MinGW/zig not found; using existing dist/bin/valhalla_lrc.dll"
fi

echo
echo "[2/3] Distributing compiled libraries to SDK folders..."
cp -f "pkg/cabi/valhalla_lrc.h" "sdk/c/include/valhalla_lrc.h"
cp -f "dist/bin/libvalhalla_lrc.so" "sdk/c/lib/libvalhalla_lrc.so"
[ -f "dist/bin/valhalla_lrc.dll" ] && cp -f "dist/bin/valhalla_lrc.dll" "sdk/c/lib/valhalla_lrc.dll"

echo " -> Injecting SO and DLL into Java SDK resources..."
cp -f "dist/bin/libvalhalla_lrc.so" "sdk/java/src/main/resources/libs/libvalhalla_lrc.so"
[ -f "dist/bin/valhalla_lrc.dll" ] && cp -f "dist/bin/valhalla_lrc.dll" "sdk/java/src/main/resources/libs/valhalla_lrc.dll"

echo
echo "[3/3] Compiling and packaging Java Client SDK (valhalla-lrc.jar)..."
cd sdk/java
mvn clean package
cd "$PROJECT_ROOT"

echo
echo "========================================================"
echo "[SUCCESS] Valhalla SDKs built successfully!"
echo "Binary outputs:"
echo "  - LRC Linux SO:    dist/bin/libvalhalla_lrc.so"
echo "  - LRC Windows DLL: dist/bin/valhalla_lrc.dll"
echo "  - Java SDK JAR:    dist/java/valhalla-lrc.jar (with embedded SO + DLL)"
echo "  - C SDK Headers:   sdk/c/include/valhalla_lrc.h"
echo "========================================================"
