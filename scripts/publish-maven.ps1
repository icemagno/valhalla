[CmdletBinding()]
param (
    [string]$Version,
    [string]$Branch = "mvn-repo",
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$sdkDir = Join-Path $projectRoot "sdk\java"
$remoteUrl = (git -C $projectRoot remote get-url origin).Trim()

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "   VALHALLA MAVEN IN-REPO RELEASE PUBLISHER (mvn-repo)    " -ForegroundColor Cyan
Write-Host "   Target Remote: $remoteUrl                              " -ForegroundColor Cyan
Write-Host "   Target Branch: $Branch                                 " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# 1. Determine version
if (-not $Version) {
    [xml]$pomXml = Get-Content (Join-Path $sdkDir "pom.xml")
    $Version = $pomXml.project.version
}
Write-Host "`n[+] Publishing Version: $Version" -ForegroundColor Yellow

# 2. Build C-ABI and package fat JAR if not skipped
if (-not $SkipBuild) {
    Write-Host "`n[1/4] Compiling native C-ABI dynamic library..." -ForegroundColor Green
    & go build -buildmode=c-shared -o "$projectRoot\dist\c\lib\valhalla_lrc.dll" "$projectRoot\pkg\cabi"
    Copy-Item -Path "$projectRoot\dist\c\lib\valhalla_lrc.dll" -Destination "$sdkDir\src\main\resources\libs\valhalla_lrc.dll" -Force
}

# 3. Setup temporary repository staging area
$tempRepo = Join-Path $env:TEMP ("valhalla_mvn_" + [System.Guid]::NewGuid().ToString().Substring(0, 8))
Write-Host "`n[2/4] Setting up temporary release clone in $tempRepo..." -ForegroundColor Green

$branchExists = git ls-remote --heads origin $Branch
if ($branchExists) {
    Write-Host " -> Cloning existing $Branch branch..." -ForegroundColor Gray
    & git clone --branch $Branch --single-branch $remoteUrl $tempRepo
} else {
    Write-Host " -> Initializing new orphan $Branch branch..." -ForegroundColor Gray
    New-Item -ItemType Directory -Force -Path $tempRepo | Out-Null
    & git -C $tempRepo init
    & git -C $tempRepo checkout -b $Branch
    & git -C $tempRepo remote add origin $remoteUrl
}

# 4. Deploy Maven artifacts to temp repo
Write-Host "`n[3/4] Deploying Maven artifacts (JAR, POM, checksums)..." -ForegroundColor Green
$deployUrl = "file:///" + ($tempRepo -replace '\\', '/')
& mvn -f "$sdkDir\pom.xml" clean deploy "-Dmvn.repo.url=$deployUrl" "-Drevision=$Version"

# 5. Commit and Push to GitHub
Write-Host "`n[4/4] Committing and pushing release $Version to origin/$Branch..." -ForegroundColor Green
Push-Location $tempRepo
try {
    & git add -A
    $status = git status --porcelain
    if ($status) {
        & git commit -m "release(maven): publish io.valhalla:valhalla-lrc:$Version"
        & git push origin $Branch
        Write-Host "`n[+] Successfully pushed release $Version to branch '$Branch' on GitHub!" -ForegroundColor Green
    } else {
        Write-Host "`n[*] Artifacts for version $Version are already up to date on branch '$Branch'." -ForegroundColor Yellow
    }
} finally {
    Pop-Location
    Remove-Item -Recurse -Force $tempRepo -ErrorAction SilentlyContinue
}

# 6. Usage summary
Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "   HOW TO USE IN ANY EXTERNAL MAVEN PROJECT (pom.xml)     " -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host @"

Add this to your external project's pom.xml:

<repositories>
    <repository>
        <id>valhalla-mvn-repo</id>
        <url>https://raw.githubusercontent.com/icemagno/valhalla/$Branch/</url>
        <snapshots>
            <enabled>true</enabled>
            <updatePolicy>always</updatePolicy>
        </snapshots>
    </repository>
</repositories>

<dependencies>
    <dependency>
        <groupId>io.valhalla</groupId>
        <artifactId>valhalla-lrc</artifactId>
        <version>$Version</version>
    </dependency>
</dependencies>

Zero authentication or personal access token required!
"@ -ForegroundColor White
