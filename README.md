<p align="center">
  <img src="site/logo.svg" alt="eyeVesa" width="320">
</p>

<h1 align="center">eyeVesa</h1>

<p align="center"><em>Identity and trust layer for the agentic economy. Know Your Agent.</em></p>

---

Connects AI agents to enterprise resources with cryptographic identity, policy-based authorization, and non-repudiable audit trails.

## Architecture

```
Agent (SDK) ──mTLS──▶ Gateway Core ──gRPC──▶ Control Plane ──mTLS──▶ Resource (Adapter)
                           │                     │
                      MCP Proxy              Registry  Policy
                     (Rust/Hyper)           (PostgreSQL) (OPA)
                                           SPIRE    Audit    HITL     PTV
```

**Dual-protocol gateway**: The Rust core (port 9443) proxies HTTP/JSON-RPC requests to the Go control plane (port 9090 gRPC, 8080 HTTP) for authorization, registration, and crypto operations. PTV (Prove-Transform-Verify) provides hardware-rooted identity.

## Key Features

- **Ed25519 Identity**: Every agent receives a keypair on registration; signatures verified on each action
- **MCP Compatibility**: Model Context Protocol (JSON-RPC 2.0) for agent-resource communication
- **Policy Engine**: Embedded OPA/Rego authorization with local fallback; defines allowed tools, never-events, and budget limits
- **PTV (Prove-Transform-Verify)**: Hardware-rooted identity attestation with TPM simulation, identity binding, and verification
- **Trust Scoring**: Session-aware dynamic trust scores that adjust per-action based on policy decisions
- **Human-in-the-Loop (HITL)**: High-risk actions require human approval with FaceID/password, with expiry and escalation
- **Non-repudiable Audit**: Every action logged with Ed25519 signature; integrity verification built in
- **Agent Delegation**: Scoped agent-to-agent delegation with depth limits (max 3), chain-of-custody, and revocation
- **SPIRE/SPIFFE**: Workload identity with mTLS for service communication (local dev fallback available)
- **mTLS/TLS**: Rust proxy supports plaintext, TLS, and mTLS modes via `GATEWAY_MODE` env var

## Packages

| Package | Language | Purpose |
|---|---|---|
| `gateway/core` | Rust | MCP proxy, crypto, mTLS termination, PTV identity |
| `gateway/control-plane` | Go | HTTP + gRPC APIs, OPA policy, audit, DB, HITL, PTV |
| `sdk/agent-sdk-rust` | Rust | Client library for AI agents (connect, invoke, discover, delegate) |
| `sdk/agent-sdk-python` | Python | Client library with LangGraph/CrewAI/AutoGen/Claude/OpenAI integrations |
| `sdk/agent-sdk-typescript` | TypeScript | Client library for Node.js agents with Claude/OpenAI framework integrations |
| `adapter/resource-adapter-go` | Go | MCP server wrapper for enterprise resources |
| `proto/agentid.proto` | Protobuf | gRPC service definition (7 RPCs) |
| `registry/migrations/` | SQL | PostgreSQL schema (16 migrations, pgvector) |
| `policies/authz.rego` | Rego | OPA authorization policies |
| `deploy/` | YAML | Docker, K8s, cloud configs |

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

## API Endpoints

### Control Plane (HTTP :8080)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
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

### gRPC (Control Plane :9090)

`GatewayService` with 7 RPCs: `RegisterAgent`, `RegisterResource`, `Authorize`, `VerifySignature`, `GetAgent`, `ListAgents`, `Audit`

### MCP Methods (Resource Adapter :8443)

`initialize`, `tools/list`, `tools/call`, `resources/list`, `resources/read`, `prompts/list`, `prompts/get` (protocol version `2024-11-05`)

## Quick Start

