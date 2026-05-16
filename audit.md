# eyeVesa Codebase Audit

## Executive Summary

The eyeVesa codebase is approximately **30% implemented**, with the remaining 70% being functional scaffolding that compiles and runs but lacks critical end-to-end wiring. The core proxy (Rust), control plane (Go), and agent SDK (Rust) all have the right structural bones, but the following five "first-class citizen" capabilities — **Identity, Permissions, Audit, Trust, and HITL** — each have significant gaps between their data models/service code and the actual request flow.

---

## 1. Service-by-Service Audit

### 1.1 Rust Gateway Core (`gateway/core/`)

| Component | File | Status | Issues |
|---|---|---|---|
| Entry point | `main.rs` | **Works** | Connects to gRPC, starts proxy in plaintext/TLS/mTLS mode |
| Proxy router | `proxy/server.rs` | **Works** | Routes `/v1/mcp`, `/v1/register`, `/v1/auth`, catch-all `/v1/*` forwarding |
| MCP handler | `proxy/mcp_handler.rs` | **Partial** | Parses JSON-RPC but `tools/list` returns empty array via gRPC stub; `tools/call` delegates to gRPC `authorize()` but doesn't proxy to resource adapter |
| Agent handler | `proxy/agent_handler.rs` | **Works** | Registration and authorization proxy to gRPC correctly |
| Forward proxy | `proxy/forward.rs` | **Works** | Forwards requests to Go HTTP API |
| gRPC client | `grpc.rs` | **Works** | Wraps tonic `GatewayServiceClient`; `verify_signature` and `get_agent` marked `#[allow(dead_code)]` |
| Crypto | `crypto/identity.rs` | **Works** | Ed25519 keygen + sign/verify; tests pass |
| Crypto | `crypto/signing.rs` | **Works** | Request signing/verification helpers |
| Identity/SVID | `identity/svid.rs` | **Works** | Fetches identity from Go control plane HTTP endpoint |
| Identity/PTV | `identity/ptv.rs` | **Works** | PTV proof generation |
| TLS | `tls/*` | **Works** | Cert watcher, TLS/mTLS server |

**Critical issues:**

1. **No signature verification in request flow** — `crypto/signing.rs` and `crypto/identity.rs` have verify functions, but `server.rs::handle_request` never validates the caller's Ed25519 signature before processing. Anyone can register agents or invoke actions.

2. **No OPA call in request flow** — The proxy forwards `/v1/auth` to gRPC, which delegates to the Go control plane. This is correct architecturally, but the gRPC `Authorize` RPC in Go doesn't call OPA either (see §1.2).

3. **No audit logging** — The proxy handles registration and authorization but never writes to `audit_logs`. Audit entries are only created in Go on direct HTTP calls.

4. **Hyper 1.x API concern** — `server.rs:22` uses `hyper_util::rt::TokioIo::new(stream)` with `http1::Builder::new().serve_connection()`, which is the correct hyper 1.x pattern. The earlier audit flag about hyper 0.14 APIs was **incorrect** — the code correctly uses `hyper = "1"` with `hyper-util = "0.1"`.

5. **MCP `tools/list` returns empty** — `list_tools_via_grpc()` at line 196-213 is a stub that always returns `json!([])`.

### 1.2 Go Control Plane (`gateway/control-plane/`)

