# AGENTS.md

## Build Commands

```bash
# Go control plane
cd gateway/control-plane && go build ./...

# Rust core proxy
cd gateway/core && cargo build --release

# Go resource adapter
cd adapter/resource-adapter-go && go build ./cmd/...

# Rust agent SDK
cd sdk/agent-sdk-rust && cargo build
```

## Test Commands

```bash
# Go unit tests (all packages)
cd gateway/control-plane && go test ./internal/... -count=1

# Rust unit tests
cd gateway/core && cargo test

# Go OPA policy tests only
cd gateway/control-plane && go test ./internal/policy/... -v

# E2E test suite (requires all services running)
bash tests/e2e-test.sh
```

## Lint Commands

```bash
# Go vet
cd gateway/control-plane && go vet ./...

# Rust clippy
cd gateway/core && cargo clippy -- -D warnings
```

## Services

| Service | Port | Start |
|---|---|---|
| Go control plane (HTTP) | 8080 | `cd gateway/control-plane && go run cmd/api/main.go` |
| Go control plane (gRPC) | 9090 | (same process) |
| Rust core proxy | 9443 | `cd gateway/core && cargo run --release` |
| Resource adapter | 8443 | `cd adapter/resource-adapter-go && go run ./cmd/` |
| PostgreSQL | 5432 | `docker-compose up -d postgres` |
| OPA | 8181 | `docker-compose up -d opa` |

## Environment

- Database: `agentid` on `localhost:5432`, user `agentid`, password `agentid_dev`
- `GATEWAY_MODE=plaintext` (default), `tls`, or `mtls`
- `AUTH_ENABLED=true` enables JWT/API key auth middleware
- Run migrations: `psql -h localhost -U agentid -d agentid -f registry/migrations/NNN_*.sql`

## Key Architecture

- Rust proxy handles MCP, registration, authorization (fast path via gRPC)
- All other `/v1/*` routes are reverse-proxied to Go HTTP API
- Go control plane owns: OPA policy, HITL, PTV, delegation, audit, push notifications
- SPIRE: `go-spiffe/v2` Workload API client for X.509 SVID; falls back to LocalProvider if unavailable
- Keys are persisted: Ed25519 at `GATEWAY_KEY_PATH`, PTV ECDSA at `PTV_KEY_PATH`