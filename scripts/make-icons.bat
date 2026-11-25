@echo off
set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

echo ========================================================
echo    GERADOR DE RECURSOS E ICONES WINDOWS (.syso)
echo ========================================================
echo.

where go-winres >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] go-winres.exe nao encontrado no PATH.
    echo Certifique-se de que C:\Users\%USERNAME%\go\bin esta no PATH.
    exit /b 1
)

echo [*] Gerando arquivos .syso para cmd\valhalla-crc...
go-winres make --in assets\winres.json --out cmd\valhalla-crc\rsrc
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Falha ao gerar recursos com go-winres.
    exit /b %ERRORLEVEL%
)

echo.
echo [DONE] Arquivos rsrc_windows_*.syso gerados com sucesso em cmd\valhalla-crc!
echo Ao rodar 'go build ./cmd/valhalla-crc', o executavel recebera o icone e metadados automaticamente.
