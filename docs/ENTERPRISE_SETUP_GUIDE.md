# Enterprise Setup Guide: Selling to AI Agents

How an enterprise exposes products and services for AI agents (like Hermes) to discover, authorize, and purchase through eyeVesa.

---

## Overview

```
Hermes (AI Agent)                    Enterprise (Seller)
     │                                      │
     │  1. Discover resources               │
     │  2. Request purchase                 │
     │  3. Gateway checks policy             │
     │  4. Gateway authorizes (or HITL)      │
     │  5. Gateway proxies to adapter ──────▶│  6. Adapter validates & fulfills
     │                                      │  7. Return result + receipt
     │  8. Audit log signed                  │
     │  9. Trust score updated               │
     ◀──────────────────────────────────────│
```

The enterprise side has **5 implementation surfaces**:

1. **Build a Resource Adapter** — MCP server wrapping your product/service
2. **Register the resource** — Tell the gateway what you offer
3. **Configure OPA policy** — Define who can access what, at what limits
4. **Set up HITL approvals** — Human approval for high-value purchases
5. **Monitor audit + trust** — Track every transaction

---

## Surface 1: Build a Resource Adapter

The resource adapter is an MCP server that wraps your enterprise product as callable tools. This is what the AI agent interacts with.

### Minimal Adapter

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"

    "github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
    adapter := server.New(
        os.Getenv("RESOURCE_NAME"),     // e.g. "cloud-marketplace"
        os.Getenv("GATEWAY_ENDPOINT"),   // e.g. "localhost:9443"
    )

    // Register your product as MCP tools
    adapter.RegisterTool("list_products", handleListProducts)
    adapter.RegisterTool("purchase_product", handlePurchase)
    adapter.RegisterTool("check_order", handleCheckOrder)

    // Expose product catalog as MCP resources
    adapter.RegisterResource("catalog://all", handleCatalogResource)
    adapter.RegisterResource("catalog://category/{category}", handleCategoryResource)

    // Provide purchase prompts for agents
    adapter.RegisterPrompt("purchase_summary", handlePurchasePrompt)

    log.Fatal(adapter.Run(context.Background()))
}
```

### Purchase Flow Tool Handler

```go
func handlePurchase(params json.RawMessage) (interface{}, error) {
    var p struct {
        ProductID   string  `json:"product_id"`
        Quantity     int     `json:"quantity"`
        AgentID      string  `json:"agent_id"`
        BudgetMax    float64 `json:"budget_max"`
        ShippingAddr string  `json:"shipping_address"`
    }
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, fmt.Errorf("invalid params: %w", err)
    }

    // Defense in depth: validate even if policy allows
    product, err := catalog.GetProduct(p.ProductID)
    if err != nil {
        return map[string]interface{}{"error": "product not found"}, nil
    }

    totalPrice := product.Price * float64(p.Quantity)

    // Check budget
    if totalPrice > p.BudgetMax {
        return map[string]interface{}{
            "error":  "exceeds budget",
            "total":  totalPrice,
            "budget": p.BudgetMax,
        }, nil
    }

    // Check stock
    if product.Stock < p.Quantity {
        return map[string]interface{}{
            "error":  "insufficient stock",
            "stock":  product.Stock,
            "requested": p.Quantity,
        }, nil
    }

    // Fulfill purchase
    order, err := billing.CreateOrder(p.AgentID, p.ProductID, p.Quantity, p.ShippingAddr)
    if err != nil {
        return nil, err
    }

    return map[string]interface{}{
        "order_id":    order.ID,
        "product":     product.Name,
        "quantity":    p.Quantity,
        "unit_price":  product.Price,
        "total":       totalPrice,
        "status":      "confirmed",
        "tracking":    order.TrackingNumber,
        "receipt_url": order.ReceiptURL,
    }, nil
}
```

### List Products Tool Handler

```go
func handleListProducts(params json.RawMessage) (interface{}, error) {
    var p struct {
        Category string `json:"category"`
        MinPrice float64 `json:"min_price"`
        MaxPrice float64 `json:"max_price"`
    }
    json.Unmarshal(params, &p)

    products := catalog.List(catalog.Filter{
        Category: p.Category,
        MinPrice: p.MinPrice,
        MaxPrice: p.MaxPrice,
    })

    items := make([]map[string]interface{}, len(products))
    for i, prod := range products {
        items[i] = map[string]interface{}{
            "product_id": prod.ID,
            "name":       prod.Name,
            "category":   prod.Category,
            "price":       prod.Price,
            "currency":    prod.Currency,
            "in_stock":    prod.Stock > 0,
            "description": prod.ShortDescription,
        }
    }
    return map[string]interface{}{"products": items}, nil
}
```

### Defense in Depth

Even if the gateway policy allows a transaction, **the adapter must still validate**. The adapter is the last line of defense:

```go
func handlePurchase(params json.RawMessage) (interface{}, error) {
    var p PurchaseParams
    json.Unmarshal(params, &p)

    // 1. Validate product exists
    // 2. Validate quantity is positive
    // 3. Validate price hasn't changed since discovery
    // 4. Validate shipping address
    // 5. Validate agent's budget from gateway context
    // 6. Check inventory
    // 7. Idempotency: check for duplicate order_id
    // 8. Only THEN create the order
}
```

### Environment Variables

```bash
RESOURCE_NAME=cloud-marketplace        # Display name for this adapter
GATEWAY_ENDPOINT=gateway:9443          # Gateway core address
```

---

## Surface 2: Register the Resource

Tell the gateway what your enterprise offers so agents can discover it.

### Example: Cloud Marketplace

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cloud-marketplace",
    "type": "mcp_server",
    "endpoint": "https://marketplace-adapter:8443",
    "auth_method": "mTLS+SVID",
    "capabilities": {
      "tools": ["list_products", "purchase_product", "check_order", "cancel_order", "refund"],
      "resources": ["catalog://all", "catalog://category/*"],
      "prompts": ["purchase_summary"],
      "product_categories": ["compute", "storage", "database", "networking"],
      "currencies": ["USD", "EUR"],
      "max_order_value": 5000.00,
      "requires_shipping": true
    },
    "risk_level": "high",
    "data_sensitivity": "confidential",
    "rate_limit_per_agent": 50
  }'
```

