#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-HafizalJohari/eyeVesa-community}"
VERSION="${VERSION:-main}"
BASE_URL="https://raw.githubusercontent.com/${REPO}/${VERSION}/cli"
BIN_NAME="eyevesa"
PREFERRED_INSTALL_DIR="${HOME}/.local/bin"

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

INSTALL_DIR="${INSTALL_DIR:-${PREFERRED_INSTALL_DIR}}"
mkdir -p "$INSTALL_DIR"

detect_hermes_wrapper() {
  local path="$1"
  [[ -f "$path" ]] || return 1
  grep -Eq 'exec[[:space:]]+hermes[[:space:]]+-p[[:space:]]+eyevesa|Hermes|hermes' "$path"
}

TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

URL="${BASE_URL}/${ASSET}"
echo "Downloading ${URL}"
curl -fsSL "$URL" -o "$TMP_FILE"
chmod +x "$TMP_FILE"

TARGET="${INSTALL_DIR}/${BIN_NAME}"
if [[ -e "$TARGET" ]]; then
  if detect_hermes_wrapper "$TARGET"; then
    BACKUP="${TARGET}.hermes-backup.$(date +%Y%m%d%H%M%S)"
    echo "Existing eyevesa command points to Hermes/profile wrapper. Backing up/replacing with eyeVesa CLI."
    mv "$TARGET" "$BACKUP"
    echo "Backed up wrapper to ${BACKUP}"
  else
    echo "Replacing existing ${TARGET}"
  fi
fi

mv "$TMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"

echo "Installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
if ! command -v "${BIN_NAME}" >/dev/null 2>&1; then
  echo "Add ${INSTALL_DIR} to PATH if needed:"
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo "Run: ${BIN_NAME} --help"
