# eyeVesa Learning Roadmap

You learn a concept, then immediately apply it to the eyeVesa codebase.

---

## Week 1-2: Go Foundations (Control Plane)

You'll build the Go control plane APIs while learning Go basics.

### Days 1-3: Go Language Fundamentals
- [ ] Complete [A Tour of Go](https://go.dev/tour/) (4-6 hours)
- [ ] Read [Go by Example](https://gobyexample.com/) — focus on:
  - Variables, functions, structs, interfaces
  - Error handling (no exceptions, always check `err`)
  - Goroutines and channels (concurrency)
- [ ] Install Go: `brew install go`
- [ ] Verify: `go version`

### Days 4-5: Go HTTP APIs
- [ ] Read [net/http package docs](https://pkg.go.dev/net/http)
- [ ] Learn `chi` or `gin` router (we use chi)
- [ ] Build: Write your first API endpoint in `gateway/control-plane/`
- [ ] **Project Task**: Implement `POST /v1/agents/register` stub

### Days 6-7: Go + PostgreSQL
- [ ] Learn [pgx](https://github.com/jackc/pgx) (PostgreSQL driver for Go)
- [ ] Learn database migrations with [goose](https://github.com/pressly/goose)
- [ ] **Project Task**: Connect control plane to `registry/migrations/`

### Days 8-9: Go gRPC & Protobuf
- [ ] Read [grpc.io/docs/languages/go/](https://grpc.io/docs/languages/go/)
- [ ] Learn protobuf definitions
- [ ] **Project Task**: Define `proto/agentid.proto` message types for eyeVesa

### Day 10: Go Testing
- [ ] Learn Go testing conventions (`_test.go`, table-driven tests)
- [ ] **Project Task**: Write tests for the register endpoint

---

## Week 3-4: Rust Foundations (Core Engine)

You'll build the Rust proxy engine while learning Rust basics.

### Days 11-14: Rust Language Fundamentals
- [ ] Complete [The Rust Book chapters 1-10](https://doc.rust-lang.org/book/)
- [ ] Focus on: ownership, borrowing, structs, enums, Result/Option
- [ ] Complete [Rustlings exercises](https://github.com/rust-lang/rustlings)
- [ ] Install Rust: `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh`
- [ ] Verify: `rustc --version`

### Days 15-16: Rust Async & Networking
- [ ] Learn [tokio](https://tokio.rs/tokio/tutorial) (async runtime)
- [ ] Learn [hyper](https://hyper.rs/) (HTTP library)
- [ ] **Project Task**: Implement basic TCP proxy in `gateway/core/`

### Days 17-18: Rust Cryptography
- [ ] Learn [ring](https://briansmith.org/ring/) or [ed25519-dalek](https://docs.rs/ed25519-dalek)
- [ ] Learn X.509 certificate handling with [rustls](https://docs.rs/rustls)
- [ ] **Project Task**: Implement signature verification in `gateway/core/src/crypto/`

### Day 19-20: Rust mTLS
- [ ] Learn rustls server/client configuration
- [ ] Learn certificate pinning and validation
- [ ] **Project Task**: Implement mTLS termination in `gateway/core/src/mTLS/`

---

## Week 5: PostgreSQL + pgvector (Registry)

### Days 21-22: PostgreSQL Administration
- [ ] Install PostgreSQL: `brew install postgresql@16`
- [ ] Learn schema design, indexes, constraints
- [ ] Learn pgvector extension for vector similarity search
- [ ] **Project Task**: Write `registry/migrations/001_agents.sql`

### Day 23: Database Connections from Rust
- [ ] Learn [sqlx](https://docs.rs/sqlx) (Rust PostgreSQL driver)
- [ ] **Project Task**: Connect Rust proxy to registry

### Day 24-25: Database Connections from Go
- [ ] Learn [pgx](https://github.com/jackc/pgx) (Go PostgreSQL driver)
- [ ] **Project Task**: Connect Go control plane to registry

---

## Week 6: MCP Protocol (Communication)

### Days 26-27: MCP Deep Dive
- [ ] Read [modelcontextprotocol.io specification](https://modelcontextprotocol.io/specification)
- [ ] Understand JSON-RPC 2.0 message format
- [ ] Understand Tools, Resources, Prompts primitives
- [ ] Understand stdio and SSE transport
- [ ] **Project Task**: Implement MCP JSON-RPC handler in `gateway/core/src/proxy/`

### Day 28: MCP Client (Agent SDK)
- [ ] Study existing MCP client implementations
- [ ] **Project Task**: Implement `sdk/agent-sdk-rust/` MCP client

### Day 29-30: MCP Server (Resource Adapter)
- [ ] Study existing MCP server implementations
- [ ] **Project Task**: Implement `adapter/resource-adapter-go/` MCP server

---

## Week 7: SPIRE (Identity Infrastructure)

### Days 31-33: SPIFFE/SPIRE Concepts
- [ ] Read [spiffe.io/docs/](https://spiffe.io/docs/)
- [ ] Understand SVID (SPIFFE Verifiable Identity Document)
- [ ] Understand workload attestation
- [ ] Install SPIRE: follow [quickstart](https://spiffe.io/docs/latest/try/)
- [ ] **Project Task**: Configure `gateway/spire/` for agent attestation

### Days 34-35: SPIRE Integration
- [ ] Learn SPIRE Go SDK for workload API
- [ ] Learn SPIRE Rust SDK (workload API via Unix socket)
- [ ] **Project Task**: Integrate SPIRE identity into control plane + core

---

## Week 8: OPA (Policy Engine)

### Days 36-38: OPA & Rego
- [ ] Read [openpolicyagent.org/docs/latest/](https://www.openpolicyagent.org/docs/latest/)
- [ ] Learn Rego query language
- [ ] Try [Rego Playground](https://play.openpolicyagent.org/)
- [ ] **Project Task**: Write authorization policies in `gateway/control-plane/cmd/policy/`

### Days 39-40: OPA Integration
- [ ] Learn OPA Go SDK (embedded policy evaluation)
- [ ] Learn OPA REST API (sidecar deployment)
- [ ] **Project Task**: Integrate OPA into control plane decision flow

---

## Week 9: Human-in-the-Loop (HITL)

### Days 41-43: Push Notifications & Biometric Auth
- [ ] Learn APNs (Apple Push Notification) / FCM (Firebase Cloud Messaging)
- [ ] Learn WebAuthn / FIDO2 for biometric verification
- [ ] **Project Task**: Implement async authorization hook in `gateway/control-plane/internal/hitl/`

### Day 44-45: HITL Flow
- [ ] Design: pending-approval queue + timeout + escalation
- [ ] **Project Task**: Wire HITL into policy engine decision flow

---

## Week 10: Audit & Trust

### Days 46-48: Audit Vault
- [ ] Learn append-only logging patterns
- [ ] Learn cryptographic commitment (Merkle trees)
- [ ] **Project Task**: Implement `gateway/control-plane/internal/audit/`

### Days 49-50: Trust Degradation
- [ ] Learn anomaly detection basics
- [ ] Learn session-aware trust scoring
- [ ] **Project Task**: Implement trust score tracker

---

## Week 11-12: Deployment & Integration

### Days 51-55: Docker & Kubernetes
- [ ] Learn Docker multi-stage builds (Rust + Go)
- [ ] Learn Kubernetes sidecar pattern
- [ ] **Project Task**: Build `deploy/sidecar/` packaging

### Days 56-58: Cloud Deployment
- [ ] Learn AWS Malaysia region setup
- [ ] Learn GCP MY region setup
- [ ] **Project Task**: Build `deploy/aws-my/` and `deploy/gcp-my/`

### Days 59-60: End-to-End Testing
- [ ] Write integration tests: agent → gateway → resource
- [ ] Load test with simulated agents
- [ ] Security audit: mTLS, policy bypass, trust degradation

---

## Resources Cheat Sheet

| Topic | Best Resource | Time |
|---|---|---|
| Go | [go.dev/tour](https://go.dev/tour/) | 4h |
| Rust | [doc.rust-lang.org/book](https://doc.rust-lang.org/book/) | 20h |
| PostgreSQL | [postgresql.org/docs](https://www.postgresql.org/docs/) | 4h |
| MCP | [modelcontextprotocol.io](https://modelcontextprotocol.io) | 3h |
| SPIRE | [spiffe.io/docs](https://spiffe.io/docs/) | 6h |
| OPA/Rego | [openpolicyagent.org/docs](https://www.openpolicyagent.org/docs/latest/) | 4h |
| mTLS | Search "mutual TLS explained" | 2h |
| gRPC | [grpc.io/docs](https://grpc.io/docs/) | 4h |
| Docker | [docs.docker.com/get-started](https://docs.docker.com/get-started/) | 4h |

---

## Daily Routine

```
Morning (2h):  Learn concept (read docs, watch tutorials)
Midday (3h):   Apply to eyeVesa codebase (write code)
Evening (1h):  Review, test, commit what you built
```

## Tips

- **Don't skip errors.** If something doesn't compile, fix it before moving on.
- **Commit often.** Every working function = 1 commit.
- **Test first, optimize later.** Make it work, then make it fast.
- **Ask for help.** Use AI (like this tool) when stuck on any concept.