### Example: API Service (Pay-per-Call)

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sentiment-api",
    "type": "mcp_server",
    "endpoint": "https://sentiment-adapter:8443",
    "auth_method": "mTLS+SVID",
    "capabilities": {
      "tools": ["analyze_sentiment", "batch_analyze", "get_usage_stats"],
      "resources": [],
      "prompts": [],
      "pricing": {"per_call": 0.001, "currency": "USD"},
      "max_batch_size": 1000,
      "supported_languages": ["en", "es", "fr", "de", "zh"]
    },
    "risk_level": "low",
    "data_sensitivity": "internal",
    "rate_limit_per_agent": 500
  }'
```

### Example: Database Access (Read-Only)

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "analytics-db",
    "type": "mcp_server",
    "endpoint": "https://db-adapter:8443",
    "auth_method": "mTLS+SVID",
    "capabilities": {
      "tools": ["query", "list_tables", "describe_table"],
      "resources": ["db://schema", "db://tables"],
      "prompts": [],
      "access_mode": "read_only",
      "max_rows": 10000
    },
    "risk_level": "medium",
    "data_sensitivity": "internal",
    "rate_limit_per_agent": 200
  }'
```

### Risk Level & Data Sensitivity Guide

| `risk_level` | When to use | HITL trigger |
|---------------|-------------|-------------|
| `low` | Public data, read-only APIs, free tiers | Never (auto-allow if trust >= 0.8) |
| `medium` | Internal data, paid APIs, limited writes | When trust < 0.5 |
| `high` | Production systems, databases, deployments | Always when trust < 0.8 |
| `critical` | Banking, healthcare, PII, destructive operations | Always (HITL required) |

