# How to Use

Complete guide for setting up, configuring, and using the AgentID Gateway.

## Prerequisites

- Go 1.22+
- Rust 1.82+
- PostgreSQL 16+ with pgvector extension
- Docker & Docker Compose

## 1. Start Infrastructure

```bash
docker-compose up -d
```

This starts PostgreSQL (5432), SPIRE server (8081), SPIRE agent (8090), and OPA (8181). Migrations in `registry/migrations/` are auto-applied via the `postgres` volume mount.

## 2. Run Services

```bash
# Terminal 1 -- Control Plane (HTTP :8080, gRPC :9090)
cd gateway/control-plane && go run cmd/api/main.go

# Terminal 2 -- Gateway Core Proxy (HTTP :9443)
cd gateway/core && cargo run

# Terminal 3 -- Resource Adapter (optional, :8443)
cd adapter/resource-adapter-go && go run cmd/main.go
```

Environment variables can override defaults:

| Variable | Service | Default |
|----------|---------|---------|
| `DATABASE_URL` | core, control | `postgres://agentid:agentid_dev@localhost:5432/agentid` |
| `CONTROL_PLANE_ADDR` | core | `http://localhost:9090` |
| `RUST_LOG` | core | `info` |
| `OPA_ENDPOINT` | control | `http://localhost:8181` |
| `SPIRE_ENDPOINT` | control | `spire-agent:8090` |
| `RESOURCE_NAME` | adapter | `unnamed-resource` |
| `GATEWAY_ENDPOINT` | adapter | `localhost:9443` |

## 3. Register an Agent

```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-agent",
    "owner": "engineering-team",
    "capabilities": ["text-generation", "code-review"],
    "allowed_tools": ["github_api", "slack_webhook", "database_query"],
    "max_budget_usd": 100.00,
    "delegation_policy": "no_chain",
    "behavioral_tags": ["production", "high-reliability"]
  }'
```

**Required fields:** `name`, `owner`

**Response (201):**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "public_key": "<base64-encoded-ed25519-public-key>",
  "name": "my-agent",
  "owner": "engineering-team",
  "status": "active",
  "trust_score": 1.0,
  "created_at": "2026-05-16T10:00:00Z"
}
```

The server generates an Ed25519 keypair. The **public key** is stored and returned; the **private key** must be saved by the client for signing requests.

### Defaults Applied

| Field | Default |
|-------|---------|
| `capabilities` | `[]` |
| `allowed_tools` | `[]` |
| `behavioral_tags` | `[]` |
| `delegation_policy` | `"no_chain"` |
| `max_budget_usd` | `0.00` |
| `trust_score` | `1.0000` |
| `status` | `"active"` |

## 4. Register a Resource

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-database",
    "type": "database",
    "endpoint": "https://db.internal:8443/mcp",
    "auth_method": "mTLS+SVID",
    "capabilities": {"tools": ["query", "insert", "delete"], "resources": ["schema"]},
    "risk_level": "high",
    "data_sensitivity": "confidential",
    "rate_limit_per_agent": 50
  }'
```

**Required fields:** `name`, `endpoint`

**Response (201):**
```json
{
  "resource_id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "production-database",
  "type": "database",
  "endpoint": "https://db.internal:8443/mcp",
  "created_at": "2026-05-16T10:00:00Z"
}
```

### Defaults Applied

| Field | Default |
|-------|---------|
| `auth_method` | `"mTLS+SVID"` |
| `risk_level` | `"medium"` |
| `data_sensitivity` | `"internal"` |
| `rate_limit_per_agent` | `100` |

## 5. Authorize an Action

```bash
curl -X POST http://localhost:8080/v1/authorize \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "resource_id": "660e8400-e29b-41d4-a716-446655440001",
    "action": "database_query",
    "params": {
      "estimated_cost": 5.50
    }
  }'
```

**Required fields:** `agent_id`, `action`

### Authorization Flow

1. Lookup agent by ID (must be `status=active`)
2. Build policy input from agent's `owner`, `trust_score`, `allowed_tools` and the action details
3. Evaluate via OPA or local fallback policy
4. Adjust trust score by `trust_delta` (clamped to `[0.0, 1.0]`)
5. Record `trust_events` row and sign audit log entry

**Allowed response:**
```json
{
  "allowed": true,
  "requires_hitl": false,
  "reason": "tool in allowed list",
  "trust_delta": 0.01
}
```

**Denied response:**
```json
{
  "allowed": false,
  "requires_hitl": true,
  "reason": "tool not in agent allowed list",
  "trust_delta": -0.05
}
```

## 6. Verify a Signature

```bash
curl -X POST http://localhost:8080/v1/verify-signature \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "message": "<base64-encoded-message>",
    "signature": "<base64-encoded-ed25519-signature>"
  }'
```

