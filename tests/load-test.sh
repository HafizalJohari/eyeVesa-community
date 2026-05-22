#!/bin/bash
# eyeVesa Gateway Load & Stress Test
# Uses curl + background processes to achieve 2000+ RPS
# Prerequisites: Go server running on :8080, Rust proxy on :9443
set -e

PROXY="http://localhost:9443"
GATEWAY_HTTP="http://localhost:8080"
DURATION=${DURATION:-30}
CONCURRENCY=${CONCURRENCY:-200}
TARGET_RPS=${TARGET_RPS:-2000}
AGENT_ID=${AGENT_ID:-"66a62aaa-53a8-472c-8973-f3ec15ae8b32"}
PASS=0
FAIL=0
TOTAL=0
START_TIME=$(date +%s)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo ""
echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}${RED}  ⚡ eyeVesa Gateway Load & Stress Test ⚡${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo -e "  Target:      ${BOLD}${TARGET_RPS} RPS${NC} for ${DURATION}s"
echo -e "  Concurrency: ${BOLD}${CONCURRENCY} workers${NC}"
echo -e "  Proxy:       ${PROXY}"
echo -e "  Control:     ${GATEWAY_HTTP}"
echo -e "  Agent:       ${AGENT_ID}"
echo ""

PROXY_OK=false
CTRL_OK=false

HEALTH=$(curl -s "$PROXY/health" 2>/dev/null || echo "FAIL")
if [ "$HEALTH" = "ok" ]; then
    echo -e "  ${GREEN}✓${NC} Rust proxy healthy at $PROXY"
    PROXY_OK=true
else
    echo -e "  ${YELLOW}⚠${NC} Rust proxy not reachable at $PROXY — proxy tests skipped"
fi

CHEALTH=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY_HTTP/health" 2>/dev/null || echo "000")
if [ "$CHEALTH" = "200" ]; then
    echo -e "  ${GREEN}✓${NC} Go control-plane healthy at $GATEWAY_HTTP"
    CTRL_OK=true
else
    echo -e "  ${RED}✗${NC} Go control-plane not reachable at $GATEWAY_HTTP"
    echo -e "  Run: ${CYAN}./start.sh${NC}"
    exit 1
fi
echo ""

RESULT_DIR=$(mktemp -d)

load_worker() {
    local worker_id=$1
    local target=$2
    local end_time=$(($(date +%s) + DURATION))
    local count=0
    local ok=0
    local err=0
    local total_time=0

    while [ $(date +%s) -lt $end_time ]; do
        local req_start=$(python3 -c 'import time; print(int(time.time()*1000))' 2>/dev/null || echo "0")
        local status="000"

        case $((count % 6)) in
            0)
                status=$(curl -s -o /dev/null -w "%{http_code}" "$target/health" 2>/dev/null)
                ;;
            1)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$target/v1/auth" \
                    -H "Content-Type: application/json" \
                    -d "{\"agent_id\":\"$AGENT_ID\",\"action\":\"read\",\"resource_id\":\"doc-load-test\"}" 2>/dev/null)
                ;;
            2)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$target/v1/mcp" \
                    -H "Content-Type: application/json" \
                    -d '{"jsonrpc":"2.0","method":"initialize","id":"lt"}' 2>/dev/null)
                ;;
            3)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_HTTP/v1/tx/issue" \
                    -H "Content-Type: application/json" \
                    -d "{\"agent_id\":\"$AGENT_ID\",\"resource_id\":\"res-1\",\"action\":\"read\",\"scopes\":[\"read\"]}" 2>/dev/null)
                ;;
            4)
                status=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY_HTTP/v1/skills" 2>/dev/null)
                ;;
            5)
                status=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$GATEWAY_HTTP/v1/authorize" \
                    -H "Content-Type: application/json" \
                    -d "{\"agent_id\":\"$AGENT_ID\",\"action\":\"search\",\"params\":{}}" 2>/dev/null)
                ;;
        esac

        count=$((count + 1))
        if [ "$status" -ge 200 ] 2>/dev/null && [ "$status" -lt 500 ] 2>/dev/null; then
            ok=$((ok + 1))
        else
            err=$((err + 1))
        fi
    done

    echo "${worker_id}:${count}:${ok}:${err}" > "$RESULT_DIR/worker_${worker_id}.result"
}

