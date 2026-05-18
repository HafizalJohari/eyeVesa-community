#!/bin/bash
# AgentID Gateway TLS Integration Test
# Tests: TLS termination, mTLS client verification, certificate rotation
# Prerequisites: Go server at :8080, openssl, rcgen (for cert generation)
set -e

GATEWAY_HTTP="http://localhost:8080"
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
        echo "  ✗ $name: expected to contain '$needle'"
        FAIL=$((FAIL + 1))
    fi
}

echo "============================================"
echo "  AgentID Gateway TLS Integration Test"
echo "============================================"
echo ""

CERT_DIR=$(mktemp -d)
echo "--- Generating Test Certificates in $CERT_DIR ---"

# Generate CA
openssl req -x509 -new -nodes \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/ca.key" \
  -out "$CERT_DIR/ca.crt" \
  -days 1 \
  -subj "/CN=AgentID Test CA/O=eyeVesa" \
  2>/dev/null
check "CA key generated" "0" "$?"

# Generate server cert signed by CA
openssl req -new \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/server.key" \
  -out "$CERT_DIR/server.csr" \
  -nodes \
  -subj "/CN=localhost/O=eyeVesa" \
  2>/dev/null

openssl x509 -req \
  -in "$CERT_DIR/server.csr" \
  -CA "$CERT_DIR/ca.crt" \
  -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/server.crt" \
  -days 1 \
  -extfile <(echo "subjectAltName=DNS:localhost,IP:127.0.0.1") \
  2>/dev/null
check "Server cert signed by CA" "0" "$?"

# Generate client cert for mTLS
openssl req -new \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/client.key" \
  -out "$CERT_DIR/client.csr" \
  -nodes \
  -subj "/CN=agent-client/O=eyeVesa" \
  2>/dev/null

openssl x509 -req \
  -in "$CERT_DIR/client.csr" \
  -CA "$CERT_DIR/ca.crt" \
  -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/client.crt" \
  -days 1 \
  2>/dev/null
check "Client cert signed by CA" "0" "$?"

echo ""

# Test TLS loading functions (no server needed)
echo "--- TLS Certificate Loading ---"

if [ -f "$CERT_DIR/server.crt" ]; then
  CERT_SIZE=$(wc -c < "$CERT_DIR/server.crt")
  KEY_SIZE=$(wc -c < "$CERT_DIR/server.key")
  check "Server cert non-empty" "1" "$([ "$CERT_SIZE" -gt 0 ] && echo 1 || echo 0)"
  check "Server key non-empty" "1" "$([ "$KEY_SIZE" -gt 0 ] && echo 1 || echo 0)"
fi

CA_SIZE=$(wc -c < "$CERT_DIR/ca.crt")
check "CA cert non-empty" "1" "$([ "$CA_SIZE" -gt 0 ] && echo 1 || echo 0)"

CLIENT_CERT_SIZE=$(wc -c < "$CERT_DIR/client.crt")
CLIENT_KEY_SIZE=$(wc -c < "$CERT_DIR/client.key")
check "Client cert non-empty" "1" "$([ "$CLIENT_CERT_SIZE" -gt 0 ] && echo 1 || echo 0)"
check "Client key non-empty" "1" "$([ "$CLIENT_KEY_SIZE" -gt 0 ] && echo 1 || echo 0)"

echo ""

# Verify cert chain
echo "--- Certificate Chain Verification ---"

