#!/usr/bin/env bash
# Cross-compile release binaries and copy install bundle into dist/
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
SCRIPT_DIR="$(dirname "$0")"

rm -rf dist
mkdir -p dist/skill/code-graph

targets=(
  "windows/amd64/codeGraphIndexer.exe"
  "linux/amd64/codeGraphIndexer-linux-amd64"
  "linux/arm64/codeGraphIndexer-linux-arm64"
  "darwin/amd64/codeGraphIndexer-darwin-amd64"
  "darwin/arm64/codeGraphIndexer-darwin-arm64"
)

for t in "${targets[@]}"; do
  IFS=/ read -r goos goarch name <<<"$t"
  echo "Building $name ($goos/$goarch)..."
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o "dist/$name" .
done

cp cg.sh config.example.json codeGraphIndexer.md LICENSE dist/
cp "$SCRIPT_DIR/release-README.md" dist/README.md
cp "$SCRIPT_DIR/release-README.ru.md" dist/README.ru.md
cp "$SCRIPT_DIR/install.sh" dist/install.sh
cp skill/code-graph/SKILL.md dist/skill/code-graph/
cp "$SCRIPT_DIR/release.gitignore" dist/.gitignore
chmod +x dist/cg.sh dist/install.sh dist/codeGraphIndexer-linux-* dist/codeGraphIndexer-darwin-* 2>/dev/null || true

echo "Done. Release bundle in dist/"
ls -la dist/
ls -la dist/skill/code-graph/
