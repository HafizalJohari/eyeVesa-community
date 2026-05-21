# eyeVesa Project Overview

This `GEMINI.md` file provides an overview of the `eyeVesa` project, its architecture, how to build and run it, and key development conventions. It is intended to serve as instructional context for future interactions with the Gemini CLI.

## Project Overview

`eyeVesa` is an identity and trust layer for the agentic economy, designed to connect AI agents to enterprise resources with cryptographic identity, policy-based authorization, and non-repudiable audit trails. It aims to provide a "Know Your Agent" (KYA) framework.

**Key Components:**
*   **Gateway Core (Rust):** Acts as an MCP proxy, handling crypto, mTLS termination, and PTV identity.
*   **Control Plane (Go):** Provides HTTP + gRPC APIs, OPA policy enforcement, audit logging, database interaction, HITL (Human-in-the-Loop), PTV, and Airport services.
*   **Airport:** An agent discovery layer for agents to register, announce presence, find others, and track interactions.
*   **SDKs (Rust, Python, TypeScript):** Client libraries for AI agents to interact with the eyeVesa gateway.
*   **Resource Adapter (Go):** An MCP server wrapper for enterprise resources.
*   **CLI (Go):** A command-line interface for agent management, authorization, and airport operations.

**Core Concepts:**
*   **Ed25519 Identity:** Agents receive keypairs on registration, with signatures verified on each action.
*   **MCP (Model Context Protocol):** A standardized JSON-RPC 2.0 protocol for agent-resource communication.
*   **Policy Engine:** Embedded OPA/Rego for authorization, defining allowed tools, never-events, and budget limits.
*   **PTV (Prove-Transform-Verify):** Hardware-rooted identity attestation.
*   **Human-in-the-Loop (HITL):** Requires human approval for high-risk actions.
*   **Non-repudiable Audit:** Every action is logged with an Ed25519 signature.

## Building and Running

The project uses Docker and Docker Compose for local development and deployment.

### Quickstart (Local Development)

The fastest way to get started is using the `start.sh` script, which sets up the local environment, generates necessary keys, and starts core services via Docker Compose.

```bash
./start.sh
```

This will start `postgres`, `opa`, `gateway-control`, and `gateway-core` services.

To shut down the services:
```bash
docker compose down
```

### Docker Compose (Manual Control)

You can manually control the services defined in `docker-compose.yml`:

To build and start all services:
```bash
docker compose build
docker compose up -d
```

To start specific services (e.g., core components):
```bash
docker compose up -d postgres opa gateway-control gateway-core
```

### Building Individual Services

The main services (`gateway-control`, `gateway-core`, `resource-adapter`) are built using Dockerfiles located within their respective directories.

For example, to build `gateway-control`:
```bash
docker build -t eyevesa-gateway-control -f gateway/control-plane/Dockerfile .
```

The `cloudbuild.yaml` also demonstrates how these images are built for Google Cloud:
```bash
gcloud builds submit --config cloudbuild.yaml \
  --substitutions=_REGION=asia-southeast1,_REPO=eyevesa,_TAG=latest \
  --project=YOUR_PROJECT_ID
```

### CLI Installation and Usage

The `eyevesa` CLI is a Go application. It can be installed via a script, Homebrew, or run via Docker.

**Install via script:**
```bash
curl -fsSL https://raw.githubusercontent.com/Hafizaljohari/eyeVesa/main/scripts/install.sh | bash
```

**Install via Homebrew:**
```bash
brew tap Hafizaljohari/eyevesa https://github.com/Hafizaljohari/eyeVesa
brew install eyevesa
```

**Run via Docker:**
```bash
docker build -t eyevesa-cli -f cli/Dockerfile .
docker run --rm eyevesa-cli --help
```

**Launch interactive TUI:**
```bash
eyevesa tui
```

**Example CLI commands:**
```bash
eyevesa register --name my-agent --owner "org:acme"
eyevesa airport search --status online
eyevesa list-agents
```

## Development Conventions

*   **Languages:** Primarily Go (for Control Plane, CLI, Resource Adapter) and Rust (for Gateway Core, SDK).
*   **Dependency Management:** Go Modules (`go.mod`, `go.sum`) for Go projects, Cargo (`Cargo.toml`, `Cargo.lock`) for Rust projects.
*   **API Definition:** gRPC services are defined using Protobuf (`proto/agentid.proto`).
*   **Database:** PostgreSQL with `pgvector` for migrations defined in `registry/migrations/`.
*   **Policies:** OPA/Rego for authorization policies (`policies/authz.rego`).
*   **Code Structure:** Go services follow a common pattern with `cmd` for main executables and `internal` for internal packages.
*   **Testing:** Unit tests for Go and Rust, integration tests, and a full E2E test suite (`tests/e2e-test.sh`).
*   **Environment Variables:** Configuration is heavily reliant on environment variables (e.g., `DATABASE_URL`, `JWT_SECRET`, `GATEWAY_MODE`).
*   **Licensing:** The project includes a licensing mechanism that enforces limits (e.g., `MaxAgents`) based on build tags (`pro` vs. `community`).

## Key Directories

*   `adapter/`: Contains the `resource-adapter-go`.
*   `cli/`: Source code for the `eyevesa` CLI tool.
*   `deploy/`: Deployment configurations (Docker Compose, K8s, GCP, Terraform).
*   `docs/`: Project documentation.
*   `gateway/`: Core services including `control-plane` (Go) and `core` (Rust).
*   `proto/`: Protobuf definitions.
*   `registry/migrations/`: Database migration scripts.
*   `sdk/`: Software Development Kits for various languages.
*   `tests/`: Various test scripts.
