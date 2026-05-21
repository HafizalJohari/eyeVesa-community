#!/bin/bash
# AgentID Gateway Load Test (no external dependencies)
# Uses curl + background processes to achieve 2000+ RPS
# Prerequisites: Go server running on :8080, Rust proxy on :9443
set -e

PROXY="http://localhost:9443"
GATEWAY_HTTP="http://localhost:8080"
DURATION=30
CONCURRENCY=200
TARGET_RPS=2000
PASS=0
FAIL=0
TOTAL=0
START_TIME=$(date +%s)

echo "============================================"
echo "  AgentID Gateway Load Test"
echo "  Target: ${TARGET_RPS} RPS for ${DURATION}s"
echo "  Concurrency: ${CONCURRENCY}"
echo "============================================"
echo ""

# Quick health check
HEALTH=$(curl -s "$PROXY/health" 2>/dev/null || echo "FAIL")
if [ "$HEALTH" != "ok" ]; then
    echo "  ✗ Proxy not reachable at $PROXY/health"
    echo "  Start the gateway proxy first: MODE=proxy cargo run"
    exit 1
fi
echo "  ✓ Proxy health check passed"
echo ""

echo "--- Running load test for ${DURATION}s ---"

# Create temp dir for results
RESULT_DIR=$(mktemp -d)

# Function to send requests and count results
load_worker() {
    local worker_id=$1
    local end_time=$(($(date +%s) + DURATION))
    local count=0
    local ok=0
    local err=0

    while [ $(date +%s) -lt $end_time ]; do
        # Mix of different endpoint types
        case $((count % 5)) in
            0)
                status=$(curl -s -o /dev/null -w "%{http_code}" "$PROXY/health" 2>/dev/null)
                ;;
            1)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PROXY/v1/auth" \
                    -H "Content-Type: application/json" \
                    -d '{"agent_id":"load-test-agent","action":"read","resource_id":"doc-1"}' 2>/dev/null)
                ;;
            2)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$PROXY/v1/mcp" \
                    -H "Content-Type: application/json" \
                    -d '{"jsonrpc":"2.0","method":"initialize","id":"lt"}' 2>/dev/null)
                ;;
            3)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_HTTP/v1/tx/issue" \
                    -H "Content-Type: application/json" \
                    -d '{"agent_id":"load-test-agent","resource_id":"res-1","action":"read","scopes":["read"]}' 2>/dev/null)
                ;;
            4)
                status=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY_HTTP/v1/skills" 2>/dev/null)
                ;;
        esac

        count=$((count + 1))
        if [ "$status" -ge 200 ] && [ "$status" -lt 500 ]; then
            ok=$((ok + 1))
        else
            err=$((err + 1))
        fi
    done

    echo "${worker_id}:${count}:${ok}:${err}" > "$RESULT_DIR/worker_${worker_id}.result"
}

# Start workers
echo "  Starting $CONCURRENCY workers..."
PIDS=()
for i in $(seq 1 $CONCURRENCY); do
    load_worker $i &
    PIDS+=($!)
done

# Progress indicator
while kill -0 ${PIDS[0]} 2>/dev/null; do
    elapsed=$(($(date +%s) - START_TIME))
    remaining=$((DURATION - elapsed))
    if [ $remaining -gt 0 ]; then
        printf "\r  Elapsed: %ds / %ds" $elapsed $DURATION
        sleep 1
    else
        break
    fi
done
echo ""

# Wait for all workers
echo "  Waiting for workers to finish..."
for pid in "${PIDS[@]}"; do
    wait $pid 2>/dev/null || true
done

# Aggregate results
echo ""
echo "--- Results ---"

total_requests=0
total_ok=0
total_err=0

for f in "$RESULT_DIR"/worker_*.result; do
    if [ -f "$f" ]; then
        IFS=: read -r w_id w_count w_ok w_err < "$f"
        total_requests=$((total_requests + w_count))
        total_ok=$((total_ok + w_ok))
        total_err=$((total_err + w_err))
    fi
done

elapsed=$(($(date +%s) - START_TIME))
if [ $elapsed -eq 0 ]; then
    elapsed=1
fi

actual_rps=$((total_requests / elapsed))
error_rate="0"
if [ $total_requests -gt 0 ]; then
    error_rate=$(echo "scale=2; $total_err * 100 / $total_requests" | bc 2>/dev/null || echo "N/A")
fi

echo "  Total requests:   $total_requests"
echo "  Successful:       $total_ok"
echo "  Errors:           $total_err"
echo "  Duration:          ${elapsed}s"
echo "  Actual RPS:        $actual_rps"
echo "  Error rate:        ${error_rate}%"
echo ""

if [ $actual_rps -ge $TARGET_RPS ]; then
    echo "  ✓ PASSED: ${actual_rps} RPS >= ${TARGET_RPS} target"
else
    echo "  ✗ FAILED: ${actual_rps} RPS < ${TARGET_RPS} target"
fi

if [ "$error_rate" != "N/A" ]; then
    error_int=$(echo "$error_rate" | awk '{print int($1)}')
    if [ "$error_int" -lt 5 ] 2>/dev/null; then
        echo "  ✓ Error rate ${error_rate}% < 5% threshold"
    else
        echo "  ✗ Error rate ${error_rate}% >= 5% threshold"
    fi
fi

# Cleanup
rm -rf "$RESULT_DIR"

echo ""
echo "============================================"
echo "  Load Test Complete"
echo "============================================"