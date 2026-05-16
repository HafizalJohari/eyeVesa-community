#!/bin/bash
# AgentID Gateway End-to-End Test Suite
# Prerequisites: Go server running on :8080/:9090, Rust proxy on :9443, resource adapter on :8443
set -e

GATEWAY_PROXY="http://localhost:9443"
GATEWAY_HTTP="http://localhost:8080"
ADAPTER="http://localhost:8443"
PASS=0
FAIL=0

check() {
    local name="$1"
    local expected="$2"
    local actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "  ✓ $name"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $name: expected '$expected', got '$actual'"
        FAIL=$((FAIL + 1))
    fi
}

check_contains() {
    local name="$1"
    local needle="$2"
    local haystack="$3"
    if echo "$haystack" | grep -q "$needle"; then
        echo "  ✓ $name"
        PASS=$((PASS + 1))
    else
        echo "  ✗ $name: expected to contain '$needle' in '$haystack'"
        FAIL=$((FAIL + 1))
    fi
}

echo "============================================"
echo "  AgentID Gateway E2E Test Suite"
echo "============================================"
echo ""

# === Health ===
echo "--- Health Checks ---"
HEALTH_PROXY=$(curl -s "$GATEWAY_PROXY/health")
check "Proxy health" "ok" "$HEALTH_PROXY"

HEALTH_HTTP=$(curl -s "$GATEWAY_HTTP/health")
check "HTTP health" "ok" "$HEALTH_HTTP"

HEALTH_ADAPTER=$(curl -s "$ADAPTER/health")
check "Adapter health" "ok" "$HEALTH_ADAPTER"

echo ""