if [ "$PROXY_OK" = true ]; then
    echo -e "${YELLOW}--- Phase 1: Rust Proxy Load Test (${DURATION}s) ---${NC}"
    PIDS=()
    for i in $(seq 1 $CONCURRENCY); do
        load_worker $i "$PROXY" &
        PIDS+=($!)
    done

    while kill -0 ${PIDS[0]} 2>/dev/null; do
        elapsed=$(($(date +%s) - START_TIME))
        remaining=$((DURATION - elapsed))
        if [ $remaining -gt 0 ]; then
            printf "\r  Elapsed: %ds / %ds  " $elapsed $DURATION
            sleep 1
        else
            break
        fi
    done
    echo ""

    for pid in "${PIDS[@]}"; do
        wait $pid 2>/dev/null || true
    done

    total_requests=0; total_ok=0; total_err=0
    for f in "$RESULT_DIR"/worker_*.result; do
        if [ -f "$f" ]; then
            IFS=: read -r w_id w_count w_ok w_err < "$f"
            total_requests=$((total_requests + w_count))
            total_ok=$((total_ok + w_ok))
            total_err=$((total_err + w_err))
        fi
    done
    rm -f "$RESULT_DIR"/worker_*.result

    elapsed=$(($(date +%s) - START_TIME))
    [ $elapsed -eq 0 ] && elapsed=1
    actual_rps=$((total_requests / elapsed))
    error_rate=$(echo "scale=2; $total_err * 100 / $total_requests" | bc 2>/dev/null || echo "N/A")

    echo ""
    echo -e "  ${CYAN}Proxy Results:${NC}"
    echo "  Total requests:   $total_requests"
    echo "  Successful:       $total_ok"
    echo "  Errors:           $total_err"
    echo "  Duration:         ${elapsed}s"
    echo "  Actual RPS:       $actual_rps"
    echo "  Error rate:       ${error_rate}%"

    if [ $actual_rps -ge $TARGET_RPS ]; then
        echo -e "  ${GREEN}✓${NC} ${actual_rps} RPS >= ${TARGET_RPS} target"
    else
        echo -e "  ${YELLOW}⚠${NC} ${actual_rps} RPS < ${TARGET_RPS} target (proxy overhead may limit throughput)"
    fi
    echo ""
fi

if [ "$CTRL_OK" = true ]; then
    echo -e "${YELLOW}--- Phase 2: Go Control-Plane Direct Stress Test (${DURATION}s) ---${NC}"
    CTRL_START=$(date +%s)
    PIDS=()
    HALF_CONC=$((CONCURRENCY / 2))
    [ $HALF_CONC -lt 10 ] && HALF_CONC=10

    for i in $(seq 1 $HALF_CONC); do
        load_worker $((i + 1000)) "$GATEWAY_HTTP" &
        PIDS+=($!)
    done

    while kill -0 ${PIDS[0]} 2>/dev/null; do
        elapsed=$(($(date +%s) - CTRL_START))
        remaining=$((DURATION - elapsed))
        if [ $remaining -gt 0 ]; then
            printf "\r  Elapsed: %ds / %ds  " $elapsed $DURATION
            sleep 1
        else
            break
        fi
    done
    echo ""

    for pid in "${PIDS[@]}"; do
        wait $pid 2>/dev/null || true
    done

    total_requests=0; total_ok=0; total_err=0
    for f in "$RESULT_DIR"/worker_*.result; do
        if [ -f "$f" ]; then
            IFS=: read -r w_id w_count w_ok w_err < "$f"
            total_requests=$((total_requests + w_count))
            total_ok=$((total_ok + w_ok))
            total_err=$((total_err + w_err))
        fi
    done

    elapsed=$(($(date +%s) - CTRL_START))
    [ $elapsed -eq 0 ] && elapsed=1
    actual_rps=$((total_requests / elapsed))
    error_rate=$(echo "scale=2; $total_err * 100 / $total_requests" | bc 2>/dev/null || echo "N/A")

    echo ""
    echo -e "  ${CYAN}Control-Plane Results:${NC}"
    echo "  Total requests:   $total_requests"
    echo "  Successful:       $total_ok"
    echo "  Errors:           $total_err"
    echo "  Duration:         ${elapsed}s"
    echo "  Actual RPS:       $actual_rps"
    echo "  Error rate:       ${error_rate}%"

    if [ "$error_rate" != "N/A" ]; then
        error_int=$(echo "$error_rate" | awk '{print int($1)}')
        if [ "$error_int" -lt 5 ] 2>/dev/null; then
            echo -e "  ${GREEN}✓${NC} Error rate ${error_rate}% < 5% threshold"
        else
            echo -e "  ${RED}✗${NC} Error rate ${error_rate}% >= 5% threshold"
        fi
    fi
fi

rm -rf "$RESULT_DIR"

echo ""
echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}  Load Test Complete${NC}"
echo -e "${CYAN}=======================================================${NC}"
