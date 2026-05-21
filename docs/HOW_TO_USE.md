# How to Use

Complete guide for setting up, configuring, and using eyeVesa.

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
| `PORT` | core | `9443` |
| `AUTH_ENABLED` | control | `true` |
| `HEARTBEAT_CLEANUP_INTERVAL` | control | `2m` (2 minutes) |

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

## 16. Airport API

The Airport is eyeVesa's agent discovery and presence system. Agents publish heartbeats to signal availability, create profiles to describe their capabilities, and search the directory to find other agents. Connections are tracked automatically.

### POST /v1/airport/heartbeat

Signal that an agent is online and available. Valid status values: `online`, `offline`, `busy`, `idle`. If no heartbeat is received within 5 minutes, the server marks the agent offline automatically. Invalid status values default to `online`.

```bash
curl -X POST http://localhost:8080/v1/airport/heartbeat \
  -H "Content-Type: application/json" \
  -H "X-API-Key: eyevesa_<your-api-key>" \
  -d '{
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "online",
    "metadata": {"region": "us-east-1", "version": "2.1.0", "uptime_seconds": 3600}
  }'
```

**Response (200):**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "online",
  "ok": true
}
```

The `metadata` field is an optional JSONB payload for arbitrary key-value data (region, version, custom status info, etc.). On the first heartbeat for an agent, a profile and heartbeat record are auto-created.

### GET /v1/airport/agents?capability=weather&min_trust=0.8&tag=real-time

Search the agent directory. Only agents with `listed=true` in their profile appear in search results.

**Query parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `capability` | string | Match against agent's `allowed_tools` |
| `skill` | string | Match against skills in the `skills` table |
| `min_trust` | float | Minimum trust score (0.0-1.0) |
| `min_proficiency` | int | Minimum skill proficiency (1-5), used with `skill` |
| `verified` | bool | Only agents with verified skills (`true`) |
| `status` | string | Filter by heartbeat status (`online`, `offline`, `busy`, `idle`) |
| `tag` | string | Match against profile tags |
| `owner` | string | Filter by agent owner |
| `limit` | int | Results per page (default 50, max 200) |
| `offset` | int | Pagination offset |

```bash
curl "http://localhost:8080/v1/airport/agents?capability=weather&min_trust=0.8&tag=real-time"
```

**Response (200):**
```json
{
  "agents": [
    {
      "agent_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "weather-agent",
      "owner": "engineering-team",
      "trust_score": 0.95,
      "status": "online",
      "description": "Real-time weather data provider",
      "services_offered": ["weather-forecast", "weather-alerts"],
      "endpoints": {"forecast": "https://weather.internal/mcp"},
      "tags": ["real-time", "weather", "production"],
      "total_actions": 1523,
      "approval_rate": 0.97,
      "last_seen": "2026-05-19T14:30:00Z"
    }
  ],
  "count": 1,
  "limit": 50,
  "offset": 0
}
```

### GET /v1/airport/online

List all agents currently online (heartbeat within last 2 minutes with `listed=true`). Results are ordered by trust score descending, capped at 100.

```bash
curl http://localhost:8080/v1/airport/online
```

**Response (200):**
```json
{
  "agents": [
    {
      "agent_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "weather-agent",
      "owner": "engineering-team",
      "trust_score": 0.95,
      "status": "online",
      "description": "Real-time weather data provider",
      "services_offered": ["weather-forecast", "weather-alerts"],
      "endpoints": {"forecast": "https://weather.internal/mcp"},
      "tags": ["real-time", "weather"],
      "total_actions": 1523,
      "approval_rate": 0.97,
      "last_seen": "2026-05-19T14:30:00Z"
    }
  ],
  "count": 1
}
```

### GET /v1/airport/agents/{agentID}

Get a single agent's full Airport profile (including heartbeat status and profile data).

```bash
curl http://localhost:8080/v1/airport/agents/550e8400-e29b-41d4-a716-446655440000
```

**Response (200):**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "weather-agent",
  "owner": "engineering-team",
  "trust_score": 0.95,
  "status": "online",
  "description": "Real-time weather data provider",
  "services_offered": ["weather-forecast", "weather-alerts"],
  "endpoints": {"forecast": "https://weather.internal/mcp"},
  "tags": ["real-time", "weather", "production"],
  "total_actions": 1523,
  "approval_rate": 0.97,
  "last_seen": "2026-05-19T14:30:00Z"
}
```

