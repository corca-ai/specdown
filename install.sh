#!/bin/sh
set -eu

REPO="corca-ai/specdown"
BINARY="specdown"
RELEASE_BASE_URL="${SPECDOWN_RELEASE_BASE_URL:-https://github.com/$REPO/releases/download}"

curl_fetch() {
  CURL_URL=$1
  shift
  curl -sSfL --connect-timeout 10 --max-time 120 "$CURL_URL" "$@"
}

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *) echo "Unsupported operating system: $OS" >&2; exit 1 ;;
esac
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Determine install directory
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
if [ ! -w "$INSTALL_DIR" ] 2>/dev/null; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

# Get latest version
VERSION="${VERSION:-$(curl_fetch "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)}"
if [ -z "$VERSION" ]; then
  echo "Failed to determine latest version" >&2
  exit 1
fi
VERSION_NUM="${VERSION#v}"

# Download and install
ARCHIVE="${BINARY}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
URL="$RELEASE_BASE_URL/$VERSION/$ARCHIVE"
CHECKSUMS_URL="$RELEASE_BASE_URL/$VERSION/checksums.txt"

echo "Installing $BINARY $VERSION ($OS/$ARCH) to $INSTALL_DIR"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl_fetch "$CHECKSUMS_URL" -o "$TMP/checksums.txt"
MATCH_COUNT=$(awk -v archive="$ARCHIVE" '$2 == archive { count++ } END { print count + 0 }' "$TMP/checksums.txt")
if [ "$MATCH_COUNT" -ne 1 ]; then
  echo "Expected exactly one checksum for $ARCHIVE, found $MATCH_COUNT" >&2
  exit 1
fi
EXPECTED_SHA=$(awk -v archive="$ARCHIVE" '$2 == archive { print $1 }' "$TMP/checksums.txt")
case "$EXPECTED_SHA" in
  *[!0-9a-fA-F]*|"")
    echo "Invalid SHA-256 checksum for $ARCHIVE" >&2
    exit 1
    ;;
esac
if [ "${#EXPECTED_SHA}" -ne 64 ]; then
  echo "Invalid SHA-256 checksum for $ARCHIVE" >&2
  exit 1
fi

curl_fetch "$URL" -o "$TMP/$ARCHIVE"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_SHA=$(sha256sum "$TMP/$ARCHIVE" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_SHA=$(shasum -a 256 "$TMP/$ARCHIVE" | awk '{ print $1 }')
else
  echo "A SHA-256 tool is required (sha256sum or shasum)" >&2
  exit 1
fi

EXPECTED_SHA=$(printf '%s' "$EXPECTED_SHA" | tr '[:upper:]' '[:lower:]')
ACTUAL_SHA=$(printf '%s' "$ACTUAL_SHA" | tr '[:upper:]' '[:lower:]')
if [ "$ACTUAL_SHA" != "$EXPECTED_SHA" ]; then
  echo "Checksum mismatch for $ARCHIVE" >&2
  echo "  expected: $EXPECTED_SHA" >&2
  echo "  actual:   $ACTUAL_SHA" >&2
  exit 1
fi

tar xzf "$TMP/$ARCHIVE" -C "$TMP"
install "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
echo "Installed $INSTALL_DIR/$BINARY"
