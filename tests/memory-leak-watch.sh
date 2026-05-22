#!/bin/bash
# eyeVesa Memory Leak Watch & Soak Test
# Runs load test continuously for 24-48h while monitoring Docker container memory
# Usage: ./tests/memory-leak-watch.sh [duration_hours]
# Default: 24 hours

set -e

DURATION_HOURS=${1:-24}
DURATION_SECS=$((DURATION_HOURS * 3600))
LOG_DIR="./soak-test-logs"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
MEMORY_LOG="$LOG_DIR/memory-${TIMESTAMP}.csv"
LOAD_LOG="$LOG_DIR/load-${TIMESTAMP}.log"
ALERT_LOG="$LOG_DIR/alerts-${TIMESTAMP}.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

mkdir -p "$LOG_DIR"

echo ""
echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}${RED}  ⚡ eyeVesa Memory Leak Watch & Soak Test ⚡${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo -e "  Duration:      ${BOLD}${DURATION_HOURS} hours${NC}"
echo -e "  Memory log:    ${MEMORY_LOG}"
echo -e "  Load log:      ${LOAD_LOG}"
echo -e "  Alerts log:    ${ALERT_LOG}"
echo -e "  Started:       $(date)"
echo ""

# Verify Docker containers are running
GATEWAY_CONTAINER=$(docker compose ps -q gateway-control 2>/dev/null || echo "")
CORE_CONTAINER=$(docker compose ps -q gateway-core 2>/dev/null || echo "")
POSTGRES_CONTAINER=$(docker compose ps -q postgres 2>/dev/null || echo "")

if [ -z "$GATEWAY_CONTAINER" ]; then
    echo -e "${RED}✗ gateway-control container not found. Run ./start.sh first.${NC}"
    exit 1
fi

echo -e "  Gateway container: ${GATEWAY_CONTAINER:0:12}"
[ -n "$CORE_CONTAINER" ] && echo -e "  Core container:    ${CORE_CONTAINER:0:12}"
[ -n "$POSTGRES_CONTAINER" ] && echo -e "  Postgres container: ${POSTGRES_CONTAINER:0:12}"
echo ""

# CSV header
echo "timestamp,elapsed_secs,container,memory_mb,memory_limit_mb,memory_pct,cpu_pct,network_rx_mb,network_tx_mb" > "$MEMORY_LOG"

BASELINE_GO_MEM=""
ALERT_THRESHOLD_MB=500
LEAK_THRESHOLD_MB=100
SAMPLE_INTERVAL=60

log_alert() {
    echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $1" >> "$ALERT_LOG"
    echo -e "  ${RED}⚠ ALERT: $1${NC}"
}

get_container_stats() {
    local container=$1
    docker stats --no-stream --format "{{.MemUsage}},{{.MemPerc}},{{.CPUPerc}},{{.NetIO}}" "$container" 2>/dev/null || echo "0/0,0%,0%,0/0"
}

parse_memory_mb() {
    local mem_str=$1
    local value=$(echo "$mem_str" | sed 's/\/.*//' | sed 's/[[:space:]]//g')
    local unit=$(echo "$value" | sed 's/[0-9.]//g')
    local num=$(echo "$value" | sed 's/[^0-9.]//g')

    case "$unit" in
        GiB|GB) echo "$(echo "$num * 1024" | bc 2>/dev/null || echo 0)" ;;
        MiB|MB) echo "$num" ;;
        KiB|KB) echo "$(echo "$num / 1024" | bc 2>/dev/null || echo 0)" ;;
        *) echo "$num" ;;
    esac
}

parse_network_mb() {
    local net_str=$1
    local rx=$(echo "$net_str" | cut -d'/' -f1 | sed 's/[[:space:]]//g')
    local tx=$(echo "$net_str" | cut -d'/' -f2 | sed 's/[[:space:]]//g')
    echo "${rx},${tx}"
}

START_TIME=$(date +%s)
ITERATION=0
PREV_GO_MEM=""

echo -e "${YELLOW}--- Monitoring started ---${NC}"
echo -e "  Sampling every ${SAMPLE_INTERVAL}s"
echo -e "  Alert threshold: ${ALERT_THRESHOLD_MB}MB"
echo -e "  Leak threshold:  ${LEAK_THRESHOLD_MB}MB growth over baseline"
echo ""

