#!/usr/bin/env bash
# Universal CLI installer — downloads binaries from GitHub Releases.
# github.com/alswl/makefile-go
#
# Author: alswl
# Version: 0.1.0
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/<user>/<repo>/main/hack/install.sh | sh
#   VERSION=v1.0.0 INSTALL_DIR=/usr/local/bin bash install.sh
#
# Replace placeholders below:
#   __GITHUB_REPO__ — github.com owner/repo
#   __BINARY_NAME__ — binary name

set -e

# ── Project configuration ──────────────────────────────────────────────
GITHUB_REPO="__GITHUB_REPO__"       # e.g. alswl/makefile-go
BINARY_NAME="__BINARY_NAME__"       # e.g. my-cli
# ────────────────────────────────────────────────────────────────────────

VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-}"

# Colors
if [ -z "$NO_COLOR" ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BOLD='\033[1m'
  NC='\033[0m'
else
  RED='' GREEN='' YELLOW='' BOLD='' NC=''
fi

info()  { echo -e "${GREEN}→${NC} $*"; }
warn()  { echo -e "${YELLOW}⚠${NC} $*"; }
error() { echo -e "${RED}✗${NC} $*"; }
header() { echo -e "${BOLD}$*${NC}"; }

# ── Detect OS / Arch ───────────────────────────────────────────────────
detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)             error "Unsupported architecture: $ARCH"; exit 1 ;;
  esac

  case "$OS" in
    linux)   OS="linux" ;;
    darwin)  OS="darwin" ;;
    *)       error "Unsupported OS: $OS"; exit 1 ;;
  esac
}

# ── Resolve version ────────────────────────────────────────────────────
resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    info "Fetching latest version..."
    VERSION=$(curl -sS "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
      | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
      error "Failed to fetch latest version from GitHub"
      exit 1
    fi
  fi
  info "Version: ${VERSION}"
}

# ── Determine install directory ────────────────────────────────────────
detect_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    mkdir -p "$INSTALL_DIR" || { error "Cannot create $INSTALL_DIR"; exit 1; }
    return
  fi

  for d in /opt/homebrew/bin "$HOME/.local/bin" /usr/local/bin "$GOPATH/bin" "$HOME/go/bin"; do
    if [ -d "$d" ] && [ -w "$d" ]; then
      INSTALL_DIR="$d"
      break
    fi
  done

  if [ -z "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi
}

# ── Build download URL ─────────────────────────────────────────────────
build_url() {
  EXT=""
  if [ "$OS" = "windows" ]; then EXT=".exe"; fi
  URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY_NAME}-${OS}-${ARCH}${EXT}"
}

# ── Main ───────────────────────────────────────────────────────────────
header "Installing ${BINARY_NAME}..."

detect_platform
resolve_version
detect_install_dir
build_url

info "Platform: ${OS}/${ARCH}"
info "Install to: ${INSTALL_DIR}"

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

info "Downloading ${URL}"
if ! curl -sSL --fail -o "$TMPFILE" "$URL"; then
  error "Download failed. Check VERSION and GITHUB_REPO."
  exit 1
fi

chmod +x "$TMPFILE"
mv "$TMPFILE" "${INSTALL_DIR}/${BINARY_NAME}"

info "Installed ${BINARY_NAME} → ${INSTALL_DIR}/${BINARY_NAME}"

# Verify
if command -v "$BINARY_NAME" >/dev/null 2>&1; then
  "$BINARY_NAME" version 2>/dev/null || true
  info "Done."
else
  warn "${INSTALL_DIR} is not in PATH."
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