| Component | File | Status | Issues |
|---|---|---|---|
| API entry | `cmd/api/main.go` | **Partial** | Broken graceful shutdown (line 278-282 creates new `http.Server`); no auth middleware on by default |
| Agent handler | `cmd/api/handlers/agent.go` | **Works** | Registration writes to DB, creates audit entry |
| Resource handler | `cmd/api/handlers/resource.go` | **Works** | CRUD for resources |
| Auth middleware | `internal/auth/middleware.go` | **Works** | API key, JWT, SSO support. `isPublicPath` makes `/v1/agents/register`, `/v1/resources/register`, `/v1/mcp` public — registration of agents/resources requires **no auth** when `AUTH_ENABLED=true` |
| OPA policy | `internal/policy/opa.go` | **Works standalone** | `PolicyEngine.Evaluate()` tries embedded OPA first, then external OPA, then `LocalEvaluate()` fallback. But **nobody calls it in the request flow** |
| Embedded OPA | `internal/policy/embedded_opa.go` | **Works** | Embeds Rego policies and evaluates locally |
| Audit logger | `internal/audit/logger.go` | **Works** | Logs to DB with Ed25519 signature. Only called on agent registration |
| HITL service | `internal/hitl/service.go` | **Works** | CRUD for approvals with expiry. `Approve()` and `GetStatus()` work |
| HITL escalation | `internal/hitl/escalation.go` | **Works** | Multi-channel escalation (webhook, Slack, PagerDuty) |
| HITL push | `internal/hitl/push.go`, `push_service.go` | **Works** | Push notification token management |
| Delegation | `internal/delegation/tracker.go` | **Works** | Delegate, validate, revoke with max-depth checks |
| PTV | `internal/ptv/service.go` | **Works** | Prove-Transform-Verify with ECDSA P-256 |
| SPIRE identity | `internal/identity/spire.go`, `svid.go` | **Works** | SPIRE Workload API client with LocalProvider fallback |
| Behavioral embeddings | `internal/behavior/embedding.go` | **Works** | LLM-powered embedding store |
| Database | `internal/database/db.go` | **Works** | PostgreSQL via pgx. Hardcoded fallback password `agentid_dev` |
| Crypto | `internal/crypto/keys.go`, `crypto.go` | **Works** | Ed25519 key load/generate, sign/verify |
| gRPC server | `internal/grpcserver/server.go` | **Partial** | Implements `Authorize` RPC but **doesn't call OPA** — uses a hardcoded default response |

**Critical issues:**

1. **OPA never called in request flow** — `PolicyEngine` is created in `main.go:148` and passed to `grpcserver.NewGatewayServer()`, but the gRPC `Authorize()` implementation doesn't use it. The `handlers.Authorize()` HTTP handler also doesn't call OPA.

2. **No audit logging for invoke/delegate/PTV** — `audit.NewAuditLogger(db)` is created but only injected into handlers. Only agent registration creates audit entries. Actions like authorization decisions, HITL approvals, delegation events, and PTV bindings are not audited.

3. **Trust score never updates** — Agents get `trust_score = 1.0` at registration and it's never modified based on OPA decisions or behavior.

4. **Graceful shutdown broken** — `main.go:278-282` creates a **new** `http.Server{Handler: r}` and calls `Shutdown()` on it. The actual server started on line 251 (`http.ListenAndServe`) is a different instance. Fix: store a reference to the running server.

5. **JWT secret random on restart** — Line 107-109: if `JWT_SECRET` env var is empty, a random secret is generated. On restart, all existing tokens become invalid. This is a dev convenience but will cause auth failures in production.

6. **Registration endpoints are public** — `isPublicPath()` (middleware.go:62-73) makes `/v1/agents/register` and `/v1/resources/register` public even when `AUTH_ENABLED=true`. This means anyone can register agents/resources without authentication.

7. **Database password in source** — `db.go:18` hardcodes `postgres://agentid:agentid_dev@localhost:5432/agentid`. Should use environment variable exclusively in production.

### 1.3 Rust Agent SDK (`sdk/agent-sdk-rust/`)

| Module | File | Status | Notes |
|---|---|---|---|
| `client.rs` | Agent client struct | **Works** | Config, signing key, HTTP client state |
| `connect.rs` | Registration | **Works** | POST to `/v1/register`, parses response, updates trust score |
| `discover.rs` | Resource discovery | **Works** | GET `/v1/resources?capability=X`, parses response into `ToolInfo` |
| `invoke.rs` | Tool invocation | **Works** | Two-step: authorize via `/v1/auth`, then call `/v1/mcp` with Ed25519 signature (computed but **not sent**) |
| `delegate.rs` | Delegation | **Works** | POST `/v1/delegate`, parses response |

**Critical issues:**

1. **Signature computed but not sent** — `invoke.rs:73` signs the payload (`let _signature = self.signing_key().sign(&payload_bytes);`) but the underscore discard means the signature is never included in the request headers. The gateway has no way to verify the caller's identity.

2. **No retry/backoff** — All HTTP calls fail immediately on network errors. Production clients need exponential backoff.

3. **No connection pooling configuration** — Uses `Client::new()` with defaults. For high-throughput agents, connection pool size and timeouts should be configurable.

---

## 2. Cross-Cutting Concerns

### 2.1 Security Gaps

