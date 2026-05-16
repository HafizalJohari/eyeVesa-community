# AgentID Gateway

Identity and trust layer for the agentic economy. Connects AI agents to enterprise resources with cryptographic identity, policy enforcement, and audit trails.

## Architecture

```
Agent (sdk) ──mTLS──▶ Gateway ──mTLS──▶ Enterprise Resource (adapter)
                       │
          ┌────────────┴────────────┐
          │   Registry   Policy    │
          │   SPIRE      OPA       │
          │   Audit      HITL      │
          └───────────────────────┘
```

## Packages

| Package | Language | Purpose |
|---|---|---|
| `gateway/core` | Rust | Proxy engine, crypto, mTLS termination |
| `gateway/control-plane` | Go | APIs, orchestration, policy, audit |
| `sdk/agent-sdk-rust` | Rust | Client library for AI agents |
| `adapter/resource-adapter-go` | Go | MCP server wrapper for enterprise resources |
| `registry/` | SQL | PostgreSQL migrations with pgvector |
| `deploy/` | YAML | Docker, K8s, cloud configs |

## Quick Start

```bash
# Prerequisites
brew install go rust postgresql@16

# Start infrastructure
docker-compose up -d

# Run gateway core
cd gateway/core && cargo run

# Run control plane
cd gateway/control-plane && go run cmd/api/main.go

# Register an agent
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","owner":"org:test"}'

# Register a resource
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-mcp","type":"mcp_server","endpoint":"https://localhost:8443"}'
```

## Learning

See [LEARNING_ROADMAP.md](./LEARNING_ROADMAP.md) for a structured 12-week plan.

## License

Proprietary