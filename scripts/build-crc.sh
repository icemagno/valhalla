#!/usr/bin/env bash
set -euo pipefail

echo "========================================================"
echo "    VALHALLA CRC SERVER COMPILATION (LINUX + WINDOWS)   "
echo "========================================================"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

mkdir -p dist/bin

echo
echo "[1/2] Compiling Linux CRC binary (dist/bin/valhalla-crc)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/bin/valhalla-crc" ./cmd/valhalla-crc

echo
echo "[2/2] Cross-compiling Windows CRC binary (dist/bin/valhalla-crc.exe)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "dist/bin/valhalla-crc.exe" ./cmd/valhalla-crc

echo
echo "========================================================"
echo "[SUCCESS] Valhalla CRC binaries built successfully!"
echo "Binary outputs:"
echo "  - Linux CRC:   dist/bin/valhalla-crc"
echo "  - Windows CRC: dist/bin/valhalla-crc.exe"
echo "========================================================"
