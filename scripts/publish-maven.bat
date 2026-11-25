@echo off
setlocal enabledelayedexpansion

set "VERSION=%~1"
if "%VERSION%"=="" set "VERSION=1.0.0"

set "BRANCH=%~2"
if "%BRANCH%"=="" set "BRANCH=mvn-repo"

set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

echo ==========================================================
echo    VALHALLA MAVEN IN-REPO RELEASE PUBLISHER (mvn-repo)
echo    Version:       %VERSION%
echo    Target Branch: %BRANCH%
echo ==========================================================

for /f "delims=" %%i in ('git remote get-url origin') do set "REMOTE_URL=%%i"
echo    Target Remote: %REMOTE_URL%
echo ==========================================================

set "TEMP_DIR=%TEMP%\valhalla_mvn_%RANDOM%_%RANDOM%"
echo.
echo [1/3] Setting up staging repository in %TEMP_DIR%...

git ls-remote --heads origin %BRANCH% | findstr /i "%BRANCH%" >nul
if %ERRORLEVEL% equ 0 (
    echo  -^> Cloning existing %BRANCH% branch with autocrlf=false...
    git clone -c core.autocrlf=false --branch %BRANCH% --single-branch "%REMOTE_URL%" "%TEMP_DIR%"
) else (
    echo  -^> Initializing new orphan %BRANCH% branch...
    mkdir "%TEMP_DIR%"
    cd /d "%TEMP_DIR%"
    git -c core.autocrlf=false init
    git config core.autocrlf false
    git checkout -b %BRANCH%
    git remote add origin "%REMOTE_URL%"
    cd /d "%PROJECT_ROOT%"
)

copy /Y "%PROJECT_ROOT%\.gitattributes" "%TEMP_DIR%\.gitattributes" >nul

echo.
echo [2/3] Deploying Maven artifacts...
set "DEPLOY_DIR=%TEMP_DIR:\=/%"
cd sdk\java
call mvn clean deploy "-Dmvn.repo.url=file:///%DEPLOY_DIR%" "-Drevision=%VERSION%"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Maven deploy failed.
    rmdir /s /q "%TEMP_DIR%" 2>nul
    cd /d "%PROJECT_ROOT%"
    exit /b %ERRORLEVEL%
)

echo.
echo [3/3] Committing and pushing release %VERSION% to origin/%BRANCH%...
cd /d "%TEMP_DIR%"
git add -A
git status --porcelain | findstr /r "." >nul
if %ERRORLEVEL% equ 0 (
    git commit -m "release(maven): publish io.valhalla:valhalla-lrc:%VERSION%"
    git push origin %BRANCH%
    echo [+] Successfully pushed release %VERSION% to branch '%BRANCH%' on GitHub!
) else (
    echo [*] Artifacts for version %VERSION% are already up to date on branch '%BRANCH%'.
)

cd /d "%PROJECT_ROOT%"
rmdir /s /q "%TEMP_DIR%" 2>nul

echo.
echo ==========================================================
echo    HOW TO USE IN ANY EXTERNAL MAVEN PROJECT (pom.xml)
echo ==========================================================
echo.
echo Add this to your project's pom.xml:
echo.
echo ^<repositories^>
echo     ^<repository^>
echo         ^<id^>valhalla-mvn-repo^</id^>
echo         ^<url^>https://raw.githubusercontent.com/icemagno/valhalla/%BRANCH%/^</url^>
echo         ^<snapshots^>
echo             ^<enabled^>true^</enabled^>
echo             ^<updatePolicy^>always^</updatePolicy^>
echo         ^</snapshots^>
echo     ^</repository^>
echo ^</repositories^>
echo.
echo ^<dependencies^>
echo     ^<dependency^>
echo         ^<groupId^>io.valhalla^</groupId^>
echo         ^<artifactId^>valhalla-lrc^</artifactId^>
echo         ^<version^>%VERSION%^</version^>
echo     ^</dependency^>
echo ^</dependencies^>
echo.
echo Zero authentication or personal access token required!