| `data_sensitivity` | When to use |
|--------------------|-------------|
| `public` | Public web data, open APIs |
| `internal` | Employee data, analytics, metrics |
| `confidential` | Customer data, financial records |
| `restricted` | PII, health records, payment data, secrets |

### List and Verify Resources

```bash
# List all registered resources
curl http://localhost:8080/v1/resources

# Get a specific resource
curl http://localhost:8080/v1/resources/660e8400-e29b-41d4-a716-446655440001
```

---

## Surface 3: Configure OPA Policy

The Rego policy decides who can access what, when, and how much. This is where the enterprise controls purchasing rules.

### Policy File Location

```
gateway/control-plane/cmd/policy/rego/agentid.rego
```

### Example: E-Commerce Policy

Replace the default policy with enterprise purchasing rules:

```rego
package agentid.authz

import future.keywords.in
import rego.v1

default allow := false

# ─── LAYER 1: AUTO-DENY (hard blocks, no override) ───

deny if {
    input.action.tool == "purchase_product"
    input.action.params.price > 5000.0
}

deny if {
    input.agent.trust_score < 0.1
}

deny if {
    budget_exceeded
}

deny if {
    input.action.tool == "purchase_product"
    input.action.params.category == "restricted_item"
}

never_event_violation if {
    input.action.tool == "purchase_product"
    input.action.params.price > 5000.0
}

# ─── LAYER 2: AUTO-ALLOW (no human needed) ───

auto_allow if {
    input.agent.trust_score >= 0.8
    input.resource.risk_level == "low"
    input.action.tool in {"list_products", "check_order"}
}

auto_allow if {
    input.agent.trust_score >= 0.9
    input.action.tool == "purchase_product"
    input.action.params.price <= 50.0
}

# ─── MAIN ALLOW RULE ───

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

# ─── LAYER 3: HITL (needs one human approval) ───

requires_hitl if {
    input.action.tool == "purchase_product"
    input.action.params.price > 100.0
}

requires_hitl if {
    input.action.tool == "purchase_product"
    input.resource.data_sensitivity == "confidential"
    input.agent.trust_score < 0.8
}

requires_hitl if {
    input.action.tool == "purchase_product"
    input.action.params.quantity > 100
}

# ─── LAYER 4: ESCALATION (needs senior approval) ───

requires_escalation if {
    input.action.tool == "purchase_product"
    input.action.params.price > 1000.0
}

requires_escalation if {
    input.action.tool == "cancel_order"
    input.action.params.order_value > 500.0
}

# ─── BUDGET CHECK ───

budget_exceeded if {
    agent := data.agents[input.agent.id]
    input.action.estimated_cost > agent.max_budget_usd
}
```

### Policy Patterns for Different Businesses

#### Digital Products (SaaS, APIs)

```rego
# Low-value digital purchases are auto-allowed
auto_allow if {
    input.action.tool == "purchase_product"
    input.action.params.price <= 10.0
    input.agent.trust_score >= 0.5
}

# Bulk purchases need approval
requires_hitl if {
    input.action.tool == "purchase_product"
    input.action.params.quantity > 50
}
```

#### Physical Products (E-Commerce)

```rego
# Only agents with shipping verification can buy physical goods
deny if {
    input.action.tool == "purchase_product"
    input.action.params.requires_shipping == true
    not input.agent.verified_shipping
}

# High-value items need HITL
requires_hitl if {
    input.action.tool == "purchase_product"
    input.action.params.price > 100.0
}

# Expedited shipping needs approval
requires_hitl if {
    input.action.tool == "purchase_product"
    input.action.params.shipping == "expedited"
}
```

#### Subscription Services

```rego
# Subscription signups are auto-allowed for trusted agents
auto_allow if {
    input.action.tool == "subscribe"
    input.action.params.plan in {"free", "starter"}
    input.agent.trust_score >= 0.5
}

# Premium subscriptions need HITL
requires_hitl if {
    input.action.tool == "subscribe"
    input.action.params.plan in {"professional", "enterprise"}
}
```