```bash
# Prerequisites: Go 1.22+, Rust 1.82+, PostgreSQL 16+ with pgvector

# Start infrastructure
docker-compose up -d

# Run control plane
cd gateway/control-plane && go run cmd/api/main.go

# Run gateway core (plaintext mode)
cd gateway/core && cargo run

# Run resource adapter
cd adapter/resource-adapter-go && go run ./cmd/ -RESOURCE_NAME=demo-resource

# Register an agent
curl -X POST http://localhost:9443/v1/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","owner":"org:test","allowed_tools":["read","write"]}'

# Authorize an action
curl -X POST http://localhost:9443/v1/auth \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<AGENT_ID>","action":"read","resource_id":"doc-001"}'

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

## Deploying to Google Cloud Platform

eyeVesa can be deployed to GCP using **Cloud Run** + **Cloud SQL** with VPC-internal networking:

| GCP Resource | eyeVesa Service | Notes |
|---|---|---|
| Cloud Run | gateway-core | Rust proxy, auto-scaling |
| Cloud Run | gateway-control | Go API server, auto-scaling |
| Cloud Run | resource-adapter | Go MCP adapter |
| Cloud SQL | PostgreSQL 16 + pgvector | Private IP, VPC-peered |
| Artifact Registry | Docker images | Built from in-tree Dockerfiles |
| Secret Manager | DB password, JWT secret, Ed25519 key | Auto-populated by deploy script |
| VPC + Connector | Private networking | Cloud Run ↔ Cloud SQL |

### Quick Deploy

```bash
# 1. Prerequisites
gcloud auth login
gcloud config set project YOUR_PROJECT_ID

# 2. Initialize (creates artifact registry, secrets, .env.gcp)
bash deploy/scripts/deploy-gcp.sh init

# 3. Review and update deploy/terraform/.env.gcp

# 4. Build and push Docker images
bash deploy/scripts/deploy-gcp.sh build

# 5. Plan infrastructure
bash deploy/scripts/deploy-gcp.sh plan

# 6. Deploy infrastructure
bash deploy/scripts/deploy-gcp.sh apply

# 7. Run database migrations
bash deploy/scripts/deploy-gcp.sh migrate

# 8. Register a test agent
bash deploy/scripts/deploy-gcp.sh register

# 9. Check status
bash deploy/scripts/deploy-gcp.sh status
```

Terraform config: `deploy/terraform/gcp.tf`
Deploy script: `deploy/scripts/deploy-gcp.sh`
Env template: `deploy/terraform/env.gcp.example`

## Database Schema

13 PostgreSQL migrations in `registry/migrations/`:

1. **agents** - Identity registry with public_key, capabilities, allowed_tools, trust_score, delegation_policy, behavioral_tags
2. **resources** - Resource catalog with type, endpoint, auth_method, capabilities (JSONB), risk_level, data_sensitivity
3. **delegations** - Agent-to-agent delegation chains with scope, max_depth, expires_at, revocation support
4. **audit_logs** - Non-repudiable trail with Ed25519 signature, params (JSONB), trust_score_before/after, session_id
5. **trust_events + hitl_approvals** - Trust score changes + human-in-the-loop approval queue with 5-minute expiry
6. **identity_bindings** - PTV identity bindings with hardware attestation, platform, runtime_hash
7. **hitl_escalation** - Multi-layer HITL escalation, approval chains, notification log, escalation config
8. **tenants + approvers** - Multi-tenant isolation with SSO config and approver management
9. **behavioral_embeddings** - pgvector-based behavior vectors, events, and anomaly detection
10. **llm_integration** - HITL summaires, audit narratives, policy translations, LLM config
11. **budget_metering** - Agent spend tracking, rate limit counters
12. **push_tokens** - APNs/FCM device tokens for HITL push notifications
13. **api_keys** - API key authentication for gateway access
14. **skills** - Skill catalog with categories, risk levels, and proficiency thresholds
15. **agent_skills** - Agent-skill assignments with proficiency scores, endorsements, verification
16. **licenses** - License management for agents and tenants
17. **airport** - Agent heartbeats, profiles, and airport_connections (where agents meet)

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
| All 13 database migrations | Working |

### Partial

| Component | Status | Gap |
|-----------|--------|-----|
| Auth middleware | Partial | API keys work; JWT/SAML are stubs; middleware not wired to router |
| SDK signature on invoke | Partial | Signs payload but doesn't send signature in HTTP headers |
| MCP tools/list via gRPC | Partial | Returns empty array, never queries control plane |
| MCP tools/call on control plane | Partial | Only list methods work; tools/call falls through |
| Adapter tool handlers | Stub | Return hardcoded demo data |
| OPA policy files | Partial | `policies/authz.rego` used in production; `agentid.rego` needs external data |

### Not Yet Built

- JWT token verification (uses "signature-placeholder")
- SAML assertion parsing (returns hardcoded claims)
- CLI tool (`eyevesa init`, `eyevesa trust`, `eyevesa audit`)
- SDK HITL approval query methods
- SDK PTV attestation/bind methods

## Learning

See [LEARNING_ROADMAP.md](./LEARNING_ROADMAP.md) for a structured 12-week plan covering Go, Rust, PostgreSQL/pgvector, MCP, SPIRE, OPA, HITL, audit, and Docker/K8s.

## License

Proprietary