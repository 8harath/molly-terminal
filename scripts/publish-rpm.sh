#!/bin/bash
set -euo pipefail

REPO_DIR="/tmp/rpm-repo-publish"
REPO_URL="https://${GITHUB_TOKEN}@github.com/ploglabs/rpm-repo.git"

echo "Publishing RPMs to rpm-repo..."

rm -rf "$REPO_DIR"
git clone --depth 1 "$REPO_URL" "$REPO_DIR" 2>/dev/null
cd "$REPO_DIR"

RPM_FILES=$(ls "${1:-.}"/*.rpm 2>/dev/null || echo "")
if [ -z "$RPM_FILES" ]; then
    echo "No RPM files found, skipping."
    exit 0
fi

for RPM_FILE in $RPM_FILES; do
    FILENAME=$(basename "$RPM_FILE")
    echo "  -> $FILENAME"

    ARCH_DIR="el/9/x86_64"
    if echo "$FILENAME" | grep -q "arm64\|aarch64"; then
        ARCH_DIR="el/9/aarch64"
    fi

    mkdir -p "$ARCH_DIR"
    cp "$RPM_FILE" "$ARCH_DIR/"
    git add "$ARCH_DIR/$FILENAME"
done

git config user.email "hello@ploglabs.dev"
git config user.name "ploglabs"
git commit -m "publish $(date -u +%Y-%m-%d)" || echo "(no new RPMs)"
git push origin main
echo "Published RPMs"
