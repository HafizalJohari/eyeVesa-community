<p align="center">
  <img src="site/logo.svg" alt="eyeVesa" width="320">
</p>

<h1 align="center">eyeVesa</h1>

<p align="center">
  <strong>Identity, authorization, discovery, and audit trails for AI agents.</strong>
</p>

<p align="center">
  <em>Know Your Agent. Control what it can do. Prove what happened.</em>
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: Apache 2.0" src="https://img.shields.io/badge/license-Apache%202.0-2f80ed"></a>
  <a href="docs/beginner-guide.md"><img alt="Beginner friendly" src="https://img.shields.io/badge/start-Beginner%20Guide-22c55e"></a>
  <a href="#community-quickstart"><img alt="Docker quickstart" src="https://img.shields.io/badge/run-Docker%20Quickstart-f97316"></a>
  <a href="#agent-native-setup"><img alt="Agent native setup" src="https://img.shields.io/badge/install-Agent%20Native-14b8a6"></a>
  <a href="#airport"><img alt="Airport discovery" src="https://img.shields.io/badge/agent%20discovery-Airport-8b5cf6"></a>
  <a href="#auth-middleware"><img alt="Auth modes" src="https://img.shields.io/badge/auth-local%20%7C%20production-0f172a"></a>
</p>

---

eyeVesa connects AI agents to tools, resources, and other agents with cryptographic identity, policy-based authorization, and non-repudiable audit trails.

New to eyeVesa or AI-agent infrastructure? Start with the plain-language [Beginner Guide](docs/beginner-guide.md).

<table>
  <tr>
    <td width="33%">
      <strong>For new users</strong><br>
      Run a local sandbox, learn what the Airport is, and see how agent audit trails work.
    </td>
    <td width="33%">
      <strong>For developers</strong><br>
      Use SDKs, MCP, OPA policies, and the gateway/control-plane stack to build agent systems.
    </td>
    <td width="33%">
      <strong>For operators</strong><br>
      Keep production credentials, International Airport access, and GCP deployment under your control.
    </td>
  </tr>
</table>

## Start Here