---

## Surface 4: Set Up HITL Approvals

When a purchase requires human approval, the enterprise defines who approves and how.

### HITL Flow for Purchases

```
Agent requests purchase ($300 product)
  │
  ├─► Gateway evaluates OPA policy
  │     └── price > $100 → requires_hitl = true
  │
  ├─► Gateway writes to hitl_approvals table
  │
  ├─► Enterprise receives approval request via:
  │     ├── Slack notification
  │     ├── Email notification
  │     ├── Dashboard notification
  │     └── Webhook callback
  │
  ├─► Human reviews:
  │     ├── Agent: hermes-ops (trust: 0.92)
  │     ├── Product: GPU Instance A100
  │     ├── Price: $300.00
  │     ├── Budget remaining: $200.00
  │     └── Previous purchases: 47 successful, 0 denied
  │
  ├─► Human decides:
  │     ├── APPROVE → Gateway executes purchase, trust += 0.01
  │     └── DENY → Gateway rejects, trust -= 0.02
  │
  └─► Audit log signed (non-repudiable)
```

### Checking Pending Approvals

```bash
# List pending HITL approvals (enterprise admin)
curl http://localhost:8080/v1/hitl/pending

# Approve a request
curl -X POST http://localhost:8080/v1/hitl/{approval_id}/approve \
  -H "Authorization: Bearer $APPROVER_TOKEN"

# Deny a request
curl -X POST http://localhost:8080/v1/hitl/{approval_id}/deny \
  -H "Authorization: Bearer $APPROVER_TOKEN"
```

### HITL Approval Table Schema

| Column | Type | Description |
|--------|------|-------------|
| `approval_id` | UUID | Primary key |
| `agent_id` | UUID | Which agent requested |
| `resource_id` | UUID | Which resource |
| `action` | VARCHAR | The tool/action name |
| `params` | JSONB | Full request parameters |
| `status` | VARCHAR | `pending`, `approved`, `rejected` |
| `approver_id` | UUID | Who approved/denied |
| `expires_at` | TIMESTAMPTZ | Auto-expire if no response |

### Expiration Policy

Default escalation timeline:

| Time | Action |
|------|--------|
| 0 min | Notify primary approver |
| 5 min | Escalate to secondary approver |
| 15 min | Escalate to team channel |
| 30 min | Mark as EXPIRED, trust -= 0.01 |

---

## Surface 5: Monitor Audit + Trust

### Querying Audit Logs

```sql
-- All purchases by an agent
SELECT log_id, agent_id, action, params, result_status, trust_score_before, trust_score_after, created_at
FROM audit_logs
WHERE agent_id = '550e8400-e29b-41d4-a716-446655440000'
  AND action LIKE 'purchase%'
ORDER BY created_at DESC;

-- All HITL approvals in the last 24 hours
SELECT approval_id, agent_id, action, status, approver_id, created_at
FROM hitl_approvals
WHERE created_at > NOW() - INTERVAL '24 hours'
ORDER BY created_at DESC;

-- Agents with degrading trust
SELECT agent_id, name, trust_score, status
FROM agents
WHERE trust_score < 0.8
ORDER BY trust_score ASC;

-- Trust events showing degradation
SELECT a.name, te.event_type, te.trust_delta, te.reason, te.created_at
FROM trust_events te
JOIN agents a ON a.agent_id = te.agent_id
WHERE te.trust_delta < 0
ORDER BY te.created_at DESC;

-- Total spend per agent
SELECT a.name, SUM((te.metadata->>'cost')::decimal) as total_spent
FROM trust_events te
JOIN agents a ON a.agent_id = te.agent_id
WHERE te.event_type = 'purchase'
GROUP BY a.name;
```

### Verify Audit Log Integrity

Every audit log entry is signed with the gateway's Ed25519 key:

