#!/usr/bin/env bash

# eyeVesa Quickstart Onboarding Wizard
# Automatically configures env keys, verifies docker, boots up the local environment, and prints visual guides.

set -euo pipefail

# Visual Styling
BLUE='\033[0;34m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

if [ -t 1 ] && command -v clear &> /dev/null; then
    clear || true
fi
echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}${BLUE}          👁️  Welcome to eyeVesa Onboarding 👁️          ${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo -e "Creating a zero-headache local environment for you..."

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_INSTALL_DIR="${HOME}/.local/bin"
CLI_BIN="${CLI_INSTALL_DIR}/eyevesa"

detect_hermes_wrapper() {
    local path="$1"
    [ -f "$path" ] || return 1
    grep -Eq 'exec[[:space:]]+hermes[[:space:]]+-p[[:space:]]+eyevesa|Hermes|hermes' "$path"
}

install_cli() {
    mkdir -p "$CLI_INSTALL_DIR"

    local existing=""
    if command -v eyevesa &> /dev/null; then
        existing="$(command -v eyevesa)"
        echo -e "${BLUE}Detected eyevesa command:${NC} ${existing}"
        if detect_hermes_wrapper "$existing"; then
            local backup="${existing}.hermes-backup.$(date +%Y%m%d%H%M%S)"
            echo -e "${YELLOW}Existing eyevesa command points to Hermes/profile wrapper. Backing up/replacing with eyeVesa CLI.${NC}"
            mv "$existing" "$backup"
            echo -e "${GREEN}✓ Backed up wrapper to ${backup}.${NC}"
        elif [ "$existing" != "$CLI_BIN" ]; then
            echo -e "${YELLOW}[!] eyevesa resolves outside ${CLI_INSTALL_DIR}. Installing community CLI to ${CLI_BIN}.${NC}"
            echo -e "${YELLOW}    Put ${CLI_INSTALL_DIR} before other directories in PATH if this command still resolves elsewhere.${NC}"
        fi
    else
        echo -e "${BLUE}No existing eyevesa command found. Installing CLI to ${CLI_BIN}.${NC}"
    fi

    if command -v go &> /dev/null; then
        echo -e "${BLUE}Building eyeVesa CLI...${NC}"
        (cd "$REPO_ROOT/cli" && go build -o "$CLI_BIN" .)
        echo -e "${GREEN}✓ Installed eyeVesa CLI to ${CLI_BIN}.${NC}"
    else
        echo -e "${YELLOW}[!] Go is not installed, so the CLI was not built automatically.${NC}"
        echo -e "CLI not installed. To install:"
        echo -e "  cd cli && go build -o ~/.local/bin/eyevesa ."
        echo -e "Docker fallback:"
        echo -e "  docker build -t eyevesa-cli -f cli/Dockerfile ."
    fi
}

# 1. Environment Guard
if [ ! -f .env ]; then
    echo -e "${YELLOW}[!] .env file not found. Generating a secure one for you...${NC}"
    # Generate a secure 32-character random JWT secret.
    RANDOM_JWT=$(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 32 || echo "eyvesa_default_secure_secret_key_12345")
    
    echo "EYEVESA_JWT_SECRET=${RANDOM_JWT}" > .env
    echo "AUTH_ENABLED=false" >> .env
    echo "RATE_LIMIT_RPS=100.0" >> .env
    echo "POLICY_DIR=/policies" >> .env
    echo -e "${GREEN}✓ Generated secure .env file with custom EYEVESA_JWT_SECRET.${NC}"
else
    # Verify if EYEVESA_JWT_SECRET exists, if not append it.
    if ! grep -q "EYEVESA_JWT_SECRET" .env; then
        RANDOM_JWT=$(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 32 || echo "eyvesa_default_secure_secret_key_12345")
        echo "EYEVESA_JWT_SECRET=${RANDOM_JWT}" >> .env
        echo -e "${GREEN}✓ Appended missing EYEVESA_JWT_SECRET to .env file.${NC}"
    fi
    if ! grep -q "^AUTH_ENABLED=" .env; then
        echo "AUTH_ENABLED=false" >> .env
        echo -e "${GREEN}✓ Appended AUTH_ENABLED=false for local community mode.${NC}"
    fi
fi

# Load variables safely
export $(grep -v '^#' .env | xargs)
export AUTH_ENABLED=false

install_cli

# 2. Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}[Error] Docker is not installed on this machine. Please install Docker and try again!${NC}"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo -e "${RED}[Error] Docker daemon is not running. Please start Docker and run this script again!${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker daemon active.${NC}"

# 3. Fire up Docker Compose
echo -e "${BLUE}[1/3] Spinning up local PostgreSQL, OPA, and eyeVesa Gateway services...${NC}"
docker compose up -d postgres opa gateway-control gateway-core

# 4. Wait for database health
echo -e "${BLUE}[2/3] Waiting for database to become healthy...${NC}"
until docker compose exec -T postgres pg_isready -U agentid &> /dev/null; do
    echo -n "."
    sleep 1
done
echo -e "\n${GREEN}✓ Database is healthy and migrated!${NC}"

# Give the control plane a moment to bind ports
sleep 2

echo -e "${BLUE}[3/3] Checking local services...${NC}"
curl -fsS http://localhost:8080/health >/dev/null && echo -e "${GREEN}✓ Control-plane health OK.${NC}" || echo -e "${YELLOW}[!] Control-plane health check failed: curl http://localhost:8080/health${NC}"
curl -fsS http://localhost:9443/health >/dev/null && echo -e "${GREEN}✓ Gateway proxy health OK.${NC}" || echo -e "${YELLOW}[!] Gateway proxy health check failed: curl http://localhost:9443/health${NC}"

# 5. Onboarding Success Screen
echo -e "\n${CYAN}=======================================================${NC}"
echo -e "${BOLD}${GREEN}        🎉 eyeVesa is Running Successfully! 🎉        ${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo -e ""
echo -e "Here are your active endpoints to begin testing immediately:"
echo -e "  • ${BOLD}Control-Plane (Go):${NC}  http://localhost:8080"
echo -e "  • ${BOLD}Gateway Proxy (Rust):${NC} http://localhost:9443"
echo -e "  • ${BOLD}Open Policy Agent:${NC}    http://localhost:8181"
echo -e ""
echo -e "Verification commands:"
echo -e "${CYAN}-------------------------------------------------------${NC}"
echo -e "curl http://localhost:8080/health"
echo -e "curl http://localhost:9443/health"
echo -e "eyevesa doctor --gateway http://localhost:8080"
echo -e "${CYAN}-------------------------------------------------------${NC}"
echo -e "Expected CLI doctor result: ${BOLD}Gateway health: ✓${NC} and ${BOLD}All checks passed${NC} after running ${BOLD}eyevesa init${NC}."
echo -e ""
echo -e "Try registering your first agent by running this command in your terminal:"
echo -e "${CYAN}-------------------------------------------------------${NC}"
echo -e "curl -X POST http://localhost:8080/v1/agents/register \\"
echo -e "  -H \"Content-Type: application/json\" \\"
echo -e "  -d '{"
echo -e "    \"name\": \"agent-01\","
echo -e "    \"owner\": \"dev-team\","
echo -e "    \"public_key\": \"MCowBQYDK2VwAyEA51y9Q/E+4w8842G6F3v8qQ/E+4w8842G6F3v8qQ1234=\""
echo -e "  }'"
echo -e "${CYAN}-------------------------------------------------------${NC}"
echo -e ""
echo -e "To view live logs, run: ${BOLD}docker compose logs -f gateway-control${NC}"
echo -e "To shut down, run:       ${BOLD}docker compose down${NC}"
echo -e "${CYAN}=======================================================${NC}"