### PUT /v1/airport/agents/{agentID}

Update an agent's Airport profile. Uses upsert -- creates the profile if it doesn't exist.

```bash
curl -X PUT http://localhost:8080/v1/airport/agents/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -H "X-API-Key: eyevesa_<your-api-key>" \
  -d '{
    "description": "Real-time weather data provider with global coverage",
    "services_offered": ["weather-forecast", "weather-alerts", "climate-analysis"],
    "endpoints": {"forecast": "https://weather.internal/mcp", "alerts": "https://weather.internal/alerts"},
    "tags": ["real-time", "weather", "production", "global"],
    "listed": true
  }'
```

**Response (200):**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "listed": true,
  "ok": true
}
```

### GET /v1/airport/connections?agent_id=uuid&limit=20

Retrieve connection history for an agent. Returns records where the agent is either the requester or the responder, ordered by most recent first.

```bash
curl "http://localhost:8080/v1/airport/connections?agent_id=550e8400-e29b-41d4-a716-446655440000&limit=20"
```

**Response (200):**
```json
{
  "connections": [
    {
      "connection_id": "990e8400-e29b-41d4-a716-446655440010",
      "requester_id": "550e8400-e29b-41d4-a716-446655440000",
      "responder_id": "660e8400-e29b-41d4-a716-446655440001",
      "action": "authorize",
      "outcome": "allowed",
      "trust_score_at_time": 0.95,
      "created_at": "2026-05-19T14:15:00Z"
    }
  ],
  "count": 1
}
```

**Query parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `agent_id` | UUID | Yes | The agent to query connections for |
| `limit` | int | No | Max results (default 50) |

### GET /v1/airport/health

Health check for the Airport subsystem. Returns the number of online agents and total profiles.

```bash
curl http://localhost:8080/v1/airport/health
```

**Response (200):**
```json
{
  "status": "healthy",
  "online_agents": 3,
  "total_profiles": 12
}
```

## 17. Airport SDK Usage

### Rust

```rust
use agent_sdk::{AgentConfig, client::AgentClient};

let config = AgentConfig {
    agent_id: uuid::Uuid::parse_str("550e8400-e29b-41d4-a716-446655440000").unwrap(),
    name: "my-agent".to_string(),
    owner: "engineering-team".to_string(),
    gateway_endpoint: "http://localhost:8080".to_string(),
};

let mut client = AgentClient::connect(config, signing_key).await?;

// Heartbeat
let result = client.airport_heartbeat("online").await?;

// Update profile
let profile = client.airport_update_profile(serde_json::json!({
    "description": "Real-time weather agent",
    "tags": ["weather", "real-time"],
    "listed": true,
    "services_offered": ["forecast"],
    "endpoints": {"mcp": "https://weather.internal/mcp"}
})).await?;

// Search for agents
let agents = client.airport_search(&[
    ("capability", "weather"),
    ("min_trust", "0.8"),
    ("tag", "real-time"),
]).await?;

// Get a specific agent's profile
let profile = client.airport_get_profile("770e8400-e29b-41d4-a716-446655440002").await?;

// List online agents
let online = client.airport_list_online().await?;

// Get connection history
let connections = client.airport_connections("550e8400-e29b-41d4-a716-446655440000", 20).await?;
```

### TypeScript

```typescript
import { AgentClient } from 'agent-sdk';

const client = new AgentClient({
  agentId: '550e8400-e29b-41d4-a716-446655440000',
  name: 'my-agent',
  owner: 'engineering-team',
  gatewayEndpoint: 'http://localhost:8080',
});

// Heartbeat
const heartbeat = await client.airportHeartbeat('online', { region: 'us-east-1' });

