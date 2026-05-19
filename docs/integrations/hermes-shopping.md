# eyeVesa + Hermes: Shopping Agent Integration

## Overview

This guide shows how to connect **Hermes** (Nous Research's autonomous AI agent) to eyeVesa for a **shopping/commerce** use case — where Hermes acts as a personal shopping agent with identity-gated purchasing, budget enforcement, HITL approval for transactions, and full audit trails.

### Why eyeVesa for Shopping?

| Concern | Without eyeVesa | With eyeVesa |
|---|---|---|
| **Identity** | No agent identity — cannot verify who placed the order | Ed25519 identity passport per agent, signed on every action |
| **Authorization** | Any agent with API access can purchase | OPA policy gates: only agents with `purchase` capability can buy |
| **Budget** | No spend limits — agent can drain accounts | Per-agent budget tracking; purchases denied when budget exceeded |
| **Approval** | No way to require human consent for expensive items | HITL escalation: items over $100 require FaceID/password approval |
| **Audit** | No proof of who ordered what | Non-repudiable Ed25519 audit trail for every purchase |
| **Trust** | No adaptive behavior | Trust score adjusts — too many denied purchases lowers trust |
| **Delegation** | Agent shares full API creds | Scoped delegation: Hermes can delegate `browse` but not `purchase` |
| **Skills** | No proof the agent can shop | Skill registry: `comparison-shopping` skill with proficiency endorsements |

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         User (Telegram/Discord/CLI)                      │
│  "Buy me noise-cancelling headphones under $300 from a reputable brand"  │
└─────────────────────────────┬───────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────┐
│              Hermes Agent                     │
│  ┌─────────────────────────────────────┐     │
│  │ LLM reasons about the request        │     │
│  │ 1. Browse products (low-risk)        │     │
│  │ 2. Compare prices (low-risk)        │     │
│  │ 3. Add to cart (medium-risk)         │     │
│  │ 4. Purchase (high-risk → HITL)       │     │
│  └─────────────┬───────────────────────┘     │
│                │                             │
│  ┌─────────────▼───────────────┐             │
│  │ eyeVesa MCP Tools           │             │
│  │ mcp_agentid_gateway_*       │             │
│  └─────────────┬───────────────┘             │
└────────────────┼────────────────────────────┘
                 │
     HTTP/MCP    ▼
┌─────────────────────────────────────────────┐
│           eyeVesa Gateway                    │
│  ┌─────────────────────────────────────┐     │
│  │ 1. Verify Ed25519 identity          │     │
│  │ 2. Evaluate OPA policy               │     │
│  │    - browse → AUTO-ALLOW (trust>0.5) │     │
│  │    - purchase → HITL if >$100        │     │
│  │    - purchase → DENY if budget used up│    │
│  │ 3. Sign audit trail                  │     │
│  │ 4. Return result + trust delta       │     │
│  └─────────────┬───────────────────────┘     │
│                │                             │
│  ┌─────────────▼──────┐  ┌────────────────┐  │
│  │ Control Plane (Go) │  │ Resource       │  │
│  │ OPA · HITL · Audit │  │ Adapter        │  │
│  │ Budget · Skills    │  │ (Shopify API)  │  │
│  └────────────────────┘  └────────────────┘  │
└─────────────────────────────────────────────┘
```

---

## Step 1: Register Shopping Resources

Register the e-commerce platforms as eyeVesa resources:

```bash
# Shopify store — high risk (purchases)
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "shopify-store",
    "type": "mcp_server",
    "endpoint": "https://shop-adapter:8443",
    "auth_method": "mTLS+SVID",
    "risk_level": "high",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 100,
    "required_skills": ["comparison-shopping", "payment-processing"],
    "capabilities_json": {
      "browse": true,
      "search": true,
      "add_to_cart": true,
      "purchase": true,
      "refund": true,
      "track_order": true
    }
  }'

# Price comparison API — low risk (read-only)
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "price-api",
    "type": "mcp_server",
    "endpoint": "https://price-adapter:8443",
    "auth_method": "api_key",
    "risk_level": "low",
    "data_sensitivity": "public",
    "rate_limit_per_agent": 500,
    "capabilities_json": {
      "search": true,
      "compare": true,
      "history": true
    }
  }'

