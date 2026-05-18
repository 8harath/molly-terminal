#!/bin/bash
set -euo pipefail

DIST="${1:-dist}"
RPM_DIR="$DIST"
REPO_DIR="/tmp/rpm-repo-publish"

RPM_FILES=$(ls "$RPM_DIR"/*.rpm 2>/dev/null || echo "")
if [ -z "$RPM_FILES" ]; then
    exit 0
fi

echo "=== Publishing RPMs to rpm-repo ==="

rm -rf "$REPO_DIR"
if ! git clone --depth 1 "https://${GITHUB_TOKEN}@github.com/ploglabs/rpm-repo.git" "$REPO_DIR" 2>&1; then
    echo "ERROR: failed to clone rpm-repo"
    exit 1
fi

cd "$REPO_DIR"

for RPM_FILE in $RPM_FILES; do
    FILENAME=$(basename "$RPM_FILE")
    echo "  -> $FILENAME"

    ARCH_DIR="el/9/x86_64"
    if echo "$FILENAME" | grep -q "arm64\|aarch64"; then
        ARCH_DIR="el/9/aarch64"
    fi

    mkdir -p "$ARCH_DIR"
    cp "$RPM_FILE" "$ARCH_DIR/"
    git add "$ARCH_DIR/$FILENAME" 2>/dev/null || true
done

git config user.email "hello@ploglabs.dev"
git config user.name "ploglabs"
if git diff --cached --quiet; then
    echo "(no new RPMs)"
    exit 0
fi

git commit -m "publish $(date -u +%Y-%m-%d-%H%M)" || true
if ! git push origin main 2>&1; then
    echo "ERROR: failed to push to rpm-repo"
    exit 1
fi
echo "=== RPMs published ==="