```
signature = Ed25519.Sign(gatewayPrivateKey, SHA256("<log_id>:<agent_id>:<resource_id>:<action>:<method>:<status>"))
```

To verify:

```bash
curl -X POST http://localhost:8080/v1/verify-signature \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "550e8400-...",
    "message": "<base64-encoded-message>",
    "signature": "<base64-encoded-signature>"
  }'
```

---

## Complete Enterprise Setup Checklist

### Step 1: Deploy Infrastructure

```bash
# Start PostgreSQL, SPIRE, OPA
docker-compose up -d

# Verify services
curl http://localhost:8080/health        # Control plane
curl http://localhost:9443/health        # Gateway core
curl http://localhost:8181/v1/data/agentid/authz/allow  # OPA
```

### Step 2: Configure OPA Policy

Edit `gateway/control-plane/cmd/policy/rego/agentid.rego` with your enterprise purchasing rules (see Surface 3 examples above).

### Step 3: Build Your Adapter

Create a Go module based on the resource adapter template:

```bash
mkdir -p my-enterprise-adapter && cd my-enterprise-adapter

# Initialize Go module
go mod init my-enterprise-adapter

# Add dependency
go get github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go
```

Implement your product tools as `ToolHandler`, `ResourceHandler`, and `PromptHandler` functions (see Surface 1).

### Step 4: Build and Run

```bash
# Build
go build -o marketplace-adapter ./cmd/main.go

# Run
RESOURCE_NAME=cloud-marketplace \
GATEWAY_ENDPOINT=localhost:9443 \
./marketplace-adapter

# Verify
curl http://localhost:8443/health
# → "ok"

curl -X POST http://localhost:8443/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
# → {"protocolVersion":"2024-11-05","capabilities":{...},"serverInfo":{"name":"cloud-marketplace",...}}
```

### Step 5: Register the Resource

```bash
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cloud-marketplace",
    "type": "mcp_server",
    "endpoint": "https://marketplace-adapter:8443",
    "auth_method": "mTLS+SVID",
    "capabilities": {
      "tools": ["list_products", "purchase_product", "check_order", "cancel_order"],
      "product_categories": ["compute", "storage", "database"],
      "max_order_value": 5000.00
    },
    "risk_level": "high",
    "data_sensitivity": "confidential",
    "rate_limit_per_agent": 50
  }'
```

Save the `resource_id` from the response.

### Step 6: Register Agent(s) That Can Purchase

```bash
# Trusted agent with purchasing capability
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-procurement",
    "owner": "org:devops",
    "capabilities": ["procurement", "billing"],
    "allowed_tools": ["list_products", "purchase_product", "check_order", "cancel_order"],
    "max_budget_usd": 1000.00,
    "delegation_policy": "single_level",
    "behavioral_tags": ["procurement", "trusted"]
  }'

# Read-only agent (catalog browsing only)
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-research",
    "owner": "org:research",
    "capabilities": ["research", "catalog_browsing"],
    "allowed_tools": ["list_products", "check_order"],
    "max_budget_usd": 0.00,
    "delegation_policy": "no_chain",
    "behavioral_tags": ["research", "read_only"]
  }'
```

### Step 7: Test the Full Flow

```bash
# Test authorization (should be allowed for procurement agent)
curl -X POST http://localhost:8080/v1/authorize \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "<procurement-agent-id>",
    "resource_id": "<marketplace-resource-id>",
    "action": "purchase_product",
    "params": {
      "price": 250.00,
      "quantity": 1
    }
  }'

# Test authorization (should be denied for research agent)
curl -X POST http://localhost:8080/v1/authorize \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "<research-agent-id>",
    "action": "purchase_product",
    "params": {
      "price": 250.00
    }
  }'

# Test OPA policies
curl -X POST http://localhost:8181/v1/data/agentid/authz/allow \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "agent": {"id": "<procurement-agent-id>", "trust_score": 0.92, "status": "active", "allowed_tools": ["purchase_product"]},
      "action": {"method": "call", "tool": "purchase_product", "params": {"price": 50}}
    }
  }'
# → {"result": true}

# Test HITL trigger (purchase > $100)
curl -X POST http://localhost:8181/v1/data/agentid/authz/requires_hitl \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "action": {"tool": "purchase_product", "params": {"price": 300}}
    }
  }'
# → {"result": true}
```