# Payment gateway — critical risk
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "payment-gateway",
    "type": "mcp_server",
    "endpoint": "https://payment-adapter:8443",
    "auth_method": "mTLS+SVID",
    "risk_level": "critical",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 50,
    "required_skills": ["payment-processing"],
    "capabilities_json": {
      "charge": true,
      "refund": true,
      "verify": true
    }
  }'
```

---

## Step 2: Register Hermes as a Shopping Agent

```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-shopper",
    "owner": "org:personal-shopping",
    "capabilities": ["browse", "search", "compare", "add_to_cart", "purchase", "track_order"],
    "allowed_tools": ["browse", "search", "compare", "add_to_cart", "purchase", "refund", "track_order"],
    "max_budget_usd": 500.0,
    "delegation_policy": "single_level",
    "behavioral_tags": ["shopping", "budget_conscious", "requires_approval"]
  }'
```

Save the `agent_id` and `public_key` from the response.

---

## Step 3: Create Shopping Skills

```bash
# Create comparison-shopping skill
curl -X POST http://localhost:8080/v1/skills \
  -H "Content-Type: application/json" \
  -d '{
    "name": "comparison-shopping",
    "description": "Ability to compare products across stores, evaluate reviews, and find best deals",
    "category": "shopping",
    "risk_level": "low",
    "required_trust_min": 0.3,
    "required_proficiency": 2
  }'

# Create payment-processing skill (higher bar)
curl -X POST http://localhost:8080/v1/skills \
  -H "Content-Type: application/json" \
  -d '{
    "name": "payment-processing",
    "description": "Ability to process payments, manage carts, and handle transactions",
    "category": "shopping",
    "risk_level": "critical",
    "required_trust_min": 0.7,
    "required_proficiency": 4
  }'
```

Assign skills to Hermes:

```bash
# Assign comparison-shopping (proficiency 3)
curl -X POST http://localhost:8080/v1/agents/{HERMES_AGENT_ID}/skills \
  -H "Content-Type: application/json" \
  -d '{"skill_id": "SKILL_ID_1", "proficiency": 3}'

# Assign payment-processing (proficiency 2 — needs endorsement)
curl -X POST http://localhost:8080/v1/agents/{HERMES_AGENT_ID}/skills \
  -H "Content-Type: application/json" \
  -d '{"skill_id": "SKILL_ID_2", "proficiency": 2}'
```

Endorse the payment skill (human endorsement raises trust):

```bash
curl -X POST http://localhost:8080/v1/agents/{HERMES_AGENT_ID}/skills/{SKILL_ID_2}/endorse \
  -H "Content-Type: application/json" \
  -d '{
    "endorser_type": "human",
    "endorser_id": "owner@company.com",
    "comment": "Hermes has completed 50+ successful purchases with zero disputes"
  }'
```

---

## Step 4: Configure OPA Policy for Shopping

Edit `policies/authz.rego` to add shopping-specific rules:

```rego
package agentid.authz

default allowed = false
default requires_hitl = false
default trust_delta = -0.05

# ── Shopping Rules ──────────────────────────────────────────────────

# Browsing and searching products: auto-allow for agents with trust > 0.3
allowed {
    input.action == "browse"
    input.trust_score >= 0.3
    requires_hitl == false
    trust_delta := 0.01
}

allowed {
    input.action == "search"
    input.trust_score >= 0.3
    requires_hitl == false
    trust_delta := 0.01
}

allowed {
    input.action == "compare"
    input.trust_score >= 0.3
    requires_hitl == false
    trust_delta := 0.01
}

# Adding to cart: auto-allow for agents with trust > 0.5
allowed {
    input.action == "add_to_cart"
    input.trust_score >= 0.5
    requires_hitl == false
    trust_delta := 0.01
}

# Purchases under $100: auto-allow for agents with trust > 0.7 and budget remaining
allowed {
    input.action == "purchase"
    input.trust_score >= 0.7
    input.params.amount < 100
    budget_remaining
    requires_hitl == false
    trust_delta := 0.01
}

# Purchases $100-$500: require HITL approval
allowed {
    input.action == "purchase"
    input.trust_score >= 0.5
    input.params.amount >= 100
    budget_remaining
    requires_hitl := true
    trust_delta := 0.01
}

