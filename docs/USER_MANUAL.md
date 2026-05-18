# eyeVesa User Manual

> A simple guide to installing, configuring, and using eyeVesa — the identity and trust layer for AI agents.

---

## What is eyeVesa?

eyeVesa is a gateway that sits between AI agents (like Hermes, Claude, or custom bots) and your company's internal tools (databases, servers, APIs). It makes sure:

- **Only authorized agents** can access your systems
- **Risky actions** require a human to approve them first
- **Every action** is recorded in a tamper-proof audit log
- **Misbehaving agents** lose trust and get restricted automatically

Think of it as a security guard for your AI agents.

---

## What You Need Before Starting

| Requirement | Why You Need It | Where to Get It |
|-------------|----------------|-----------------|
| **Docker** | Runs the database and policy engine | [docker.com](https://www.docker.com/products/docker-desktop/) |
| **Go 1.22+** | Runs the main API server | `brew install go` or [go.dev](https://go.dev/dl/) |
| **Rust** | Runs the proxy server | `curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \| sh` |
| **Terminal** | To run commands | Built into your computer (Terminal on Mac, Command Prompt on Windows) |
| **curl** | To test the services | Pre-installed on Mac/Linux. Windows: use the eyeVesa CLI instead |

### Check What You Already Have

Open your terminal and run these commands to check:

```bash
docker --version
go version
rustc --version
```

If you see version numbers, you're good. If you get "command not found", install the missing ones using the links above.

---

## Installation

### Step 1: Get the eyeVesa Code

```bash
# Download the project
git clone https://github.com/hafizaljohari/eyeVesa.git
cd eyeVesa
```

### Step 2: Start the Infrastructure

These are the supporting services eyeVesa needs to run.

```bash
# Start the database and policy engine
docker-compose up -d

# Wait a few seconds, then check they're running
docker-compose ps
```

You should see:
```
agentid-postgres   running (healthy)
agentid-opa        running
```

> **Note for Mac users**: If `docker-compose` doesn't work, try `docker compose` (without the hyphen).

### Step 3: Start the API Server (Control Plane)

Open a new terminal window and run:

```bash
cd eyeVesa
cd gateway/control-plane
go run cmd/api/main.go
```

Wait until you see:
```
INFO connected to database
INFO HTTP server starting addr=:8080
```

Keep this terminal window open.

### Step 4: Start the Proxy (Gateway Core)

Open another terminal window and run:

```bash
cd eyeVesa
cd gateway/core
cargo run
```

Wait until you see:
```
INFO Proxy server listening on 0.0.0.0:9443 (plaintext)
```

Keep this terminal window open too.

### Step 5: Verify Everything is Running

```bash
# Check the API server
curl http://localhost:8080/health

# Check the proxy
curl http://localhost:9443/health

# Check the policy engine
curl http://localhost:8181/v1/data
```

If all three return something (not an error), you're good!

### Step 6: Build the Command-Line Tool (CLI)

```bash
cd eyeVesa/cli
go build -o eyevesa .
```

This creates the `eyevesa` command you'll use to manage everything.

---

## Configuration

### Setting Up Your First Agent (Quick Start)

The fastest way to get started:

```bash
cd eyeVesa/cli
./eyevesa init --name my-agent --owner "my-company"
```

What this does:
1. Creates a cryptographic identity for your agent (like a digital passport)
2. Registers the agent with the gateway
3. Saves the configuration to `~/.eyevesa/config.toml`
4. Saves the secret key to `~/.eyevesa/keys/`

After running this, run the doctor command to confirm everything is set up correctly:

```bash
./eyevesa doctor
```

You should see all checks passing with green checkmarks.

### What Was Created

| File | What It Is |
|------|-----------|
| `~/.eyevesa/config.toml` | Your configuration file (gateway address, agent ID) |
| `~/.eyevesa/keys/` | Your secret key (keep this safe — it's your agent's identity) |

### Using a Specific Gateway

If your gateway is on a different server:

```bash
./eyevesa init \
  --name my-agent \
  --owner "my-company" \
  --gateway https://gateway.mycompany.com:8080
```

---

## Basic Usage

### Checking System Health

```bash
# Quick health check
./eyevesa doctor

# Interactive dashboard
./eyevesa tui
```

The TUI (Terminal User Interface) lets you browse everything using your keyboard:

```
Tab        → Switch between views
↑/↓        → Navigate items
r          → Refresh data
q          → Quit
```

### Registering Resources (Tools & Services)

Resources are the things your agents can use — databases, APIs, servers, etc.

```bash
# Register a Kubernetes API
./eyevesa resources register \
  --name "k8s-api" \
  --type mcp_server \
  --endpoint "https://k8s-adapter:8443" \
  --risk-level high

# Register an analytics database
./eyevesa resources register \
  --name "analytics-db" \
  --type mcp_server \
  --endpoint "https://db-adapter:8443" \
  --risk-level medium
```

### Viewing Registered Agents

```bash
# List all agents
./eyevesa agents list

# View details for a specific agent
./eyevesa agents get <agent-id>

# View an agent's trust score
./eyevesa agents trust <agent-id>
```

### Viewing Registered Resources

```bash
# List all resources
./eyevesa resources list

# View details for a specific resource
./eyevesa resources get <resource-id>
```

### Checking If an Action is Allowed

Before an agent performs an action, you can check if it's authorized:

```bash
./eyevesa authorize \
  --agent-id <agent-id> \
  --action deploy \
  --resource-id <resource-id>
```

Possible results:
- **Allowed** → The action can proceed
- **Denied** → The action is blocked (check the reason)
- **HITL Required** → A human needs to approve this first

### Managing Approvals (HITL)

Some risky actions require a human to approve them first. This is called HITL (Human-In-The-Loop).

```bash
# View pending approval requests
./eyevesa hitl list

# Approve a request
./eyevesa hitl approve <approval-id>

# Deny a request
./eyevesa hitl deny <approval-id>
```

### Viewing the Audit Trail

Every action is recorded in a tamper-proof audit log.

```bash
# View recent audit logs for an agent
./eyevesa audit <agent-id>

# View more logs
./eyevesa audit <agent-id> --limit 50

# View older logs (paginated)
./eyevesa audit <agent-id> --limit 20 --offset 40
```

### Discovering Available Tools

```bash
# Discover all tools and resources
./eyevesa discover

# Discover tools for a specific capability
./eyevesa discover database
./eyevesa discover deployment
```

### Managing Delegation

You can let one agent act on behalf of another (with limits).

```bash
# Delegate capabilities from parent to child
./eyevesa delegate create \
  --parent <parent-agent-id> \
  --child <child-agent-id> \
  --scope "read,write" \
  --depth 1 \
  --duration 2h

# Validate a delegation
./eyevesa delegate validate --parent <id> --child <id>

# List delegations for an agent
./eyevesa delegate list <agent-id>

# Revoke a delegation
./eyevesa delegate revoke <delegation-id>
```

---

## Using the Interactive Dashboard (TUI)

The TUI gives you a visual interface to manage everything.

```bash
# Launch the dashboard
./eyevesa tui
```

### Dashboard View

Shows system status, statistics, and recent agents at a glance.

```
┌─ Gateway Status ──────────┐
│ ✓ Gateway: ok              │
└───────────────────────────┘
┌─ Statistics ───────────────┐
│ Agents:        5            │
│ Resources:     3            │
│ HITL Pending:  1            │
└───────────────────────────┘
```

### Agents View

Browse and inspect all registered agents. Use `↑/↓` to scroll.

### Resources View

Browse all registered resources.

### HITL View

View pending approval requests. Press `a` to approve or `d` to deny.

### Audit View

View the audit trail for the selected agent.

---

## How It Works (Simple Explanation)

```
                    ┌─────────────────────┐
                    │    AI Agent          │
                    │  (Hermes, Claude,    │
                    │   custom bot)        │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │                     │
                    │   eyeVesa Gateway    │
                    │                     │
                    │  ┌───────────────┐  │
                    │  │ 1. Verify ID  │  │
                    │  │ 2. Check      │  │
                    │  │    policy     │  │
                    │  │ 3. Ask human  │  │
                    │  │    (if risky) │  │
                    │  │ 4. Log action │  │
                    │  └───────────────┘  │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │                     │
                    │  Enterprise Tool     │
                    │  (DB, K8s, API,     │
                    │   Slack, etc.)       │
                    │                     │
                    └─────────────────────┘
```

### The Three Decision Layers

Every agent request goes through these checks:

1. **AUTO-DENY** — Instantly blocks dangerous actions (no override possible)
   - Transferring more than $5,000
   - An agent with very low trust score
   
2. **AUTO-ALLOW** — Instantly approves safe actions (no human needed)
   - Reading logs
   - Low-risk queries from trusted agents
   
3. **HITL** — Asks a human to decide
   - Deploying to production
   - Accessing sensitive data
   - Any action between safe and dangerous

---

## Troubleshooting

### "Command not found"

If you get this error:
```bash
zsh: command not found: ./eyevesa
```

Make sure you're in the right directory:
```bash
cd eyeVesa/cli
```

### "Port already in use"

If you see this when starting services:

```
Error: listen tcp :8080: bind: address already in use
```

Something else is already running on that port. Stop it first:
```bash
# Find what's using the port
lsof -i :8080

# Kill it (replace PID with the number you see)
kill -9 <PID>
```

### "Connection refused"

If the CLI can't connect:
```bash
# Make sure the services are running
./eyevesa doctor

# Check if the gateway is accessible
curl http://localhost:8080/health
```

### "Trust_bundles does not exist"

This database error is harmless. It means SPIRE (a optional security feature) isn't running. eyeVesa will still work fine without it.

### Reset Everything

If something goes wrong and you want to start fresh:

```bash
# Stop all services
docker-compose down

# Delete the database data
docker volume rm eyevesa_pgdata

# Remove your CLI configuration
rm -rf ~/.eyevesa

# Start fresh
docker-compose up -d
```

---

## Quick Reference Card

### Commands Summary

| Task | Command |
|------|---------|
| Launch dashboard | `./eyevesa tui` |
| Register agent | `./eyevesa init --name <name> --owner <org>` |
| List agents | `./eyevesa agents list` |
| List resources | `./eyevesa resources list` |
| Register resource | `./eyevesa resources register --name <n> --type mcp_server --endpoint <url>` |
| Check authorization | `./eyevesa authorize --agent-id <id> --action <action>` |
| View pending approvals | `./eyevesa hitl list` |
| Approve | `./eyevesa hitl approve <id>` |
| View audit | `./eyevesa audit <agent-id>` |
| Health check | `./eyevesa doctor` |
| Display config | `./eyevesa config show` |
| Delegate | `./eyevesa delegate create --parent <p> --child <c> --scope <s>` |

### Keyboard Shortcuts (TUI)

| Key | Action |
|-----|--------|
| Tab | Switch views |
| ↑/↓ | Navigate items |
| r | Refresh |
| a | Approve (in HITL view) |
| d | Deny (in HITL view) |
| q | Quit |

### Ports

| Service | Port | Purpose |
|---------|------|---------|
| eyeVesa API | 8080 | Main API for managing agents and resources |
| eyeVesa Proxy | 9443 | Agent connection endpoint |
| Database | 5432 | PostgreSQL + pgvector |
| Policy Engine | 8181 | OPA policy evaluation |

---

## Getting Help

- **CLI help**: `./eyevesa --help` or `./eyevesa <command> --help`
- **TUI help**: Press `?` or check the eyeVesa-tui.md file
- **Full documentation**: See the `docs/` folder in the project

---

## Appendix: Installing Prerequisites

### Install Docker

**Mac:**
```bash
# Using Homebrew
brew install --cask docker

# Or download from https://www.docker.com/products/docker-desktop/
```

**Linux:**
```bash
curl -fsSL https://get.docker.com | sh
```

### Install Go

```bash
# Using Homebrew (Mac)
brew install go

# Or download from https://go.dev/dl/
```

### Install Rust

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

After installing, restart your terminal or run:
```bash
source ~/.cargo/env
```

---

> eyeVesa — Identity and Trust Layer for AI Agents