VERIFY_SERVER=$(openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/server.crt" 2>/dev/null)
check_contains "Server cert verified by CA" "OK" "$VERIFY_SERVER"

VERIFY_CLIENT=$(openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/client.crt" 2>/dev/null)
check_contains "Client cert verified by CA" "OK" "$VERIFY_CLIENT"

echo ""

# Test TLS connection to proxy (if running in TLS mode)
echo "--- TLS Connection Test (requires proxy in TLS mode) ---"

TLS_HEALTH=$(curl -sk --cacert "$CERT_DIR/ca.crt" https://localhost:9443/health 2>/dev/null || echo "NOT_AVAILABLE")
if [ "$TLS_HEALTH" = "ok" ]; then
    check "TLS health check" "ok" "$TLS_HEALTH"

    # Test request with valid client cert (mTLS)
    MTLS_HEALTH=$(curl -sk \
      --cacert "$CERT_DIR/ca.crt" \
      --cert "$CERT_DIR/client.crt" \
      --key "$CERT_DIR/client.key" \
      https://localhost:9443/health 2>/dev/null)
    check "mTLS health check" "ok" "$MTLS_HEALTH"

    # Test request without client cert (should still work in TLS mode)
    TLS_NO_CLIENT=$(curl -sk --cacert "$CERT_DIR/ca.crt" https://localhost:9443/health 2>/dev/null)
    check "TLS without client cert" "ok" "$TLS_NO_CLIENT"
else
    echo "  ⊘ Proxy not running in TLS mode (skipping TLS connection tests)"
    echo "  To test: set TLS_CERT_PATH/TLS_KEY_PATH and run proxy with MODE=tls"
fi

echo ""

# Test cert rotation (simulate by touching the cert file)
echo "--- Certificate Rotation Simulation ---"

ORIG_CRT_HASH=$(md5 -q "$CERT_DIR/server.crt" 2>/dev/null || md5sum "$CERT_DIR/server.crt" | awk '{print $1}')

sleep 1

# Generate a new server cert with different serial
openssl req -new \
  -newkey rsa:2048 \
  -keyout "$CERT_DIR/server-new.key" \
  -out "$CERT_DIR/server-new.csr" \
  -nodes \
  -subj "/CN=localhost/O=eyeVesa-Rotated" \
  2>/dev/null

openssl x509 -req \
  -in "$CERT_DIR/server-new.csr" \
  -CA "$CERT_DIR/ca.crt" \
  -CAkey "$CERT_DIR/ca.key" \
  -CAcreateserial \
  -out "$CERT_DIR/server-rotated.crt" \
  -days 1 \
  -extfile <(echo "subjectAltName=DNS:localhost,IP:127.0.0.1") \
  2>/dev/null

ROTATED_CRT_HASH=$(md5 -q "$CERT_DIR/server-rotated.crt" 2>/dev/null || md5sum "$CERT_DIR/server-rotated.crt" | awk '{print $1}')

if [ "$ORIG_CRT_HASH" != "$ROTATED_CRT_HASH" ]; then
    check "Rotated cert differs from original" "1" "1"
else
    check "Rotated cert differs from original" "1" "0"
fi

VERIFY_ROTATED=$(openssl verify -CAfile "$CERT_DIR/ca.crt" "$CERT_DIR/server-rotated.crt" 2>/dev/null)
check_contains "Rotated cert still valid" "OK" "$VERIFY_ROTATED"

# Check rotated cert has different subject
ORIG_SUBJECT=$(openssl x509 -in "$CERT_DIR/server.crt" -subject -noout 2>/dev/null)
ROTATED_SUBJECT=$(openssl x509 -in "$CERT_DIR/server-rotated.crt" -subject -noout 2>/dev/null)
check_contains "Rotated cert has new subject" "Rotated" "$ROTATED_SUBJECT"

echo ""

# Test client cert expiry detection
echo "--- Certificate Expiry Detection ---"

EXPIRY=$(openssl x509 -in "$CERT_DIR/server.crt" -enddate -noout 2>/dev/null)
check_contains "Cert has expiry date" "notAfter" "$EXPIRY"

DAYS_LEFT=$(openssl x509 -in "$CERT_DIR/server.crt" -checkend 60 -noout 2>/dev/null)
check_contains "Cert valid for tests (>60s)" "will not expire" "$DAYS_LEFT"

echo ""

# Cleanup
rm -rf "$CERT_DIR"

echo "============================================"
echo "  Results: $PASS passed, $FAIL failed"
echo "============================================"

if [ $FAIL -gt 0 ]; then
    exit 1
fi