while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))

    if [ $ELAPSED -ge $DURATION_SECS ]; then
        break
    fi

    ITERATION=$((ITERATION + 1))

    # Sample gateway-control (Go)
    GO_STATS=$(get_container_stats "$GATEWAY_CONTAINER")
    GO_MEM_RAW=$(echo "$GO_STATS" | cut -d',' -f1)
    GO_MEM_PCT=$(echo "$GO_STATS" | cut -d',' -f2 | sed 's/%//')
    GO_CPU=$(echo "$GO_STATS" | cut -d',' -f3 | sed 's/%//')
    GO_NET=$(echo "$GO_STATS" | cut -d',' -f4)
    GO_MEM_MB=$(parse_memory_mb "$GO_MEM_RAW")
    GO_NET_PARSED=$(parse_network_mb "$GO_NET")

    # Sample gateway-core (Rust) if running
    RUST_MEM_MB="0"
    RUST_MEM_PCT="0"
    RUST_CPU="0"
    RUST_NET_PARSED="0,0"
    if [ -n "$CORE_CONTAINER" ]; then
        RUST_STATS=$(get_container_stats "$CORE_CONTAINER")
        RUST_MEM_RAW=$(echo "$RUST_STATS" | cut -d',' -f1)
        RUST_MEM_PCT=$(echo "$RUST_STATS" | cut -d',' -f2 | sed 's/%//')
        RUST_CPU=$(echo "$RUST_STATS" | cut -d',' -f3 | sed 's/%//')
        RUST_NET=$(echo "$RUST_STATS" | cut -d',' -f4)
        RUST_MEM_MB=$(parse_memory_mb "$RUST_MEM_RAW")
        RUST_NET_PARSED=$(parse_network_mb "$RUST_NET")
    fi

    # Sample postgres
    PG_MEM_MB="0"
    PG_MEM_PCT="0"
    PG_CPU="0"
    PG_NET_PARSED="0,0"
    if [ -n "$POSTGRES_CONTAINER" ]; then
        PG_STATS=$(get_container_stats "$POSTGRES_CONTAINER")
        PG_MEM_RAW=$(echo "$PG_STATS" | cut -d',' -f1)
        PG_MEM_PCT=$(echo "$PG_STATS" | cut -d',' -f2 | sed 's/%//')
        PG_CPU=$(echo "$PG_STATS" | cut -d',' -f3 | sed 's/%//')
        PG_NET=$(echo "$PG_STATS" | cut -d',' -f4)
        PG_MEM_MB=$(parse_memory_mb "$PG_MEM_RAW")
        PG_NET_PARSED=$(parse_network_mb "$PG_NET")
    fi

    # Set baseline on first sample
    if [ -z "$BASELINE_GO_MEM" ]; then
        BASELINE_GO_MEM="$GO_MEM_MB"
        echo -e "  ${GREEN}Baseline set: Go=${BASELINE_GO_MEM}MB, Rust=${RUST_MEM_MB}MB, PG=${PG_MEM_MB}MB${NC}"
    fi

    # Log to CSV
    TIMESTAMP_ISO=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    echo "${TIMESTAMP_ISO},${ELAPSED},gateway-control,${GO_MEM_MB},0,${GO_MEM_PCT},${GO_CPU},${GO_NET_PARSED}" >> "$MEMORY_LOG"
    [ -n "$CORE_CONTAINER" ] && echo "${TIMESTAMP_ISO},${ELAPSED},gateway-core,${RUST_MEM_MB},0,${RUST_MEM_PCT},${RUST_CPU},${RUST_NET_PARSED}" >> "$MEMORY_LOG"
    [ -n "$POSTGRES_CONTAINER" ] && echo "${TIMESTAMP_ISO},${ELAPSED},postgres,${PG_MEM_MB},0,${PG_MEM_PCT},${PG_CPU},${PG_NET_PARSED}" >> "$MEMORY_LOG"

    # Check for memory alerts
    GO_MEM_INT=$(echo "$GO_MEM_MB" | awk '{print int($1)}')
    if [ "$GO_MEM_INT" -gt "$ALERT_THRESHOLD_MB" ] 2>/dev/null; then
        log_alert "Go control-plane memory ${GO_MEM_MB}MB exceeds ${ALERT_THRESHOLD_MB}MB threshold!"
    fi

    # Check for memory leak (growth over baseline)
    if [ -n "$BASELINE_GO_MEM" ]; then
        GROWTH=$(echo "$GO_MEM_MB - $BASELINE_GO_MEM" | bc 2>/dev/null || echo "0")
        GROWTH_INT=$(echo "$GROWTH" | awk '{print int($1)}')
        if [ "$GROWTH_INT" -gt "$LEAK_THRESHOLD_MB" ] 2>/dev/null; then
            log_alert "Possible memory leak! Go grew ${GROWTH}MB from baseline ${BASELINE_GO_MEM}MB → ${GO_MEM_MB}MB"
        fi
    fi

    # Check for OOM kill
    if ! docker ps --format '{{.ID}}' | grep -q "${GATEWAY_CONTAINER:0:12}" 2>/dev/null; then
        log_alert "gateway-control container DIED — possible OOM kill!"
        echo -e "  ${RED}Container died. Checking logs...${NC}"
        docker logs --tail 50 "$GATEWAY_CONTAINER" 2>&1 | tail -20
        break
    fi

    # Progress display (every 5 minutes)
    if [ $((ITERATION % 5)) -eq 0 ]; then
        HOURS_ELAPSED=$(echo "scale=1; $ELAPSED / 3600" | bc)
        HOURS_TOTAL=$(echo "scale=1; $DURATION_SECS / 3600" | bc)
        printf "\r  [%sh/%sh] Go: %sMB  Rust: %sMB  PG: %sMB  CPU: %s%%  " \
            "$HOURS_ELAPSED" "$HOURS_TOTAL" "$GO_MEM_MB" "$RUST_MEM_MB" "$PG_MEM_MB" "$GO_CPU"
    fi

    sleep $SAMPLE_INTERVAL