// Update profile
const profile = await client.airportUpdateProfile({
  description: 'Real-time weather agent',
  tags: ['weather', 'real-time'],
  listed: true,
  servicesOffered: ['forecast'],
  endpoints: { mcp: 'https://weather.internal/mcp' },
});

// Search for agents
const results = await client.airportSearch({ capability: 'weather', minTrust: 0.8, tag: 'real-time' });

// Get a specific agent's profile
const agentProfile = await client.airportGetProfile('770e8400-e29b-41d4-a716-446655440002');

// List online agents
const online = await client.airportListOnline();

// Get connection history
const connections = await client.airportConnections('550e8400-e29b-41d4-a716-446655440000', 20);
```

### Python

```python
from agentid_sdk import AgentClient, AgentConfig

client = AgentClient(
    config=AgentConfig(
        agent_id="550e8400-e29b-41d4-a716-446655440000",
        name="my-agent",
        owner="engineering-team",
        gateway_endpoint="http://localhost:8080",
    ),
    api_key="eyevesa_<your-api-key>",
)

# Heartbeat
result = await client.airport_heartbeat(status="online")

# Update profile
profile = await client.airport_update_profile(
    description="Real-time weather agent",
    tags=["weather", "real-time"],
    listed=True,
    services_offered=["forecast"],
    endpoints={"mcp": "https://weather.internal/mcp"},
)

# Search for agents
results = await client.airport_search(capability="weather", min_trust=0.8, tag="real-time")

# Get a specific agent's profile
agent_profile = await client.airport_get_profile("770e8400-e29b-41d4-a716-446655440002")

# List online agents
online = await client.airport_list_online()