# Purchases over $500: always deny (policy limit)
allowed = false {
    input.action == "purchase"
    input.params.amount >= 500
}

# Refunds: always require HITL
allowed {
    input.action == "refund"
    requires_hitl := true
    trust_delta := 0.0
}

# Track orders: auto-allow
allowed {
    input.action == "track_order"
    requires_hitl == false
    trust_delta := 0.005
}

# ── Budget Check ────────────────────────────────────────────────────

budget_remaining {
    input.budget_remaining > 0
}
```

---

## Step 5: Configure Hermes for Shopping

In `~/.hermes/config.yaml`:

```yaml
model: openrouter/anthropic/claude-sonnet-4

terminal:
  backend: docker
  docker_image: "nikolaik/python-nodejs:python3.11-nodejs20"

approvals:
  mode: smart    # LLM assesses risk, auto-approve low-risk

mcp_servers:
  agentid-gateway:
    url: "http://localhost:9443/v1/mcp"
    headers:
      X-Agent-ID: "YOUR-HERMES-AGENT-ID"
      X-API-Key: "ak_live_your_api_key"
    tools:
      include: [tools/list, tools/call, resources/list, prompts/list, skills/list, skills/search]

  shopify-store:
    url: "https://shop-adapter:8443/v1/mcp"
    headers:
      X-Agent-ID: "YOUR-HERMES-AGENT-ID"
    tools:
      include: [browse_products, search_products, add_to_cart, checkout, track_order]

  price-api:
    url: "https://price-adapter:8443/v1/mcp"
    tools:
      include: [search, compare, price_history]
```

In `~/.hermes/.env`:

```bash
EYEVESA_AGENT_ID=YOUR-HERMES-AGENT-ID
EYEVESA_AGENT_NAME=hermes-shopper
EYEVESA_AGENT_OWNER=org:personal-shopping
EYEVESA_GATEWAY=https://gateway.yourcompany.com:9443
EYEVESA_KEY_PATH=/run/secrets/hermes.key
```

Reload MCP:

```
/reload-mcp
```

---

## Step 6: Shopping Conversations

### Low-risk: Browse and Compare (Auto-Allowed)

```
User:  Find me noise-cancelling headphones under $300

Hermes: I'll search for noise-cancelling headphones under $300.
        [eyeVesa: search → AUTO-ALLOW (trust: 0.92)]
        [eyeVesa: compare → AUTO-ALLOW (trust: 0.92)]

        Here are the top 3 options:
        1. Sony WH-1000XM5 — $278 (4.6★, 12,340 reviews)
        2. Bose QC Ultra — $299 (4.5★, 8,921 reviews)
        3. Apple AirPods Max — $289 (4.3★, 5,112 reviews)

        The Sony has the best noise cancellation and longest battery life.
        Want me to purchase one?
```

### Medium-risk: Add to Cart (Auto-Allowed at Trust ≥ 0.5)

```
User:  Add the Sony ones to my cart

Hermes: Adding Sony WH-1000XM5 to your cart.
        [eyeVesa: add_to_cart → AUTO-ALLOW (trust: 0.92)]

        ✅ Added to cart. Total: $278.
        Ready to checkout?
```

### High-risk: Purchase with HITL Approval

```
User:  Buy it

Hermes: This purchase requires your approval since it's over $100.
        [eyeVesa: purchase → HITL REQUIRED (amount: $278)]

        Requesting approval...

        📱 HITL NOTIFICATION SENT TO: owner@company.com (Slack + Push)

        [Waiting for approval... approved via FaceID]

        [eyeVesa: purchase → APPROVED (trust: 0.93, delta: +0.01)]

        ✅ Purchase complete! Order #ORD-2026-4821
        Sony WH-1000XM5 — $278.00
        Estimated delivery: May 22, 2026
```

### Budget Exceeded: Denied

```
User:  Also buy the Bose ones for backup

Hermes: I can't complete that purchase. Your budget has been exceeded.
        [eyeVesa: purchase → DENIED (budget_remaining: $222.00, amount: $299.00)]

        Budget used: $278 of $500.
        Remaining: $222.
        The Bose headphones ($299) exceed your remaining budget.
        Would you like to:
        1. Request a budget increase (requires HITL approval)
        2. Find headphones under $222
        3. Refund the previous order first
