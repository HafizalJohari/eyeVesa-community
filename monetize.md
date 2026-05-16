# eyeVesa Monetization Strategy

> How eyeVesa makes money while staying open source.

---

## Table of Contents

1. [The Fundamental Math](#the-fundamental-math)
2. [Who Pays for What](#who-pays-for-what)
3. [Revenue Models](#revenue-models)
4. [Pricing Tiers](#pricing-tiers)
5. [Feature Comparison](#feature-comparison)
6. [Why Per-Agent Pricing Works](#why-per-agent-pricing-works)
7. [Revenue Projections](#revenue-projections)
8. [Marketplace Vision](#marketplace-vision)
9. [Infrastructure Costs](#infrastructure-costs)
10. [Go-to-Market Strategy](#go-to-market-strategy)
11. [Open Source License Strategy](#open-source-license-strategy)

---

## The Fundamental Math

```
Your cost:    Infrastructure + Development time
Your revenue: Subscriptions + Cloud markup + Services

Profit = Revenue - Infrastructure Cost

Infrastructure cost per customer: ~$30-200/month
Revenue per customer:              $99-199/agent/month

You need: Revenue >> Infrastructure Cost
Result:  75-95% margins
```

---

## Who Pays for What

```
WITHOUT eyeVesa (how agents work today):

  Agent Developer                Enterprise
  ┌─────────────┐              ┌─────────────┐
  │ Builds agent │              │ Owns k8s DB  │
  │ Manages API │──API key────▶│ Manages API  │
  │ No audit    │              │ No identity  │
  │ No policy   │              │ No trust     │
  └─────────────┘              └─────────────┘

  Problem: Agent developer manages N API keys
           Enterprise has zero visibility into what agents do
           No standard way to govern agent access


WITH eyeVesa:

  Agent Developer          eyeVesa (YOU)           Enterprise
  ┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
  │ Builds agent │    │ Run infrastructure│    │ Owns k8s DB  │
  │ Uses SDK     │────│ Enforce policy    │────│ Owns API     │
  │ Pays $99/    │    │ Manage identity  │    │ Pays $99/    │
  │ agent/month  │    │ Audit everything  │    │ resource/mo  │
  └─────────────┘    │                   │    └─────────────┘
                     │  YOU PAY:         │
                     │  AWS/GCP infra    │
                     │  Development      │
                     │  Support staff    │
                     │                   │
                     │  YOU EARN:         │
                     │  Subscriptions    │
                     │  Cloud markup     │
                     │  Services         │
                     └──────────────────┘
```

---

## Revenue Models

### Model 1: Self-Hosted (Community + Pro)

```
You ship:     Software (Docker images, Helm charts, binaries)
You host:     NOTHING
Customer:     Runs eyeVesa on their own AWS/GCP/on-prem
You earn:     Pro subscription fees

Customer pays:
  ┌──────────────────────────────────────────────────┐
  │ Their AWS/GCP bill        $200-500/month         │
  │ eyeVesa Pro license       $99/agent/month        │
  └──────────────────────────────────────────────────┘

Your cost:
  ┌──────────────────────────────────────────────────┐
  │ Development               Your time                │
  │ Support staff             $0 (community) or       │
  │                           included in Pro          │
  │ Infrastructure            $0 (customer hosts it)  │
  └──────────────────────────────────────────────────┘

Your margin: ~95% (you're selling software, not infra)

Example:
  50 Pro customers × 20 agents each × $99/agent/month
  = 1,000 agents × $99 = $99,000/month revenue
  Your cost: ~$15,000/month (3 support engineers)
  Your profit: ~$84,000/month (85% margin)
```

### Model 2: eyeVesa Cloud (Enterprise)

```
You ship:     Software + managed infrastructure
You host:     EVERYTHING on your AWS/GCP account
Customer:     Logs into a dashboard, agents connect to your gateway
You earn:     Monthly subscription + infrastructure markup

Customer pays:
  ┌──────────────────────────────────────────────────┐
  │ eyeVesa Cloud             $199/agent/month       │
  │   Includes:                                       │
  │   - Infrastructure (your AWS bill)  ~$50/mo     │
  │   - Software license                 ~$99/mo      │
  │   - Managed service premium         ~$50/mo      │
  └──────────────────────────────────────────────────┘

Your cost:
  ┌──────────────────────────────────────────────────┐
  │ AWS/GCP per customer      $20-80/month            │
  │ Support                   Included                 │
  │ Development               Your time                │
  └──────────────────────────────────────────────────┘

Your margin: ~75% (you're paying for infra)

Example:
  20 Enterprise customers × 50 agents each × $199/agent/month
  = 1,000 agents × $199 = $199,000/month revenue
  Your cost: ~$50,000/month (infra + 5 engineers)
  Your profit: ~$149,000/month (75% margin)
```

### Model 3: Marketplace (Year 2+)

```
You ship:     Platform where agents discover tools
You host:     Gateway + marketplace
Agent dev:    Pays to list tools (or free for Community)
Enterprise:   Pays to use tools
You earn:     Transaction fee on every agent→tool call

Flow:
  Agent developer lists "log_search" tool on marketplace
  Enterprise lists their k8s API on marketplace
  Agent calls log_search through eyeVesa
  eyeVesa takes 5-15% of the transaction

  Enterprise earns:  $0.01/call
  eyeVesa earns:     $0.002/call (20% of $0.01)
  Agent pays:        $0.01/call (or their company pays)

At scale:
  10,000 agents × 1,000 calls/day × $0.01/call
  = $100,000/day total
  eyeVesa takes 20% = $20,000/day = $600,000/month

  Cost: same infra as before, ~$50,000/month at this scale
  Profit: ~$550,000/month
```

---

## Pricing Tiers

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  COMMUNITY                Free forever                       │
│  ─────────                ─────────────                       │
│  Up to 5 agents          Great for:                         │
│  Up to 10 resources       • Individual developers             │
│  Single tenant            • Small teams (< 5 agents)       │
│  Community support        • Open source projects              │
│  Docker Compose only      • Testing and evaluation            │
│                                                              │
│  What's included:                                            │
│  ✓ Gateway Core (Rust proxy)                                │
│  ✓ Control Plane (Go API)                                   │
│  ✓ Agent SDK (Rust)                                          │
│  ✓ Resource Adapter (Go)                                     │
│  ✓ Ed25519 cryptographic identity                           │
│  ✓ OPA/Rego policy engine                                   │
│  ✓ Single-layer HITL (basic approve/deny)                   │
│  ✓ Basic trust scoring (formula)                            │
│  ✓ Signed audit logs                                        │
│  ✓ Basic delegation (1-level)                               │
│  ✓ PostgreSQL registry                                       │
│  ✓ Docker Compose deployment                                │
│  ✓ CLI tool (eyevesa init, discover, trust, audit)          │
│  ✓ Community support (GitHub Issues, Discord)              │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  PRO                     $99/agent/month                    │
│  ───                     ─────────────────                   │
│  Unlimited agents        Great for:                         │
│  Unlimited resources     • Growing teams                    │
│  Multi-tenant            • Production deployments          │
│  HITL + LLM              • Compliance requirements         │
│  Anomaly detection       • Multiple agent types             │
│  Priority support                                          │
│                                                              │
│  Everything in Community +:                                 │
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
│  ✓ Kubernetes Helm charts                                   │
│  ✓ Priority support (8h response SLA)                      │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ENTERPRISE              Custom pricing                     │
│  ──────────              ──────────────                      │
│  Managed cloud          Great for:                          │
│  SOC 2 / HIPAA           • Banks, healthcare               │
│  Dedicated infrastructure  • Large enterprises              │
│  HSM integration           • Regulated industries           │
│  Dedicated support          • Multi-region deployments     │
│  99.9% SLA                                                 │
│  Typical range: $50K - $500K/year                          │
│                                                              │
│  Everything in Pro +:                                       │
│  ✓ eyeVesa Cloud (managed, no infrastructure to run)       │
│  ✓ SOC 2 Type II compliance                                │
│  ✓ HIPAA compliance (for healthcare)                        │
│  ✓ Dedicated tenant (isolated database, isolated OPA)    │
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
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

---

## Feature Comparison

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

---

## Why Per-Agent Pricing Works

```
The customer's perspective:

  "I have 20 AI agents accessing my infrastructure"
  "Each agent costs me $99/month for eyeVesa Pro"
  "Total: $1,980/month"

  Is that worth it?

  1 production outage prevented per year:        ~$50,000 saved
  1 security incident (API key leak) prevented:   ~$100,000 saved
  1 compliance audit without eyeVesa:             ~$20,000 cost
  1 compliance audit with eyeVesa:                 ~$5,000 cost
  Developer time building auth/policy/audit:       ~$200,000/year
  Developer time with eyeVesa SDK:                ~$20,000/year

  Total value: ~$345,000/year
  Cost: $1,980/month × 12 = $23,760/year

  ROI: 14.5x

  The $99/agent/month is cheap compared to the alternatives.
```

---

## Revenue Projections

```
Year 1 (Community Growth):
  Community users: 500
  Pro customers: 0
  Enterprise: 0
  Revenue: $0
  Cost: $10,000-20,000/month (1-2 developers)
  Net: -$120,000-240,000/year (investment phase)

Year 2 (Pro Launch):
  Community users: 2,000
  Pro customers: 15 (avg 30 agents each)
  Enterprise: 1
  Revenue: $15K/month Pro + $100K/year Enterprise = $280K ARR
  Cost: $20,000/month
  Net: +$40K/month

Year 3 (Enterprise Scale):
  Community users: 5,000
  Pro customers: 50 (avg 50 agents each)
  Enterprise: 5
  Revenue: $247K/month Pro + $500K/year Enterprise = $3.5M ARR
  Cost: $50,000/month
  Net: +$240K/month

Year 4 (Marketplace):
  Community users: 10,000
  Pro customers: 120
  Enterprise: 15
  Marketplace: 100 transactions/day × $0.01 = $365K/year
  Revenue: $594K/month Pro + $1.5M/year Enterprise + $365K marketplace = $9M ARR
  Cost: $80,000/month
  Net: +$670K/month

Revenue breakdown by source:
                    Year 1    Year 2      Year 3        Year 4
  ─────────────────────────────────────────────────────────────
  Pro licenses       $0       $180K/yr   $2.5M/yr     $7.1M/yr    (60%)
  Enterprise          $0       $100K/yr    $500K/yr     $1.5M/yr    (17%)
  Cloud hosted        $0       $0         $200K/yr      $800K/yr    (9%)
  Marketplace         $0       $0         $0            $365K/yr    (4%)
  Services/training   $0       $30K/yr    $150K/yr      $300K/yr    (3%)
  ─────────────────────────────────────────────────────────────
  Total ARR           $0       $310K      $3.35M        $10M+
```

---

## Marketplace Vision

```
Year 2+: eyeVesa becomes the app store for AI agent tools

  ┌──────────────────┐
  │  Agent Developer  │
  │  Pays $99/agent/mo│
  │  Discovers tools   │
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐
  │  eyeVesa Gateway  │ ◄── Takes 5-15% commission per call
  │  (the marketplace) │
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐
  │  Enterprise       │
  │  Publishes tool    │
  │  Earns $0.01/call │
  │  Pays $99/res/mo  │
  └──────────────────┘

How the marketplace creates network effects:

  More agents → more tool buyers → more enterprises publish tools
  More tools → more value for agents → more agents join
  More agents + tools → more data for eyeVesa → better trust scoring
  Better trust scoring → more enterprises trust eyeVesa → more adoption

The flywheel:

  ┌──────────────────────────────────────────┐
  │                                          │
  │   More Agents ──▶ More Tool Demand       │
  │        ▲                          │       │
  │        │                          ▼       │
  │   More Value ◀── More Tools Published    │
  │                          │               │
  │                          ▼               │
  │                    More Data             │
  │                          │               │
  │                          ▼               │
  │                  Better Trust Scoring     │
  │                          │               │
  │                          └──────┐        │
  │                                 ▼        │
  │                          More Enterprises │
  └──────────────────────────────────────────┘

Marketplace pricing:
  Tool listing:     Free (Community) or $29/month (Pro)
  Transaction fee:  5-15% of per-call cost
  Enterprise tools: Custom pricing
```

---

## Infrastructure Costs

```
Self-Hosted (Customer Pays):
─────────────────────────────
  Customer runs on their own AWS/GCP/on-prem
  eyeVesa cost: $0 for infrastructure
  Customer cost: $200-500/month depending on scale

eyeVesa Cloud (You Pay):
────────────────────────
  Infrastructure cost per customer:
   1-10 agents:      $30/month     (small DB, 1 pod)
   10-100 agents:   $100/month     (medium DB, 2 pods)
   100-1000 agents: $500/month     (large DB, 3 pods, read replica)
   1000+ agents:    $2,000/month   (RDS multi-AZ, 5+ pods, caching)

  At $199/agent/month (Cloud):
   10 agents  → $1,990 revenue, $30 cost     → 98% margin
   100 agents → $19,900 revenue, $100 cost    → 99% margin
   1000 agents→ $199,000 revenue, $500 cost   → 99.7% margin

  Infrastructure cost is negligible compared to revenue.
  The business is software, not hosting.

Cloud provider pricing (as reference):
  ┌─────────────────────────────────────────────────────┐
  │  Component          Self-Hosted    eyeVesa Cloud  │
  │  ────────────────   ──────────    ─────────────   │
  │  PostgreSQL           $0-30/mo      $0 (included)  │
  │  Compute (2 pods)     $0-60/mo      $0 (included)  │
  │  Load balancer        $0-20/mo      $0 (included)  │
  │  OPA + SPIRE          $0-20/mo      $0 (included)  │
  │  Monitoring           $0-20/mo      $0 (included)  │
  │  ────────────────   ──────────    ─────────────   │
  │  Total infra          $0-150/mo     $0 (you pay)   │
  │                                                     │
  │  What customer sees:                               │
  │  Self-hosted:     They pay AWS + $99/agent/mo       │
  │  Cloud:           They pay $199/agent/mo (all-in)  │
  └─────────────────────────────────────────────────────┘
```

---

## Go-to-Market Strategy

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
  • 5-15% commission on transactions
  • Target: 20+ Enterprise, $1M+ ARR

The Funnel:
──────────
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

---

## Open Source License Strategy

```
Core (Apache 2.0):              Keep community trust
───────────────────
  gateway/core           Rust proxy engine
  gateway/control-plane  Go API server
  sdk/agent-sdk-rust     Rust agent SDK
  adapter/resource-adapter-go  Go MCP adapter
  registry/migrations    PostgreSQL schema
  CLI tool               eyevesa command

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

BSL 1.1 means the Pro features are source-available but not free for production use. After 3 years, they convert to Apache 2.0. This keeps the code open while preventing cloud providers from offering it as a managed service without contributing back.

---

## Key Metrics to Track

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

## Revenue Summary

```
Revenue Source           When        Margin    % of Revenue
──────────────────────────────────────────────────────────
Community Edition        Day 1       0%        $0 (on purpose)
Self-hosted Pro          Month 6     95%       60%
Self-hosted Enterprise   Month 12    85%       25%
eyeVesa Cloud            Month 12    75%       15%
Marketplace commission   Month 18    80%       10% (growing)
Professional services    Month 6     80%       5%
Training/workshops       Month 6     90%       2%

Dominant revenue:        $99/agent/month Pro subscriptions (60%)
Growth revenue:          Cloud + Enterprise contracts (25%)
Upside revenue:          Marketplace commission (15% and growing)
```