| If you are... | Read this |
|---|---|
| New to the project | [Beginner Guide](docs/beginner-guide.md) |
| Trying it locally | [Community Quickstart](#community-quickstart) |
| Asking an AI agent to install it | [Agent-Native Setup](#agent-native-setup) |
| Building an integration | [Agent Integrations](#agent-integrations) |
| Learning agent discovery | [Airport](#airport) |
| Setting up auth | [API Keys](#api-keys) and [Auth Middleware](#auth-middleware) |

## Community Quickstart

Run a local eyeVesa sandbox with Docker. This does not connect to the official International Airport or any GCP production resources.

```bash
git clone https://github.com/HafizalJohari/eyeVesa-community.git
cd eyeVesa-community
./start.sh
```

The quickstart starts local PostgreSQL, OPA, the Go control plane, and the Rust gateway. Local development runs with `AUTH_ENABLED=false`, so API keys are not required for basic testing.

| Local endpoint | Purpose |
|---|---|
| `http://localhost:8080` | Go control-plane API |
| `http://localhost:9443` | Rust gateway proxy |
| `http://localhost:8181` | OPA policy server |

To connect to the official International Airport, you need an invite or API key from the operator. Public code access is not Airport access.

## Agent-Native Setup

Install eyeVesa by asking your AI agent.

If you use Hermes, OpenClaw, Claude, Codex, or another agent with terminal access, give it this prompt:

```text
Install and run eyeVesa from https://github.com/HafizalJohari/eyeVesa-community.git.
Use the local community sandbox only. Do not request production credentials.
Clone the repo, run ./start.sh, verify health endpoints, and report the local URLs back to me.
```

The agent should follow [AGENT_INSTALL.md](AGENT_INSTALL.md) and report these local URLs when setup succeeds:

| Local URL | What it is |
|---|---|
| `http://localhost:8080` | Control-plane API |
| `http://localhost:9443` | Gateway proxy |
| `http://localhost:8181` | OPA policy server |

This is local-only by default. Do not give your agent GCP credentials, Terraform state, production secrets, or official International Airport keys unless you intentionally want it to operate a production environment.

See [Agent-Native Install](docs/agent-native-install.md) for the full guide.

## Architecture

```
Agent (SDK) ──mTLS──▶ Gateway Core ──gRPC──▶ Control Plane ──mTLS──▶ Resource (Adapter)
                            │                     │
                       MCP Proxy              Registry  Policy
                      (Rust/Hyper)           (PostgreSQL) (OPA)
                                            SPIRE    Audit    HITL     PTV
                                                            Airport
```

**Dual-protocol gateway**: The Rust core (port 9443, configurable via `PORT`) proxies HTTP/JSON-RPC requests to the Go control plane (port 9090 gRPC, 8080 HTTP) for authorization, registration, and crypto operations. PTV (Prove-Transform-Verify) provides hardware-rooted identity. **Airport** provides agent discovery, heartbeat tracking, profile management, and connection logging.

## Key Features

<table>
  <tr>
    <td width="50%">
      <strong>Identity</strong><br>
      Ed25519 agent identity, signatures, JWT/API-key auth, and optional SPIRE/SPIFFE workload identity.
    </td>
    <td width="50%">
      <strong>Authorization</strong><br>
      OPA/Rego policy decisions, scoped delegation, trust scoring, and budget-aware action checks.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <strong>Airport</strong><br>
      Agent discovery with searchable profiles, online presence, heartbeat tracking, and connection logs.
    </td>
    <td width="50%">
      <strong>Audit</strong><br>
      Non-repudiable audit records with signed actions, trust deltas, sessions, and reviewable history.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <strong>MCP Gateway</strong><br>
      Model Context Protocol support through a Rust gateway and Go control plane.
    </td>
    <td width="50%">
      <strong>Human Approval</strong><br>
      HITL workflows for risky actions, approval chains, notifications, expiry, and escalation.
    </td>
  </tr>
</table>

## Packages

| Package | Language | Purpose |
|---|---|---|
| `gateway/core` | Rust | MCP proxy, crypto, mTLS termination, PTV identity |
| `gateway/control-plane` | Go | HTTP + gRPC APIs, OPA policy, audit, DB, HITL, PTV, Airport |
| `sdk/agent-sdk-rust` | Rust | Client library for AI agents (connect, invoke, discover, delegate, airport) |
| `sdk/agent-sdk-python` | Python | Client library with LangGraph/CrewAI/AutoGen/Claude/OpenAI/Hermes/OpenClaw integrations |
| `sdk/agent-sdk-typescript` | TypeScript | Client library for Node.js agents with Claude/OpenAI/Hermes/NanoClaw integrations |
| `adapter/resource-adapter-go` | Go | MCP server wrapper for enterprise resources |
| `proto/agentid.proto` | Protobuf | gRPC service definition (7 RPCs) |
| `registry/migrations/` | SQL | PostgreSQL schema (17 migrations, pgvector) |
| `policies/authz.rego` | Rego | OPA authorization policies |
| `deploy/` | YAML | Docker, K8s, cloud configs |
| `cli/` | Go | `eyevesa` CLI tool with airport subcommands |

## Agent Integrations

eyeVesa provides SDK integrations for major agentic AI frameworks:

| Provider | Integration Class | Method |
|---|---|---|
| **Claude (Anthropic)** | `ClaudeIntegration` | Tool calling via Messages API + MCP server for Claude Code |
| **OpenAI** | `OpenAIIntegration` | Responses API `computer` + `function_call` + MCP connector |
| **LangGraph** | `LangGraphIntegration` | LangChain function-calling format |
| **CrewAI** | `CrewAIIntegration` | CrewAI tool definitions |
| **AutoGen** | `AutoGenIntegration` | AutoGen function definitions |
| **Hermes** | `HermesIntegration` | Action specs with Airport heartbeat + peer discovery |
| **OpenClaw** | `OpenClawIntegration` | Tool registry with Airport registration |
| **NanoClaw** | `NanoClawIntegration` | Guardrails function defs with trust gating |

### Quick Start: Claude

```python
from agentid_sdk import ClaudeIntegration

claude = ClaudeIntegration.from_config(gateway_endpoint="http://localhost:9443")
await claude.connect()
tools = claude.get_tool_definitions()  # Anthropic tool format
result = await claude.handle_tool_call("eyevesa_read", {"resource_id": "res-001"})
```

### Quick Start: OpenAI

```python
from agentid_sdk import OpenAIIntegration

openai_int = OpenAIIntegration.from_config(gateway_endpoint="http://localhost:9443")
await openai_int.connect()
function_tools = openai_int.get_function_tools()       # OpenAI function format
all_tools = openai_int.get_computer_and_function_tools()  # computer + functions
result = await openai_int.handle_function_call("eyevesa_read", {"resource_id": "res-001"})
```

See [docs/integrations/](docs/integrations/) for detailed guides.

## Airport

The **Airport** is eyeVesa's agent discovery layer — the place where agents meet, announce their presence, find each other by capability, and track their interactions.

### Core Concepts

- **Heartbeat**: Agents send periodic heartbeats to signal they are online. Stale heartbeats (>5 min) are automatically marked offline.
- **Profile**: Each agent has a searchable profile (description, tags, services, endpoints). Profiles can be listed or unlisted.
- **Search**: Find agents by capability, skill, trust score, status, tag, or owner.
- **Connections**: Every authorization interaction between two agents is logged as a connection record, creating a social graph.
- **Health**: A public endpoint returns airport status (online agent count, total profiles).
- **Auto-Registration**: When an agent registers via `POST /v1/agents/register`, it automatically receives an airport heartbeat (status: online) and a profile (listed: true). Connections are logged automatically during the authorize flow.

### Airport API Endpoints

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| POST | `/v1/airport/heartbeat` | Send agent heartbeat (online/offline/busy/idle) | Required |
| GET | `/v1/airport/agents` | Search agents with filters (capability, skill, min_trust, status, tag, owner) | Public |
| GET | `/v1/airport/online` | List agents currently online (heartbeat < 2 min) | Public |
| GET | `/v1/airport/agents/{id}` | Get a single agent's airport profile | Public |
| PUT | `/v1/airport/agents/{id}` | Update agent profile (description, tags, services, endpoints, listed) | Required |
| GET | `/v1/airport/connections` | List connections for an agent (`?agent_id=...&limit=...`) | Required |
| GET | `/v1/airport/health` | Airport health (online agent count, total profiles) | Public |

### Airport Auth Policy

| Endpoint | Auth Required | Notes |
|----------|--------------|-------|
| `GET /v1/airport/agents` | No (public) | Browse/search agents |
| `GET /v1/airport/online` | No (public) | See who's online |
| `GET /v1/airport/health` | No (public) | Health/stats |
| `GET /v1/airport/agents/{id}` | No (public) | View individual profile |
| `POST /v1/airport/heartbeat` | Yes | Must authenticate to announce presence |
| `PUT /v1/airport/agents/{id}` | Yes | Must authenticate to update own profile |
| `GET /v1/airport/connections` | Yes | Must authenticate to view connections |

### MCP (Model Context Protocol)

MCP is the **execution layer** on top of KYA + Airport.

- **KYA** = Identity & Trust (who is this agent?)
- **Airport** = Discovery (where are the other agents?)
- **MCP** = Tool Execution (what can this agent *do*?)

eyeVesa implements MCP as a standardized way for agents to discover and call tools through registered resource adapters.

| Method | Maps to | Description |
|--------|---------|-------------|
| `airport/search` | `GET /v1/airport/agents` | Search agents by capability, skill, min_trust, status, limit |
| `airport/heartbeat` | `POST /v1/airport/heartbeat` | Send heartbeat with agent_id, status, metadata |
| `airport/profile` | `GET /v1/airport/agents/{id}` or `PUT` | Get or update profile (update if `update` param present) |
| `airport/online` | `GET /v1/airport/online` | List online agents |
| `airport/connections` | `GET /v1/airport/connections` | Query connections by agent_id, limit |

### Auto-Registration Behavior

When an agent registers via `POST /v1/agents/register`:
1. An airport heartbeat is automatically created with status `online`
2. An airport profile is automatically created with `listed: true`
3. The agent is immediately discoverable via search and online endpoints

During the authorize flow (`POST /v1/authorize`), every authorization creates an `airport_connections` record with:
- `requester_id` — the agent requesting the action
- `responder_id` — the resource (or agent) responding
- `action` — the action being authorized
- `outcome` — success, denied, hitl_required, timeout, or error
- `trust_score_at_time` — the agent's trust score at the moment of authorization

### Connection Tracking

Every `POST /v1/authorize` call logs an `airport_connections` record, building a social graph of agent interactions:

```sql
SELECT connection_id, requester_id, responder_id, action, outcome, trust_score_at_time, created_at
FROM airport_connections
WHERE requester_id = $1 OR responder_id = $1
ORDER BY created_at DESC
LIMIT 50
```

## CLI

The `eyevesa` CLI provides a terminal UI and commands for agent management, authorization, and airport operations.

### Install

Install the latest CLI from `main`:

```bash
curl -fsSL https://raw.githubusercontent.com/Hafizaljohari/eyeVesa/main/scripts/install.sh | bash
```

Install from a specific release tag:

```bash
VERSION=v0.1.1 curl -fsSL https://raw.githubusercontent.com/Hafizaljohari/eyeVesa/main/scripts/install.sh | bash
```

Install via Bun:

```bash
bunx --bun bash -c "$(curl -fsSL https://raw.githubusercontent.com/Hafizaljohari/eyeVesa/main/scripts/install.sh)"
```

Install via Homebrew tap:

```bash
brew tap Hafizaljohari/eyevesa https://github.com/Hafizaljohari/eyeVesa
brew install eyevesa
```

Run via Docker:

```bash
docker build -t eyevesa-cli -f cli/Dockerfile .
docker run --rm eyevesa-cli --help
```

Launch the interactive terminal dashboard:
```bash
eyevesa tui
```

Subcommands:

### Agent & Resource Commands

```bash
eyevesa register --name my-agent --owner "org:acme" --allowed-tools read,write
eyevesa register-resource --name my-resource --type api ...
eyevesa list-agents
eyevesa get-agent <agent-id>
eyevesa authorize --agent-id <id> --action read --resource-id <res-id>
eyevesa verify-signature --agent-id <id> --data "hello" --signature <sig>
eyevesa delegate --agent-id <id> --scope "read:doc-001" --max-depth 2
eyevesa list-delegations --agent-id <id>
eyevesa validate-delegation --delegation-id <id>
eyevesa hitl-request --agent-id <id> --action transfer --risk-level high
eyevesa hitl-pending
eyevesa audit --agent-id <id>
```

### Airport Commands

```bash
# Search for agents at the airport
eyevesa airport search [--capability read] [--skill research] [--status online] [--tag data] [--owner org:acme] [--min-trust 0.8] [--limit 50]

# List agents currently online
eyevesa airport online

# Get an agent's airport profile
eyevesa airport profile <agent-id>

# Send a heartbeat for an agent
eyevesa airport heartbeat <agent-id> [--status online]

# Update an agent's airport profile
eyevesa airport update-profile <agent-id> [--description "Research agent"] [--tags ai,ml] [--listed true]

# List an agent's connections
eyevesa airport connections <agent-id> [--limit 50]

# Check airport health and stats
eyevesa airport health
```

### API Key Commands

```bash
# Create a new API key
eyevesa api-keys create --name my-agent-key --tenant-id org:phos

# List all API keys
eyevesa api-keys list

# Revoke an API key
eyevesa api-keys revoke <key-id>
```

## API Keys

API keys (`eyevesa_xxx`) authenticate agent and gateway requests through the `X-API-Key` header.

There are two modes:

| Mode | What happens |
|---|---|
| Community local sandbox | `AUTH_ENABLED=false`; API keys are optional for learning because all local routes are open. |
| Production / International Airport | `AUTH_ENABLED=true`; API keys are admin-issued, named, revocable, and delivered outside git. |

### Key Format

```
eyevesa_REPLACE_WITH_YOUR_API_KEY
         ^-- 43 chars base64 URL-safe (32-byte random)
```

### Usage

```bash
# Production: create a key with an admin JWT
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Authorization: Bearer <ADMIN_JWT>" \
  -H "Content-Type: application/json" \
  -d '{"name": "dev:amir", "tenant_id": "org:community"}'

# Use the assigned key for authenticated requests
curl -X POST http://localhost:8080/v1/delegate \
  -H "Content-Type: application/json" \
  -H "X-API-Key: eyevesa_REPLACE_WITH_YOUR_API_KEY" \
  -d '{"agent_id": "...", "target": "..."}'
```

For the official International Airport, developers do not generate their own production keys. The operator creates one key per developer or gateway and shares it through a password manager or Secret Manager.

## Auth Middleware

When `AUTH_ENABLED=true`, the middleware checks:
1. `X-API-Key` header → lookup in `api_keys` table (must be `is_active = TRUE`)
2. Falls back to `Authorization: Bearer <jwt>` or SSO session cookie

Public routes with auth enabled:
- `/health`, `/ready`, `/identity`, `/metrics`
- `/v1/auth/challenge`, `/v1/auth/login`
- `GET /v1/airport/health`, `GET /v1/airport/stats`
- `GET /v1/airport/agents`, `GET /v1/airport/agents/{id}`, `GET /v1/airport/online`
- `POST /v1/airport/handshake`, `POST /v1/airport/connect`
- `/v1/resources/register` and `/v1/mcp` are currently public compatibility endpoints and should be reviewed before production exposure.

Authenticated routes include API key creation, agent registration, heartbeat writes, profile updates, connections, delegation, authorization, tenants, key rotation, and most operational APIs.

When `AUTH_ENABLED=false`, all routes are open. Use this only for local community/dev sandboxes.

## API Endpoints

### Control Plane (HTTP :8080)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/identity` | SPIFFE identity info |
| POST | `/v1/agents/register` | Register a new AI agent |
| GET | `/v1/agents` | List all agents |
| GET | `/v1/agents/{agentID}` | Get agent by ID |
| POST | `/v1/resources/register` | Register an enterprise resource |
| GET | `/v1/resources` | List all resources |
| GET | `/v1/resources/{resourceID}` | Get resource by ID |
| POST | `/v1/authorize` | Authorize an agent action (OPA) |
| POST | `/v1/verify-signature` | Verify Ed25519 signature |
| POST | `/v1/mcp` | MCP JSON-RPC 2.0 endpoint |
| POST | `/v1/delegate` | Delegate scope to another agent |
| GET | `/v1/delegations/{agentID}` | Get delegation chain |
| GET | `/v1/delegations/validate` | Validate a delegation |
| DELETE | `/v1/delegations/{delegationID}` | Revoke a delegation |
| POST | `/v1/hitl/request` | Request human approval |
| GET | `/v1/hitl/pending` | List pending approvals |
| GET | `/v1/hitl/{approvalID}` | Get approval status |
| POST | `/v1/hitl/{approvalID}/decide` | Approve/reject with FaceID/password |
| POST | `/v1/ptv/attest` | PTV: Attest hardware identity |
| POST | `/v1/ptv/bind` | PTV: Transform attestation to binding |
| GET | `/v1/ptv/verify/{bindingID}` | PTV: Verify identity binding |
| POST | `/v1/hitl/escalate` | Escalated HITL approval (multi-approver) |
| POST | `/v1/hitl/{approvalID}/chain` | Process chain-level approval decision |
| GET | `/v1/hitl/{approvalID}/chain` | Get approval chain entries |
| GET | `/v1/hitl/{approvalID}/notifications` | Get notification history for approval |
| POST | `/v1/llm/hitl-summary/{approvalID}` | Generate LLM summary for HITL approval |
| POST | `/v1/llm/audit-narrative` | Generate LLM audit narrative |
| POST | `/v1/llm/policy-translate` | Translate natural language to Rego |
| POST | `/v1/behavior/{agentID}/embedding` | Update behavioral embedding |
| GET | `/v1/behavior/{agentID}/anomalies` | Detect behavioral anomalies |
| GET | `/v1/behavior/{agentID}/similar` | Find similar agents |
| POST | `/v1/tenants` | Create tenant |
| GET | `/v1/tenants` | List tenants |
| GET | `/v1/tenants/{tenantID}` | Get tenant |
| GET | `/v1/budget/check` | Check agent budget |
| POST | `/v1/budget/spend` | Record agent spend |
| POST | `/v1/push/register` | Register push notification device token |
| GET | `/v1/push/tokens` | List push tokens for approver |
| DELETE | `/v1/push/tokens/{tokenID}` | Deactivate push token |
| POST | `/v1/audit` | Query audit trail |
| **Airport endpoints** | | |
| POST | `/v1/airport/heartbeat` | Send agent heartbeat |
| GET | `/v1/airport/agents` | Search agents (browse) |
| GET | `/v1/airport/online` | List online agents |
| GET | `/v1/airport/agents/{agentID}` | Get agent airport profile |
| PUT | `/v1/airport/agents/{agentID}` | Update agent airport profile |
| GET | `/v1/airport/connections` | List agent connections |
| GET | `/v1/airport/health` | Airport health/stats |
| **Auth endpoints** | | |
| POST | `/v1/auth/challenge` | Get auth challenge |
| POST | `/v1/auth/login` | Login with API key or credentials |
| GET | `/v1/auth/challenge` | Get SSO challenge |
| **API Key endpoints** | | |
| POST | `/v1/api-keys` | Create API key (admin-only when auth is enabled) |
| GET | `/v1/api-keys` | List API keys |
| DELETE | `/v1/api-keys/{keyID}` | Revoke API key |

### Core Proxy (HTTP/TLS/mTLS :9443)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/mcp` | MCP JSON-RPC proxy |
| POST | `/v1/register` | Agent registration (proxied to control plane) |
| POST | `/v1/auth` | Authorization (proxied via gRPC) |
| * | `/v1/ptv/*` | PTV endpoints (proxied to control plane) |
| * | `/v1/hitl/*` | HITL endpoints (proxied to control plane) |
| * | `/v1/agents/*` | Agent management (proxied) |
| * | `/v1/delegate*` | Delegation (proxied) |
| * | `/v1/audit*` | Audit trail (proxied) |
| * | `/v1/airport/*` | Airport endpoints (proxied) |

### gRPC (Control Plane :9090)

`GatewayService` with 7 RPCs: `RegisterAgent`, `RegisterResource`, `Authorize`, `VerifySignature`, `GetAgent`, `ListAgents`, `Audit`

### MCP Methods (Resource Adapter :8443)

`initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`, `prompts/list`, `prompts/get` (protocol version `2024-11-05`)

## SDK Airport Methods

### Python SDK

```python
from agentid_sdk import AgentClient

client = AgentClient(gateway_endpoint="http://localhost:9443", agent_id=agent_id)

# Send heartbeat
result = await client.airport_heartbeat(status="online")

# Update profile
result = await client.airport_update_profile(
    description="Research agent",
    services_offered=["search", "analyze"],
    endpoints={"api": "https://..."},
    tags=["research", "ml"],
    listed=True,
)

# Search agents
agents = await client.airport_search(capability="read", min_trust=0.8, status="online", limit=10)

# Get agent profile
profile = await client.airport_get_profile(agent_id)

# List online agents
online = await client.airport_list_online()

# List connections
connections = await client.airport_connections(agent_id=agent_id, limit=50)
```

### TypeScript SDK

```typescript
import { AgentClient } from 'agentid-sdk';

const client = new AgentClient({ gatewayEndpoint: 'http://localhost:9443', agentId });

// Send heartbeat
const heartbeat = await client.airportHeartbeat('online');

// Update profile
const profile = await client.airportUpdateProfile({
  description: 'Research agent',
  servicesOffered: ['search', 'analyze'],
  endpoints: { api: 'https://...' },
  tags: ['research', 'ml'],
  listed: true,
});

// Search agents
const agents = await client.airportSearch({ capability: 'read', minTrust: 0.8, status: 'online', limit: 10 });

// Get agent profile
const agent = await client.airportGetProfile(agentId);

// List online agents
const online = await client.airportListOnline();

// List connections
const connections = await client.airportConnections(agentId, 50);
```

### Rust SDK

```rust
use agentid_sdk::airport::{AirportAgent, AirportConnection, AirportError};

// Send heartbeat
let result = client.airport_heartbeat("online").await?;

// Update profile
let profile = client.airport_update_profile(serde_json::json!({
    "description": "Research agent",
    "tags": vec!["research", "ml"],
    "listed": true,
})).await?;

// Search agents
let agents: Vec<AirportAgent> = client.airport_search(&[
    ("capability", "read"),
    ("min_trust", "0.8"),
    ("status", "online"),
]).await?;

// Get agent profile
let agent: AirportAgent = client.airport_get_profile("agent-uuid").await?;

// List online agents
let online: Vec<AirportAgent> = client.airport_list_online().await?;

// List connections
let connections: Vec<AirportConnection> = client.airport_connections("agent-uuid", 50).await?;
```

**Rust structs** returned by airport methods:

```rust
pub struct AirportAgent {
    pub agent_id: String,
    pub name: String,
    pub owner: String,
    pub trust_score: f64,
    pub status: String,
    pub description: String,
    pub services_offered: serde_json::Value,
    pub endpoints: serde_json::Value,
    pub tags: Vec<String>,
    pub total_actions: i64,
    pub approval_rate: f64,
    pub last_seen: String,
}

pub struct AirportConnection {
    pub connection_id: String,
    pub requester_id: String,
    pub responder_id: String,
    pub action: String,
    pub outcome: String,
    pub trust_score_at_time: f64,
    pub created_at: String,
}
```

## Examples & Integrations

Ready to integrate your actual AI agents with eyeVesa? Check out the [`examples/`](./examples) directory for framework-specific recipes:

- **[Vanilla Python Agent](./examples/python/01_basic_identity.py)**: How to register a cryptographic identity, log in via challenge-response, and maintain an Airport heartbeat.
- **[LangChain Audited Agent](./examples/langchain/01_audited_agent.py)**: How to route an LLM's external Tool requests through the eyeVesa proxy, guaranteeing an immutable audit trail for every action.

Once running, you can immediately test agent registration via the API:

```bash
# Register an agent (auto-creates airport heartbeat + profile)
curl -X POST http://localhost:9443/v1/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","owner":"org:test","allowed_tools":["read","write"]}'

# Authorize an action (auto-logs airport connection)
curl -X POST http://localhost:9443/v1/auth \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<AGENT_ID>","action":"read","resource_id":"doc-001"}'

# Send airport heartbeat
curl -X POST http://localhost:8080/v1/airport/heartbeat \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<AGENT_ID>","status":"online"}'

# Search agents at the airport
curl http://localhost:8080/v1/airport/agents?status=online&min_trust=0.5

# List online agents
curl http://localhost:8080/v1/airport/online

# Request HITL approval
curl -X POST http://localhost:9443/v1/hitl/request \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<AGENT_ID>","action":"bank_transfer","reason":"Transfer $10K","risk_level":"high"}'

# PTV: Attest → Bind → Verify
curl -X POST http://localhost:9443/v1/ptv/attest \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"my-agent","platform":"macos-secure-enclave","firmware_version":"1.0.0"}'
```

## Running Tests

```bash
# Rust unit tests (5 tests)
cd gateway/core && cargo test

# Go unit tests (8 packages)
cd gateway/control-plane && go test ./internal/... -v

# Go integration tests (requires running server)
cd gateway/control-plane && DATABASE_URL="postgres://agentid:agentid_dev@localhost:5432/agentid?sslmode=disable" \
  go test ./internal/integration/... -v -tags=integration

# Full E2E test suite (30 tests, requires all services running)
bash tests/e2e-test.sh
```

## Gateway Modes

| `GATEWAY_MODE` | Description | Port |
|---|---|---|
| `plaintext` (default) | HTTP, no TLS | 9443 |
| `tls` | Server TLS, no client cert | 9443 |
| `mtls` | Mutual TLS with client cert | 9443 |

```bash
# TLS mode
GATEWAY_MODE=tls TLS_CERT_PATH=/tmp/agentid-gateway.crt TLS_KEY_PATH=/tmp/agentid-gateway.key cargo run

# mTLS mode (requires client certs)
GATEWAY_MODE=mtls TLS_CERT_PATH=/tmp/agentid-gateway.crt TLS_KEY_PATH=/tmp/agentid-gateway.key TLS_CA_PATH=/tmp/agentid-ca.crt cargo run
```

## Environment Variables

| Variable | Service | Default | Purpose |
|----------|---------|---------|---------|
| `DATABASE_URL` | control | `postgres://agentid:agentid_dev@localhost:5432/agentid` | PostgreSQL connection |
| `CONTROL_PLANE_ADDR` | core | `http://localhost:9090` | gRPC control plane address |
| `CONTROL_PLANE_HTTP_ADDR` | core | `localhost:8080` | HTTP control plane address (for proxy forwarding) |
| `PORT` | core | `9443` | Gateway Core listen port (supports Cloud Run `$PORT`) |
| `GATEWAY_MODE` | core | `plaintext` | Gateway mode: plaintext, tls, mtls |
| `TLS_CERT_PATH` | core | `/tmp/agentid-gateway.crt` | TLS certificate path |
| `TLS_KEY_PATH` | core | `/tmp/agentid-gateway.key` | TLS private key path |
| `TLS_CA_PATH` | core | `/tmp/agentid-ca.crt` | CA certificate for client cert verification |
| `RUST_LOG` | core | `info` | Rust log level |
| `OPA_ENDPOINT` | control | (empty) | External OPA server (optional, uses embedded Rego if empty) |
| `POLICY_DIR` | control | `policies` | Directory containing Rego policy files |
| `SPIRE_ENDPOINT` | control | `spire-agent:8090` | SPIRE agent address |
| `RESOURCE_NAME` | adapter | `unnamed-resource` | Resource display name |
| `GATEWAY_ENDPOINT` | adapter | `localhost:9443` | Gateway core address |
| `GATEWAY_KEY_PATH` | control | `/tmp/agentid-gateway-ed25519.key` | Ed25519 gateway key (persisted across restarts) |
| `PTV_KEY_PATH` | control | `/tmp/agentid-ptv-ecdsa.key` | PTV ECDSA key (persisted across restarts) |
| `AUTH_ENABLED` | control | `false` | Enable auth middleware (API key/JWT/SSO) |
| `JWT_SECRET` | control | (auto-generated) | JWT signing secret |
| `HEARTBEAT_CLEANUP_INTERVAL` | control | `2m` | Interval for marking stale heartbeats as offline |
| `SPIFFE_ENDPOINT_SOCKET` | control | `unix:///tmp/spire-agent/public/api.sock` | SPIRE Workload API socket |
| `APNS_KEY_PATH` | control | (empty) | APNs push notification key (PEM) |
| `APNS_KEY_ID` | control | (empty) | APNs key ID |
| `APNS_TEAM_ID` | control | (empty) | APNs team ID |
| `APNS_BUNDLE_ID` | control | (empty) | APNs bundle ID |
| `APNS_PRODUCTION` | control | `false` | Use APNs production endpoint |
| `FCM_SERVER_KEY` | control | (empty) | FCM server key |
| `FCM_PROJECT_ID` | control | (empty) | FCM project ID |
| `SLACK_WEBHOOK_URL` | control | (empty) | Slack webhook for HITL notifications |
| `PAGERDUTY_INTEGRATION_KEY` | control | (empty) | PagerDuty integration key |

## OPA Policy Engine

Authorization uses embedded Rego policies (`policies/authz.rego`) evaluation via the OPA Go SDK. The policy engine supports three modes:

1. **Embedded OPA** (default): Policies loaded from `policies/authz.rego`, evaluated in-process
2. **External OPA**: Query an OPA server at `OPA_ENDPOINT` for policy decisions
3. **Local fallback**: Hardcoded rules if both OPA modes fail

Policy decisions:
- **Allowed tool in list** → `allowed=true, requires_hitl=false, trust_delta=+0.01`
- **Tool not in list** → `allowed=false, requires_hitl=true, trust_delta=-0.05`
- **Cost exceeds trust budget** → `allowed=false, requires_hitl=true, trust_delta=-0.1`

## Infrastructure

| Service | Image | Port |
|---------|-------|------|
| PostgreSQL + pgvector | `pgvector/pgvector:pg16` | 5432 |
| SPIRE Server | `ghcr.io/spiffe/spire-server:1.9.6` | 8081 |
| SPIRE Agent | `ghcr.io/spiffe/spire-agent:1.9.6` | 8090 |
| OPA | `openpolicyagent/opa:latest` | 8181 |
| Gateway Core | Built in-tree | 9443 |
| Gateway Control | Built in-tree | 8080, 9090 |
| Resource Adapter | Built in-tree | 8443 |

## Database Schema

17 PostgreSQL migrations in `registry/migrations/`:

1. **agents** - Identity registry with public_key, capabilities, allowed_tools, trust_score, delegation_policy, behavioral_tags
2. **resources** - Resource catalog with type, endpoint, auth_method, capabilities (JSONB), risk_level, data_sensitivity
3. **delegations** - Agent-to-agent delegation chains with scope, max_depth, expires_at, revocation support
4. **audit_logs** - Non-repudiable trail with Ed25519 signature, params (JSONB), trust_score_before/after, session_id
5. **trust_events + hitl_approvals** - Trust score changes + human-in-the-loop approval queue with 5-minute expiry
6. **identity_bindings** - PTV identity bindings with hardware attestation, platform, runtime_hash
7. **hitl_escalation** - Multi-layer HITL escalation, approval chains, notification log, escalation config
8. **tenants + approvers** - Multi-tenant isolation with SSO config and approver management
9. **behavioral_embeddings** - pgvector-based behavior vectors, events, and anomaly detection
10. **llm_integration** - HITL summaries, audit narratives, policy translations, LLM config
11. **budget_metering** - Agent spend tracking, rate limit counters
12. **push_tokens** - APNs/FCM device tokens for HITL push notifications
13. **api_keys** - API key authentication for gateway access
14. **spire_federation** - SPIRE federation endpoints and relationships
15. **skills** - Skill catalog with categories, risk levels, and proficiency thresholds
16. **transaction_tokens** - Transaction tokens for idempotent operations
17. **airport** - Agent heartbeats, profiles, and airport connections:
    - **agent_heartbeats** — agent_id (PK), last_heartbeat, status (online/offline/busy/idle), metadata (JSONB), updated_at
    - **agent_profiles** — agent_id (PK), description, services_offered (JSONB), endpoints (JSONB), tags (text[]), total_actions, approval_rate, listed (bool), updated_at
    - **airport_connections** — connection_id (PK UUID), requester_id, responder_id, action, outcome (success/denied/hitl_required/timeout/error), trust_score_at_time, created_at
    - **airport_mark_stale_offline()** — marks agents offline if heartbeat > 2 minutes stale

## Deploying

### Docker Compose (Local)

```bash
docker-compose up -d
```

### VPS (Manual)

```bash
# Build and deploy to a VPS
cd gateway/control-plane && go build -o eyevesa-control cmd/api/main.go
cd gateway/core && cargo build --release
cd adapter/resource-adapter-go && go build -o eyevesa-adapter ./cmd/
```

### Google Cloud Platform (Cloud Run)

eyeVesa can be deployed to GCP using **Cloud Run** + **Cloud SQL** with VPC-internal networking:

| GCP Resource | eyeVesa Service | Notes |
|---|---|---|
| Cloud Run | gateway-core | Rust proxy, auto-scaling |
| Cloud Run | gateway-control | Go API server, auto-scaling |
| Cloud Run | resource-adapter | Go MCP adapter |
| Cloud SQL | PostgreSQL 16 + pgvector | Private IP, VPC-peered |
| Artifact Registry | Docker images | Built via `cloudbuild.yaml` |
| Secret Manager | DB password, JWT secret, Ed25519 key | Auto-populated by deploy script |
| VPC + Connector | Private networking | Cloud Run ↔ Cloud SQL |


## Development Status

**Phase 2 — Core Complete, Integration In Progress**

### Working

| Component | Status |
|-----------|--------|
| Agent & Resource CRUD (register, get, list) | Working |
| Authorization with 3-tier OPA (embedded, external, local fallback) | Working |
| HITL approval workflow with multi-layer escalation | Working |
| Notification backends (Slack, PagerDuty, Webhook) | Working |
| Delegation with chain validation (max depth 3) | Working |
| Ed25519 signing and verification | Working |
| Audit logging with signatures + integrity verification | Working |
| PTV (Prove-Transform-Verify) identity attestation | Working |
| SPIRE/SPIFFE dual-provider (SPIRE → local fallback) | Working |
| Behavioral embeddings (pgvector) + anomaly detection | Working |
| LLM service (OpenAI/Anthropic with graceful fallback) | Working |
| Budget metering and rate limiting | Working |
| Multi-tenant CRUD + approver management | Working |
| gRPC server (all 7 RPCs) | Working |
| Rust gateway proxy (plaintext/TLS/mTLS) | Working |
| MCP protocol handling (initialize, tools/call, resources) | Working |
| SDK connect, discover, invoke, delegate | Working |
| Adapter MCP server + gateway registration | Working |
| Auth middleware (API key, JWT, SSO stubs) | Working |
| Airport: heartbeat, profile, search, online, connections, health | Working |
| Airport: auto-registration on agent create | Working |
| Airport: connection logging on authorize | Working |
| Airport: heartbeat cleanup (stale → offline) | Working |
| CLI airport subcommands | Working |
| All 17 database migrations | Working |

### Partial

| Component | Status | Gap |
|-----------|--------|-----|
| SDK signature on invoke | Partial | Signs payload but doesn't send signature in HTTP headers |
| MCP tools/list via gRPC | Partial | Returns empty array, never queries control plane |
| MCP tools/call on control plane | Partial | Only list methods work; tools/call falls through |
| Adapter tool handlers | Stub | Return hardcoded demo data |
| OPA policy files | Partial | `policies/authz.rego` used in production; `agentid.rego` needs external data |
| SSO/SAML | Partial | SSO challenge/login endpoints exist; SAML assertion parsing is a stub |

### Not Yet Built

- JWT token verification (uses "signature-placeholder")
- CLI tool (partial — airport subcommands exist, other commands not yet built)
- SDK HITL approval query methods
- SDK PTV attestation/bind methods

## License

Apache License 2.0. See [LICENSE](LICENSE).

The open-source license covers the code in this repository. It does not grant access to any hosted International Airport, production GCP environment, private API keys, trademarks, or operator-managed services.
