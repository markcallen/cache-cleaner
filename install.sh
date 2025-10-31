#!/usr/bin/env sh

set -eu

REPO_OWNER="markcallen"
REPO_NAME="cache-cleaner"
APP_NAME="mac-cache-cleaner"

usage() {
  cat <<EOF
Install ${APP_NAME}

Usage:
  install.sh [-b <bin_dir>] [<version>]

Options:
  -b <bin_dir>   Install destination directory (default: GOBIN or GOPATH/bin)

Arguments:
  <version>      Version tag to install (e.g., v1.2.3). Default: latest release

Examples:
  # Install latest to GOBIN/GOPATH/bin
  curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s --

  # Install specific version to /usr/local/bin
  curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sudo sh -s -- -b /usr/local/bin v1.2.3
EOF
}

error() { printf "Error: %s\n" "$1" >&2; exit 1; }

BIN_DIR=""
VERSION=""

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -b) shift; [ $# -gt 0 ] || error "-b requires a directory"; BIN_DIR="$1" ;;
    -*) error "unknown option: $1" ;;
    *) VERSION="$1" ;;
  esac
  shift || true
done

# Determine OS/ARCH
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
[ "$OS" = "darwin" ] || error "only macOS (darwin) is supported"

UNAME_M="$(uname -m)"
case "$UNAME_M" in
  x86_64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) error "unsupported architecture: $UNAME_M" ;;
esac

# Determine BIN_DIR
if [ -z "$BIN_DIR" ]; then
  # Prefer GOBIN, fallback to GOPATH/bin
  if command -v go >/dev/null 2>&1; then
    BIN_DIR="$(go env GOBIN 2>/dev/null || true)"
    if [ -z "$BIN_DIR" ]; then
      GOPATH="$(go env GOPATH 2>/dev/null || true)"
      [ -n "$GOPATH" ] || error "cannot determine GOPATH; set -b <bin_dir> explicitly"
      BIN_DIR="$GOPATH/bin"
    fi
  else
    # No Go; default to /usr/local/bin
    BIN_DIR="/usr/local/bin"
  fi
fi

# Determine VERSION
if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
  # Fetch latest tag via GitHub API
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | sed -n 's/^  \?\"tag_name\": \"\(v[^\"]*\)\".*/\1/p' | head -n1)" || true
  fi
  [ -n "$VERSION" ] || error "failed to determine latest version; pass a version like v1.2.3"
fi

# Validate tag format vx.x.x
case "$VERSION" in
  v[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*) : ;;  # strict semver check: vMAJOR.MINOR.PATCH
  *) error "version must be a semver tag like v1.2.3" ;;
esac

ASSET_NAME="${APP_NAME}-darwin-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${ASSET_NAME}"

TMPFILE="$(mktemp -t ${APP_NAME}.XXXXXX)"
trap 'rm -f "$TMPFILE"' EXIT INT HUP TERM

printf "Installing %s %s for %s/%s to %s\n" "$APP_NAME" "$VERSION" "$OS" "$ARCH" "$BIN_DIR"

curl -fL "${DOWNLOAD_URL}" -o "$TMPFILE" || error "download failed: ${DOWNLOAD_URL}"

chmod 0755 "$TMPFILE"
mkdir -p "$BIN_DIR"
DEST="$BIN_DIR/${APP_NAME}"

if mv "$TMPFILE" "$DEST" 2>/dev/null; then :; else
  # Try install to handle cross-filesystem permissions
  if install -m 0755 "$TMPFILE" "$DEST" 2>/dev/null; then :; else
    error "failed to install to $DEST (try with sudo or set -b)"
  fi
fi

printf "Installed: %s\n" "$DEST"
"$DEST" --version >/dev/null 2>&1 || true
