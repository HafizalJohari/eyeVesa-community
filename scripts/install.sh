#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-Hafizaljohari/eyeVesa}"
VERSION="${VERSION:-main}"
BASE_URL="https://raw.githubusercontent.com/${REPO}/${VERSION}/cli"
BIN_NAME="eyevesa"

OS="$(uname -s)"
ARCH="$(uname -m)"

if [[ "$OS" != "Darwin" && "$OS" != "Linux" ]]; then
  echo "Unsupported OS: $OS"
  exit 1
fi

case "$ARCH" in
  arm64|aarch64) ASSET="eyevesa-arm64" ;;
  x86_64|amd64) ASSET="eyevesa-amd64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

if [[ -w "/usr/local/bin" ]]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

URL="${BASE_URL}/${ASSET}"
echo "Downloading ${URL}"
curl -fsSL "$URL" -o "$TMP_FILE"
chmod +x "$TMP_FILE"
mv "$TMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
if ! command -v "${BIN_NAME}" >/dev/null 2>&1; then
  echo "Add ${INSTALL_DIR} to PATH if needed:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo "Run: ${BIN_NAME} --help"
