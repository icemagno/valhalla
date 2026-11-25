#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

CRC_ADDR="${1:-127.0.0.1:52295}"
LOCAL_CRC=0
CRC_BIN="../../dist/bin/valhalla-crc"

# Check if CRC is listening
if ! nc -z 127.0.0.1 52295 2>/dev/null; then
    if [ -f "${CRC_BIN}" ]; then
        echo "[*] Starting local Valhalla CRC in background on :52295..."
        "${CRC_BIN}" -port :52295 &
        CRC_PID=$!
        LOCAL_CRC=1
        sleep 2
    fi
fi

cleanup() {
    if [ ${LOCAL_CRC} -eq 1 ] && [ -n "${CRC_PID}" ]; then
        echo "[*] Stopping local Valhalla CRC..."
        kill "${CRC_PID}" 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo "[*] Executing Valhalla Java Homologation Federate..."
mvn compile exec:java -Dexec.args="${CRC_ADDR}"

echo "[+] Homologation run completed successfully!"
