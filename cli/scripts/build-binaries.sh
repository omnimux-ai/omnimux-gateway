#!/usr/bin/env bash
# Build single-file omnimux binaries for all supported platforms via
# `bun build --compile`. Output: dist/omnimux-<platform>-<arch>[.exe]
#
# Usage: bash scripts/build-binaries.sh
#
# Release note: CI (.github/workflows/cli-release.yml) uploads dist/ artifacts
# to the public repo laozhong86/omnimux-cli Releases; install.sh downloads them
# by the same file names. Do not run npm publish / create releases from this
# script — use tag cli-vX.Y.Z (see cli/AGENTS.md).

set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v bun >/dev/null 2>&1; then
  echo "error: bun is required (https://bun.sh)" >&2
  exit 1
fi

VERSION="$(node -p "require('./package.json').version" 2>/dev/null || bun -e "console.log(require('./package.json').version)")"
OUT_DIR="dist"
mkdir -p "$OUT_DIR"

TARGETS=(
  "bun-darwin-arm64:omnimux-darwin-arm64"
  "bun-darwin-x64:omnimux-darwin-x64"
  "bun-linux-x64:omnimux-linux-x64"
  "bun-windows-x64:omnimux-windows-x64.exe"
)

for entry in "${TARGETS[@]}"; do
  target="${entry%%:*}"
  outfile="${entry##*:}"
  echo "building ${outfile} (${target}) ..."
  bun build --compile --target="$target" src/bin.js --outfile "${OUT_DIR}/${outfile}"
done

echo
echo "done. artifacts in ${OUT_DIR}/ (version ${VERSION}):"
ls -lh "$OUT_DIR"