### Step 8: Monitor

```bash
# Watch audit logs (replace with your agent ID)
curl http://localhost:8080/v1/agents

# Check trust scores
psql -U agentid -d agentid -c "SELECT name, trust_score, status FROM agents ORDER BY trust_score ASC;"

# Check pending HITL approvals
psql -U agentid -d agentid -c "SELECT * FROM hitl_approvals WHERE status = 'pending';"
```

---

## Docker Deployment

### Adapter Dockerfile

```dockerfile
# Build
FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /marketplace-adapter ./cmd/main.go

# Run
FROM debian:bookworm-slim
COPY --from=builder /marketplace-adapter /usr/local/bin/
EXPOSE 8443
CMD ["marketplace-adapter"]
```

### Add to docker-compose.yml

```yaml
  marketplace-adapter:
    build:
      context: ./my-enterprise-adapter
      dockerfile: Dockerfile
    environment:
      RESOURCE_NAME: cloud-marketplace
      GATEWAY_ENDPOINT: gateway-core:9443
      DATABASE_URL: postgres://agentid:agentid_dev@postgres:5432/agentid
    depends_on:
      - gateway-core
    ports:
      - "8443:8443"
```

---

## Transaction Flow: Agent Buys a Product

Here's the complete sequence when Hermes purchases a product:

```
1. Hermes LLM decides: "I need to purchase a GPU instance"
2. Hermes discovers tools: client.discover("purchase") → finds "purchase_product"
3. Hermes invokes: client.invoke(&resource_id, "purchase_product", {product_id: "gpu-a100", quantity: 1, ...})
4. Request reaches Gateway Core (:9443)
5. Gateway verifies Ed25519 signature
6. Gateway evaluates OPA policy:
   ├── Is "purchase_product" in agent.allowed_tools? YES
   ├── Is price > $100? YES → requires_hitl
   ├── Is price > $5000? NO → not a never event
   └── Is budget available? YES
   Result: ALLOWED but REQUIRES HITL
7. Gateway writes to hitl_approvals (status: pending)
8. Enterprise receives approval request
9. Human reviews and APPROVES
10. Gateway proxies MCP request to marketplace-adapter (:8443)
11. Adapter validates product, quantity, budget
12. Adapter creates order, returns result
13. Gateway signs audit log entry
14. Gateway updates trust score: +0.01
15. Hermes receives result with order_id, tracking number, receipt
```

If the human **denies**:
```
9. Human reviews and DENIES
10. Gateway records denial in audit_logs
11. Gateway updates trust score: -0.02
12. Hermes receives "denied" with reason
13. Hermes LLM finds alternative or informs user
```

---

## Security Checklist for Enterprise Sellers

| Area | Action |
|------|--------|
| **Adapter validation** | Validate all inputs even if gateway allows — defense in depth |
| **Idempotency** | Use order IDs to prevent duplicate purchases |
| **Price locking** | Verify price hasn't changed since agent discovered the product |
| **Budget enforcement** | Check `estimated_cost` against `max_budget_usd` in both policy and adapter |
| **Rate limiting** | Set `rate_limit_per_agent` appropriately |
| **Data sensitivity** | Mark resources correctly (`public` → `restricted`) |
| **HITL escalation** | Configure multi-person approval for high-value items |
| **Audit integrity** | Verify Ed25519 signatures on all purchase audit logs |
| **Expiration** | Set HITL `expires_at` to prevent indefinite pending orders |
| **Shipping verification** | Validate shipping addresses before fulfilling physical orders |
| **Refund policy** | Implement `cancel_order` and `refund` tools with appropriate policy rules |