| Gap | Severity | Location | Description |
|---|---|---|---|
| No request signature verification | **Critical** | `proxy/server.rs` | Gateway accepts all requests without verifying Ed25519 signature |
| Registration endpoints are public | **High** | `auth/middleware.go:62-73` | Anyone can register agents/resources even with auth enabled |
| JWT secret random on restart | **Medium** | `main.go:107-109` | All existing tokens invalidated on process restart |
| DB password hardcoded | **Medium** | `db.go:18` | `agentid_dev` password in source code |
| docker-compose OPA volume typo | **Low** | `docker-compose.yml` | Volume mount uses `regp` instead of `rego` |
| SDK signature not sent | **High** | `invoke.rs:73` | Ed25519 signature computed but discarded, never included in request |

### 2.2 Missing End-to-End Flows

```
┌─────────────────────────────────────────────────────────────────────────┐
│  FLOW: Agent Registration                                               │
│  ─────────────────────────                                              │
│  SDK ──POST /v1/register──▶ Rust proxy ──gRPC──▶ Go server            │
│      │                       │                    │                     │
│      │                       │                    ├─ Write to DB ✓      │
│      │                       │                    ├─ Audit log ✓        │
│      │                       │                    └─ Return agent_id ✓   │
│      │                       │                                          │
│      └─ No signature sent ◀──┘── No signature verified                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  FLOW: Tool Invocation (should be)                                      │
│  ──────────────────────────────                                         │
│  SDK ──POST /v1/auth──▶ Rust proxy ──gRPC──▶ Go server                 │
│      │                    │                    │                         │
│      │  + signature       │  verify sig       ├─ OPA evaluate ✗        │
│      │                    │                    ├─ Audit log ✗           │
│      │                    │                    ├─ Update trust ✗         │
│      │                    │                    └─ Return decision ✓      │
│      │                    │                                              │
│      └─ Then POST /v1/mcp ──▶ Rust proxy ──▶ forward to adapter? ✗   │
│          (currently returns static response, doesn't proxy)            │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Feature Completion Matrix

| Feature | Data Model | Service Code | API Endpoint | Wired in Flow | Score |
|---|---|---|---|---|---|
| **Identity** (Ed25519 keypairs, SPIRE SVID) | ✓ | ✓ | ✓ | ✗ (no sig verification) | **6/10** |
| **Permissions** (OPA policy engine) | ✓ | ✓ | ✓ | ✗ (OPA never called) | **5/10** |
| **Audit** (signed audit log) | ✓ | ✓ | ✓ | ✗ (only on registration) | **7/10** |
| **Trust** (score updates) | ✓ | ✓ | ✓ | ✗ (never updates from 1.0) | **3/10** |
| **HITL** (approval workflow) | ✓ | ✓ | ✓ | Partial (no approve/deny wiring in proxy) | **4/10** |
| **Delegation** (chain tracking) | ✓ | ✓ | ✓ | ✓ (works via HTTP) | **8/10** |
| **PTV** (prove-transform-verify) | ✓ | ✓ | ✓ | ✓ (works via HTTP) | **8/10** |
| **MCP** (tool listing/call) | ✓ | Partial | ✓ | ✗ (tools/list returns empty) | **3/10** |

**Average: 5.5/10** (up from 4.8/10 in earlier assessment due to SDK improvements)

---

## 3. Priority Fix List

### P0 — Must Fix Before Any Demo

| # | Fix | File(s) | Effort |
|---|---|---|---|
| 1 | Wire OPA into `Authorize` RPC (gRPC server) and `handlers.Authorize()` (HTTP) | `grpcserver/server.go`, `handlers/authz.go` | 2h |
| 2 | Send Ed25519 signature in SDK invoke request headers | `sdk/agent-sdk-rust/src/invoke.rs` | 1h |
| 3 | Verify Ed25519 signature in Rust proxy before processing register/auth requests | `proxy/server.rs` or `proxy/agent_handler.rs` | 2h |
| 4 | Fix graceful shutdown — store reference to running http.Server | `cmd/api/main.go:248-282` | 30m |

### P1 — Required for Minimal Viable Product

| # | Fix | File(s) | Effort |
|---|---|---|---|
| 5 | Audit-log authorization decisions, HITL approvals, delegation events, PTV bindings | `handlers/authz.go`, `handlers/ptv_hitl.go`, `handlers/delegation.go` | 3h |
| 6 | Update trust score after OPA evaluation (positive delta → increase, negative → decrease) | `handlers/authz.go` or `grpcserver/server.go` | 1h |
| 7 | Remove `/v1/agents/register` and `/v1/resources/register` from public paths, or add API key requirement | `auth/middleware.go:62-73` | 30m |
| 8 | Wire `tools/list` MCP method to actually query registered resources/tools from DB | `proxy/mcp_handler.rs` + new gRPC method or forward to Go | 2h |
| 9 | docker-compose.yml: fix OPA volume `regp` → `rego` | `docker-compose.yml` | 5m |

### P2 — Required for Production

| # | Fix | File(s) | Effort |
|---|---|---|---|
| 10 | JWT_SECRET must come from env var exclusively (fail fast if unset in production) | `cmd/api/main.go:106-109` | 30m |
| 11 | DB password: remove hardcoded fallback, require `DATABASE_URL` env var | `internal/database/db.go:17-18` | 15m |
| 12 | Add retry/backoff to SDK HTTP calls | `sdk/agent-sdk-rust/src/connect.rs`, `invoke.rs`, `discover.rs`, `delegate.rs` | 2h |
| 13 | MCP `tools/call` should proxy to resource adapter instead of returning static response | `proxy/mcp_handler.rs` | 4h |
| 14 | Expose OPA data push endpoint or auto-sync agent data from DB | `policy/opa.go` + `cmd/api/main.go` | 3h |

---

## 4. Architecture Strengths

Despite the gaps, the codebase has solid architectural foundations:

- **Clean separation**: Rust proxy (fast path) and Go control plane (business logic) is the right split. The proxy handles MCP, registration, and authorization routing; Go handles policy, HITL, PTV, delegation, audit.
- **gRPC contract**: The protobuf-defined `GatewayService` with `RegisterAgent`, `Authorize`, `VerifySignature`, and `GetAgent` RPCs is well-designed for the proxy↔control-plane interface.
- **Policy engine fallback**: Embedded OPA → External OPA → LocalEvaluate provides resilience.
- **HITL escalation**: Multi-channel (webhook, Slack, PagerDuty, push) with auto-expiry is production-ready.
- **PTV (Prove-Transform-Verify)**: Novel identity binding with hardware attestation is a differentiator.
- **SPIRE integration**: Workload API client with LocalProvider fallback handles both production and development.

---

## 5. Test Coverage Assessment

| Package | Has Tests | Test Files | Notes |
|---|---|---|---|
| Go handlers | ✓ | `handler_test.go` | Basic HTTP tests |
| Go auth | ✓ | `middleware_test.go` | JWT/API key auth tests |
| Go audit | ✓ | `logger_test.go` | Audit entry + signature tests |
| Go policy | ✓ | `embedded_opa_test.go` | Embedded OPA evaluation tests |
| Go PTV | ✓ | `service_test.go` | Key generation tests |
| Go tenant | ✓ | `service_test.go` | Tenant CRUD tests |
| Go delegation | ✓ | `tracker_test.go` | Delegation chain tests |
| Go HITL | ✓ | `push_test.go`, `escalation_test.go` | Push notification + escalation tests |
| Go crypto | ✓ | `crypto_test.go` | Ed25519 key tests |
| Go integration | ✓ | `integration_test.go` | End-to-end API tests |
| Rust crypto | ✓ | Inline in `identity.rs` | Ed25519 sign/verify unit tests |
| SDK | ✗ | None | No tests yet |

**Missing test coverage:**
- No test for OPA policy enforcement in the `Authorize` flow
- No test for trust score updates after policy decisions
- No end-to-end test that verifies signature chain from SDK → proxy → control plane
- No test for MCP tool listing/calling through the full path

---

## 6. Recommendations

1. **Focus on the P0 fixes first** — They close the three most critical gaps: no auth, no policy, no signature verification. Without these, the system trusts all callers unconditionally.

2. **Add a single integration test** that exercises the full flow: SDK registers → SDK discovers → SDK invokes → OPA evaluates → HITL if needed → Audit logged. This one test would prove the entire "first-class citizen" feature chain.

3. **Trust score must be dynamic** — Start simple: after each `Authorize` call, apply `trust_delta` from the OPA decision to the agent's `trust_score` in the DB. This is the minimum viable trust model.

4. **Standardize audit** — Every mutation (register, authorize, approve, delegate, PTV bind) should go through `auditLogger.Log()` with the appropriate action type. This is the compliance foundation.

5. **Deploy directories are empty** — `deploy/aws-my/`, `deploy/gcp-my/`, `deploy/sidecar/` all need Terraform/Docker configs. Start with `docker-compose.yml` fixes and a single-cloud deploy script.