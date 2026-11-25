#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-1.0.0}"
BRANCH="${2:-mvn-repo}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SDK_DIR="$PROJECT_ROOT/sdk/java"
REMOTE_URL="$(git -C "$PROJECT_ROOT" remote get-url origin)"

echo "=========================================================="
echo "   VALHALLA MAVEN IN-REPO RELEASE PUBLISHER (mvn-repo)    "
echo "   Target Remote: $REMOTE_URL                             "
echo "   Target Branch: $BRANCH                                 "
echo "   Version:       $VERSION                                "
echo "=========================================================="

echo "[1/4] Compiling native C-ABI shared library..."
go build -buildmode=c-shared -o "$PROJECT_ROOT/dist/c/lib/libvalhalla_lrc.so" "$PROJECT_ROOT/pkg/cabi"
mkdir -p "$SDK_DIR/src/main/resources/libs"
cp "$PROJECT_ROOT/dist/c/lib/libvalhalla_lrc.so" "$SDK_DIR/src/main/resources/libs/"

TEMP_REPO=$(mktemp -d -t valhalla_mvn_XXXXXX)
echo "[2/4] Setting up staging repository in $TEMP_REPO..."

if git ls-remote --heads origin "$BRANCH" | grep -q "$BRANCH"; then
    echo " -> Cloning existing $BRANCH branch..."
    git clone --branch "$BRANCH" --single-branch "$REMOTE_URL" "$TEMP_REPO"
else
    echo " -> Initializing new orphan $BRANCH branch..."
    git -C "$TEMP_REPO" init
    git -C "$TEMP_REPO" checkout -b "$BRANCH"
    git -C "$TEMP_REPO" remote add origin "$REMOTE_URL"
fi

echo "[3/4] Deploying Maven artifacts..."
DEPLOY_URL="file://$TEMP_REPO"
mvn -f "$SDK_DIR/pom.xml" clean deploy "-Dmvn.repo.url=$DEPLOY_URL" "-Drevision=$VERSION"

echo "[4/4] Pushing release $VERSION to origin/$BRANCH..."
cd "$TEMP_REPO"
git add -A
if [ -n "$(git status --porcelain)" ]; then
    git commit -m "release(maven): publish io.valhalla:valhalla-lrc:$VERSION"
    git push origin "$BRANCH"
    echo "[+] Successfully pushed release $VERSION to branch '$BRANCH' on GitHub!"
else
    echo "[*] Artifacts for version $VERSION are already up to date on branch '$BRANCH'."
fi

rm -rf "$TEMP_REPO"

echo "=========================================================="
echo "   HOW TO USE IN ANY EXTERNAL MAVEN PROJECT (pom.xml)     "
echo "=========================================================="
cat <<EOF

Add this to your external project's pom.xml:

<repositories>
    <repository>
        <id>valhalla-mvn-repo</id>
        <url>https://raw.githubusercontent.com/icemagno/valhalla/$BRANCH/</url>
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
        <version>$VERSION</version>
    </dependency>
</dependencies>

Zero authentication or personal access token required!
EOF
