#!/usr/bin/env bash
# omnimux CLI installer.
#
# Detects the current platform, downloads the matching prebuilt binary from
# the public GitHub Releases repo, and installs it into /usr/local/bin
# (or ~/.local/bin as a user-level fallback).
#
# Usage (user-facing one-liner once CDN is live):
#   curl -fsSL https://omnimux.ai/install.sh | bash
#
# Direct / versioned:
#   curl -fsSL https://raw.githubusercontent.com/laozhong86/omnimux-cli/main/install.sh | bash
#   bash install.sh [--version cli-vX.Y.Z|vX.Y.Z|X.Y.Z|latest] [--prefix /custom/bin]
#
# Source of truth for this script: monorepo cli/scripts/install.sh
# Release surface: https://github.com/laozhong86/omnimux-cli/releases

set -euo pipefail

OMNIMUX_RELEASE_REPO="laozhong86/omnimux-cli"
VERSION="latest"
PREFIX=""

while [ $# -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="$2"; shift 2 ;;
    --version=*)
      VERSION="${1#*=}"; shift ;;
    --prefix)
      PREFIX="$2"; shift 2 ;;
    --prefix=*)
      PREFIX="${1#*=}"; shift ;;
    -h|--help)
      grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)
      echo "error: unknown argument: $1" >&2; exit 1 ;;
  esac
done

# Normalize tag: accept latest | X.Y.Z | vX.Y.Z | cli-vX.Y.Z
normalize_tag() {
  local v="$1"
  if [ "$v" = "latest" ]; then
    echo "latest"
    return
  fi
  v="${v#cli-}"
  v="${v#v}"
  echo "cli-v${v}"
}

TAG="$(normalize_tag "$VERSION")"

# --- Platform detection ---

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin) platform="darwin" ;;
  linux)  platform="linux" ;;
  msys*|mingw*|cygwin*)
    echo "error: on Windows download omnimux-windows-x64.exe from GitHub Releases, or: npm i -g @omnimux/cli" >&2
    exit 1 ;;
  *) echo "error: unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64)  arch="x64" ;;
  *) echo "error: unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$platform" = "linux" ] && [ "$arch" = "arm64" ]; then
  echo "error: no prebuilt linux/arm64 binary yet; install via npm: npm i -g @omnimux/cli" >&2
  exit 1
fi

ASSET="omnimux-${platform}-${arch}"

# --- Download ---

if [ "$TAG" = "latest" ]; then
  URL="https://github.com/${OMNIMUX_RELEASE_REPO}/releases/latest/download/${ASSET}"
  SUMS_URL="https://github.com/${OMNIMUX_RELEASE_REPO}/releases/latest/download/SHA256SUMS"
else
  URL="https://github.com/${OMNIMUX_RELEASE_REPO}/releases/download/${TAG}/${ASSET}"
  SUMS_URL="https://github.com/${OMNIMUX_RELEASE_REPO}/releases/download/${TAG}/SHA256SUMS"
fi

tmp="$(mktemp)"
sums_tmp="$(mktemp)"
trap 'rm -f "$tmp" "$sums_tmp"' EXIT

echo "downloading ${URL}"
if ! curl -fsSL "$URL" -o "$tmp"; then
  echo "error: download failed. Check that release asset ${ASSET} exists on ${OMNIMUX_RELEASE_REPO}." >&2
  echo "hint: npm i -g @omnimux/cli" >&2
  exit 1
fi
chmod +x "$tmp"

# Optional checksum verification (warn if SUMS missing; fail if present but mismatch)
if curl -fsSL "$SUMS_URL" -o "$sums_tmp" 2>/dev/null; then
  expected="$(grep -E "[[:space:]]${ASSET}\$" "$sums_tmp" | awk '{print $1}' | head -1 || true)"
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "$tmp" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      actual="$(shasum -a 256 "$tmp" | awk '{print $1}')"
    else
      actual=""
    fi
    if [ -n "$actual" ] && [ "$actual" != "$expected" ]; then
      echo "error: SHA256 mismatch for ${ASSET}" >&2
      echo "  expected: ${expected}" >&2
      echo "  actual:   ${actual}" >&2
      exit 1
    fi
    if [ -n "$actual" ]; then
      echo "checksum ok"
    fi
  fi
else
  echo "note: SHA256SUMS not found for this release; skipping checksum verification"
fi

# --- Install ---

choose_prefix() {
  if [ -n "$PREFIX" ]; then
    echo "$PREFIX"
  elif [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
  else
    echo "${HOME}/.local/bin"
  fi
}

dest_dir="$(choose_prefix)"
mkdir -p "$dest_dir" 2>/dev/null || true
dest="${dest_dir}/omnimux"

if mv "$tmp" "$dest" 2>/dev/null; then
  :
else
  echo "error: cannot write to ${dest_dir}. Re-run with --prefix ~/.local/bin or sudo." >&2
  exit 1
fi

# Clear trap file (moved)
tmp=""

echo "installed: ${dest}"
case ":${PATH}:" in
  *":${dest_dir}:"*) ;;
  *) echo "note: ${dest_dir} is not on your PATH — add it to your shell profile." ;;
esac

if "$dest" --version >/dev/null 2>&1; then
  version="$("$dest" --version 2>/dev/null | sed -n 's/.*"version":"\([0-9][0-9.]*\)".*/\1/p')"
  echo "omnimux ${version:-installed} ready."
  echo
  echo "next:"
  echo "  omnimux help                      # default instance https://omnimux.ai"
  echo "  omnimux login                     # browser device-code sign-in"
  echo
  echo "note: this standalone binary does not include the optional Agent skill packs."
  echo "To install them, use the npm package instead of this binary:"
  echo "  npm install -g @omnimux/cli"
  echo "  omnimux skill install --user      # or --project inside a repository"
  echo "  # then restart your AI agent so it rescans skills"
  echo "  # make sure the npm global bin directory precedes ${dest_dir} on PATH"
  echo "With a source checkout, point the binary at its skill tree instead:"
  echo "  OMNIMUX_SKILL_SOURCE=/path/to/OmniMux/cli/skill omnimux skill install --user"
else
  echo "installed binary could not run --version; check architecture and permissions."
fi
