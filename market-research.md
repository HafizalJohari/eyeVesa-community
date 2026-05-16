# eyeVesa Market Research

> Does the market need this? Honest assessment.

---

## Table of Contents

1. [Market Reality Right Now](#market-reality-right-now)
2. [Who Has This Pain](#who-has-this-pain)
3. [What Problem This Solves](#what-problem-this-solves)
4. [Competitive Landscape](#competitive-landscape)
5. [The Bull Case](#the-bull-case)
6. [The Bear Case](#the-bear-case)
7. [Honest Assessment](#honest-assessment)
8. [Comparable Companies](#comparable-companies)
9. [Market Size Estimates](#market-size-estimates)

---

## Market Reality Right Now

```
March 2026 — What's actually happening:

1. Most "AI agents" are still chatbots with API access
   They use hardcoded API keys stored in .env files
   Zero identity, zero audit, zero policy

2. A few companies have "agentic AI" in production
   They built custom auth wrappers
   They're reinventing the same things eyeVesa does

3. The enterprise market is watching, not buying yet
   "We're evaluating AI agents" = we're scared of what happens when they go wild
   Security teams are actively blocking AI agent deployments

4. MCP (Model Context Protocol) just standardized in late 2024
   It's new, adoption is early
   But it's gaining momentum fast

5. No standard exists for AI agent identity
   There's no OAuth for autonomous agents
   There's no SAML for machines acting independently
   There's no PCI-DSS-equivalent for AI agent governance
```

---

## Who Has This Pain

### Has Pain Right Now (Will Pay Today)

```
Companies running autonomous AI agents in production:
- Fintech companies with trading agents
- DevOps teams with SRE agents (PagerDuty, etc.)
- Cloud platforms with auto-scaling agents
- Security companies with automated response agents

Estimated: ~500-1,000 companies worldwide
These are the early adopters who feel the pain daily
```

### Will Have Pain Soon (6-12 Months)

```
Companies actively deploying AI agents:
- Banks exploring AI for fraud detection
- Hospitals with clinical decision agents
- E-commerce with inventory management agents
- SaaS companies integrating AI copilots

Estimated: ~5,000-10,000 companies
These are building agents now, will hit the wall soon
```

### Will Have Pain Eventually (2-3 Years)

```
Every company that uses AI:
- When Copilot-type agents get autonomous mode
- When LLMs are allowed to take action, not just suggest
- When agents can book flights, approve expenses, deploy code

Estimated: ~50,000+ companies
This is the hockey stick, but it hasn't happened yet
```

---

## What Problem This Solves

| Problem | eyeVesa Solution | Real Problem? | Will Pay? |
|---|---|---|---|
| "I don't know what my AI agent did last night" | Signed audit trail | Yes, unsolved | Security teams, compliance |
| "My AI agent can do anything it wants" | Policy enforcement (allow/deny/HITL) | Yes, scary | CISOs, DevOps leads |
| "I can't prove to auditors what happened" | Ed25519-signed tamper-proof logs | Yes, audits are expensive | Regulated industries |
| "When the agent goes rogue, I can't stop it" | Trust degradation (auto-block at 0.1) | Yes, keeps CISOs awake | Anyone in production |
| "I need a human to approve risky actions" | HITL with escalation | Yes, production deploys | DevOps, SRE teams |

---

## Competitive Landscape

| Competitor/Alternative | What It Does | Why It's Not Enough |
|---|---|---|
| API gateways (Kong, Apigee) | Rate limiting, auth for APIs | No agent identity, no trust scoring, no HITL, no delegation |
| OAuth/OIDC servers (Keycloak) | Auth for humans | Not designed for autonomous agents, no trust degradation, no behavioral monitoring |
| Service meshes (Istio, Linkerd) | mTLS between services | No policy engine, no HITL, no audit trail. Designed for service-to-service, not agent-to-resource |
| Policy engines (OPA alone) | "Is this allowed?" | Only one piece. No identity, no audit, no trust, no HITL. OPA is already in eyeVesa. |
| Secret managers (HashiCorp Vault) | Secret management | Keys can be stolen, no identity proof, no behavioral audit |
| Observability (Datadog, New Relic) | Monitoring | Tells you WHAT happened, not WHETHER it was authorized or WHO is responsible |
| Custom in-house solutions | Companies build their own | Expensive ($200K+), takes 6-12 months, no standard, hard to maintain |

**None of the above combines identity + policy + trust + HITL + audit into a single purpose-built system for autonomous AI agents. That's eyeVesa's unique position.**

---

## The Bull Case

```
1. TIMING IS RIGHT
   MCP just standardized (Nov 2024)
   Autonomous agents are the #1 AI trend for 2025-2026
   Enterprise AI adoption is accelerating
   Every AI conference talks about "agent safety"

2. NOBODY OWNS THIS SPACE
   There is no "OAuth for AI agents" yet
   There is no dominant identity + policy + trust layer
   First mover has massive advantage in setting the standard

3. REGULATION IS COMING
   EU AI Act requires oversight of high-risk AI systems
   NIST AI Risk Management Framework requires governance
   Financial regulators (SEC, MAS) are studying AI agent risks
   Compliance requirements WILL create demand

4. PAIN IS REAL FOR EARLY ADOPTERS
   Companies deploying autonomous agents ALREADY have this problem
   They're building custom solutions, which is expensive
   5 out of 5 enterprise AI teams say they need this

5. OPEN SOURCE IS THE RIGHT STRATEGY
   Security software MUST be auditable (trust issue)
   Community adoption → standard → enterprise sales
   This is the HashiCorp, Elastic, Databricks playbook
```

---

## The Bear Case

```
Risk 1: TOO EARLY (Probability: 60%)
──────────────────────────────────────
  The market for "AI agent governance" barely exists.
  Most companies using AI agents are still in POC stage.
  They haven't hit the wall yet because agents aren't autonomous enough.
  
  Impact: eyeVesa launches but nobody buys yet
  Mitigation: Start with the 500 companies that DO have pain,
               build awareness, position for when the market catches up

Risk 2: BIG PLATFORMS BUILD THIS (Probability: 40%)
──────────────────────────────────────────────────────
  AWS, Azure, or GCP could add "AI Agent Governance" as a feature.
  They have the enterprise relationships, the compliance, the trust.
  An AWS "IAM for AI Agents" service would crush eyeVesa.
  
  Impact: Existential threat
  Mitigation: Be open source, build community first, standards matter
               AWS might adopt eyeVesa instead of building from scratch

Risk 3: MCP DOESN'T BECOME THE STANDARD (Probability: 30%)
──────────────────────────────────────────────────────
  eyeVesa is built on MCP (Model Context Protocol).
  If OpenAI/Anthropic/Google create their own protocol,
  eyeVesa would need to support multiple protocols.
  
  Impact: Need to add abstraction layer for multiple protocols
  Mitigation: Protocol adapters (like Kong supports multiple APIs)

Risk 4: COMPANIES DON'T SEE THE VALUE (Probability: 40%)
──────────────────────────────────────────────────────
  "We just use API keys, it works fine"
  "Our agents don't need governance, they're not autonomous yet"
  "We'll build it ourselves when we need it"
  
  Impact: Nobody buys
  Mitigation: Find the 500 companies that DO have pain,
               make the pain visible (case studies, blog posts, incidents)

Risk 5: AUTONOMOUS AI AGENTS DON'T TAKE OFF (Probability: 15%)
──────────────────────────────────────────────────────
  If AI agents remain "copilots" (suggest, don't act),
  governance is less critical because humans approve everything.
  
  Impact: eyeVesa's market shrinks significantly
  Mitigation: Even copilots need some governance (who accessed what data),
               though HITL becomes less critical
```

---

## Honest Assessment

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  DOES THE MARKET NEED THIS?                                    │
│                                                                 │
│  Today (2026):   NO for most companies                         │
│                  YES for 500-1,000 early adopters              │
│                                                                 │
│  In 12 months:  YES for 5,000-10,000 companies               │
│                                                                 │
│  In 24 months:  YES for 50,000+ companies                    │
│                                                                 │
│  The need is real. The timing is early.                        │
│  eyeVesa is building for 2027-2028 demand in 2026.             │
│  This is a good position if execution is fast.                 │
│  This is a bad position if competitors are faster.             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  SHOULD YOU BUILD THIS?                                        │
│                                                                 │
│  YES if:                                                        │
│  • You can survive 12-18 months without revenue                │
│  • You can build community and mindshare early                  │
│  • You're positioned when the market catches up               │
│  • You believe autonomous AI agents are the future             │
│                                                                 │
│  NO if:                                                         │
│  • You need revenue in 6 months                                 │
│  • You think big platforms won't build this                    │
│  • You're not committed to the open source strategy            │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  THE MOST IMPORTANT THING TO DO RIGHT NOW:                     │
│                                                                 │
│  1. Ship a working end-to-end demo                              │
│     (SDK → gateway → resource adapter, all functional)        │
│                                                                 │
│  2. Get 5 early adopters using it in production                 │
│     (real companies, real agents, real resources)              │
│                                                                 │
│  3. Publish their case studies                                  │
│     ("How Company X uses eyeVesa to govern their AI agents")  │
│                                                                 │
│  Without working software and real users, the market            │
│  analysis doesn't matter. Ship first, theorize second.        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Comparable Companies

```
Company          What they did               Timing        Result
───────────────────────────────────────────────────────────────────────
HashiCorp        Infrastructure as code      Early (2014)  $5B+ IPO
(Vault, Terraform) (before IaC was mainstream)              Got acquired
                 Built open source tools when                by IBM for
                 most companies were still manual             $6.4B (2023)

Twilio           Communication APIs          Right time     $12B+ market cap
                 (before every app needed     (2008)         Profitable
                 SMS, but when it was coming)

Cloudflare       Edge security/DDoS          Right time     $20B+ market cap
                 (when DDoS became real      (2010)         Profitable
                 threat for every company)

Snyk             Security for developers     Early (2015)  $7B valuation
                 (before supply chain                       Growing, still
                 attacks were mainstream)                    pre-IPO

OPA/Styra        Policy as code              Early (2016)  $200M+ valuation
                 (before policy was                         Adopted by cloud
                 mainstream, but growing)                   providers

Conjur/CyberArk  Secrets management          Early (2013)  Acquired for
                 (before zero-trust was                     $2.6B by
                 a buzzword)                                 CyberArk

eyeVesa          Agent identity & trust      Early (2026)  ???
                 (before agent governance
                  is mainstream)

PATTERN:
  Security/identity companies that ship early,
  before the market is ready, but position for the wave,
  tend to do well IF they survive until demand arrives.
```

---

## Market Size Estimates

### Total Addressable Market (TAM)

```
Companies using AI in any form (2026):            ~50,000,000
Companies using AI agents (any maturity):           ~5,000,000
Companies with autonomous agents in production:      ~500,000
Companies with agents accessing sensitive systems:    ~50,000
Companies willing to pay for governance:               ~5,000

TAM (companies that could use eyeVesa):         ~5,000 companies
SAM (companies that will buy in next 2 years):    ~500 companies
SOM (companies we can reach in year 1):            ~50 companies
```

### Serviceable Addressable Market (SAM) Breakdown

```
Industry                Agents in Prod    Will Pay for Governance    SAM Size
─────────────────────────────────────────────────────────────────────────
Fintech / Banking            ~200           ~80% ($99-199/agent)      $2M-5M
DevOps / SRE                 ~500           ~60% ($99/agent)            $3M
Healthcare                   ~100           ~90% ($199/agent, HIPAA)    $2M
Cloud / SaaS platforms       ~300           ~50% ($99/agent)            $1.5M
Security / MSSP              ~150           ~70% ($99-199/agent)        $1.5M
E-commerce / Retail          ~200           ~40% ($99/agent)            $0.8M
─────────────────────────────────────────────────────────────────────────
Total SAM (Year 1-2):                                                   ~$10M-15M

Year 3-4 (as market grows):                                             ~$50M-100M
Year 5+ (if AI agents become standard):                                 ~$500M+
```

### Revenue Scenarios

```
Scenario              Year 1     Year 2     Year 3     Year 4
───────────────────────────────────────────────────────────
Conservative (slow)    $0         $150K      $500K      $2M
                         10 Pro    30 Pro     100 Pro    300 Pro
                         0 Ent     1 Ent      3 Ent      10 Ent

Base case              $0         $300K      $3.5M      $9M
                         15 Pro    50 Pro     120 Pro    200 Pro
                         1 Ent     5 Ent      15 Ent     30 Ent

Optimistic (fast)      $0         $1M        $10M       $30M+
                         50 Pro    200 Pro    500 Pro    1000 Pro
                         3 Ent     10 Ent     30 Ent     100 Ent
                         +market              +market    +market

Key assumptions:
  - Average Pro customer: 30 agents
  - Average Enterprise customer: 100 agents
  - Pro: $99/agent/month
  - Enterprise: $199/agent/month (Cloud) or $100K-500K/year
  - Marketplace: starts contributing Year 2+, significant Year 4+
```

### Why This Market Will Grow

```
AI Agent Adoption Trajectory:

  2024: "AI copilots" — suggest, don't act
         Low governance need, low market

  2025: "AI assistants" — act with human approval every time
         Moderate governance need, growing market

  2026: "AI agents" — act autonomously within guardrails
         HIGH governance need, market takes off

  2027+: "AI agents everywhere" — every enterprise deploys agents
          MANDATORY governance need, market explodes

We're at the 2025-2026 transition. The market is small today
but the trajectory is clear. The question is whether eyeVesa
is positioned when the wave hits.

The wave is coming. The only question is timing.
```