# Get connection history
connections = await client.airport_connections(agent_id="550e8400-e29b-41d4-a716-446655440000", limit=20)
```

## 18. Authentication

eyeVesa supports an authentication middleware that can be enabled or disabled via the `AUTH_ENABLED` environment variable.

### AUTH_ENABLED

| Value | Behavior |
|-------|----------|
| `true` (default in production) | All requests require authentication unless on the public routes list |
| `false` | Authentication middleware is skipped entirely (development only) |

In `docker-compose.yml` (dev), `AUTH_ENABLED` defaults to `"false"`. In `docker-compose.prod.yml` and Kubernetes, it defaults to `"true"`.

### Public Routes (No Authentication Required)

The following routes are accessible without credentials regardless of `AUTH_ENABLED`:

- `GET /health`, `GET /identity`, `GET /ready`, `GET /metrics`
- `POST /v1/agents/register`
- `POST /v1/resources/register`
- `POST /v1/mcp` (JSON-RPC)
- `POST /v1/api-keys`
- `POST /v1/auth/challenge`
- `POST /v1/auth/login`
- Airport read-only routes:
  - `GET /v1/airport/health`
  - `GET /v1/airport/online`
  - `GET /v1/airport/agents` (search)
  - `GET /v1/airport/agents/{agentID}` (get profile)

Airport write routes (`POST /v1/airport/heartbeat`, `PUT /v1/airport/agents/{agentID}`) **require authentication**.

### Authentication Methods

Three methods are supported, checked in order:

#### 1. API Key

Pass an API key via the `X-API-Key` header:

```bash
curl -H "X-API-Key: eyevesa_<key>" http://localhost:8080/v1/airport/heartbeat
```

API keys are stored in the `api_keys` table, scoped to a tenant, and can be created via `POST /v1/api-keys`. Keys can have an optional expiration (`expires_at`).

#### 2. Bearer Token (JWT)

Pass a JWT in the `Authorization` header:

```bash
curl -H "Authorization: Bearer <jwt-token>" http://localhost:8080/v1/agents
```

JWTs contain `tenant_id`, `email`, `role`, and `exp` claims, signed with HMAC-SHA256 using the `JWT_SECRET` environment variable (auto-generated if not set).

#### 3. SSO Session Cookie

After SAML SSO login, a cookie named `eyevesa_sso` contains a JWT. This is used for browser-based workflows.

### Roles

The middleware supports role-based access via `RequireRole`:

| Role | Level |
|------|-------|
| `admin` | 3 (highest) |
| `operator` | 2 |
| `viewer` | 1 |

Roles are extracted from JWT claims and enforced per-route.

## 19. Connection Tracking

When an authorize or invoke action occurs between an agent and a resource, eyeVesa automatically creates an `airport_connections` record logging the interaction. This happens transparently -- no explicit API call is needed.

### How It Works

1. An agent calls `POST /v1/authorize` (or a tool is invoked via MCP)
2. The authorization handler records the outcome (allowed/denied/hitl_required)
3. The `logAirportConnection()` function is called, inserting a row into `airport_connections` with:
   - `requester_id`: the agent ID
   - `responder_id`: the resource ID
   - `action`: the action type (e.g., `"authorize"`)
   - `outcome`: `"allowed"`, `"denied"`, `"hitl_required"`, `"timeout"`, or `"error"`
   - `trust_score_at_time`: the agent's trust score at the moment of the action
4. These records can be queried via `GET /v1/airport/connections?agent_id=<uuid>`

### Connection Outcomes

| Outcome | Description |
|---------|-------------|
| `success` | Action completed successfully (default for non-authorization paths) |
| `allowed` | Authorization was granted |
| `denied` | Authorization was denied |
| `hitl_required` | Action requires human-in-the-loop approval |
| `timeout` | Action timed out |
| `error` | An error occurred |

### Auto-created Heartbeats and Profiles

When an agent first interacts with the Airport subsystem, a heartbeat record and profile are auto-created with default values if they don't already exist:

- `autoCreateHeartbeat()`: Sets status to `online` and `last_heartbeat` to now
- `autoCreateProfile()`: Sets `description` to empty, `services_offered` to `[]`, `endpoints` to `{}`, `tags` to `{}`, `listed` to `true`, `total_actions` to `0`, `approval_rate` to `1.0`

## 20. Database Schema

17 PostgreSQL migrations in `registry/migrations/`:

| # | Table(s) | Key Columns |
|---|----------|-------------|
| 1 | `agents` | `agent_id`, `name`, `owner`, `public_key` (BYTEA), `capabilities` (TEXT[]), `allowed_tools` (TEXT[]), `trust_score` (DECIMAL 5,4), `delegation_policy`, `behavioral_tags` (TEXT[]), `status` |
| 2 | `resources` | `resource_id`, `name`, `resource_type`, `endpoint`, `auth_method`, `capabilities` (JSONB), `risk_level`, `data_sensitivity`, `rate_limit_per_agent` |
| 3 | `delegations` | `delegation_id`, `parent_agent_id`, `child_agent_id`, `scope` (TEXT[]), `max_depth`, `expires_at`, `approved_by` |
| 4 | `audit_logs` | `log_id`, `agent_id`, `resource_id`, `action`, `method`, `params` (JSONB), `result_status`, `trust_score_before/after`, `session_id`, `ip_address` (INET), `signature` (BYTEA) |
| 5 | `trust_events` + `hitl_approvals` | `event_id`, `agent_id`, `event_type`, `trust_delta`, `trust_score_after` / `approval_id`, `status` (pending/approved/rejected), `approver_id` |
| 6 | `identity_bindings` | `binding_id`, `agent_id`, `platform`, `runtime_hash` (BYTEA), `hardware_public_key` (BYTEA), `binding_signature` (BYTEA), `status`, `expires_at` |
| 7 | `hitl_approval_chain` + `hitl_notifications` + `hitl_escalation_config` | `chain_id`, `approval_id`, `approver_id`, `decision` / `notification_id`, `channel`, `escalation_level` / `config_id`, `tenant_id`, `timeout_seconds` |
| 8 | `tenants` + `approvers` | `tenant_id`, `name`, `slug`, `plan`, `max_agents`, `max_resources`, `sso_enabled` / `approver_id`, `tenant_id`, `email`, `role`, `notification_channel` |
| 9 | `behavioral_events` + `behavioral_anomalies` | `event_id`, `agent_id`, `tool`, `action_outcome` / `anomaly_id`, `similarity_score`, `anomaly_type`, `resolved` + pgvector `behavior_vec` on agents |
| 10 | `hitl_summaries` + `audit_narratives` + `policy_translations` + `llm_config` | `summary_id`, `narrative_id`, `translation_id`, `config_id` / LLM integration tables |
| 11 | `agent_spend` + `rate_limit_counters` | `spend_id`, `agent_id`, `resource_id`, `estimated_cost`, `actual_cost` / `counter_id`, `request_count`, window timestamps |
| 12 | `push_tokens` | `token_id`, `approver_id`, `device_token`, `platform`, `is_active` |
| 13 | `api_keys` | `key_id`, `tenant_id`, `api_key` (UNIQUE), `name`, `is_active`, `expires_at` |
| 14 | `trust_bundles` + `workload_registrations` | `bundle_id`, `trust_domain`, `bundle_data`, `is_federated`, `verified` / `registration_id`, `spiffe_id`, `selectors` (TEXT[]), `status` |
| 15 | `skills` + `agent_skills` + `skill_trust_scores` + `skill_endorsements` | `skill_id`, `name`, `category`, `risk_level` / `proficiency`, `verified`, `endorsements_count` / `trust_score` (DECIMAL 5,4) / `endorsement_id`, `endorser_type` |
| 16 | `revoked_tokens` | `token_id`, `reason`, `revoked_at` |
| 17 | `agent_heartbeats` + `agent_profiles` + `airport_connections` | See below |

### Airport Tables (Migration 017)

#### agent_heartbeats

Tracks agent presence and availability.

| Column | Type | Description |
|--------|------|-------------|
| `agent_id` | UUID PK (FK → agents) | The agent |
| `last_heartbeat` | TIMESTAMP | Time of last heartbeat |
| `status` | TEXT | `online`, `offline`, `busy`, or `idle` |
| `metadata` | JSONB | Arbitrary key-value data |
| `updated_at` | TIMESTAMP | Last update time |

Indexes: `idx_agent_heartbeats_status`, `idx_agent_heartbeats_last`

#### agent_profiles

Extended directory listing for agents in the Airport.

| Column | Type | Description |
|--------|------|-------------|
| `agent_id` | UUID PK (FK → agents) | The agent |
| `description` | TEXT | Agent description |
| `services_offered` | JSONB | Array of service names |
| `endpoints` | JSONB | Object mapping service names to URLs |
| `tags` | TEXT[] | Searchable tags |
| `total_actions` | INT | Cumulative action count |
| `approval_rate` | FLOAT | Fraction of actions approved (0.0-1.0) |
| `avg_response_ms` | INT | Average response time in ms |
| `listed` | BOOLEAN | Whether the agent appears in search (default true) |
| `updated_at` | TIMESTAMP | Last update time |

Indexes: `idx_agent_profiles_listed` (partial on `listed=true`), `idx_agent_profiles_tags` (GIN)

#### airport_connections

Interaction log between agents and resources.

| Column | Type | Description |
|--------|------|-------------|
| `connection_id` | UUID PK | Unique connection record |
| `requester_id` | UUID (FK → agents) | The requesting agent |
| `responder_id` | UUID (FK → agents) | The responding agent/resource |
| `action` | TEXT | Action type (e.g., `"authorize"`) |
| `outcome` | TEXT | `success`, `allowed`, `denied`, `hitl_required`, `timeout`, or `error` |
| `trust_score_at_time` | FLOAT | Agent's trust score at time of action |
| `created_at` | TIMESTAMP | When the connection occurred |

Indexes: `idx_airport_requester`, `idx_airport_responder`, `idx_airport_created`

The migration also defines a PostgreSQL function `airport_mark_stale_offline()` that marks agents as offline if their heartbeat is older than 2 minutes, which is called periodically by the control plane's `StartHeartbeatCleanup` goroutine.