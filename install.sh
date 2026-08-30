#!/bin/sh
# Install chatta from a GitHub Release.
#
#   curl -fsSL https://raw.githubusercontent.com/alswl/chatta/master/install.sh | sh
#
# Optional environment variables:
#   CHATTA_VERSION      A release tag to install (for example v0.1.0).
#   CHATTA_INSTALL_DIR  Directory receiving the binary.
set -eu

REPO="alswl/chatta"
PROJECT="chatta"
BASE_URL="https://github.com/${REPO}"
INSTALL_URL="https://raw.githubusercontent.com/${REPO}/master/install.sh"

log() { printf '%s\n' "==> $*"; }
warn() { printf '%s\n' "==> $*" >&2; }
die() { printf '%s\n' "Error: $*" >&2; exit 1; }

if [ -n "${CHATTA_VERSION:-}" ]; then
	VERSION="$CHATTA_VERSION"
else
	VERSION=""
	if VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' "${BASE_URL}/releases/latest" 2>/dev/null)"; then
		case "$VERSION" in
			*/releases/tag/v*) VERSION="${VERSION##*/}" ;;
			*) VERSION="" ;;
		esac
	else
		VERSION=""
	fi
	if [ -z "$VERSION" ]; then
		log "No stable release found; falling back to the newest release."
		VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=1" 2>/dev/null | awk -F'\"' '/\"tag_name\"/ {print $4; exit}')" || VERSION=""
	fi
fi
case "$VERSION" in
	v*) ;;
	*) die "Could not determine a release tag. Pin one explicitly, e.g. CHATTA_VERSION=v0.1.0 sh -c 'curl -fsSL ${INSTALL_URL} | sh'." ;;
esac

case "$(uname -s)" in
	Darwin) OS="darwin" ;;
	Linux) OS="linux" ;;
	*) die "Unsupported OS. chatta releases support macOS and Linux." ;;
esac
case "$(uname -m)" in
	x86_64|amd64) ARCH="amd64" ;;
	aarch64|arm64) ARCH="arm64" ;;
	*) die "Unsupported architecture. chatta releases support amd64 and arm64." ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

asset="${PROJECT}-${VERSION}-${OS}-${ARCH}.tar.gz"
asset_url="${BASE_URL}/releases/download/${VERSION}/${asset}"
log "Downloading ${asset}"
curl -fsSL --retry 3 --retry-delay 2 -C - "$asset_url" -o "$tmpdir/$asset" \
	|| die "Failed to download ${asset_url}. Check CHATTA_VERSION and release availability."

if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$tmpdir/$asset" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')"
else
	die "Neither sha256sum nor shasum is available for checksum verification."
fi
curl -fsSL "${BASE_URL}/releases/download/${VERSION}/checksums.txt" -o "$tmpdir/checksums.txt" \
	|| die "Release ${VERSION} has no checksums.txt."
expected="$(awk -v asset="$asset" '$2 == asset {print $1}' "$tmpdir/checksums.txt")"
[ -n "$expected" ] || die "No checksum entry for ${asset}."
[ "$actual" = "$expected" ] || die "Checksum mismatch for ${asset}."
log "Checksum verified"

tar -xzf "$tmpdir/$asset" -C "$tmpdir" "${PROJECT}-${OS}-${ARCH}" \
	|| die "Release archive has an unexpected layout."

if [ -n "${CHATTA_INSTALL_DIR:-}" ]; then
	install_dir="$CHATTA_INSTALL_DIR"
else
	install_dir=""
	for candidate in /opt/homebrew/bin "$HOME/.local/bin" /usr/local/bin "$HOME/bin"; do
		if [ -d "$candidate" ] && [ -w "$candidate" ]; then
			install_dir="$candidate"
			break
		fi
	done
fi
[ -n "$install_dir" ] || die "No writable install directory found. Set CHATTA_INSTALL_DIR, for example: CHATTA_INSTALL_DIR=\"\$HOME/.local/bin\" sh -c 'curl -fsSL ${INSTALL_URL} | sh'."
mkdir -p "$install_dir"
[ -w "$install_dir" ] || die "Install directory is not writable: $install_dir"

install -m 0755 "$tmpdir/${PROJECT}-${OS}-${ARCH}" "$install_dir/$PROJECT"
log "Installed ${PROJECT} ${VERSION} -> ${install_dir}/${PROJECT}"
"$install_dir/$PROJECT" version

case ":${PATH}:" in
	*":${install_dir}:"*) ;;
	*) warn "${install_dir} is not on PATH. Add: export PATH=\"${install_dir}:\$PATH\"" ;;
esac