**Response:**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "valid": true
}
```

## 7. MCP Protocol

The gateway supports Model Context Protocol (JSON-RPC 2.0) on both the control plane (`POST /v1/mcp` on port 8080) and the core proxy (`POST /v1/mcp` on port 9443).

### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {}
}
```

**Response:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {"listChanged": true},
      "resources": {"subscribe": true},
      "prompts": {"listChanged": true}
    },
    "serverInfo": {
      "name": "agentid-gateway",
      "version": "0.1.0"
    }
  }
}
```

### List Tools

```json
{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}
```

### Call a Tool

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "database_query",
    "arguments": {
      "agent_id": "550e8400-e29b-41d4-a716-446655440000",
      "query": "SELECT * FROM users"
    }
  }
}
```

The gateway authorizes the tool call before executing. If allowed:
```json
{
  "content": [{"type": "text", "text": "Action 'database_query' authorized for agent 550e8400-..."}],
  "authorization": {"allowed": true, "requires_hitl": false, "reason": "tool in allowed list", "trust_delta": 0.01}
}
```

If denied:
```json
{
  "isError": true,
  "content": [{"type": "text", "text": "Action 'database_query' denied: tool not in agent allowed list"}],
  "authorization": {"allowed": false, "requires_hitl": true, "reason": "tool not in agent allowed list", "trust_delta": -0.05}
}
```

### List Resources / Prompts

```json
{"jsonrpc": "2.0", "id": 4, "method": "resources/list"}
{"jsonrpc": "2.0", "id": 5, "method": "prompts/list"}
```

### Error Codes

| Code | Meaning |
|------|---------|
| `-32700` | Parse error |
| `-32601` | Method not found |

## 8. Delegation

### Create a Delegation

```bash
curl -X POST http://localhost:8080/v1/delegate \
  -H "Content-Type: application/json" \
  -d '{
    "parent_agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "child_agent_id": "770e8400-e29b-41d4-a716-446655440002",
    "scope": ["database_query", "slack_webhook"],
    "max_depth": 2,
    "duration": "2h30m"
  }'
```

**Defaults:** `max_depth` = 1, `duration` = 1 hour

**Max chain depth:** 3 (enforced globally)

**Response (201):**
```json
{
  "delegation_id": "880e8400-...",
  "parent_agent_id": "550e8400-...",
  "child_agent_id": "770e8400-...",
  "scope": ["database_query", "slack_webhook"],
  "max_depth": 2,
  "expires_at": "2026-05-16T12:30:00Z",
  "spiffe_id": "spiffe://agentid.dev/gateway"
}
```

### View Delegation Chain

```bash
curl http://localhost:8080/v1/delegations/550e8400-e29b-41d4-a716-446655440000
```

### Validate a Delegation

```bash
curl "http://localhost:8080/v1/delegations/validate?parent=550e8400-...&child=770e8400-..."
```

**Response:**
```json
{
  "parent_agent_id": "550e8400-...",
  "child_agent_id": "770e8400-...",
  "valid": true
}
```

### Revoke a Delegation

```bash
curl -X DELETE http://localhost:8080/v1/delegations/880e8400-...
```

**Response:**
```json
{
  "delegation_id": "880e8400-...",
  "status": "revoked"
}
```

## 9. Query Agents and Resources

```bash
# List all agents
curl http://localhost:8080/v1/agents

# Get a specific agent
curl http://localhost:8080/v1/agents/550e8400-e29b-41d4-a716-446655440000

# List all resources
curl http://localhost:8080/v1/resources

# Get a specific resource
curl http://localhost:8080/v1/resources/660e8400-e29b-41d4-a716-446655440001
```

## 10. Health and Identity

```bash
# Health check
curl http://localhost:8080/health
# -> "ok"

# SPIFFE identity info
curl http://localhost:8080/identity
# -> {"spiffe_id":"spiffe://agentid.dev/gateway","trust_domain":"agentid.dev","expires_at":"..."}
```

## 11. Using the Rust Agent SDK

```rust
use agent_sdk::{AgentConfig, client::AgentClient, InvokeResult, ToolInfo};
use ed25519_dalek::SigningKey;

// Configure
let config = AgentConfig {
    agent_id: uuid::Uuid::new_v4(),
    name: "my-agent".to_string(),
    owner: "engineering-team".to_string(),
    gateway_endpoint: "http://localhost:9443".to_string(),
};
let signing_key = SigningKey::generate(&mut rand::rngs::OsRng);

// Connect
let mut client = AgentClient::connect(config, signing_key).await?;

// Discover tools
let tools: Vec<ToolInfo> = client.discover("database").await?;

// Invoke a tool
let result: InvokeResult = client.invoke(
    &resource_id,
    "database_query",
    serde_json::json!({"query": "SELECT * FROM users"})
).await?;

// Delegate to another agent
client.delegate(&child_agent_id, vec!["database_query".into()], 2).await?;

// Check trust score
let score = client.trust_score();
```

### SDK Error Types