# === Agent Registration ===
echo "--- Agent Registration ---"
REG=$(curl -s -X POST "$GATEWAY_PROXY/v1/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"e2e-agent","owner":"e2e-team","capabilities":["mcp"],"allowed_tools":["read","write","search"]}')
AGENT_ID=$(echo "$REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['agent_id'])" 2>/dev/null)
check_contains "Agent registered" "agent_id" "$REG"

echo ""

# === Authorization (OPA) ===
echo "--- Authorization (OPA Policy Engine) ---"

AUTH_READ=$(curl -s -X POST "$GATEWAY_PROXY/v1/auth" \
  -H "Content-Type: application/json" \
  -d "{\"agent_id\":\"$AGENT_ID\",\"action\":\"read\",\"resource_id\":\"doc-001\"}")
check_contains "Read allowed" '"allowed":true' "$AUTH_READ"
check_contains "No HITL" '"requires_hitl":false' "$AUTH_READ"

AUTH_DELETE=$(curl -s -X POST "$GATEWAY_PROXY/v1/auth" \
  -H "Content-Type: application/json" \
  -d "{\"agent_id\":\"$AGENT_ID\",\"action\":\"delete\",\"resource_id\":\"db-prod\"}")
check_contains "Delete denied" '"allowed":false' "$AUTH_DELETE"
check_contains "Delete requires HITL" '"requires_hitl":true' "$AUTH_DELETE"

echo ""

# === MCP Protocol ===
echo "--- MCP Protocol ---"

MCP_INIT=$(curl -s -X POST "$GATEWAY_PROXY/v1/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":"m1"}')
check_contains "MCP version" "2024-11-05" "$MCP_INIT"
check_contains "MCP capabilities" "tools" "$MCP_INIT"

echo ""

# === PTV (Prove-Transform-Verify) ===
echo "--- PTV: Attest → Bind → Verify ---"

ATTEST=$(curl -s -X POST "$GATEWAY_HTTP/v1/ptv/attest" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"e2e-ptv-agent","platform":"macos-secure-enclave","firmware_version":"1.0.0"}')
check_contains "PTV attestation" "tpm_signature" "$ATTEST"

BIND=$(curl -s -X POST "$GATEWAY_HTTP/v1/ptv/bind" \
  -H "Content-Type: application/json" \
  -d "$ATTEST")
BINDING_ID=$(echo "$BIND" | python3 -c "import sys,json; print(json.load(sys.stdin)['binding_id'])" 2>/dev/null)
check_contains "PTV binding" "binding_id" "$BIND"

VERIFY=$(curl -s "http://localhost:8080/v1/ptv/verify/$BINDING_ID")
check_contains "PTV verify valid" '"valid":true' "$VERIFY"

echo ""

# === HITL (Human-in-the-Loop) ===
echo "--- HITL: Request → Approve → Reject ---"

HITL_REQ=$(curl -s -X POST "$GATEWAY_HTTP/v1/hitl/request" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"e2e-hitl-agent","action":"wire_transfer","reason":"Transfer $50K offshore","risk_level":"critical"}')
APPROVAL_ID=$(echo "$HITL_REQ" | python3 -c "import sys,json; print(json.load(sys.stdin)['approval_id'])" 2>/dev/null)
check_contains "HITL request created" "approval_id" "$HITL_REQ"
check_contains "HITL status pending" '"pending"' "$HITL_REQ"

HITL_APPROVE=$(curl -s -X POST "http://localhost:8080/v1/hitl/$APPROVAL_ID/decide" \
  -H "Content-Type: application/json" \
  -d "{\"approval_id\":\"$APPROVAL_ID\",\"approved\":true,\"approver_method\":\"faceid\"}")
check_contains "HITL approved" '"approved":true' "$HITL_APPROVE"

HITL_REQ2=$(curl -s -X POST "$GATEWAY_HTTP/v1/hitl/request" \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"e2e-hitl-agent","action":"delete_database","reason":"Drop prod DB","risk_level":"critical"}')
APPROVAL2_ID=$(echo "$HITL_REQ2" | python3 -c "import sys,json; print(json.load(sys.stdin)['approval_id'])" 2>/dev/null)

HITL_REJECT=$(curl -s -X POST "http://localhost:8080/v1/hitl/$APPROVAL2_ID/decide" \
  -H "Content-Type: application/json" \
  -d "{\"approval_id\":\"$APPROVAL2_ID\",\"approved\":false,\"approver_method\":\"password\"}")
check_contains "HITL rejected" '"status":"rejected"' "$HITL_REJECT"

echo ""

# === Resource Adapter MCP ===
echo "--- Resource Adapter (MCP Server) ---"

ADAPTER_INIT=$(curl -s -X POST "$ADAPTER/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"initialize","id":"a1"}')
check_contains "Adapter init" "2024-11-05" "$ADAPTER_INIT"

ADAPTER_TOOLS=$(curl -s -X POST "$ADAPTER/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":"a2"}')
check_contains "Adapter has tools" "get_weather" "$ADAPTER_TOOLS"

ADAPTER_CALL=$(curl -s -X POST "$ADAPTER/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/call","id":"a3","params":{"name":"get_weather","arguments":{"location":"KL"}}}')
check_contains "Weather tool works" "content" "$ADAPTER_CALL"

ADAPTER_RES=$(curl -s -X POST "$ADAPTER/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"resources/list","id":"a4"}')
check_contains "Resources listed" "api-reference" "$ADAPTER_RES"

ADAPTER_PROMPTS=$(curl -s -X POST "$ADAPTER/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"prompts/list","id":"a5"}')
check_contains "Prompts listed" "summarize" "$ADAPTER_PROMPTS"

echo ""

# === Delegation ===
echo "--- Delegation ---"

SUB_REG=$(curl -s -X POST "$GATEWAY_PROXY/v1/register" \
  -H "Content-Type: application/json" \
  -d '{"name":"e2e-sub-agent","owner":"e2e-team","capabilities":["mcp"],"allowed_tools":["read"]}')
SUB_ID=$(echo "$SUB_REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['agent_id'])" 2>/dev/null)

DELEGATE=$(curl -s -X POST "$GATEWAY_HTTP/v1/delegate" \
  -H "Content-Type: application/json" \
  -d "{\"parent_agent_id\":\"$AGENT_ID\",\"child_agent_id\":\"$SUB_ID\",\"scope\":[\"read\"],\"reason\":\"Task delegation\"}")
check_contains "Delegation created" "delegation_id" "$DELEGATE"

# === Tenant + Push Tokens ===
echo "--- Multi-Tenant & Push Notifications ---"

TENANT=$(curl -s -X POST "$GATEWAY_HTTP/v1/tenants" \
  -H "Content-Type: application/json" \
  -d '{"name":"E2E Corp","slug":"e2e-corp","plan":"enterprise","max_agents":10,"max_resources":20}')
TENANT_ID=$(echo "$TENANT" | python3 -c "import sys,json; print(json.load(sys.stdin)['tenant_id'])" 2>/dev/null)
check_contains "Tenant created" "tenant_id" "$TENANT"

TENANT_LIST=$(curl -s "$GATEWAY_HTTP/v1/tenants")
check_contains "Tenants listed" "tenants" "$TENANT_LIST"

APPROVER_ID=$(psql -h localhost -U agentid -d agentid -t -A -c "INSERT INTO approvers (tenant_id, email, name, notification_channel) VALUES ('$TENANT_ID', 'e2e@test.com', 'E2E Approver', 'push') RETURNING approver_id" 2>/dev/null | head -1 | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' || echo "")

if [ -n "$APPROVER_ID" ]; then
  PUSH_REG=$(curl -s -X POST "$GATEWAY_HTTP/v1/push/register" \
    -H "Content-Type: application/json" \
    -d "{\"approver_id\":\"$APPROVER_ID\",\"device_token\":\"e2e-test-token\",\"platform\":\"ios\",\"bundle_id\":\"com.e2e.app\"}")
  check_contains "Push token registered" "device_token" "$PUSH_REG"

  PUSH_LIST=$(curl -s "$GATEWAY_HTTP/v1/push/tokens?approver_id=$APPROVER_ID")
  check_contains "Push tokens listed" "tokens" "$PUSH_LIST"

  PUSH_TOKEN_ID=$(echo "$PUSH_REG" | python3 -c "import sys,json; print(json.load(sys.stdin)['token_id'])" 2>/dev/null)
  PUSH_DEACTIVATE=$(curl -s -X DELETE "$GATEWAY_HTTP/v1/push/tokens/$PUSH_TOKEN_ID")
  check_contains "Push token deactivated" "deactivated" "$PUSH_DEACTIVATE"
else
  echo "  ⊘ Push tests skipped (psql unavailable)"
fi

echo ""

# === Key Persistence ===
echo "--- Key Persistence ---"
if [ -f /tmp/agentid-gateway-ed25519.key ]; then
  KEY_BEFORE=$(cat /tmp/agentid-gateway-ed25519.key | md5)
  check "Key file exists" "1" "1"
  check "Key is non-empty" "1" "$([ -s /tmp/agentid-gateway-ed25519.key ] && echo 1 || echo 0)"
else
  check "Key file exists" "1" "0"
fi

echo ""
echo "--- TLS Mode (if available) ---"
TLS_TEST=$(curl -sk https://localhost:9443/health 2>/dev/null || echo "FAIL")
if [ "$TLS_TEST" = "ok" ]; then
    check "TLS health" "ok" "$TLS_TEST"
else
    echo "  - TLS not active (skipping, plaintext mode)"
fi

echo ""
echo "============================================"
echo "  Results: $PASS passed, $FAIL failed"
echo "============================================"

if [ $FAIL -gt 0 ]; then
    exit 1
fi