```

### Refund: Always Requires HITL

```
User:  Cancel the Sony order

Hermes: I'll request a refund for Order #ORD-2026-4821.
        [eyeVesa: refund → HITL REQUIRED (all refunds require approval)]

        📱 HITL NOTIFICATION SENT TO: owner@company.com

        [Waiting for approval... approved]

        ✅ Refund of $278.00 initiated for Order #ORD-2026-4821.
        Budget restored: $500.00 remaining.
```

---

## Step 7: Python SDK — Automated Shopping Agent

For programmatic control beyond Hermes's MCP integration:

```python
import asyncio
from agentid_sdk import AgentClient, AgentConfig, HitlRequiredError, NotAuthorizedError

HITL_POLL_INTERVAL = 5
HITL_MAX_WAIT = 300

async def shopping_agent():
    config = AgentConfig(
        agent_id="",
        name="hermes-shopper",
        owner="org:personal-shopping",
        gateway_endpoint="http://localhost:9443",
    )

    async with AgentClient(config, api_key="ak_live_abc123") as client:
        await client.connect()
        print(f"Connected! Agent: {client.agent_id}, Trust: {client.trust_score}")

        # ── Browse products (low-risk, auto-allowed) ──
        browse_result = await client.invoke(
            resource_id="shopify-store",
            tool="browse",
            params={"category": "headphones", "max_price": 300},
        )
        print(f"Products: {browse_result.data}")

        # ── Compare prices (low-risk, auto-allowed) ──
        compare_result = await client.invoke(
            resource_id="price-api",
            tool="compare",
            params={"product_ids": ["sony-xm5", "bose-qc-ultra", "airpods-max"]},
        )
        print(f"Comparison: {compare_result.data}")

        # ── Purchase with HITL fallback ──
        purchase_result = await invoke_with_hitl_fallback(
            client,
            resource_id="shopify-store",
            tool="purchase",
            params={"product_id": "sony-xm5", "amount": 278.00},
        )
        print(f"Purchase result: {purchase_result}")

        # ── Track order (low-risk, auto-allowed) ──
        track_result = await client.invoke(
            resource_id="shopify-store",
            tool="track_order",
            params={"order_id": "ORD-2026-4821"},
        )
        print(f"Tracking: {track_result.data}")

        # ── Check skill trust ──
        scores = await client.get_skill_trust(client.agent_id)
        for s in scores:
            print(f"  {s.skill_name}: trust={s.trust_score}")


async def invoke_with_hitl_fallback(
    client: AgentClient,
    resource_id: str,
    tool: str,
    params: dict | None = None,
) -> dict:
    """Try to invoke; if HITL required, request approval and poll."""
    try:
        result = await client.invoke(resource_id, tool, params)
        return result.data
    except HitlRequiredError as e:
        approval = await client.request_approval(
            action=tool,
            reason=f"Shopping purchase requires approval: {e}",
            risk_level="high",
        )
        print(f"🔄 HITL approval requested: {approval.approval_id}")

        elapsed = 0
        while elapsed < HITL_MAX_WAIT:
            status = await client.get_approval_status(approval.approval_id)
            if status in ("approved", "rejected", "expired"):
                break
            await asyncio.sleep(HITL_POLL_INTERVAL)
            elapsed += HITL_POLL_INTERVAL

        if status == "approved":
            result = await client.invoke(resource_id, tool, params)
            return result.data
        return {"error": f"HITL {status}", "approval_id": approval.approval_id}

    except NotAuthorizedError as e:
        return {"error": "NOT_AUTHORIZED", "reason": str(e)}


if __name__ == "__main__":
    asyncio.run(shopping_agent())
```

---

## Step 8: Budget Tracking

Monitor and enforce spending per agent:

```bash
# Check budget before purchase
curl http://localhost:8080/v1/budget/check?agent_id=HERMES_AGENT_ID

# Record a spend
curl -X POST http://localhost:8080/v1/budget/spend \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "HERMES_AGENT_ID", "amount_usd": 278.00, "description": "Sony WH-1000XM5"}'
```

---

## Step 9: Audit Trail

Every shopping action is logged with Ed25519 signatures:

```bash
# Query audit trail for a specific agent
curl -X POST http://localhost:8080/v1/audit \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "HERMES_AGENT_ID", "limit": 50}'

