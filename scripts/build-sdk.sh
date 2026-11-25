#!/usr/bin/env bash
set -euo pipefail

echo "========================================================"
echo "    VALHALLA SDK COMPILATION AND PACKAGING (LINUX)      "
echo "========================================================"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo
echo "[1/3] Compiling native C-ABI shared library (libvalhalla_lrc.so)..."
mkdir -p dist/bin dist/c/lib dist/c/include sdk/java/src/main/resources/libs

go build -buildmode=c-shared -o "dist/bin/libvalhalla_lrc.so" ./pkg/cabi

echo
echo "[2/3] Distributing compiled libraries and C headers..."
cp -f "dist/bin/libvalhalla_lrc.so" "dist/c/lib/libvalhalla_lrc.so"
cp -f "dist/bin/libvalhalla_lrc.h" "dist/c/include/valhalla_lrc.h"
cp -f "dist/bin/libvalhalla_lrc.so" "sdk/java/src/main/resources/libs/libvalhalla_lrc.so"

echo
echo "[3/3] Compiling and packaging Java Client SDK (valhalla-lrc.jar)..."
cd sdk/java
mvn clean package
cd "$PROJECT_ROOT"

echo
echo "========================================================"
echo "[SUCCESS] Valhalla SDK built successfully!"
echo "Binary outputs:"
echo "  - SO:   dist/bin/libvalhalla_lrc.so"
echo "  - C:    dist/c/lib/libvalhalla_lrc.so"
echo "  - Java: dist/java/valhalla-lrc.jar"
echo "========================================================"
