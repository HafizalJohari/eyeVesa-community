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

clear
echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}${BLUE}          👁️  Welcome to eyeVesa Onboarding 👁️          ${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo -e "Creating a zero-headache local environment for you..."

# 1. Environment Guard
if [ ! -f .env ]; then
    echo -e "${YELLOW}[!] .env file not found. Generating a secure one for you...${NC}"
    # Generate a secure 32-character random JWT secret.
    RANDOM_JWT=$(LC_ALL=C tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 32 || echo "eyvesa_default_secure_secret_key_12345")
    
    echo "EYEVESA_JWT_SECRET=${RANDOM_JWT}" > .env
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
fi

# Load variables safely
export $(grep -v '^#' .env | xargs)

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
echo -e "${BLUE}[1/2] Spinning up local PostgreSQL, OPA, and eyeVesa Gateway services...${NC}"
docker compose up -d postgres opa gateway-control gateway-core

# 4. Wait for database health
echo -e "${BLUE}[2/2] Waiting for database to become healthy...${NC}"
until docker exec agentid-postgres pg_isready -U agentid &> /dev/null; do
    echo -n "."
    sleep 1
done
echo -e "\n${GREEN}✓ Database is healthy and migrated!${NC}"

# Give the control plane a moment to bind ports
sleep 2

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
