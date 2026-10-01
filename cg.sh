#!/usr/bin/env bash
# Pick the right codeGraphIndexer binary for this OS/arch.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

os="$(uname -s 2>/dev/null || echo unknown)"
arch="$(uname -m 2>/dev/null || echo unknown)"

case "$os" in
  Linux*)  goos=linux ;;
  Darwin*) goos=darwin ;;
  MINGW*|MSYS*|CYGWIN*) goos=windows ;;
  *) goos=unknown ;;
esac

case "$arch" in
  x86_64|amd64) goarch=amd64 ;;
  aarch64|arm64) goarch=arm64 ;;
  *) goarch=unknown ;;
esac

candidates=()
if [[ "$goos" == "windows" ]]; then
  candidates+=("$DIR/codeGraphIndexer.exe")
fi
if [[ "$goos" != "unknown" && "$goarch" != "unknown" ]]; then
  candidates+=("$DIR/codeGraphIndexer-${goos}-${goarch}")
fi
# Generic names (local build / renamed)
candidates+=("$DIR/codeGraphIndexer" "$DIR/codeGraphIndexer.exe")

for bin in "${candidates[@]}"; do
  if [[ -f "$bin" && -x "$bin" ]]; then
    exec "$bin" "$@"
  fi
  # Windows Git Bash: .exe may lack +x
  if [[ -f "$bin" ]]; then
    exec "$bin" "$@"
  fi
done

echo "codeGraphIndexer binary not found in $DIR" >&2
echo "Expected one of:" >&2
printf '  %s\n' "${candidates[@]}" >&2
exit 1
