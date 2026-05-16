# eyeVesa — Identity and Trust Layer for AI Agents

> Connects AI agents to enterprise resources with cryptographic identity, policy enforcement, and audit trails.

---

## Table of Contents

1. [What is eyeVesa](#what-is-eyevesa)
2. [Architecture](#architecture)
3. [How It Works](#how-it-works)
4. [Who Benefits](#who-benefits)
5. [Best Use Case](#best-use-case)
6. [Setup Guide](#setup-guide)
7. [Testing](#testing)
8. [Implementing on the Agent Side (Hermes)](#implementing-on-the-agent-side-hermes)
9. [Implementing on the Enterprise Side](#implementing-on-the-enterprise-side)
10. [HITL: Multi-Layer Approval](#hitl-multi-layer-approval)
11. [LLM Integration](#llm-integration)
12. [CLI Quick Setup (Planned)](#cli-quick-setup-planned)
13. [Functionality Assessment](#functionality-assessment)
14. [Package Reference](#package-reference)

---

## What is eyeVesa

When AI agents (like Hermes, OpenClaw) need to access enterprise systems (databases, Kubernetes, banking APIs), three problems exist:

1. **Identity** — Who is the agent? API keys don't prove identity. JWTs can be stolen.
2. **Authorization** — Should this agent be allowed to do this thing right now? Static RBAC doesn't adapt.
3. **Accountability** — When something goes wrong, prove which agent did what and that the log wasn't tampered with.

eyeVesa is the gateway that solves all three:

```
Agent (SDK) ──mTLS──▶ Gateway ──mTLS──▶ Enterprise Resource (Adapter)
                        │
           ┌────────────┴────────────┐
           │   Registry   Policy    │
           │   PostgreSQL  OPA/Rego │
           │   SPIRE      HITL      │
           │   Audit      Trust     │
           └───────────────────────┘
```

**The one-line pitch**: eyeVesa lets autonomous AI agents act on production systems in seconds while guaranteeing that risky actions require human approval, bad behavior is automatically contained, and every action is cryptographically provable.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ENTERPRISE                               │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐            │
│  │ K8s Adapter │  │  DB Adapter │  │ Slack Adapter │            │
│  │  (Go :8443) │  │  (Go :8443) │  │  (Go :8443)   │            │
│  └──────┬──────┘  └──────┬──────┘  └──────┬───────┘            │
│         └────────────────┼─────────────────┘                     │
│                          │                                      │
│                          ▼                                      │
│              ┌──────────────────────┐                           │
│              │  eyeVesa Gateway     │                           │
│              │                      │                           │
│              │  ┌────────────────┐  │                           │
│              │  │ Gateway Core   │  │                           │
│              │  │ (Rust :9443)   │  │                           │
│              │  │ mTLS, proxy,   │  │                           │
│              │  │ crypto, MCP    │  │                           │
│              │  └────────────────┘  │                           │
│              │                      │                           │
│              │  ┌────────────────┐  │                           │
│              │  │ Control Plane  │  │                           │
│              │  │ (Go :8080)     │  │                           │
│              │  │ REST API,      │  │                           │
│              │  │ HITL, audit    │  │                           │
│              │  └────────────────┘  │                           │
│              └──────────┬───────────┘                           │
│                         │                                       │
│         ┌───────────────┼───────────────┐                       │
│         │               │               │                        │
│         ▼               ▼               ▼                       │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐                 │
│  │ PostgreSQL │  │   SPIRE    │  │    OPA     │                 │
│  │ +pgvector │  │ :8081/:8090│  │   :8181    │                 │
│  │  :5432     │  │ (identity) │  │  (policy)  │                 │
│  └────────────┘  └────────────┘  └────────────┘                 │
│                                                                  │
│  ┌──────────────────────────────────────────────┐               │
│  │ Agent SDK (Rust)                              │               │
│  │ connect() → discover() → invoke() → delegate()│               │
│  └──────────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
1. Agent registers          → POST /v1/agents/register → PostgreSQL
2. Resource registers       → POST /v1/resources/register → PostgreSQL
3. Agent connects via SDK   → mTLS to Gateway Core (:9443)
4. Agent discovers tools    → GET /v1/agents/{id} (finds allowed tools)
5. Agent invokes a tool     → Ed25519-signed MCP request → Gateway Core
6. Gateway verifies sig    → Checks Ed25519 against stored public key
7. Gateway checks policy   → OPA evaluates Rego rules
8. If HITL required        → Write to hitl_approvals → Notify human
9. If allowed              → Proxy MCP request to Resource Adapter
10. Result returns          → Agent receives response + trust score
11. Audit log written       → Ed25519-signed entry in audit_logs
12. Trust score updated     → +0.01 for success, -0.05 for denied
```

### Database Schema

| Table | Purpose |
|---|---|
| `agents` | Agent identities, trust scores, behavioral vectors |
| `resources` | Enterprise resource catalog with risk levels |
| `delegations` | Scoped agent-to-agent delegation chains |
| `audit_logs` | Cryptographically signed action logs |
| `trust_events` | Trust score change history |
| `hitl_approvals` | Human-in-the-loop approval queue |

---

## How It Works

### The Three Layers

Every agent request goes through three decision layers:

```
┌─────────────────────────────────────────────┐
│  Layer 1: AUTO-DENY                         │
│  Hard blocks that no human can override     │
│  - bank_transfer > $5000                    │
│  - agent trust < 0.1                         │
│  - budget exceeded                          │
│  - agent suspended                          │
│  Result: instant DENY, trust -= 0.05        │
└──────────────────────┬──────────────────────┘
                       │ passes
                       ▼
┌─────────────────────────────────────────────┐
│  Layer 2: AUTO-ALLOW                         │
│  Low-risk actions that don't need humans    │
│  - trust > 0.8 + low-risk resource         │
│  - read-only operations                     │
│  - scaling within limits                    │
│  Result: instant ALLOW, trust += 0.01      │
└──────────────────────┬──────────────────────┘
                       │ needs review
                       ▼
┌─────────────────────────────────────────────┐
│  Layer 3: HITL                              │
│  Actions requiring human judgment           │
│  - production deployments                   │
│  - bank transfers > $100                    │
│  - restricted data access                   │
│  Result: PENDING until human approves/denies│
└──────────────────────┬──────────────────────┘
                       │ if escalated
                       ▼
┌─────────────────────────────────────────────┐
│  Layer 4: ESCALATION                         │
│  Critical actions needing 2+ approvals      │
│  - bank transfers > $1000                   │
│  - database schema changes                  │
│  - low-trust agents on high-risk resources  │
│  Result: needs N separate approvals          │
└─────────────────────────────────────────────┘
```

### Trust Scoring

```
Trust starts at 1.0000

Action outcome          Trust change
─────────────────────   ────────────
Successful call         +0.01
Policy denied           -0.05
Budget exceeded         -0.10
Never event violation   BLOCKED (auto-deny)

Trust thresholds:
  ≥ 0.8   → auto-allow for low-risk
  ≥ 0.5   → normal operation, HITL for risky
  ≥ 0.1   → restricted, HITL for everything
  < 0.1   → BLOCKED (Layer 1 auto-deny)
```

### Cryptographic Identity

```
Agent Registration:
  1. Gateway generates Ed25519 keypair
  2. Public key stored in agents table
  3. Private key given to agent (stored in vault/K8s secret)

Every Request:
  1. Agent signs MCP payload with private key
  2. Gateway verifies signature with stored public key
  3. Request is authenticated — cannot be forged

Audit Integrity:
  1. Every audit log entry is signed by gateway's Ed25519 key
  2. Signature covers: log_id + agent_id + resource_id + action + method + status
  3. VerifyIntegrity() can prove logs weren't tampered with
```

### Delegation

```
Hermes (level 0, trust: 0.92)
  └── Worker Agent (level 1, delegated by Hermes)
        │  scope: ["log_search"] only
        │  max_depth: 1 (cannot delegate further)
        │  expires: 1 hour
        │
        └── NOT ALLOWED — delegation_policy prevents depth > 1

Delegation is tracked in the delegations table:
  parent_agent_id, child_agent_id, scope, max_depth, expires_at
```

---

## Who Benefits

### Direct Beneficiaries

| Role | Benefit | Why |
|---|---|---|
| **CISO / Security** | Proof and control | Cryptographic identity, tamper-proof audit, trust degradation |
| **DevOps / SRE** | Sleep and speed | Auto-handle 80% of ops, only woken for HITL approvals |
| **Compliance / Legal** | Defensible evidence | Signed audit trail answers who/what/when/why/who-approved |
| **Enterprise IT** | Centralized control | One registry, one policy engine, one audit source |
| **Agent Developers** | Easy integration | Standard SDK: connect() → discover() → invoke() |
| **Business Leaders** | ROI | Incidents prevented, compliance simplified, risk reduced |

### Beneficiary Fit Score

```
CISO / Security       ████████████████████████████████████████  95/100
DevOps / SRE          ██████████████████████████████████████  90/100
Compliance / Audit    ████████████████████████████████████  85/100
Enterprise IT         ████████████████████████████████  75/100
Agent Developers      ██████████████████████████████  65/100
Business Leaders      ████████████████████████████  60/100
End Users             ██████████████████  45/100
Regulators            █████████████████  40/100
```

### Who Does NOT Benefit

- Human-operated tools (no agent autonomy)
- Low-risk internal tools (no consequence)
- Read-only agents on public data (no trust risk)
- Sandbox/toy projects (overkill)

---

## Best Use Case

**Autonomous AI agents accessing production systems where mistakes cause real damage.**

The three required properties:

1. **Autonomy** — the agent decides and acts without a human in the loop
2. **Consequence** — wrong actions affect real systems (money, data, uptime)
3. **Trust** — the enterprise needs proof of who did what and why

### Top 3 Verticals

**1. DevOps / SRE** (Best fit)

```
3am incident → agent reads logs (auto-allow) → scales up (auto-allow)
→ deploys hotfix (HITL, human taps approve on phone)
→ 4 minutes total, 1 human tap, full audit trail

Without eyeVesa: agent has full prod access (dangerous)
                 or human approves everything (slow)
```

**2. Finance / Banking**

```
Agent transfers $200 (auto-allow, below threshold)
Agent transfers $300 (HITL, between $100-$500)
Agent transfers $6000 (auto-DENY, hard block, no override)

Hard limits, budget tracking, cryptographic audit for compliance
```

**3. Healthcare / Pharma**

```
Agent queries public data (auto-allow)
Agent accesses patient records (HITL, mandatory human approval)
Agent tries bulk data export (auto-DENY for restricted data)

HIPAA compliance, PHI access audit, mandatory HITL
```

### Why eyeVesa vs Alternatives

```
Without eyeVesa:
  Give agents full access       → fast but dangerous
  Require human approval always → safe but slow
  No agents at all             → safe, slow, missing opportunities

With eyeVesa:
  Auto-allow low-risk           → fast
  Auto-deny dangerous           → safe
  HITL for the gray area        → human judgment
  Trust scoring adapts          → agents earn or lose access
  Cryptographic audit            → provable
```

---

## Setup Guide

### Prerequisites

```bash
# Check installations
docker --version        # Docker for infrastructure
go version              # Go 1.22+ for control plane
rustc --version         # Rust for gateway core + SDK

# Install if missing
brew install go rust postgresql@16
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

### Step 1: Start Infrastructure

```bash
cd eyeVesa

# Start PostgreSQL + pgvector, SPIRE server/agent, OPA
docker-compose up -d

# Verify
docker-compose ps
# agentid-postgres       running (healthy)
# agentid-spire-server   running
# agentid-spire-agent    running
# agentid-opa            running
```

What each service does:
- **PostgreSQL + pgvector** — stores agents, resources, audit logs, trust scores
- **SPIRE** — issues cryptographic identities (SVIDs) for mTLS
- **OPA** — evaluates Rego policy rules for authorization

### Step 2: Start Gateway

```bash
# Terminal 1: Gateway core (Rust proxy)
cd gateway/core
cargo run
# "eyeVesa Core Gateway starting... Listening on 0.0.0.0:9443"

# Terminal 2: Control plane (Go API)
cd gateway/control-plane
go run cmd/api/main.go
# "eyeVesa Control Plane starting on :8080"
# "Connected to database"
```

### Step 3: Verify Services

```bash
curl http://localhost:8080/health     # Control plane: "ok"
curl http://localhost:9443/health     # Gateway core: "ok"
curl http://localhost:8181/v1/data/agentid/authz/allow  # OPA: {"result":false}
```

### Step 4: Register an Agent

```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-ops",
    "owner": "org:devops",
    "capabilities": ["infrastructure_read", "infrastructure_write", "deployment"],
    "allowed_tools": ["k8s_deploy", "k8s_scale", "log_search", "incident_create"],
    "max_budget_usd": 500.0,
    "delegation_policy": "single_level",
    "behavioral_tags": ["production", "sre", "high_autonomy"]
  }'
```

Response:
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "public_key": "MCowBQYDK2VwAyEA...",
  "name": "hermes-ops",
  "owner": "org:devops",
  "status": "active",
  "trust_score": 1.0,
  "created_at": "2025-01-15T10:30:00Z"
}
```

Save `agent_id` and `public_key`.

### Step 5: Register a Resource

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8s-api",
    "type": "mcp_server",
    "endpoint": "https://k8s-adapter:8443",
    "auth_method": "mTLS+SVID",
    "capabilities_json": "{\"tools\":[\"k8s_deploy\",\"k8s_scale\",\"log_search\"]}",
    "risk_level": "high",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 50
  }'
```

Save `resource_id`.

### Step 6: Connect Agent with SDK

```rust
use agentid_sdk::{AgentConfig, client::AgentClient};
use ed25519_dalek::SigningKey;
use uuid::Uuid;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let signing_key = SigningKey::generate(&mut rand::rngs::OsRng);
    // TODO: in production, load from secure vault

    let config = AgentConfig {
        agent_id: Uuid::parse_str("YOUR-AGENT-ID")?,
        name: "hermes-ops".into(),
        owner: "org:devops".into(),
        gateway_endpoint: "https://localhost:9443".into(),
    };

    let mut client = AgentClient::connect(config, signing_key).await?;

    // Discover available tools
    let tools = client.discover("deployment").await?;

    // Invoke a tool
    let result = client.invoke(
        &resource_id,
        "k8s_deploy",
        serde_json::json!({"service": "api-server", "image": "v2.1.0"})
    ).await?;

    println!("Success: {}, Trust: {}", result.success, result.trust_score);
    Ok(())
}
```

### Environment Variables (for containers)

```bash
EYEVESA_AGENT_ID=550e8400-e29b-41d4-a716-446655440000
EYEVESA_AGENT_NAME=hermes-ops
EYEVESA_AGENT_OWNER=org:devops
EYEVESA_GATEWAY=https://gateway:9443
EYEVESA_KEY_PATH=/run/secrets/hermes.key
```

---

## Testing

### Rust Unit Tests

```bash
# Gateway core tests (crypto identity signing/verification)
cd gateway/core && cargo test

# SDK tests
cd sdk/agent-sdk-rust && cargo test

# Lint
cd gateway/core && cargo clippy -- -D warnings
```

### Go Tests

```bash
cd gateway/control-plane && go test ./...
go vet ./...
```

### Integration Tests

```bash
# Start everything
docker-compose up -d --wait
sleep 5

# Health checks
curl http://localhost:8080/health
curl http://localhost:9443/health

# Register agent
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent","owner":"org:test","capabilities":["read"],"allowed_tools":["log_search"],"max_budget_usd":50}'

# Register resource
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{"name":"test-mcp","type":"mcp_server","endpoint":"https://localhost:8443"}'

# Test MCP endpoint
curl -X POST http://localhost:9443/v1/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize"}'

# Test OPA policies
curl -X POST http://localhost:8181/v1/data/agentid/authz/allow \
  -H "Content-Type: application/json" \
  -d '{"input":{"agent":{"id":"test","trust_score":1.0,"status":"active","allowed_tools":["read"]},"action":{"method":"call","tool":"read"}}}'
# → {"result":true}

curl -X POST http://localhost:8181/v1/data/agentid/authz/deny \
  -H "Content-Type: application/json" \
  -d '{"input":{"agent":{"id":"test","trust_score":1.0},"action":{"tool":"bank_transfer","params":{"amount":600}}}}'
# → {"result":true} (denied)

curl -X POST http://localhost:8181/v1/data/agentid/authz/requires_hitl \
  -H "Content-Type: application/json" \
  -d '{"input":{"action":{"tool":"bank_transfer","params":{"amount":200}}}}'
# → {"result":true} (requires HITL)

# Tear down
docker-compose down
```

### OPA Policy Testing

```bash
# Start OPA only
docker-compose up -d opa

# Test allow (should return true)
curl -X POST http://localhost:8181/v1/data/agentid/authz/allow \
  -d '{"input":{"agent":{"id":"a1","trust_score":1.0,"status":"active","allowed_tools":["read"]},"action":{"method":"call","tool":"read"}}}'

# Test deny (bank_transfer > 500)
curl -X POST http://localhost:8181/v1/data/agentid/authz/deny \
  -d '{"input":{"agent":{"id":"a1","trust_score":1.0},"action":{"tool":"bank_transfer","params":{"amount":600}}}}'

# Test HITL (bank_transfer > 100)
curl -X POST http://localhost:8181/v1/data/agentid/authz/requires_hitl \
  -d '{"input":{"action":{"tool":"bank_transfer","params":{"amount":200}}}}'
```

---

## Implementing on the Agent Side (Hermes)

### Agent Registration

```bash
# Hermes - high autonomy, production access
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-ops",
    "owner": "org:devops",
    "capabilities": ["infrastructure_read", "infrastructure_write", "deployment"],
    "allowed_tools": ["k8s_deploy", "k8s_scale", "log_search", "incident_create"],
    "max_budget_usd": 500.0,
    "delegation_policy": "single_level",
    "behavioral_tags": ["production", "sre", "high_autonomy"]
  }'
```

### SDK Integration

```rust
use agentid_sdk::{AgentConfig, client::AgentClient};
use ed25519_dalek::SigningKey;

async fn init_hermes() -> Result<AgentClient, Box<dyn std::error::Error>> {
    let signing_key = load_key_from_vault("hermes-ops")?;

    let config = AgentConfig {
        agent_id: uuid::Uuid::parse_str(&env::var("AGENT_ID")?)?,
        name: "hermes-ops".into(),
        owner: "org:devops".into(),
        gateway_endpoint: "https://gateway:9443".into(),
    };

    let client = AgentClient::connect(config, signing_key).await?;
    Ok(client)
}
```

### Agent Decision Loop

```rust
async fn hermes_loop(
    client: &mut AgentClient,
    user_request: &str,
) -> Result<(), Box<dyn std::error::Error>> {

    // 1. LLM reasons about what the user wants
    let plan = llm_plan(user_request).await?;

    for step in plan.steps {
        match step.action {
            ActionType::Discover { capability } => {
                let tools = client.discover(&capability).await?;
                let descriptions = format_tools(&tools);
                llm_refine_plan(&plan, &descriptions).await?;
            }

            ActionType::Invoke { tool, resource_id, params } => {
                match client.invoke(&resource_id, &tool, params).await {
                    Ok(result) => {
                        println!("Success: {}, Trust: {}", result.success, result.trust_score);
                        llm_observe_success(&step, &result).await?;
                    }
                    Err(InvokeError::NotAuthorized(reason)) => {
                        println!("Denied: {}", reason);
                        // LLM finds alternative approach
                        let alternatives = llm_find_alternatives(&reason, &step).await?;
                    }
                    Err(InvokeError::HitlRequired(approval_id)) => {
                        println!("Needs human approval: {}", approval_id);
                        let summary = llm_summarize_for_hitl(&step).await?;
                        submit_hitl_request(&approval_id, &summary).await?;
                    }
                    Err(InvokeError::ResourceUnavailable(reason)) => {
                        println!("Unavailable: {}", reason);
                        llm_handle_failure(&step, &reason).await?;
                    }
                    Err(InvokeError::Gateway(reason)) => {
                        println!("Gateway error: {}", reason);
                    }
                }
            }

            ActionType::Delegate { child_id, scope } => {
                match client.delegate(&child_id, scope, 1).await {
                    Ok(()) => println!("Delegated to {}", child_id),
                    Err(DelegateError::NotAllowed(reason)) => println!("Delegation denied: {}", reason),
                    Err(DelegateError::MaxDepth(reason)) => println!("Max depth exceeded: {}", reason),
                    Err(DelegateError::Gateway(reason)) => println!("Gateway error: {}", reason),
                }
            }
        }
    }
    Ok(())
}
```

### Trust Score Adaptation

```rust
async fn handle_trust_change(client: &mut AgentClient, result: &InvokeResult) {
    match result.trust_score {
        s if s >= 0.8 => println!("Trust healthy: {:.2} — proceed normally", s),
        s if s >= 0.5 => println!("Trust degraded: {:.2} — avoid risky actions", s),
        s if s >= 0.1 => println!("Trust critical: {:.2} — only essential operations", s),
        _ => println!("Trust too low — cannot operate. Request human review."),
    }
}
```

### Integration Difference

```rust
// BEFORE: Direct API call — no identity, no audit, no policy
async fn deploy_service(name: &str, image: &str) -> Result<(), Error> {
    let k8s_client = KubernetesClient::new()?;
    k8s_client.deploy(name, image).await
}

// AFTER: Through eyeVesa — identity, policy, audit, trust, HITL
async fn deploy_service(client: &AgentClient, k8s_id: &Uuid, name: &str, image: &str) -> Result<(), Error> {
    client.invoke(k8s_id, "k8s_deploy", json!({"name": name, "image": image})).await?;
    Ok(())
}
```

---

## Implementing on the Enterprise Side

The enterprise has 5 implementation surfaces:

### Surface 1: Register Resources

```bash
# Kubernetes API
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8s-api",
    "type": "mcp_server",
    "endpoint": "https://k8s-adapter:8443",
    "auth_method": "mTLS+SVID",
    "risk_level": "high",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 50
  }'

# Analytics Database
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "analytics-db",
    "type": "mcp_server",
    "endpoint": "https://db-adapter:8443",
    "risk_level": "medium",
    "data_sensitivity": "internal",
    "rate_limit_per_agent": 200
  }'
```

### Surface 2: Build Resource Adapters

```go
// adapter/k8s-adapter/main.go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
    adapter := server.New("k8s-api", os.Getenv("GATEWAY_ENDPOINT"))

    adapter.RegisterTool("k8s_deploy", func(params json.RawMessage) (interface{}, error) {
        var p struct {
            Service  string `json:"service"`
            Image    string `json:"image"`
            Replicas int    `json:"replicas"`
        }
        json.Unmarshal(params, &p)

        // Defense in depth: even if policy allows, adapter validates
        if p.Replicas > 10 {
            return map[string]interface{}{"error": "max replicas is 10"}, nil
        }

        // Call real Kubernetes API
        // result, err := k8sClient.AppsV1().Deployments(p.Namespace).Create(...)
        return map[string]interface{}{
            "service": p.Service, "image": p.Image,
            "replicas": p.Replicas, "status": "deployed",
        }, nil
    })

    adapter.RegisterTool("log_search", func(params json.RawMessage) (interface{}, error) {
        var p struct { Query string `json:"query"`; Namespace string `json:"namespace"` }
        json.Unmarshal(params, &p)
        return map[string]interface{}{"logs": []string{"line 1", "line 2"}}, nil
    })

    log.Fatal(adapter.Run(context.Background()))
}
```

**Defense in depth**: Even if OPA allows a request, the adapter should still validate parameters. The adapter is the last line of defense.

### Surface 3: Set OPA Policies

```rego
// gateway/control-plane/cmd/policy/rego/agentid.rego

package agentid.authz
import future.keywords.in
import rego.v1

default allow := false

# Layer 1: AUTO-DENY (no override possible)
deny if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 5000.0
}

deny if {
    input.agent.trust_score < 0.1
}

deny if {
    budget_exceeded
}

never_event if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 500.0
}

# Layer 2: AUTO-ALLOW (no human needed)
auto_allow if {
    input.agent.trust_score >= 0.8
    input.resource.risk_level == "low"
    input.action.tool in {"log_search", "alert_send", "dashboard_create"}
}

# Main allow rule
allow if {
    input.agent.id
    input.action.method
    valid_agent
    action_permitted
    no_never_event
}

valid_agent if {
    agent := data.agents[input.agent.id]
    agent.status == "active"
    agent.trust_score >= 0.1
}

action_permitted if {
    tool_name := input.action.tool
    agent := data.agents[input.agent.id]
    tool_name in agent.allowed_tools
}

no_never_event if {
    not never_event_violation
}

never_event_violation if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 500.0
}

# Layer 3: HITL (needs one human approval)
requires_hitl if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 100.0
}

requires_hitl if {
    input.action.tool == "k8s_deploy"
    input.action.params.namespace == "production"
}

requires_hitl if {
    input.resource.data_sensitivity == "restricted"
    input.agent.trust_score < 0.8
}

# Layer 4: ESCALATION (needs senior approval)
requires_escalation if {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 1000.0
}

requires_escalation if {
    input.action.tool == "database_schema_change"
}

requires_escalation if {
    input.agent.trust_score < 0.5
    input.resource.risk_level == "high"
}

budget_exceeded if {
    agent := data.agents[input.agent.id]
    input.action.estimated_cost > agent.max_budget_usd
}
```

### Surface 4: HITL Approval Management

```bash
# List pending approvals
curl http://localhost:8080/v1/hitl/pending | jq .

# Approve a request
curl -X POST http://localhost:8080/v1/hitl/{approval_id}/approve \
  -H "Authorization: Bearer $APPROVER_TOKEN"

# Deny a request
curl -X POST http://localhost:8080/v1/hitl/{approval_id}/deny \
  -H "Authorization: Bearer $APPROVER_TOKEN"
```

### Surface 5: Monitor Audit + Trust

```sql
-- Agents with degrading trust
SELECT agent_id, name, trust_score, status
FROM agents WHERE trust_score < 0.8 ORDER BY trust_score ASC;

-- HITL approvals pending too long
SELECT approval_id, agent_id, action, created_at, expires_at
FROM hitl_approvals WHERE status = 'pending'
  AND created_at < NOW() - INTERVAL '10 minutes';

-- Trust events showing degradation
SELECT a.name, te.event_type, te.trust_delta, te.reason, te.created_at
FROM trust_events te JOIN agents a ON a.agent_id = te.agent_id
WHERE te.trust_delta < 0 ORDER BY te.created_at DESC;

-- Verify audit log integrity
-- Uses Ed25519 verification from audit/logger.go:VerifyIntegrity()
```

---

## HITL: Multi-Layer Approval

### The Four Layers

```
Layer 1: AUTO-DENY — Hard blocks, no override
  - bank_transfer > $5000
  - trust_score < 0.1
  - budget exceeded
  - Result: instant DENY, trust -= 0.05

Layer 2: AUTO-ALLOW — Low-risk, no human needed
  - trust > 0.8 + low-risk resource
  - read-only operations
  - Result: instant ALLOW, trust += 0.01

Layer 3: HITL — Needs one human approval
  - production deployments
  - bank transfers $100-$500
  - restricted data access with trust < 0.8
  - Result: PENDING until human approves/denies

Layer 4: ESCALATION — Needs N approvals
  - bank transfers > $1000
  - database schema changes
  - low-trust agents on high-risk resources
  - Result: needs 2+ separate approvals
```

### Approval Guarantee: No Missed Approvals

```
Request enters HITL:
  1. Write to hitl_approvals table (persistent, won't disappear)
  2. Notify primary approver (Slack DM / email / push)
  3. Start expiry timer (default: 30 minutes)
  4. Start escalation timer (5 min → 15 min → team channel)

Minute 0:    Notify primary approver
Minute 5:    No response → Escalate to secondary approver
Minute 15:   No response → Escalate to team channel (anyone can approve)
Minute 30:   No response → Mark as EXPIRED (never auto-approve)
             Trust -= 0.01 for expired approval
             Agent receives "HITL expired" error

For Layer 4 (escalation):
  Requires 2 separate approvals (e.g., VP Engineering + CTO)
  First approval: marks 1/2 approved, still pending
  Second approval: marks 2/2 approved, request executes
```

### Database Schema for Multi-Layer Approval

```sql
-- Extended hitl_approvals (add to existing table)
ALTER TABLE hitl_approvals ADD COLUMN risk_level VARCHAR(20) DEFAULT 'medium';
ALTER TABLE hitl_approvals ADD COLUMN required_approvals INTEGER DEFAULT 1;
ALTER TABLE hitl_approvals ADD COLUMN current_approvals INTEGER DEFAULT 0;
ALTER TABLE hitl_approvals ADD COLUMN escalation_level INTEGER DEFAULT 0;

-- Approval chain: tracks each person who approved
CREATE TABLE hitl_approval_chain (
    chain_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    approval_id UUID NOT NULL REFERENCES hitl_approvals(approval_id),
    approver_id UUID NOT NULL,
    approval_level INTEGER NOT NULL,
    decision VARCHAR(20) NOT NULL,
    decision_reason TEXT,
    decided_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(approval_id, approval_level)
);

-- Notification log: tracks every notification sent
CREATE TABLE hitl_notifications (
    notification_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    approval_id UUID NOT NULL REFERENCES hitl_approvals(approval_id),
    channel VARCHAR(50) NOT NULL,
    recipient_id UUID NOT NULL,
    sent_at TIMESTAMPTZ DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    escalation_level INTEGER DEFAULT 0
);

CREATE INDEX idx_hitl_notifications_approval ON hitl_notifications(approval_id);
CREATE INDEX idx_hitl_notifications_pending ON hitl_notifications(approval_id)
    WHERE acknowledged_at IS NULL;
```

### HITL Status Flow

```
                    ┌──────────┐
                    │ pending  │  ← request created, notification sent
                    └────┬─────┘
                         │
              ┌──────────┼──────────┐
              │          │          │
              ▼          ▼          ▼
        ┌─────────┐ ┌─────────┐ ┌─────────┐
        │approved │ │  denied │ │ expired │
        └────┬────┘ └────┬────┘ └────┬────┘
             │           │           │
             ▼           ▼           ▼
        Execute      Reject       Reject
        request      request      request
        Trust +0.01  Trust -0.02  Trust -0.01
```

### HITL Scenarios

| Scenario | Layer 1? | Layer 2? | Layer 3? | Layer 4? |
|---|---|---|---|---|
| Read logs | — | Auto-allow | — | — |
| Deploy to staging | — | Auto-allow (trust > 0.8) | — | — |
| Deploy to production | — | — | HITL (1 approval) | — |
| Bank transfer $300 | — | — | HITL (1 approval) | — |
| Bank transfer $6000 | Auto-DENY | — | — | — |
| DB schema change | — | — | HITL (1st) | Escalation (2nd) |
| Low-trust agent + high-risk | — | — | HITL (1st) | Escalation (2nd) |

---

## LLM Integration

### How LLMs Fit In

The LLM is the **brain** (decides what to do). eyeVesa is the **guardrail** (decides if it can).

```
LLM decides WHAT to do:
  "The user wants to deploy v2.1.0. I should call k8s_deploy."

eyeVesa decides IF it can:
  "k8s_deploy is in allowed_tools. Trust is 0.92. Production needs HITL."
```

### LLM Touchpoints

| Touchpoint | Role | Status |
|---|---|---|
| **Reasoning** | LLM decides what tool to call | Agent implements |
| **HITL summarizer** | Generate human-readable approval requests | To be built |
| **Policy translator** | Natural language → Rego rules | To be built |
| **Behavior embedding** | Generate behavior vectors for anomaly detection | To be built |
| **Delegation reasoning** | Auto-scope delegation based on context | To be built |
| **Audit narrator** | Raw logs → human-readable summaries | To be built |

### HITL + LLM (Planned)

When HITL triggers, an LLM generates a summary for the human:

```
Agent: hermes-ops (trust: 0.92)
Action: k8s_deploy
Details: Deploy api-server v2.1.0 to production

LLM Summary:
"Hermes has deployed this service 47 times successfully.
 This is a routine version bump. Recommend: APPROVE."

[Approve]  [Deny]  [View Details]
```

---

## CLI Quick Setup (Planned)

The target experience:

```bash
# Install
curl -sSL https://get.eyevesa.dev | bash

# One command to register an agent
eyevesa init \
  --name hermes-ops \
  --owner org:devops \
  --gateway https://gateway:9443 \
  --capabilities "infrastructure_read,infrastructure_write,deployment" \
  --allowed-tools "k8s_deploy,k8s_scale,log_search" \
  --max-budget 500

# Output:
# ✓ Agent registered: hermes-ops
# ✓ Agent ID: 550e8400-e29b-41d4-a716-446655440000
# ✓ Keypair saved to ~/.eyevesa/keys/hermes-ops.key
# ✓ Config saved to ~/.eyevesa/agents/hermes-ops.toml
# ✓ Gateway connection verified

# Then in code:
let client = AgentClient::from_config("hermes-ops").await?;
client.invoke(&resource_id, "k8s_deploy", params).await?;

# Monitoring
eyevesa trust hermes-ops           # View trust score
eyevesa hitl list                  # List pending approvals
eyevesa audit hermes-ops --limit 10 # View recent actions
eyevesa discover deployment         # Find deployment tools
```

### Config File (~/.eyevesa/agents/hermes-ops.toml)

```toml
[agent]
name = "hermes-ops"
agent_id = "550e8400-e29b-41d4-a716-446655440000"
owner = "org:devops"

[gateway]
endpoint = "https://gateway:9443"
timeout_secs = 30

[identity]
key_path = "~/.eyevesa/keys/hermes-ops.key"

[capabilities]
tools = ["k8s_deploy", "k8s_scale", "log_search", "incident_create"]
delegation_policy = "single_level"
max_budget_usd = 500.0

[trust]
initial_score = 1.0
minimum_score = 0.1
```

---

## Functionality Assessment

### What's Working

| Component | Status |
|---|---|
| Agent registration | Working — creates agent, generates keypair, writes to DB |
| Resource registration | Working — creates resource with defaults, writes to DB |
| MCP JSON-RPC routing | Working — handles initialize, tools/list, resources/list |
| Ed25519 sign/verify | Working — both Rust and Go implementations |
| OPA policy evaluation | Working — calls OPA, parses allow/deny/HITL |
| Audit logging with signatures | Working — signs every log entry |
| Local policy fallback | Working — LocalEvaluate() without OPA |
| Agent listing | Working — GET /v1/agents queries DB |
| Docker compose infrastructure | Working — PostgreSQL+pgvector, SPIRE, OPA |

### What's Stub / Not Working

| Component | Status | What's Missing |
|---|---|---|
| SDK `discover()` | Stub | Returns empty vec, no gateway query |
| SDK `invoke()` | Stub | Signs locally but returns stub response, no HTTP |
| SDK `connect()` | Stub | No mTLS, no HTTP call |
| SDK `delegate()` | Stub | Returns Ok(()) without calling anything |
| Gateway proxy to adapter | Stub | MCP handler returns static response |
| Signature verification in gateway | Missing | No verification of agent signatures |
| mTLS termination | Missing | No rustls/tls in gateway core |
| SPIRE integration | Missing | Config exists, no code calls Workload API |
| `tools/call` in adapter | Partial | Routes MCP methods but doesn't execute handlers |

### Phase 2 — Implemented (Previously Missing)

| Component | Status | API Endpoints |
|---|---|---|
| HITL approval API | Working | POST /v1/hitl/request, GET /pending, GET /{id}, POST /{id}/decide |
| Trust event recording | Working | Written on every authorize call |
| Delegate enforcement | Working | Create, validate, chain, revoke |

### Phase 3 — Implemented (Pro Features, BSL 1.1)

| Component | Status | Details |
|---|---|---|
| Multi-layer HITL escalation | Working | Layer 1-4: auto-deny, auto-allow, HITL, escalation with 2+ approvals |
| HITL approval chains | Working | hitl_approval_chain table, multi-approver flow |
| HITL notifications | Working | Slack, webhook, PagerDuty dispatch with escalation config |
| LLM HITL summaries | Working | POST /v1/llm/hitl-summary/{id} — OpenAI/Anthropic/local fallback |
| LLM audit narratives | Working | POST /v1/llm/audit-narrative — human-readable audit summaries |
| LLM policy translator | Working | POST /v1/llm/policy-translate — English → Rego |
| pgvector behavioral embeddings | Working | POST /v1/behavior/{id}/embedding, GET anomalies, GET similar agents |
| SSO/SAML auth middleware | Working | JWT + API key + SAML SSO with tenant scoping |
| Multi-tenant isolation | Working | tenants table, tenant_id on agents/resources/audit/hitl, plan limits |
| Budget enforcement + metering | Working | agent_spend table, CheckBudget, RecordSpend endpoints |
| Rate limiting | Working | rate_limit_counters table, EnforceRateLimit method |
| Helm charts | Working | deploy/helm/eyevesa/ — gateway-core, control-plane, postgres, OPA |

### Build Priority

```
1.  SDK HTTP transport           — connect(), discover(), invoke() need real HTTP
2.  Gateway signature verify     — verify Ed25519 sigs before processing
3.  Auth middleware               — at minimum API key auth on control plane
4.  mTLS termination              — rustls in gateway core
5.  Gateway proxy forwarding      — route MCP requests to resource adapters
6.  HITL approval API              — POST /v1/hitl/{id}/approve, /deny
7.  Trust event writer            — update trust_score on every action
8.  tools/call in adapter         — execute registered tool handlers
9.  Delegate enforcement          — check delegation table before allowing
10. Rate limiting                  — enforce rate_limit_per_agent
11. Billing metering               — accumulate cost per agent
12. pgvector embeddings            — generate + compare behavior vectors
13. SPIRE integration              — call Workload API for SVIDs
14. OPA data sync                  — push agent/resource data to OPA on change
15. CLI tool                       — eyevesa init, trust, audit, hitl commands
```

---

## Package Reference

| Package | Language | Purpose |
|---|---|---|
| `gateway/core` | Rust | Proxy engine, crypto, mTLS termination, MCP handling |
| `gateway/control-plane` | Go | REST API, registration, HITL, audit, policy |
| `sdk/agent-sdk-rust` | Rust | Client library for AI agents |
| `adapter/resource-adapter-go` | Go | MCP server wrapper for enterprise resources |
| `registry/` | SQL | PostgreSQL migrations with pgvector |
| `gateway/spire/` | Config | SPIRE server/agent configuration |
| `gateway/control-plane/cmd/policy/` | Rego | OPA authorization policies |
| `proto/` | Protobuf | gRPC service definitions |
| `deploy/` | YAML | Docker, K8s, cloud configs |

### Key Files

| File | Purpose |
|---|---|
| `gateway/core/src/proxy/server.rs` | HTTP proxy server |
| `gateway/core/src/proxy/mcp_handler.rs` | MCP JSON-RPC handler |
| `gateway/core/src/crypto/identity.rs` | Ed25519 keypair + sign/verify |
| `gateway/control-plane/cmd/api/main.go` | Control plane API entry point |
| `gateway/control-plane/cmd/api/handlers/agent.go` | Agent registration + listing |
| `gateway/control-plane/cmd/api/handlers/mcp.go` | MCP handler (Go) |
| `gateway/control-plane/internal/policy/opa.go` | OPA client + LocalEvaluate |
| `gateway/control-plane/internal/audit/logger.go` | Signed audit logging |
| `gateway/control-plane/internal/crypto/keys.go` | Ed25519 key generation |
| `sdk/agent-sdk-rust/src/client.rs` | Agent client struct |
| `sdk/agent-sdk-rust/src/connect.rs` | Connection logic |
| `sdk/agent-sdk-rust/src/invoke.rs` | Tool invocation |
| `sdk/agent-sdk-rust/src/discover.rs` | Tool discovery |
| `sdk/agent-sdk-rust/src/delegate.rs` | Delegation logic |
| `adapter/resource-adapter-go/cmd/server/server.go` | MCP server adapter |
| `registry/migrations/001_agents.sql` | Agent identity table |
| `registry/migrations/002_resources.sql` | Resource catalog table |
| `registry/migrations/003_delegations.sql` | Delegation chain table |
| `registry/migrations/004_audit_logs.sql` | Signed audit trail |
| `registry/migrations/005_trust_and_hitl.sql` | Trust events + HITL approvals |
| `gateway/control-plane/cmd/policy/rego/agentid.rego` | OPA policy rules |
| `gateway/spire/server.conf` | SPIRE server config |
| `gateway/spire/agent.conf` | SPIRE agent config |
| `docker-compose.yml` | Full stack infrastructure |

### Quick Reference

```bash
# Start infrastructure
docker-compose up -d

# Start services
cd gateway/core && cargo run              # Terminal 1
cd gateway/control-plane && go run cmd/api/main.go  # Terminal 2

# Health checks
curl localhost:8080/health    # Control plane
curl localhost:9443/health   # Gateway core
curl localhost:8181/v1/data/agentid/authz/allow  # OPA

# Register
curl -X POST localhost:8080/v1/agents/register ...
curl -X POST localhost:8080/v1/resources/register ...

# MCP
curl -X POST localhost:9443/v1/mcp -d '{"jsonrpc":"2.0","id":1,"method":"initialize"}'

# Stop
docker-compose down
```

---

## Monetization: Open Source + Paid Tiers

### Philosophy

```
Open Source:   Get adopted. Get trusted. Get community.
Paid Tiers:   Enterprises pay for what they actually need: scale, compliance, support.
```

The core identity and trust layer is open source because security that isn't transparent isn't trustworthy. Enterprises pay for the operational layer — managed infrastructure, compliance features, and human support.

### Feature Split

```
┌─────────────────────────────────────────────────────────────┐
│                   eyeVesa Community (Free)                   │
│                                                             │
│  ✓ Gateway Core (Rust proxy)                               │
│  ✓ Control Plane (Go API)                                  │
│  ✓ Agent SDK (Rust)                                         │
│  ✓ Resource Adapter (Go)                                    │
│  ✓ Ed25519 cryptographic identity                           │
│  ✓ OPA/Rego policy engine                                   │
│  ✓ Single-layer HITL (basic approve/deny)                   │
│  ✓ Trust scoring (basic formula)                            │
│  ✓ Signed audit logs                                        │
│  ✓ Basic delegation (1-level)                               │
│  ✓ PostgreSQL registry                                      │
│  ✓ Docker Compose deployment                                │
│  ✓ CLI tool (eyevesa init, discover, trust, audit)         │
│  ✓ Community support (GitHub Issues, Discord)              │
│                                                             │
│  Limit: 5 agents, 10 resources, single tenant              │
└─────────────────────────────────────────────────────────────┘
              │
              │  Enterprises need more
              ▼
┌─────────────────────────────────────────────────────────────┐
│                   eyeVesa Pro ($99/agent/month)              │
│                                                             │
│  Everything in Community +                                  │
│                                                             │
│  ✓ Multi-layer HITL (escalation, approval chains)          │
│  ✓ LLM-powered HITL summaries                              │
│  ✓ LLM-powered policy translator (English → Rego)          │
│  ✓ LLM-powered audit narratives                            │
│  ✓ pgvector behavioral anomaly detection                   │
│  ✓ Budget enforcement + metering                            │
│  ✓ Rate limiting per agent                                  │
│  ✓ Multi-level delegation (depth > 1)                      │
│  ✓ HITL via Slack / Teams / PagerDuty                      │
│  ✓ SSO/SAML authentication for human approvers              │
│  ✓ Email/push notifications for HITL                        │
│  ✓ Multi-tenant (team isolation)                            │
│  ✓ Unlimited agents and resources                           │
│  ✓ Kubernetes Helm charts                                   │
│  ✓ Priority support (8h response SLA)                      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
              │
              │  Enterprises need guarantees
              ▼
┌─────────────────────────────────────────────────────────────┐
│                   eyeVesa Enterprise (Custom Pricing)       │
│                                                             │
│  Everything in Pro +                                        │
│                                                             │
│  ✓ eyeVesa Cloud (managed, no infrastructure to run)        │
│  ✓ SOC 2 Type II compliance                                │
│  ✓ HIPAA compliance (for healthcare)                        │
│  ✓ Dedicated tenant (isolated database, isolated OPA)      │
│  ✓ Custom Rego policy development                          │
│  ✓ Custom resource adapter development                      │
│  ✓ On-premise deployment (air-gapped)                      │
│  ✓ HSM key management integration                           │
│  ✓ SIEM integration (Splunk, Datadog, Elastic)             │
│  ✓ Custom trust scoring models                             │
│  ✓ 99.9% SLA with uptime guarantee                         │
│  ✓ Dedicated support engineer                              │
│  ✓ Incident response retainer                               │
│  ✓ Architecture review + onboarding                        │
│  ✓ Multi-region deployment                                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Detailed Feature Comparison

| Feature | Community | Pro | Enterprise |
|---|---|---|---|
| **Identity & Crypto** | | | |
| Ed25519 agent identity | ✓ | ✓ | ✓ |
| Signed audit logs | ✓ | ✓ | ✓ |
| mTLS (SPIRE) | ✓ | ✓ | ✓ |
| HSM key management | — | — | ✓ |
| **Policy** | | | |
| OPA/Rego policies | ✓ | ✓ | ✓ |
| Custom Rego development | — | — | ✓ |
| LLM policy translator | — | ✓ | ✓ |
| Multi-tenant policy isolation | — | ✓ | ✓ |
| **HITL** | | | |
| Basic approve/deny | ✓ | ✓ | ✓ |
| Multi-layer escalation | — | ✓ | ✓ |
| Approval chains (2+ people) | — | ✓ | ✓ |
| Slack/Teams/PagerDuty | — | ✓ | ✓ |
| Email/push notifications | — | ✓ | ✓ |
| LLM summaries for HITL | — | ✓ | ✓ |
| SSO/SAML for approvers | — | ✓ | ✓ |
| **Trust** | | | |
| Basic trust formula | ✓ | ✓ | ✓ |
| pgvector anomaly detection | — | ✓ | ✓ |
| Custom trust models | — | — | ✓ |
| Behavioral embeddings | — | ✓ | ✓ |
| **Delegation** | | | |
| 1-level delegation | ✓ | ✓ | ✓ |
| Multi-level delegation (depth > 1) | — | ✓ | ✓ |
| Delegation graph visualization | — | ✓ | ✓ |
| **Audit** | | | |
| Signed audit logs | ✓ | ✓ | ✓ |
| Audit log search | ✓ | ✓ | ✓ |
| LLM audit narratives | — | ✓ | ✓ |
| SIEM integration | — | — | ✓ |
| Compliance export (SOC 2) | — | — | ✓ |
| **Operations** | | | |
| Docker Compose | ✓ | ✓ | — (use Cloud) |
| Kubernetes Helm | — | ✓ | ✓ |
| Managed Cloud | — | — | ✓ |
| Multi-region | — | — | ✓ |
| On-premise / air-gapped | — | — | ✓ |
| **Scale** | | | |
| Agents | 5 | Unlimited | Unlimited |
| Resources | 10 | Unlimited | Unlimited |
| Tenants | 1 | Multiple | Dedicated |
| **Support** | | | |
| Community (GitHub/Discord) | ✓ | ✓ | ✓ |
| Priority support (8h SLA) | — | ✓ | ✓ |
| Dedicated engineer | — | — | ✓ |
| Incident response retainer | — | — | ✓ |

### Pricing Model

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│  Community                        Free forever                  │
│  ─────────                        ─────────────                  │
│  5 agents max                    Great for:                     │
│  10 resources max                 • Individual developers        │
│  Single tenant                    • Small teams (< 5 agents)    │
│  Community support                • Open source projects         │
│  Docker Compose only             • Testing and evaluation       │
│                                                                  │
│  Pro                             $99/agent/month                │
│  ───                             ─────────────────              │
│  Unlimited agents                Great for:                     │
│  Unlimited resources              • Growing teams                │
│  Multi-tenant                     • Production deployments      │
│  HITL + LLM                      • Compliance requirements      │
│  Anomaly detection               • Multiple agent types         │
│  Priority support                                                 │
│                                                                  │
│  Example: 20 agents = $1,980/month                              │
│                                                                  │
│  Enterprise                      Custom pricing                 │
│  ──────────                      ──────────────                  │
│  Managed cloud                   Great for:                     │
│  SOC 2 / HIPAA                    • Banks, healthcare           │
│  Dedicated infrastructure          • Large enterprises           │
│  HSM integration                   • Regulated industries        │
│  Dedicated support                 • Multi-region deployments   │
│  99.9% SLA                                                         │
│                                                                  │
│  Typical range: $50K - $500K/year                               │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Revenue Streams

```
Primary Revenue (70%):
───────────────────
  Pro subscriptions       $99/agent/month × N agents
  Enterprise contracts    $50K-500K/year per customer

Secondary Revenue (20%):
───────────────────
  Managed Cloud           EyeVesa Cloud (no infra to manage)
                          $199/agent/month (includes infra costs)
  HITL per-approval       $0.01/approval (for high-volume)
  Marketplace commission  15% on tool/service transactions
                          (when agents discover paid resources)

Services Revenue (10%):
───────────────────
  Custom Rego development $15K one-time
  Custom adapter dev      $20K one-time
  Architecture review     $10K one-time
  Onboarding package      $5K one-time
  Training workshops      $3K/day
```

### What Makes the Open Source Version Good Enough

The Community tier is deliberately generous because the strategy is **adoption first, monetize operational complexity**:

```
Community gets you STARTED:
  ✓ Full identity layer (Ed25519, mTLS, SPIRE)
  ✓ Full policy engine (OPA/Rego)
  ✓ Full audit trail (signed logs)
  ✓ Basic HITL (approve/deny)
  ✓ Basic trust scoring
  ✓ 5 agents, 10 resources

Enterprise PAYS because they NEED:
  → More than 5 agents (immediate bottleneck)
  → Multi-layer HITL with escalation
  → Compliance (SOC 2, HIPAA documentation)
  → LLM summaries (humans won't read raw JSON)
  → Anomaly detection (can't build pgvector models themselves)
  → Slack/Teams integration for approvals
  → Managed infrastructure (don't want to run Postgres/SPIRE/OPA)
  → Guaranteed uptime and support
```

The open source version is production-quality for small teams. The paid version is for organizations that need scale, compliance, and operational features they can't easily build themselves.

### Go-to-Market Strategy

```
Phase 1: Community Growth (Month 0-6)
─────────────────────────────────────
  • Open source core on GitHub (Apache 2.0)
  • Documentation site with tutorials
  • Discord community
  • Blog posts about autonomous agent governance
  • Conference talks (KubeCon, AI conferences)
  • Target: 500+ stars, 50+ active users

Phase 2: Pro Tier Launch (Month 6-12)
──────────────────────────────────────
  • Launch Pro tier with HITL, LLM, anomaly detection
  • Case studies from early Community users
  • "eyeVesa Pro" landing page
  • Free trial: 30 days Pro for existing Community users
  • Target: 10 Pro customers, 100+ agents managed

Phase 3: Enterprise Sales (Month 12-18)
───────────────────────────────────────
  • SOC 2 Type II audit in progress
  • eyeVesa Cloud beta
  • Enterprise sales team (2-3 reps)
  • Partnerships with cloud providers (AWS Marketplace)
  • Target: 3 Enterprise customers, $200K ARR

Phase 4: Marketplace (Month 18-24)
──────────────────────────────────
  • Resource adapter marketplace
  • Enterprise publishes their APIs as adapters
  • Agents discover paid resources through eyeVesa
  • 15% commission on transactions
  • Target: 20+ Enterprise, $1M+ ARR
```

### The Funnel

```
                    ┌───────────────────────┐
                    │  Developer discovers   │
                    │  eyeVesa on GitHub     │
                    │  (Community, Free)     │
                    └───────────┬───────────┘
                                │
                                │  "This is great, but
                                │   I need more agents and
                                │   HITL escalation"
                                │
                    ┌───────────▼───────────┐
                    │  Pro Tier             │
                    │  $99/agent/month       │
                    │  (HITL, LLM, Anomaly) │
                    └───────────┬───────────┘
                                │
                                │  "We need SOC 2,
                                │   managed infra,
                                │   and guaranteed SLA"
                                │
                    ┌───────────▼───────────┐
                    │  Enterprise Tier       │
                    │  Custom pricing        │
                    │  (Cloud, Compliance,   │
                    │   Dedicated Support)   │
                    └───────────────────────┘
```

### Open Source License Strategy

```
Core (Apache 2.0):              Keep community trust
───────────────────
  gateway/core           Rust proxy engine
  gateway/control-plane  Go API server
  sdk/agent-sdk-rust     Rust agent SDK
  adapter/resource-adapter-go  Go MCP adapter
  registry/migrations    PostgreSQL schema

Pro Features (BSL 1.1):        Convert to Apache 2.0 after 3 years
───────────────────
  HITL escalation       Multi-layer approval chains
  HITL Slack/Teams      Notification integrations
  LLM summaries         HITL, audit, policy translator
  Anomaly detection     pgvector behavioral embeddings
  Budget enforcement    Per-agent spend tracking
  Multi-tenant          Team isolation

Enterprise Features (Proprietary):
───────────────────
  eyeVesa Cloud        Managed infrastructure
  SOC 2 compliance     Audit documentation
  HSM integration      Hardware key management
  SIEM integration     Splunk, Datadog, Elastic
```

BSL 1.1 (Business Source License) means the Pro features are source-available but not free for production use. After 3 years, they convert to Apache 2.0 — this keeps the code open while preventing AWS from offering it as a managed service without contributing back.

### Revenue Projections

```
Year 1 (Community Growth):
  Community users: 500
  Pro customers: 0
  Enterprise: 0
  Revenue: $0

Year 2 (Pro Launch):
  Community users: 2,000
  Pro customers: 15 (avg 30 agentseach)
  Enterprise: 1
  Revenue: $15K/month Pro + $100K/year Enterprise = $280K ARR

Year 3 (Enterprise Scale):
  Community users: 5,000
  Pro customers: 50 (avg 50 agents each)
  Enterprise: 5
  Revenue: $247K/month Pro + $500K/year Enterprise = $3.5M ARR

Year 4 (Marketplace):
  Community users: 10,000
  Pro customers: 120
  Enterprise: 15
  Marketplace: 100 transactions/day × $0.01 = $365K/year
  Revenue: $594K/month Pro + $1.5M/year Enterprise + $365K marketplace = $9M ARR
```

### Key Metrics to Track

```
Community Health:
  • GitHub stars
  • Docker pulls
  • npm/cargo downloads
  • Discord members
  • Community PRs

Product-Market Fit:
  • Pro trial → paid conversion rate
  • Agents managed per customer
  • HITL approvals per week (proxy for production use)
  • Trust score distribution (are agents behaving?)

Revenue:
  • Monthly Recurring Revenue (MRR)
  • Annual Recurring Revenue (ARR)
  • Net Revenue Retention (NRR)
  • Customer Acquisition Cost (CAC)
  • Lifetime Value (LTV)
```

---

## License

Community Edition: Apache 2.0
Pro Edition: BSL 1.1 (converts to Apache 2.0 after 3 years)
Enterprise Edition: Proprietary