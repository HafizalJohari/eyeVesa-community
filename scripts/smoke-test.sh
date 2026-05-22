#!/usr/bin/env bash
set -euo pipefail

CONTROL_URL="${CONTROL_URL:-http://localhost:8080}"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:9443}"
OPA_URL="${OPA_URL:-http://localhost:8181}"

pass() {
  echo "✓ $1"
}

check_get() {
  local name="$1"
  local url="$2"
  curl -fsS "$url" >/dev/null
  pass "$name"
}

check_post_json() {
  local name="$1"
  local url="$2"
  local body="$3"
  curl -fsS -X POST "$url" -H "Content-Type: application/json" -d "$body" >/dev/null
  pass "$name"
}

check_get "control-plane health" "${CONTROL_URL}/health"
check_get "gateway proxy health" "${GATEWAY_URL}/health"
check_get "OPA data API" "${OPA_URL}/v1/data"
check_get "agents list" "${CONTROL_URL}/v1/agents"
check_get "airport online" "${CONTROL_URL}/v1/airport/online"
check_post_json "MCP initialize" "${CONTROL_URL}/v1/mcp" '{"jsonrpc":"2.0","id":1,"method":"initialize"}'

echo "Smoke test passed."
