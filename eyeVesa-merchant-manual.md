# eyeVesa Merchant Manual

> A guide for enterprises and merchants who want to sell agent-accessible services using eyeVesa.

---

## Table of Contents

1. [What is eyeVesa for Merchants?](#what-is-eyevesa-for-merchants)
2. [Installation](#installation)
3. [Your First Resource Adapter](#your-first-resource-adapter)
4. [Registering Resources](#registering-resources)
5. [Writing OPA Policies](#writing-opa-policies)
6. [Human-in-the-Loop (HITL) Setup](#human-in-the-loop-hitl-setup)
7. [Notifications: Slack, PagerDuty, Push](#notifications-slack-pagerduty-push)
8. [Rate Limiting & Budget Enforcement](#rate-limiting--budget-enforcement)
9. [Multi-Tenant Configuration](#multi-tenant-configuration)
10. [SSO/SAML for Approvers](#ssosaml-for-approvers)
11. [SPIRE & mTLS Setup](#spire--mtls-setup)
12. [Audit, Monitoring & Compliance](#audit-monitoring--compliance)
13. [Production Deployment](#production-deployment)
14. [Pricing Tiers](#pricing-tiers)
15. [CLI Quick Reference](#cli-quick-reference)

---

## What is eyeVesa for Merchants?

If you own enterprise infrastructure — databases, Kubernetes clusters, banking APIs, internal tools — eyeVesa lets you **sell access to AI agents** while staying in complete control.

```
  Agent (Customer)              eyeVesa (You)           Your Service
  ┌──────────────┐    ┌──────────────────────────┐    ┌──────────────┐
  │ Hermes       │    │                          │    │ K8s API      │
  │ Claude       │───▶│ 1. Verify identity       │───▶│ DB Adapter   │
  │ Custom Bot   │    │ 2. Check policy          │    │ Banking API  │
  └──────────────┘    │ 3. HITL if needed        │    └──────────────┘
                       │ 4. Log everything        │
                       │ 5. Rate limit & meter    │
                       └──────────────────────────┘
```

**What you control:**
- Which agents can access your services
- What actions each agent is allowed to perform
- Which actions need human approval
- How much each agent can spend
- Rate limits per agent or tenant
- A complete audit trail of every action

---

## Installation

### Prerequisites

```bash
docker --version     # Docker for running eyeVesa + infrastructure
go version           # Go 1.22+ for building control plane
rustc --version      # Rust for building gateway proxy
```

### Quick Start (Docker Compose)

```bash
# Clone the project
git clone https://github.com/hafizaljohari/eyeVesa.git
cd eyeVesa

# Start everything
docker compose up -d

# Verify all services
docker compose ps
```

Expected output:
```
agentid-postgres       Up (healthy)    0.0.0.0:5432->5432/tcp
agentid-opa            Up              0.0.0.0:8181->8181/tcp
agentid-control        Up              0.0.0.0:8080->8080/tcp, 9090->9090/tcp
agentid-core           Up              0.0.0.0:9443->9443/tcp
agentid-resource-adapter Up            0.0.0.0:8443->8443/tcp
```

### Health Check

```bash
curl http://localhost:8080/health
# {"status":"healthy","components":[{"name":"postgresql","status":"healthy"},{"name":"opa_policy","status":"healthy"}]}

curl http://localhost:9443/health
# ok

curl http://localhost:8181/v1/data
# {"result":{}}
```

---

## Your First Resource Adapter

A **resource adapter** is an MCP server that wraps your internal service so eyeVesa can control access to it.

### Step 1: Create an Adapter

```go
// adapter/my-service/main.go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    
    "github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
    adapter := server.New("my-service", os.Getenv("GATEWAY_ENDPOINT"))

    // Register a tool that agents can call
    adapter.RegisterTool("query_database", func(params json.RawMessage) (interface{}, error) {
        var req struct {
            Query string `json:"query"`
            Limit int    `json:"limit"`
        }
        json.Unmarshal(params, &req)

        // Validate parameters (defense in depth)
        if req.Limit > 1000 {
            return map[string]interface{}{"error": "max limit is 1000"}, nil
        }

        // Call your real service
        // results, err := yourDB.Query(req.Query, req.Limit)
        return map[string]interface{}{
            "results": []string{"row1", "row2"},
            "count":   2,
        }, nil
    })

    // Register a resource that agents can discover
    adapter.RegisterResource("docs://api-reference", "Documentation for the API")

    // Register a prompt template
    adapter.RegisterPrompt("summarize", "Summarize the query results")

    log.Fatal(adapter.Run(context.Background()))
}
```

### Step 2: Configure and Run

```bash
# Build and run
cd adapter/my-service
go build -o my-adapter .
GATEWAY_ENDPOINT=http://localhost:9443 ./my-adapter
```

### Step 3: Deploy with Docker

```dockerfile
# adapter/my-service/Dockerfile
FROM golang:1.25-bookworm AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o /adapter ./cmd/
FROM debian:bookworm-slim
COPY --from=builder /adapter /usr/local/bin/adapter
EXPOSE 8443
CMD ["adapter"]
```

```yaml
# docker-compose.yml
  my-service:
    build: ./adapter/my-service
    environment:
      GATEWAY_ENDPOINT: http://gateway-core:9443
```

---

## Registering Resources

Once your adapter is running, register it with eyeVesa so agents can discover and use it.

### Via CLI

```bash
# Register your service as a resource
cd eyeVesa/cli
./eyevesa resources register \
  --name "production-database" \
  --type mcp_server \
  --endpoint "http://my-service:8443/mcp" \
  --auth-method mTLS+SVID \
  --risk-level high \
  --data-sensitivity restricted \
  --rate-limit 50 \
  --capabilities '{"tools":["query_database","export_data"]}'
```

### Via API

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-database",
    "type": "mcp_server",
    "endpoint": "http://my-service:8443/mcp",
    "auth_method": "mTLS+SVID",
    "risk_level": "high",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 50,
    "capabilities_json": "{\"tools\":[\"query_database\",\"export_data\"]}"
  }'
```

### Resource Fields

| Field | Required | Description | Example |
|-------|----------|-------------|---------|
| `name` | Yes | Human-readable name | `production-database` |
| `type` | Yes | Resource type | `mcp_server` |
| `endpoint` | Yes | Adapter URL | `http://service:8443/mcp` |
| `auth_method` | No | Authentication type | `mTLS+SVID`, `api_key` |
| `risk_level` | No | Risk classification | `low`, `medium`, `high`, `critical` |
| `data_sensitivity` | No | Data classification | `public`, `internal`, `confidential`, `restricted` |
| `rate_limit_per_agent` | No | Max requests per agent | `50` |
| `capabilities` | No | Available tools/features | JSON object |

### Listing Resources

```bash
./eyevesa resources list
# or via API
curl http://localhost:8080/v1/resources
```

---

## Writing OPA Policies

OPA (Open Policy Agent) evaluates every agent request and decides whether it should be allowed, denied, or require human approval.

### Policy File Location

Policies are loaded from `gateway/control-plane/policies/authz.rego` and are hot-reloaded on changes.

### Policy Structure

Every request goes through **four layers**:

```
Layer 1: AUTO-DENY  → Instantly blocked, no override
Layer 2: AUTO-ALLOW → Instantly allowed, no human needed
Layer 3: HITL       → Needs one human approval
Layer 4: ESCALATION → Needs 2+ approvals (senior management)
```

### Example Policy

```rego
package agentid.authz

# Layer 1: AUTO-DENY - These are ALWAYS blocked
deny_always {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 5000.0
}

deny_always {
    input.agent.trust_score < 0.1
}

# Layer 2: AUTO-ALLOW - These are ALWAYS allowed
auto_allow {
    input.agent.trust_score >= 0.8
    input.resource.risk_level == "low"
    input.action.tool in {"log_search", "dashboard_create"}
}

# Layer 3: HITL - Needs human approval
requires_hitl {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 100.0
}

# Layer 4: ESCALATION - Needs 2+ approvals
requires_escalation {
    input.action.tool == "bank_transfer"
    input.action.params.amount > 1000.0
}

requires_escalation {
    input.action.tool == "database_schema_change"
}

# Budget check
budget_exceeded {
    input.action.estimated_cost > input.agent.max_budget_usd
}
```

### Testing Policies

```bash
# Test if an action is allowed
curl -X POST http://localhost:8181/v1/data/agentid/authz/allow \
  -H "Content-Type: application/json" \
  -d '{"input":{"agent":{"id":"a1","trust_score":0.95,"allowed_tools":["query_database"]},"action":{"tool":"query_database"}}}'

# Expected: {"result":true}

# Test if an action requires HITL
curl -X POST http://localhost:8181/v1/data/agentid/authz/requires_hitl \
  -H "Content-Type: application/json" \
  -d '{"input":{"action":{"tool":"bank_transfer","params":{"amount":200}}}}'

# Expected: {"result":true}
```

---

## Human-in-the-Loop (HITL) Setup

HITL ensures risky actions require a human to approve them before execution.

### The Four Decision Layers

```
┌──────────────────────────────────────────────┐
│ Layer 1: AUTO-DENY                            │
│ bank_transfer > $5000  →  instant DENY       │
│ trust_score < 0.1      →  instant DENY        │
│ budget exceeded        →  instant DENY        │
└──────────────────────┬───────────────────────┘
                       │ passes
                       ▼
┌──────────────────────────────────────────────┐
│ Layer 2: AUTO-ALLOW                            │
│ trust > 0.8 + low-risk  →  instant ALLOW      │
│ read-only operations    →  instant ALLOW      │
└──────────────────────┬───────────────────────┘
                       │ needs review
                       ▼
┌──────────────────────────────────────────────┐
│ Layer 3: HITL (1 approval)                    │
│ production deploys    →  human approves/denies│
│ bank transfer > $100  →  human approves/denies│
└──────────────────────┬───────────────────────┘
                       │ if escalated
                       ▼
┌──────────────────────────────────────────────┐
│ Layer 4: ESCALATION (2+ approvals)            │
│ bank transfer > $1000  →  VP + CTO approve    │
│ DB schema changes      →  2 engineers approve │
└──────────────────────────────────────────────┘
```

### Approval Escalation Timeline

```
Minute 0:     Notify primary approver (Slack DM / push)
Minute 5:     No response → Escalate to secondary approver
Minute 15:    No response → Escalate to team channel
Minute 30:    No response → Mark as EXPIRED (never auto-approve)
```

### Managing Approvals via CLI

```bash
# View pending approvals
./eyevesa hitl list

# Approve a request
./eyevesa hitl approve <approval-id>

# Deny a request
./eyevesa hitl deny <approval-id>
```

### Managing Approvals via TUI

```bash
./eyevesa tui
```
- Tab to HITL view
- `↑/↓` to select an approval
- `a` to approve
- `d` to deny

---

## Notifications: Slack, PagerDuty, Push

### Slack Notifications

Set the environment variable on the gateway-control service:

```yaml
environment:
  SLACK_WEBHOOK_URL: https://hooks.slack.com/services/T00/B00/xxxxx
```

### PagerDuty Notifications

```yaml
environment:
  PAGERDUTY_INTEGRATION_KEY: your-pagerduty-key
```

### Push Notifications (Mobile App)

Push notifications use Apple Push Notification Service (APNs) and Firebase Cloud Messaging (FCM):

```yaml
environment:
  APNS_KEY_PATH: /path/to/apns-key.p8
  APNS_KEY_ID: ABC123DEFG
  APNS_TEAM_ID: TEAM123456
  APNS_TOPIC: com.yourapp.bundle
```

### Webhook Notifications

Generic HTTP webhooks are always enabled. Configure the URL in code:

```go
webhookNotifier := hitl.NewWebhookNotifier()
webhookNotifier.AddURL("https://your-webhook-endpoint.com/eyevesa")
```

---

## Rate Limiting & Budget Enforcement

### Rate Limiting

Limit how many requests each agent can make per second:

```yaml
environment:
  RATE_LIMIT_RPS: "100"  # Global max requests per second
```

Rate limits are enforced per-agent using a PostgreSQL-backed counter.

### Budget Enforcement

Set budgets when registering agents to control spending:

```bash
./eyevesa init \
  --name my-agent \
  --owner "my-company" \
  --max-budget 500
```

The budget is checked against `estimated_cost` in each authorization request. Use the budget CLI to monitor spending:

```bash
# Check remaining budget for an agent
./eyevesa budget check --agent-id <agent-id>

# Record a spend
./eyevesa budget spend --agent-id <agent-id> --amount 50 --description "Query execution"
```

---

## Multi-Tenant Configuration

eyeVesa supports isolating customers into separate tenants.

### Creating a Tenant

```bash
curl -X POST http://localhost:8080/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "plan": "pro",
    "max_agents": 100,
    "max_resources": 200
  }'
```

### Adding Approvers

```bash
curl -X POST http://localhost:8080/v1/tenants/{tenantID}/approvers \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "role": "admin"
  }'
```

### Tenant Limits

| Feature | Community | Pro | Enterprise |
|---------|-----------|-----|------------|
| Agents per tenant | 5 | Unlimited | Unlimited |
| Resources per tenant | 10 | Unlimited | Unlimited |
| Tenants | 1 | Multiple | Dedicated |

---

## SSO/SAML for Approvers

SSO allows enterprise approvers to use their company credentials to approve requests.

### SAML Configuration

```yaml
environment:
  SAML_CERT_PATH: /path/to/saml-cert.pem
  SAML_KEY_PATH: /path/to/saml-key.pem
  SAML_IDP_METADATA_URL: https://your-idp.com/metadata.xml
  SSO_BASE_URL: https://eyevesa.yourcompany.com
```

### SSO Flow

```
1. Approver visits:   https://eyevesa.yourcompany.com/v1/auth/sso/login
2. eyeVesa redirects:  → Your identity provider (Okta, Azure AD, OneLogin)
3. User logs in:       → Enters company credentials
4. IDP redirects back:  → eyeVesa verifies SAML assertion
5. JWT issued:          → Approver can now approve/deny requests
```

### API Key Authentication (Alternative)

For simpler setups, use API keys instead of SSO:

```bash
# Create an API key
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Content-Type: application/json" \
  -d '{"name": "admin-key", "role": "admin"}'

# Use the API key
curl http://localhost:8080/v1/agents \
  -H "Authorization: Bearer <api-key>"
```

---

## SPIRE & mTLS Setup

SPIRE provides cryptographic identities for mTLS between agents, gateway, and your adapters.

### Configure SPIRE Server

```ini
# gateway/spire/server.conf
server {
    trust_domain = "agentid.dev"
    data_dir = "/opt/spire/data/server"
    log_level = "DEBUG"

    ca_subject = {
        country = ["US"]
        organization = ["eyeVesa"]
        common_name = "eyeVesa CA"
    }

    federation {
        bundle_endpoint {
            address = "0.0.0.0"
            port = 8443
        }
    }
}
```

### Start SPIRE

```bash
# Already in docker-compose, starts automatically
docker compose up -d spire-server spire-agent
```

### Gateway mTLS Mode

Set the gateway mode to mTLS:

```yaml
environment:
  GATEWAY_MODE: mtls
  BACKEND_TLS_CERT_PATH: /tmp/agentid-gateway.crt
  BACKEND_TLS_KEY_PATH: /tmp/agentid-gateway.key
```

When SPIRE is unavailable, eyeVesa auto-falls back to local development certificates.

---

## Audit, Monitoring & Compliance

### Signed Audit Logs

Every action is recorded with an Ed25519 signature, making logs tamper-proof:

```sql
SELECT log_id, agent_id, action, result_status, trust_score_before, trust_score_after, created_at
FROM audit_logs WHERE agent_id = '9cabb37e-1f2e-447a-a930-7610e17700b4'
ORDER BY created_at DESC LIMIT 10;
```

View via CLI:
```bash
./eyevesa audit <agent-id>
./eyevesa audit <agent-id> --limit 50 --offset 0
```

### Verify Audit Integrity

```bash
./eyevesa verify-signature <agent-id> --message "log entry data" --signature <sig>
```

### Monitor Trust Degradation

```sql
-- Agents whose trust is dropping
SELECT a.name, a.trust_score, te.event_type, te.reason, te.created_at
FROM trust_events te
JOIN agents a ON a.agent_id = te.agent_id
WHERE te.trust_delta < 0
ORDER BY te.created_at DESC;
```

### Behavioral Anomaly Detection (Pro)

Use pgvector to detect anomalous agent behavior:

```bash
# Update behavioral embedding for an agent
curl -X POST http://localhost:8080/v1/behavior/{agentID}/embedding

# Detect anomalies
curl http://localhost:8080/v1/behavior/{agentID}/anomalies

# Find similar agents
curl http://localhost:8080/v1/behavior/{agentID}/similar
```

### LLM-Powered Audit Narratives (Pro)

Generate human-readable audit summaries:

```bash
curl -X POST http://localhost:8080/v1/llm/audit-narrative \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "9cabb37e-1f2e-447a-a930-7610e17700b4",
    "period": "24h"
  }'
```

### Compliance Checklist

| Requirement | Status | Tooling |
|------------|--------|---------|
| Signed audit logs | ✅ Built-in | Ed25519 signatures |
| Access control | ✅ Built-in | OPA/Rego policies + auth middleware |
| HITL approvals | ✅ Built-in | Multi-layer HITL + escalation |
| Rate limiting | ✅ Built-in | Per-agent + global limits |
| Budget enforcement | ✅ Built-in | Spend tracking + budget checks |
| Multi-tenant isolation | ✅ Built-in | Tenant service with plan limits |
| SSO/SAML | ✅ Built-in | SAML + JWT + API keys |
| mTLS with SPIRE | ✅ Built-in | go-spiffe/v2 + rustls |
| Monitoring | ✅ Built-in | Prometheus metrics + health endpoints |
| Audit integrity verification | ✅ Built-in | VerifyIntegrity() function |

---

## Production Deployment

### Kubernetes with Helm

```bash
# Deploy using Helm
helm install eyevesa deploy/helm/eyevesa/ \
  --set postgres.enabled=true \
  --set opa.enabled=true \
  --set gateway-control.replicas=2 \
  --set gateway-core.replicas=2 \
  --set global.auth.enabled=true \
  --set global.auth.jwtSecret=your-secret
```

### Environment Variables Reference

| Variable | Service | Description | Default |
|----------|---------|-------------|---------|
| `DATABASE_URL` | control-plane | PostgreSQL connection string | `postgres://agentid:agentid_dev@localhost:5432/agentid` |
| `OPA_ENDPOINT` | control-plane | OPA server URL | `http://opa:8181` |
| `GATEWAY_MODE` | gateway-core | Proxy mode | `plaintext` |
| `CONTROL_PLANE_ADDR` | gateway-core | gRPC address | `http://gateway-control:9090` |
| `AUTH_ENABLED` | control-plane | Enable authentication | `false` |
| `JWT_SECRET` | control-plane | JWT signing secret | Auto-generated |
| `SLACK_WEBHOOK_URL` | control-plane | Slack notifications | — |
| `PAGERDUTY_INTEGRATION_KEY` | control-plane | PagerDuty notifications | — |
| `RATE_LIMIT_RPS` | control-plane | Global rate limit | `100` |
| `GATEWAY_KEY_PATH` | control-plane | Ed25519 key file | `/tmp/agentid-gateway-ed25519.key` |
| `PTV_KEY_PATH` | control-plane | PTV ECDSA key file | `/tmp/agentid-ptv-ecdsa.key` |
| `SPIRE_ENDPOINT` | control-plane | SPIRE socket path | `localhost:8090` |

---

## Pricing Tiers

| Feature | Community (Free) | Pro ($99/agent/mo) | Enterprise ($50K-$500K/yr) |
|---------|-----------------|---------------------|---------------------------|
| Agents | 5 max | Unlimited | Unlimited |
| Resources | 10 max | Unlimited | Unlimited |
| Tenants | 1 | Multiple | Dedicated |
| HITL | Single-layer | Multi-layer + escalation | Multi-layer + SSO/SAML |
| Notifications | — | Slack, PagerDuty, Push | All + custom webhooks |
| Rate limiting | — | ✓ | ✓ |
| Budget enforcement | — | ✓ | ✓ |
| LLM audit narratives | — | ✓ | ✓ |
| Anomaly detection | — | ✓ | ✓ |
| Kubernetes / Helm | — | ✓ | ✓ |
| mTLS / SPIRE | ✓ | ✓ | ✓ |
| SSO/SAML | — | ✓ | ✓ |
| Compliance support | — | — | SOC 2, HIPAA |
| On-premise / air-gap | — | — | ✓ |
| Support | GitHub/Discord | 8h SLA | Dedicated engineer |

---

## CLI Quick Reference

```bash
# Setup
./eyevesa init --name <agent> --owner <org>    # Register an agent
./eyevesa doctor                                # Check everything is healthy
./eyevesa tui                                   # Interactive dashboard

# Resources
./eyevesa resources list                        # List all resources
./eyevesa resources get <resource-id>           # View resource details
./eyevesa resources register --name <n> --type mcp_server --endpoint <url>
./eyevesa discover                              # Discover available tools

# Agents
./eyevesa agents list                           # List all agents
./eyevesa agents get <agent-id>                 # View agent details
./eyevesa agents trust <agent-id>               # View trust score

# Authorization
./eyevesa authorize --agent-id <id> --action <action> --resource-id <id>

# HITL
./eyevesa hitl list                             # View pending approvals
./eyevesa hitl approve <id>                     # Approve a request
./eyevesa hitl deny <id>                        # Deny a request

# Audit
./eyevesa audit <agent-id>                      # View audit trail
./eyevesa verify-signature <agent-id>           # Verify log integrity

# Delegation
./eyevesa delegate create --parent <p> --child <c> --scope <s>

# Configuration
./eyevesa config show                           # View configuration
./eyevesa budget check --agent-id <id>          # Check remaining budget
./eyevesa tenants list                          # List multi-tenant config
```

---

## Key Ports

| Port | Service | Purpose |
|------|---------|---------|
| 8080 | API Server | REST API for agents, resources, HITL, audit |
| 9090 | gRPC Server | Internal service communication |
| 9443 | Gateway Proxy | MCP proxy for agent connections |
| 8443 | Resource Adapter | Your adapter's MCP server |
| 5432 | PostgreSQL | Database |
| 8181 | OPA | Policy engine |
| 8081 | SPIRE Server | Certificate authority |
| 8090 | SPIRE Agent | Workload identity provider |

---

## Troubleshooting

### "Policy parse error: `if` keyword required"

Your OPA version is newer than the policy syntax. Update your policies to use the `if` keyword:

```rego
# Old syntax
allow {
    input.agent.trust_score > 0.8
}

# New syntax (OPA 0.68+)
allow if {
    input.agent.trust_score > 0.8
}
```

Or pin an older OPA version in docker-compose:
```yaml
image: openpolicyagent/opa:0.68.0
```

### "Migration error: relation already exists"

The database was initialized with migrations but the tracking table doesn't have records. Reset:

```bash
docker compose down -v   # WARNING: deletes all data
docker compose up -d
```

### "Connection refused"

A service isn't running. Check with:
```bash
docker compose ps
docker compose logs <service-name>
```

---

> eyeVesa — Identity and Trust Layer for AI Agents