# Response includes:
# - timestamp
# - agent_id + agent_name
# - action (browse, search, purchase, refund)
# - resource_id
# - OPA decision (allowed/denied/hitl_required)
# - trust_score_before and trust_score_after
# - Ed25519 signature (non-repudiable)
# - session_id
```

---

## Decision Flow Summary

```
User: "Buy me headphones under $300"
  │
  ├── Hermes → eyeVesa: browse (resource: shopify-store)
  │     └── OPA: trust=0.92 ≥ 0.3, browsing=low-risk → AUTO-ALLOW
  │
  ├── Hermes → eyeVesa: compare (resource: price-api)
  │     └── OPA: trust=0.92 ≥ 0.3, compare=low-risk → AUTO-ALLOW
  │
  ├── Hermes → eyeVesa: add_to_cart (resource: shopify-store)
  │     └── OPA: trust=0.92 ≥ 0.5, add_to_cart=low-risk → AUTO-ALLOW
  │
  ├── Hermes → eyeVesa: purchase (resource: shopify-store, amount=$278)
  │     └── OPA: $278 ≥ $100 → HITL REQUIRED
  │           └── Notification sent → Human approves → APPROVED
  │           └── Trust: 0.92 → 0.93 (+0.01)
  │           └── Audit: Ed25519 signed ✓
  │
  └── Result: Order confirmed, budget updated, full audit trail
```

---

## Security Best Practices

1. **Budget limits**: Always set `max_budget_usd` per agent. Purchases denied when budget exceeded.

2. **HITL thresholds**: Configure OPA to require HITL for purchases above your comfort level (e.g., $100+).

3. **Never events**: Use OPA `never` rules for categories your agent should never buy (e.g., alcohol, weapons).

4. **Delegation restrictions**: Only delegate `browse` and `search` to sub-agents — never `purchase`:

```bash
# Delegate browsing only to a comparison-shopping sub-agent
curl -X POST http://localhost:8080/v1/delegate \
  -H "Content-Type: application/json" \
  -d '{
    "delegator_id": "HERMES_AGENT_ID",
    "delegatee_id": "comparison-sub-agent",
    "scope": ["browse", "search", "compare"],
    "reason": "Delegating product research only"
  }'
```

5. **Skill requirements**: Require `payment-processing` skill (proficiency ≥ 4) for purchase access. Agents without the skill endorsement can browse but not buy.

6. **Refund policy**: Always require HITL for refunds — prevents both accidental and fraudulent returns.

---

## Airport Integration

Hermes shopping agents can discover other shopping, payment, and logistics agents at the Airport:

```python
from agentid_sdk import HermesIntegration

hermes = HermesIntegration.from_config(
    gateway_endpoint="http://localhost:9443",
    agent_name="hermes-shopping",
    owner="org:acme",
)

await hermes.connect()

# Arrive at the Airport
await hermes.heartbeat("online")
await hermes.update_airport_profile(
    description="Hermes shopping agent — comparison shopping and purchasing",
    tags=["shopping", "comparison", "payment", "hermes"],
    listed=True,
)

# Find payment agents
payment_agents = await hermes.client.airport_search(
    skill="payment-processing",
    min_trust=0.9,
    status="online",
)

# Find logistics agents
logistics_agents = await hermes.client.airport_search(
    capability="shipping",
    tag="logistics",
    status="online",
)

# Check who's available before delegating
online_agents = await hermes.client.airport_list_online()

# After interacting with a payment agent, check connection history
connections = await hermes.client.airport_connections(
    agent_id=payment_agent_id,
    limit=10,
)
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `EYEVESA_AGENT_ID` | Yes | Hermes agent ID from registration |
| `EYEVESA_AGENT_NAME` | Yes | `hermes-shopper` |
| `EYEVESA_AGENT_OWNER` | Yes | `org:personal-shopping` |
| `EYEVESA_GATEWAY` | Yes | Gateway URL (e.g., `https://gateway.yourcompany.com:9443`) |
| `EYEVESA_KEY_PATH` | No | Ed25519 key path for signing |
| `SHOPIFY_ADAPTER_URL` | Yes | Shopify MCP adapter URL |
| `PRICE_ADAPTER_URL` | Yes | Price comparison adapter URL |