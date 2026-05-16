# eyeVesa Project Plan

> Identity and trust layer for AI agents. Open source core, paid advanced features.

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Open Source vs Paid](#open-source-vs-paid)
3. [Feature Roadmap](#feature-roadmap)
4. [Team & Responsibilities](#team--responsibilities)
5. [Timeline](#timeline)
6. [Technical Priorities](#technical-priorities)
7. [DevOps Checklist](#devops-checklist)
8. [Compliance Roadmap](#compliance-roadmap)
9. [Success Metrics](#success-metrics)

---

## Project Overview

**What**: eyeVesa is a gateway that sits between AI agents and enterprise resources, providing cryptographic identity, policy enforcement, trust scoring, human-in-the-loop approval, and audit trails.

**Why**: When autonomous AI agents access production systems, enterprises need proof of who did what, whether it was allowed, and whether a human approved it. Without eyeVesa, agents use shared API keys with no identity, no policy, and no audit trail.

**Vision**: eyeVesa becomes the standard identity and trust layer for AI agents — like OAuth for machines, but with trust scoring and human oversight.

---

## Open Source vs Paid

### The Three Tiers

```
┌──────────────────────────────────────────────────────────────┐
│                                                              │
│  COMMUNITY (Free Forever)         Apache 2.0 License          │
│  ─────────────────────            ──────────────────           │
│                                                                │
│  What's included:                                              │
│  ✓ Gateway Core (Rust proxy)                                  │
│  ✓ Control Plane (Go API)                                     │
│  ✓ Agent SDK (Rust)                                           │
│  ✓ Resource Adapter (Go)                                      │
│  ✓ Ed25519 cryptographic identity                            │
│  ✓ OPA/Rego policy engine                                     │
│  ✓ Single-layer HITL (approve/deny)                          │
│  ✓ Basic trust scoring                                        │
│  ✓ Signed audit logs                                          │
│  ✓ 1-level delegation                                         │
│  ✓ Docker Compose deployment                                  │
│  ✓ CLI tool                                                   │
│  ✓ Community support                                          │
│                                                                │
│  Limits: 5 agents, 10 resources, single tenant              │
│  License: Apache 2.0 (free, open, production-ready)          │
│                                                                │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  PRO ($99/agent/month)           BSL 1.1 License              │
│  ─── ─────────────────           ──────────────                │
│                                                                │
│  Everything in Community +:                                   │
│  ✓ Multi-layer HITL (escalation, approval chains)            │
│  ✓ LLM-powered HITL summaries                                 │
│  ✓ LLM-powered policy translator                             │
│  ✓ LLM-powered audit narratives                               │
│  ✓ pgvector behavioral anomaly detection                    │
│  ✓ Budget enforcement + metering                               │
│  ✓ Rate limiting per agent                                    │
│  ✓ Multi-level delegation                                     │
│  ✓ HITL via Slack/Teams/PagerDuty                              │
│  ✓ SSO/SAML for approvers                                    │
│  ✓ Unlimited agents + resources                                │
│  ✓ Kubernetes Helm charts                                     │
│  ✓ Priority support (8h SLA)                                  │
│                                                                │
│  License: BSL 1.1 (source-available, production requires paid│
│  license, converts to Apache 2.0 after 3 years)              │
│                                                                │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ENTERPRISE (Custom, $50K-$500K/year)                        │
│  ──────────  ──────────────────────────                        │
│                                                                │
│  Everything in Pro +:                                         │
│  ✓ eyeVesa Cloud (managed infrastructure)                    │
│  ✓ SOC 2 Type II compliance                                  │
│  ✓ HIPAA compliance                                          │
│  ✓ Dedicated tenant (isolated DB + OPA)                      │
│  ✓ Custom Rego policy development                            │
│  ✓ Custom resource adapter development                        │
│  ✓ On-premise / air-gapped deployment                       │
│  ✓ HSM key management                                        │
│  ✓ SIEM integration (Splunk, Datadog, Elastic)               │
│  ✓ Custom trust scoring models                               │
│  ✓ 99.9% SLA                                                 │
│  ✓ Dedicated support engineer                                │
│  ✓ Incident response retainer                                 │
│  ✓ Multi-region deployment                                   │
│                                                                │
│  License: Proprietary (custom contract)                      │
│                                                                │
└──────────────────────────────────────────────────────────────┘
```

### Open Source ≠ Free

```
"Free" can mean three things:

  1. Free as in FREEDOM    →  You can read, modify, and redistribute
                              Apache 2.0 guarantees this

  2. Free as in BEER       →  You don't pay money to use it
                              eyeVesa Community is this

  3. Free as in PRODUCTION →  You can run it in production without paying
                              eyeVesa Community is this (with limits)

eyeVesa Community is ALL THREE.
It's open source, free, and free for production use (up to 5 agents).
```

### Why This Model Works

```
WHY give away the core for free?

  1. TRUST — Security software must be auditable. CISOs need to read every line.
  2. ADOPTION — Developers try free things first. Nobody pays before testing.
  3. COMMUNITY — Bug reports, PRs, documentation, word-of-mouth. Can't buy this.
  4. STANDARDS — If eyeVesa becomes the standard, everyone pays for pro features.

WHY charge for Pro features?

  1. COMPLEXITY — Multi-layer HITL, LLM summaries, anomaly detection are hard to build.
  2. VALUE — A production outage costs $50K+. $99/agent/month is cheap.
  3. SUSTAINABILITY — Open source projects die without revenue.

WHY BSL 1.1 specifically?

  1. Prevents AWS/GCP from hosting it as a service without contributing back.
  2. Converts to Apache 2.0 after 3 years (features don't stay locked forever).
  3. Source-available means security auditors can still read the code.
  4. Developers can test and evaluate for free (non-production use is free).
```

### Revenue Model

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
```

### Revenue Projections

```
Year 1:  Community growth, $0 revenue, -$120-240K investment
Year 2:  Pro launch, $280K ARR, +$40K/month net
Year 3:  Enterprise scale, $3.5M ARR, +$240K/month net
Year 4:  Marketplace, $9M+ ARR, +$670K/month net
```

---

## Feature Roadmap

### Phase 1: Core Functionality (Current — Needs Building)

```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────
P0        SDK HTTP transport                 Stub         3-5 days
P0        Gateway signature verification     Missing      2-3 days
P0        Auth middleware (API key/JWT)       Missing      1-2 days
P0        mTLS termination (rustls)          Missing      5-7 days
P0        Gateway proxy forwarding            Stub         2-3 days
────────────────────────────────────────────────────────────────
Subtotal: 2-3 weeks for a working end-to-end demo
```

### Phase 2: Production Features (Month 3-4)

```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────
P1        HITL approval API                  Missing      3-4 days
P1        HITL notification (Slack/webhook)  Missing      2-3 days
P1        Trust event recording              Missing      2-3 days
P1        tools/call in resource adapter      Partial     1-2 days
P1        Delegate enforcement               Missing      2-3 days
P1        Rate limiting middleware            Missing      1-2 days
P1        Budget enforcement                 Missing      2-3 days
────────────────────────────────────────────────────────────────
Subtotal: 2-3 weeks
```

### Phase 3: Pro Features (Month 5-8) — BSL 1.1 Licensed

```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────────────
P2        Multi-layer HITL escalation        Implemented   3-4 days
P2        HITL approval chains (2+ people)   Implemented   2-3 days
P2        LLM HITL summaries                 Implemented   3-5 days
P2        LLM policy translator              Implemented   3-5 days
P2        LLM audit narratives               Implemented   2-3 days
P2        pgvector behavioral embeddings     Implemented   5-7 days
P2        SSO/SAML for approvers             Implemented   5-7 days
P2        Multi-tenant isolation             Implemented   3-5 days
P2        Helm charts                        Implemented   2-3 days
P2        Budget enforcement + metering      Implemented   2-3 days
P2        Rate limiting middleware            Implemented   1-2 days
P2        Notification dispatcher (Slack/webhook/PagerDuty) Implemented 2-3 days
─────────────────────────────────────────────────────────────────────────
Subtotal: Completed — Phase 3 fully implemented
```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────
P2        Multi-layer HITL escalation        Missing      3-4 days
P2        HITL approval chains (2+ people)   Missing      2-3 days
P2        LLM HITL summaries                 Missing      3-5 days
P2        LLM policy translator              Missing      3-5 days
P2        LLM audit narratives               Missing      2-3 days
P2        pgvector behavioral embeddings     Missing      5-7 days
P2        SSO/SAML for approvers             Missing      5-7 days
P2        Multi-tenant isolation             Missing      3-5 days
P2        Helm charts                        Missing      2-3 days
────────────────────────────────────────────────────────────────
Subtotal: 4-6 weeks
```

### Phase 4: Enterprise Features (Month 9-12)

```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────
P3        eyeVesa Cloud (managed infra)      Missing      6-8 weeks
P3        SOC 2 Type II preparation          Missing      4-6 weeks
P3        Custom Rego development            Service      Ongoing
P3        Custom adapter development          Service      Per-client
P3        On-premise / air-gapped deploy     Missing      3-4 weeks
P3        HSM key management                 Missing      2-3 weeks
P3        SIEM integration                   Missing      2-3 weeks per SIEM
────────────────────────────────────────────────────────────────
Subtotal: 3-4 months
```

### Phase 5: Marketplace (Month 12-18)

```
Priority  Component                          Status       Effort
────────────────────────────────────────────────────────────────
P4        Resource adapter marketplace       Missing      6-8 weeks
P4        Discovery + listing API            Missing      3-4 weeks
P4        Transaction metering + billing     Missing      3-4 weeks
P4        Provider payout system             Missing      2-3 weeks
────────────────────────────────────────────────────────────────
Subtotal: 3-4 months
```

---

## Team & Responsibilities

```
Role                  Responsibilities                         When
─────────────────────────────────────────────────────────────────────
Founding Engineer     Rust gateway, Go control plane,            Day 1
                      architecture decisions

DevOps Engineer       Docker, K8s, CI/CD, monitoring,           Month 2
                      security, deployment

SDK Engineer          Rust SDK HTTP transport, CLI tool,         Month 2
                      developer experience

Backend Engineer      HITL API, trust events, billing,          Month 3
                      database queries

Security Engineer     mTLS, SPIRE, auth, pen testing            Month 3

Product Manager       Roadmap, user research, prioritization     Month 4

Developer Advocate    Documentation, tutorials, community       Month 4

LLM Integration Eng   HITL summaries, policy translator,         Month 6
                      audit narratives, embeddings

Sales Engineer        Enterprise demos, custom deployments      Month 8
```

---

## Timeline

```
Month 1-2:  Core functionality (SDK transport, gateway proxy, auth)
            → Working end-to-end demo

Month 3-4:  Production features (HITL, trust, delegation, rate limits)
            → Production-ready for early adopters

Month 5-6:  Security hardening (mTLS, SPIRE, encryption at rest)
            → Secure enough for production

Month 7-8:  Pro features (HITL escalation, LLM, anomaly detection)
            → Pro tier launch

Month 9-10: Enterprise features (Cloud, SOC 2, compliance)
            → Enterprise tier launch

Month 11-12: Marketplace (adapters, billing, discovery)
             → Marketplace beta

Month 13+:  Scale (multi-region, performance, reliability)
             → Growth phase
```

---

## Technical Priorities

### Critical Path (Must Have for Demo)

```
1. SDK HTTP transport        — connect(), discover(), invoke() need real HTTP
2. Gateway proxy forwarding  — route MCP requests to resource adapters
3. Auth middleware            — at minimum API key auth on control plane
4. HITL approval API          — approve/deny/expire endpoints
5. Trust event recording     — write trust_events on every action
```

### Important (Must Have for Production)

```
6. mTLS termination          — rustls + SPIRE integration
7. Encryption at rest         — PostgreSQL encryption config
8. Secrets management        — environment variable injection, no hardcoded creds
9. Rate limiting              — per-agent request quotas
10. Budget enforcement       — track spend per agent
```

### Nice to Have (Must Have for Pro)

```
11. Multi-layer HITL          — escalation, approval chains
12. LLM HITL summaries       — natural language approval requests
13. Behavioral embeddings     — pgvector anomaly detection
14. CLI tool                  — eyevesa init, trust, audit, hitl commands
15. SSO/SAML                 — enterprise identity for human approvers
```

---

## DevOps Checklist

### Critical (Do First)

```
☐ Fix Dockerfiles (non-root user, health checks, .dockerignore)
☐ Fix docker-compose.yml (env vars, typo regp→rego, restart policies)
☐ Create .env file with secure passwords
☐ Add .dockerignore files
☐ Fix OPA volume mount typo
```

### High (Do Before Production)

```
☐ Kubernetes manifests (deploy/ folders are empty)
☐ CI/CD pipeline (build, test, security scan, push)
☐ Secrets management (K8s secrets or Vault)
☐ Database backups (daily pg_dump + retention)
☐ Prometheus metrics endpoint on both services
☐ Grafana dashboard (trust, HITL, audit, errors)
☐ Alerting rules (trust < 0.3, HITL queue, 5xx rate)
☐ Network isolation (no DB/OPA/SPIRE exposed to internet)
☐ API authentication middleware
☐ Rate limiting middleware
```

### Medium (Do After Stable Production)

```
☐ TLS certificates (Let's Encrypt or internal CA)
☐ mTLS with SPIRE (certificate rotation)
☐ Horizontal pod autoscaling (HPA)
☐ Pod disruption budgets (PDB)
☐ Resource adapter deployment templates
☐ Database connection pooling config
☐ Log aggregation (ELK or Loki)
☐ Distributed tracing (OpenTelemetry)
☐ Disaster recovery procedure documented
☐ Runbooks for common incidents
```

### Nice to Have

```
☐ Multi-region deployment
☐ Blue/green or canary deploys
☐ Performance load testing
☐ Penetration testing
☐ Infrastructure as Code (Terraform/Pulumi)
☐ GitOps (ArgoCD/Flux)
```

---

## Compliance Roadmap

### Phase 1: Security Basics (2-3 weeks) — Blocks ALL Certifications

```
☐ API authentication (JWT or API key middleware)
☐ mTLS implementation (rustls + SPIRE)
☐ Database encryption at rest (PostgreSQL config)
☐ Secrets management (remove hardcoded credentials)
☐ Input validation on ALL endpoints
☐ Rate limiting on API endpoints
☐ Dependency vulnerability scanning in CI
☐ Security headers (CORS, CSP, HSTS)
```

### Phase 2: Audit & Monitoring (2-3 weeks) — Required for SOC 2, HIPAA

```
☐ Structured logging (JSON, severity levels)
☐ Metrics collection (Prometheus)
☐ Health check endpoints (liveness, readiness)
☐ Alerting rules
☐ Log retention policy (90 days hot, 7 years cold)
☐ Session management with timeout
```

### Phase 3: Access Control (2-3 weeks) — Required for SOC 2, ISO 27001

```
☐ Role-based access control (admin, operator, viewer)
☐ API key rotation
☐ Break-glass procedure (emergency access)
☐ Pseudonymization for audit logs (GDPR resolution)
☐ Data retention policies (configurable per tenant)
```

### Phase 4: Compliance Documentation (3-4 weeks) — Audit Preparation

```
☐ Security policy document
☐ Risk assessment
☐ Incident response playbook
☐ Business continuity plan
☐ DPIA (Data Protection Impact Assessment) for GDPR
☐ Penetration test (external)
☐ SOC 2 Type I audit preparation
```

### Phase 5: Certification (4-6 weeks)

```
☐ SOC 2 Type I audit (fastest path to certification)
☐ ISO 27001 certification (6-12 months)
☐ HIPAA self-assessment (if serving healthcare)
☐ GDPR compliance review (if processing EU data)
```

### Current Compliance Score

```
Framework          Current   After Phase 1-3   After Phase 4-5
──────────────────────────────────────────────────────────────
SOC 2 Type I       3/10      7/10              9/10 ✅
SOC 2 Type II      3/10      6/10              8/10 ✅
HIPAA               4/10      6/10              8/10 ✅
ISO 27001           3/10      6/10              8/10 ✅
GDPR                 2/10      5/10              7/10 ⚠️ (erasure conflict)
PCI DSS              2/10      5/10              7/10 ⚠️ (if processing cards)
```

---

## Success Metrics

### Community Health (Month 1-6)

```
GitHub stars:             Target 500+ by month 6
Docker pulls:             Track monthly
npm/cargo downloads:       Track monthly
Discord members:          Track monthly
Community PRs:            Track monthly
```

### Product-Market Fit (Month 6-12)

```
Pro trial → paid conversion:  Target 20%
Agents managed per customer:  Track average (target: 10+)
HITL approvals per week:      Track (proxy for production use, target: 50+)
Trust score distribution:     Track (are agents behaving?)
Churn rate:                   Target < 5% monthly
```

### Revenue (Month 12+)

```
Monthly Recurring Revenue (MRR):  Target $100K by month 18
Annual Recurring Revenue (ARR):   Target $1M by month 24
Net Revenue Retention (NRR):       Target > 110%
Customer Acquisition Cost (CAC):   Track per channel
Lifetime Value (LTV):              Target > 10x CAC
LTV/CAC ratio:                     Target > 3
```

---

## License Summary

```
Component                          License         Free for Production?
─────────────────────────────────────────────────────────────────────
gateway/core (Rust proxy)          Apache 2.0      Yes (5 agent limit)
gateway/control-plane (Go API)     Apache 2.0      Yes (5 agent limit)
sdk/agent-sdk-rust                 Apache 2.0      Yes
adapter/resource-adapter-go        Apache 2.0      Yes
registry/migrations (SQL schema)   Apache 2.0      Yes
CLI tool                            Apache 2.0      Yes
─────────────────────────────────────────────────────────────────────
HITL escalation                    BSL 1.1         No (production requires license)
HITL Slack/Teams integration       BSL 1.1         No
LLM summaries                      BSL 1.1         No
LLM policy translator              BSL 1.1         No
LLM audit narratives               BSL 1.1         No
Anomaly detection (pgvector)        BSL 1.1         No
Budget enforcement                  BSL 1.1         No
Multi-tenant isolation             BSL 1.1         No
─────────────────────────────────────────────────────────────────────
eyeVesa Cloud                      Proprietary      No
SOC 2 compliance package          Proprietary      No
HSM integration                    Proprietary      No
SIEM integration                   Proprietary      No

BSL 1.1 features convert to Apache 2.0 after 3 years from release date.
```