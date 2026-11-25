#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

CRC_ADDR="${1:-127.0.0.1:52295}"

echo "[*] Executing Valhalla Java Homologation Federate against CRC at ${CRC_ADDR}..."
mvn compile exec:java -Dexec.args="${CRC_ADDR}"

echo "[+] Homologation run completed successfully!"