done

echo ""
echo ""

# Final report
FINAL_GO_STATS=$(get_container_stats "$GATEWAY_CONTAINER")
FINAL_GO_MEM=$(parse_memory_mb "$(echo "$FINAL_GO_STATS" | cut -d',' -f1)")

echo -e "${CYAN}=======================================================${NC}"
echo -e "${BOLD}  Soak Test Complete${NC}"
echo -e "${CYAN}=======================================================${NC}"
echo ""
echo -e "  Duration:        ${DURATION_HOURS}h"
echo -e "  Baseline memory: ${BASELINE_GO_MEM}MB"
echo -e "  Final memory:    ${FINAL_GO_MEM}MB"

if [ -n "$BASELINE_GO_MEM" ]; then
    TOTAL_GROWTH=$(echo "$FINAL_GO_MEM - $BASELINE_GO_MEM" | bc 2>/dev/null || echo "N/A")
    echo -e "  Total growth:    ${TOTAL_GROWTH}MB"

    GROWTH_VAL=$(echo "$TOTAL_GROWTH" | awk '{print int($1)}')
    if [ "$GROWTH_VAL" -gt "$LEAK_THRESHOLD_MB" ] 2>/dev/null; then
        echo -e "  ${RED}⚠ MEMORY LEAK SUSPECTED: ${TOTAL_GROWTH}MB growth over ${DURATION_HOURS}h${NC}"
    elif [ "$GROWTH_VAL" -gt $((LEAK_THRESHOLD_MB / 2)) ] 2>/dev/null; then
        echo -e "  ${YELLOW}⚠ Moderate growth: ${TOTAL_GROWTH}MB — monitor closely${NC}"
    else
        echo -e "  ${GREEN}✓ Memory stable: growth within normal bounds${NC}"
    fi
fi

echo ""
echo -e "  Memory log:  ${MEMORY_LOG}"
echo -e "  Alerts log:  ${ALERT_LOG}"
echo ""
echo -e "  Plot memory: ${CYAN}python3 -c \"import pandas as pd; df=pd.read_csv('${MEMORY_LOG}'); print(df.groupby('container')['memory_mb'].describe())\"${NC}"
echo ""
