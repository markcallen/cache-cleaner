#!/usr/bin/env sh

set -eu

REPO_OWNER="markcallen"
REPO_NAME="cache-cleaner"

# Available apps
APPS="dev-cache git-cleaner mac-cache-cleaner"

usage() {
  cat <<EOF
Install cache-cleaner tools

RECOMMENDED: Use Homebrew instead of this script:
  brew tap ${REPO_OWNER}/${REPO_NAME}
  brew install ${REPO_NAME}

Usage:
  install.sh -b <bin_dir> [-a <app>] [<version>]

Options:
  -b <bin_dir>   Install destination directory (REQUIRED)
  -a <app>       Install specific app only: dev-cache, git-cleaner, or mac-cache-cleaner
                 (default: install all 3 apps)

Arguments:
  <version>      Version tag to install (e.g., v1.2.3). Default: latest release

Examples:
  # Install all 3 apps (latest) to ~/.local/bin
  curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s -- -b \$HOME/.local/bin

  # Install only mac-cache-cleaner
  curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s -- -b \$HOME/.local/bin -a mac-cache-cleaner

  # Install specific version
  curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s -- -b \$HOME/.local/bin v1.2.3

Note: Make sure your bin directory is in PATH:
  export PATH="\$HOME/.local/bin:\$PATH"
EOF
}

error() { printf "Error: %s\n" "$1" >&2; exit 1; }

BIN_DIR=""
VERSION=""
APP_FILTER=""

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -b) shift; [ $# -gt 0 ] || error "-b requires a directory"; BIN_DIR="$1" ;;
    -a) shift; [ $# -gt 0 ] || error "-a requires an app name"; APP_FILTER="$1" ;;
    -*) error "unknown option: $1" ;;
    *) VERSION="$1" ;;
  esac
  shift
done

# Validate app filter if provided
if [ -n "$APP_FILTER" ]; then
  case "$APP_FILTER" in
    dev-cache|git-cleaner|mac-cache-cleaner) : ;;
    *) error "invalid app: $APP_FILTER (must be one of: dev-cache, git-cleaner, mac-cache-cleaner)" ;;
  esac
  APPS="$APP_FILTER"
fi

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
  # Check if Homebrew is available - recommend using it instead
  if command -v brew >/dev/null 2>&1; then
    cat >&2 <<EOF
Error: Homebrew installation is recommended over direct installation.

Please install using Homebrew instead:

    brew tap ${REPO_OWNER}/${REPO_NAME}
    brew install ${REPO_NAME}

If you prefer direct installation, specify an installation directory:

    curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s -- -b \$HOME/.local/bin

For more information, visit:
    https://github.com/${REPO_OWNER}/${REPO_NAME}
EOF
    exit 1
  fi

  # No Homebrew - require explicit -b flag
  cat >&2 <<EOF
Error: Installation directory must be specified with -b flag.

Example:
    curl -sSfL https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/HEAD/install.sh | sh -s -- -b \$HOME/.local/bin

Make sure the directory is in your PATH:
    export PATH="\$HOME/.local/bin:\$PATH"

Alternatively, if you have Homebrew, use:
    brew tap ${REPO_OWNER}/${REPO_NAME}
    brew install ${REPO_NAME}
EOF
  exit 1
fi

# Determine VERSION
if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
  # Fetch latest tag via GitHub API
  if command -v curl >/dev/null 2>&1; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | awk -F'"' '/tag_name/ {print $4}')" || true
  fi
  [ -n "$VERSION" ] || error "failed to determine latest version; pass a version like v1.2.3"
fi

# Validate tag format vx.x.x
case "$VERSION" in
  v[0-9]*\.[0-9]*\.[0-9]*) : ;;  # strict semver check: vMAJOR.MINOR.PATCH
  *) error "version must be a semver tag like v1.2.3" ;;
esac

# Cleanup function for temp files
cleanup() {
  if [ -n "${TMPFILE:-}" ] && [ -f "$TMPFILE" ]; then
    rm -f "$TMPFILE"
  fi
}
trap cleanup EXIT INT HUP TERM

# Install each app
for APP_NAME in $APPS; do
  ASSET_NAME="${APP_NAME}-darwin-${ARCH}"
  DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/${ASSET_NAME}"

  TMPFILE="$(mktemp -t ${APP_NAME}.XXXXXX)"

  printf "Installing %s %s for %s/%s to %s\n" "$APP_NAME" "$VERSION" "$OS" "$ARCH" "$BIN_DIR"

  curl -fL "${DOWNLOAD_URL}" -o "$TMPFILE" || error "download failed: ${DOWNLOAD_URL}"

  chmod 0755 "$TMPFILE"
  mkdir -p "$BIN_DIR"
  DEST="$BIN_DIR/${APP_NAME}"

  if mv "$TMPFILE" "$DEST" 2>/dev/null; then
    # Successfully moved, clear TMPFILE so cleanup doesn't try to remove it
    TMPFILE=""
  else
    # Try install to handle cross-filesystem permissions
    if install -m 0755 "$TMPFILE" "$DEST" 2>/dev/null; then
      # Successfully installed, remove temp file since install copies it
      rm -f "$TMPFILE"
      TMPFILE=""
    else
      error "failed to install to $DEST (try with sudo or set -b)"
    fi
  fi

  printf "Installed: %s\n" "$DEST"
  "$DEST" --version >/dev/null 2>&1 || true
done
