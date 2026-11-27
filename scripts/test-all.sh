#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/.."

echo "===================================================================="
echo "  Project Valhalla (HLA-4 RTI) - Full Verification & Test Suite"
echo "===================================================================="

echo "[1/5] Running go vet ./..."
go vet ./...

echo "[2/5] Running go vet -tags cabi_test ./pkg/cabi/..."
go vet -tags cabi_test ./pkg/cabi/...

echo "[3/5] Running go build ./..."
go build ./...

echo "[4/5] Running go test -race -count=1 -cover ./..."
go test -race -count=1 -cover ./...

echo "[5/5] Running go test -race -tags cabi_test -count=1 -cover ./pkg/cabi/..."
go test -race -tags cabi_test -count=1 -cover ./pkg/cabi/...

echo ""
echo "===================================================================="
echo "  ALL VERIFICATION GATES PASSED (0 vet warnings, 0 data races)"
echo "===================================================================="
