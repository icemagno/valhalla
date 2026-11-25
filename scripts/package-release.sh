#!/bin/bash
set -e

VERSION="${1:-1.0.0}"
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
PROJECT_ROOT="$DIR/.."
cd "$PROJECT_ROOT"

echo "===================================================================="
echo "   PROJECT VALHALLA - OFFICIAL RELEASE PACKAGER"
echo "   Target Version: v$VERSION"
echo "===================================================================="

mkdir -p "releases"
ZIP_NAME="valhalla-v${VERSION}.zip"
ZIP_PATH="releases/${ZIP_NAME}"

if [ -f "$ZIP_PATH" ]; then
    echo "[*] Removing existing release archive: $ZIP_PATH"
    rm -f "$ZIP_PATH"
fi

echo ""
echo "[1/4] Cleaning build artifacts in examples..."
if [ -d "examples/java-federate/target" ]; then
    echo " -> Cleaning examples/java-federate/target..."
    cd examples/java-federate
    mvn clean >/dev/null 2>&1 || true
    cd "$PROJECT_ROOT"
fi

echo ""
echo "[2/4] Verifying required distribution artifacts..."
MISSING=0
[ ! -f "dist/bin/valhalla-crc.exe" ] && echo " [-] Missing dist/bin/valhalla-crc.exe" && MISSING=1
[ ! -f "dist/bin/valhalla_lrc.dll" ] && echo " [-] Missing dist/bin/valhalla_lrc.dll" && MISSING=1
[ ! -f "dist/bin/libvalhalla_lrc.so" ] && echo " [-] Missing dist/bin/libvalhalla_lrc.so" && MISSING=1
[ ! -f "dist/lib/librti1516e.a" ] && echo " [-] Missing dist/lib/librti1516e.a" && MISSING=1
[ ! -f "dist/java/valhalla-lrc.jar" ] && echo " [-] Missing dist/java/valhalla-lrc.jar" && MISSING=1
[ ! -f "dist/web/valhalla-hla.js" ] && echo " [-] Missing dist/web/valhalla-hla.js" && MISSING=1

if [ "$MISSING" -eq 1 ]; then
    echo "[-] Error: Core distribution artifacts are missing. Run ./scripts/build-sdk.sh first."
    exit 1
fi
echo " [+] All core distribution artifacts verified!"

echo ""
echo "[3/4] Creating self-contained release archive: $ZIP_PATH..."
zip -r "$ZIP_PATH" dist examples LICENSE README.md -x "*.exe~" "*target/*" "*.class"

echo ""
echo "[4/4] Computing SHA-256 Checksum..."
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$ZIP_PATH" > "releases/${ZIP_NAME}.sha256"
    cat "releases/${ZIP_NAME}.sha256"
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$ZIP_PATH" > "releases/${ZIP_NAME}.sha256"
    cat "releases/${ZIP_NAME}.sha256"
fi

echo ""
echo "===================================================================="
echo "[+] Release bundle successfully generated:"
echo "    Archive:  releases/$ZIP_NAME"
echo "    Checksum: releases/$ZIP_NAME.sha256"
echo "===================================================================="
