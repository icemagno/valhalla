@echo off
setlocal enabledelayedexpansion

echo ====================================================================
echo   Project Valhalla (HLA-4 RTI) - Full Verification ^& Test Suite
echo ====================================================================

pushd "%~dp0.."

echo [1/5] Running go vet ./...
go vet ./...
if errorlevel 1 goto :fail

echo [2/5] Running go vet -tags cabi_test ./pkg/cabi/...
go vet -tags cabi_test ./pkg/cabi/...
if errorlevel 1 goto :fail

echo [3/5] Running go build ./...
go build ./...
if errorlevel 1 goto :fail

echo [4/5] Running go test -race -count=1 -cover ./...
go test -race -count=1 -cover ./...
if errorlevel 1 goto :fail

echo [5/5] Running go test -race -tags cabi_test -count=1 -cover ./pkg/cabi/...
go test -race -tags cabi_test -count=1 -cover ./pkg/cabi/...
if errorlevel 1 goto :fail

echo.
echo ====================================================================
echo   ALL VERIFICATION GATES PASSED (0 vet warnings, 0 data races)
echo ====================================================================
popd
exit /b 0

:fail
echo.
echo [FAIL] Verification gate failed with exit code %errorlevel%.
popd
exit /b %errorlevel%