| Method | Error Type | Variants |
|--------|-----------|----------|
| `connect` | ConnectError | Connection failures |
| `discover` | DiscoverError | `NotFound`, `Gateway` |
| `invoke` | InvokeError | `NotAuthorized`, `ResourceUnavailable`, `HitlRequired`, `Gateway` |
| `delegate` | DelegateError | `NotAllowed`, `MaxDepth`, `Gateway` |

## 12. Creating a Resource Adapter (Go)

```go
package main

import (
    "context"
    "github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
    srv := server.New("my-database", "localhost:9443")

    // Register MCP tools
    srv.RegisterTool("query", func(params json.RawMessage) (interface{}, error) {
        return map[string]interface{}{"rows": []string{}}, nil
    })

    // Register MCP resources
    srv.RegisterResource("db://schema", func(uri string) (interface{}, error) {
        return map[string]interface{}{"tables": []string{"users", "orders"}}, nil
    })

    // Register MCP prompts
    srv.RegisterPrompt("summarize", func(args map[string]string) (string, error) {
        return "Summary of data...", nil
    })

    srv.Run(context.Background())
}
```

The adapter serves MCP JSON-RPC on `:8443/mcp` and health on `:8443/health`.

## 13. Policy Engine (OPA)

The policy is defined in `gateway/control-plane/cmd/policy/rego/agentid.rego`.

### Allow Conditions

An action is **allowed** when ALL are true:
1. Agent ID and action method are present
2. Agent is valid (`status=active` AND `trust_score >= 0.1`)
3. Tool name is in `agent.allowed_tools`
4. No "never event" violation

### Never Events

Bank transfers over $500 are **always denied**.

### HITL (Human-in-the-Loop)

Bank transfers over $100 require human approval.

### Budget Check

Actions where `estimated_cost > max_budget_usd` are denied.

### Local Fallback

When OPA is unavailable, local evaluation applies:
- Tool in `allowed_tools` + cost within budget: ALLOW, `trust_delta=+0.01`
- Tool NOT in `allowed_tools`: DENY, `requires_hitl=true`, `trust_delta=-0.05`
- Cost exceeds `trust_score * 100`: DENY, `trust_delta=-0.1`

## 14. Cryptographic Identity

- **Algorithm:** Ed25519
- On registration, the server generates a keypair. The public key is stored and returned as base64; the private key must be saved by the client
- Agent requests are signed: `Ed25519.Sign(privateKey, payload_bytes)`
- Signature verification: `Ed25519.Verify(publicKey, message, signature)`
- Audit logs are signed by the gateway: `Ed25519.Sign(gatewayPrivateKey, SHA256("<log_id>:<agent_id>:<resource_id>:<action>:<method>:<status>"))`

## 15. gRPC Service (Port 9090)

The `GatewayService` proto is at `proto/agentid.proto` with 7 RPCs:

| RPC | Request | Response |
|-----|---------|----------|
| `RegisterAgent` | `name, owner, capabilities[], allowed_tools[], max_budget_usd, delegation_policy, behavioral_tags[]` | `agent_id, public_key, status, trust_score, created_at` |
| `RegisterResource` | `name, type, endpoint, auth_method, capabilities_json, risk_level, data_sensitivity, rate_limit_per_agent` | `resource_id, created_at` |
| `Authorize` | `agent_id, resource_id, action, params_json` | `allowed, requires_hitl, reason, trust_delta` |
| `VerifySignature` | `agent_id, message, signature` | `valid, agent_id` |
| `GetAgent` | `agent_id` | Agent details |
| `ListAgents` | _(empty)_ | List of agents |
| `Audit` | `agent_id, limit, offset` | List of audit entries |

## 16. Database Schema

5 PostgreSQL migrations in `registry/migrations/`:

| # | Table | Key Columns |
|---|-------|-------------|
| 1 | `agents` | `agent_id`, `name`, `owner`, `public_key` (BYTEA), `capabilities` (TEXT[]), `allowed_tools` (TEXT[]), `trust_score` (DECIMAL 5,4), `delegation_policy`, `behavioral_tags` (TEXT[]), `status` |
| 2 | `resources` | `resource_id`, `name`, `resource_type`, `endpoint`, `auth_method`, `capabilities` (JSONB), `risk_level`, `data_sensitivity`, `rate_limit_per_agent` |
| 3 | `delegations` | `delegation_id`, `parent_agent_id`, `child_agent_id`, `scope` (TEXT[]), `max_depth`, `expires_at`, `approved_by` |
| 4 | `audit_logs` | `log_id`, `agent_id`, `resource_id`, `action`, `method`, `params` (JSONB), `result_status`, `trust_score_before/after`, `session_id`, `ip_address` (INET), `signature` (BYTEA) |
| 5 | `trust_events` + `hitl_approvals` | `event_id`, `agent_id`, `event_type`, `trust_delta`, `trust_score_after`, / `approval_id`, `status` (pending/approved/rejected), `approver_id` |