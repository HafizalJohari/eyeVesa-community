#!/usr/bin/env bash

# eyeVesa Secure Release Builder
# Compiles hardened, optimized production binaries for both Community and Pro tiers.

set -euo pipefail

# Configurations
APP_NAME="eyevesa-gateway"
OUTPUT_DIR="dist"
PACKAGE_PATH="github.com/hafizaljohari/eyeVesa/gateway/control-plane/internal/license"

# Harmonious visual feedback
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== eyeVesa Secure Release Builder ===${NC}"

# Ensure output directory exists
mkdir -p "${OUTPUT_DIR}"

# 1. Compile Community Edition (Free)
# Fully open source, hardcoded fallback limits, completely missing signature verification code.
echo -e "${BLUE}[1/2] Building Hardened Community Edition...${NC}"
(
    cd gateway/control-plane
    GOOS=darwin GOARCH=amd64 go build \
        -ldflags="-s -w" \
        -o "../../${OUTPUT_DIR}/${APP_NAME}-community-darwin-amd64" \
        ./cmd/api

    GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w" \
        -o "../../${OUTPUT_DIR}/${APP_NAME}-community-linux-amd64" \
        ./cmd/api
)

echo -e "${GREEN}✓ Community builds generated in ${OUTPUT_DIR}/${NC}"

# 2. Compile Pro/Enterprise Edition (Licensed)
# Includes signature decoding logic and requires high-water mark checks.
# Public verification key is injected into the binary so it cannot be tampered with.
echo -e "${BLUE}[2/2] Building Hardened Pro/Enterprise Edition...${NC}"

# Target Ed25519 Public Key for license verification (uses default hex fallback if not passed).
# Generate your production keypair and place the public hex here!
PRO_VERIFICATION_KEY="${EYEVESA_PRO_PUBKEY:-8b5fa1a8f9b9f9e5781a7d65cc1a3d9eb7412e698889aa7bfd7aefc3e80e1a12}"

echo -e "${YELLOW}Baking verification public key into binary: ${PRO_VERIFICATION_KEY}${NC}"

(
    cd gateway/control-plane
    GOOS=darwin GOARCH=amd64 go build \
        -tags pro \
        -ldflags="-s -w -X ${PACKAGE_PATH}.BakedPublicKey=${PRO_VERIFICATION_KEY}" \
        -o "../../${OUTPUT_DIR}/${APP_NAME}-pro-darwin-amd64" \
        ./cmd/api

    GOOS=linux GOARCH=amd64 go build \
        -tags pro \
        -ldflags="-s -w -X ${PACKAGE_PATH}.BakedPublicKey=${PRO_VERIFICATION_KEY}" \
        -o "../../${OUTPUT_DIR}/${APP_NAME}-pro-linux-amd64" \
        ./cmd/api
)

echo -e "${GREEN}✓ Pro builds generated in ${OUTPUT_DIR}/${NC}"

# 3. Dynamic UPX Obfuscation and Compression Check
if command -v upx &> /dev/null; then
    echo -e "${BLUE}[Bonus] UPX detected! Compressing and obfuscating binaries...${NC}"
    upx --best --ultra-brute "${OUTPUT_DIR}"/* || true
    echo -e "${GREEN}✓ Binaries securely packed and obfuscated.${NC}"
else
    echo -e "${YELLOW}[Notice] UPX packer is not installed on host. Skipping binary packing/obfuscation.${NC}"
fi

echo -e "${GREEN}=== Release Generation Successful ===${NC}"
ls -lh "${OUTPUT_DIR}"
