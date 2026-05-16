# Hermes Agent Setup Guide

Complete guide for setting up Hermes Agent by Nous Research and connecting it to eyeVesa for identity, authorization, and audit.

---

## What is Hermes?

Hermes is a self-improving autonomous AI agent built by [Nous Research](https://nousresearch.com). It features a closed learning loop — creating skills from experience, improving them during use, and building a deepening model of who you are across sessions. It runs anywhere (VPS, GPU cluster, serverless), lives on 20+ messaging platforms, and supports MCP for extended tool capabilities.

**Key capabilities:**
- 70+ built-in tools (terminal, file, web, browser, code execution)
- MCP client and server support
- Persistent memory across sessions
- Agent-created skills (procedural memory)
- 20+ messaging platforms (Telegram, Discord, Slack, WhatsApp, etc.)
- Dangerous command approval system (manual, smart, off)
- Docker/Modal/Daytona sandboxing
- Delegation to sub-agents for parallel work
- Scheduled automation (cron)

**Repository:** [github.com/NousResearch/hermes-agent](https://github.com/NousResearch/hermes-agent)
**Docs:** [hermes-agent.nousresearch.com/docs/](https://hermes-agent.nousresearch.com/docs/)

---

## 1. Install Hermes

### Linux / macOS / WSL2

```bash
curl -fsSL https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.sh | bash
```

Or via pip:

```bash
pip install hermes-agent
hermes postinstall
```

### Windows (native, PowerShell) — Early Beta

```powershell
irm https://raw.githubusercontent.com/NousResearch/hermes-agent/main/scripts/install.ps1 | iex
```

### After Installation

Reload your shell:

```bash
source ~/.bashrc   # or: source ~/.zshrc
```

Verify:

```bash
hermes doctor
```

---

## 2. Choose an LLM Provider

```bash
hermes model
```

This launches an interactive picker. Popular options:

| Provider | Setup | Notes |
|----------|-------|-------|
| **Nous Portal** | OAuth login | Zero-config, subscription-based |
| **OpenRouter** | API key | Multi-provider routing |
| **Anthropic** | API key or OAuth | Claude models |
| **OpenAI Codex** | OAuth | ChatGPT subscription |
| **Custom Endpoint** | Base URL + API key | vLLM, Ollama, SGLang, etc. |

**Minimum requirement:** 64K token context window.

Secrets go to `~/.hermes/.env`, settings to `~/.hermes/config.yaml`. Use `hermes config set` to route values automatically:

```bash
hermes config set OPENROUTER_API_KEY sk-or-...
hermes config set model openrouter/anthropic/claude-sonnet-4
```

---

## 3. Run Your First Chat

```bash
hermes              # Classic CLI
hermes --tui        # Modern TUI (recommended)
```

Try:

```
Summarize this repo in 5 bullets and tell me what the main entrypoint is.
```

Verify session resume works:

```bash
hermes --continue    # Resume last session
```

---

## 4. Connect a Messaging Platform (Optional)

```bash
hermes gateway setup
```

Choose from: Telegram, Discord, Slack, WhatsApp, Signal, Matrix, Mattermost, Email, SMS, Microsoft Teams, Google Chat, and more.

For example, Telegram:
1. Create a bot via [@BotFather](https://t.me/BotFather)
2. Enter the bot token when prompted
3. Set allowed users: `TELEGRAM_ALLOWED_USERS=123456789` in `~/.hermes/.env`
4. Start the gateway: `hermes gateway`

---

## 5. Configure MCP Servers (Optional)

Add MCP servers in `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  # GitHub — stdio server
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "ghp_xxx"
    tools:
      include: [list_issues, create_issue, search_code]

  # Custom API — HTTP server
  company_api:
    url: "https://mcp.internal.example.com"
    headers:
      Authorization: "Bearer xxx"
```

Tools from MCP servers appear prefixed: `mcp_github_list_issues`, `mcp_company_api_query_data`.

Reload MCP config at runtime:

```
/reload-mcp
```

---

## 6. Configure Security

### Approval Modes

```yaml
# ~/.hermes/config.yaml
approvals:
  mode: manual    # manual | smart | off
  timeout: 60     # seconds
```

| Mode | Behavior |
|------|----------|
| `manual` (default) | Prompt user for every dangerous command |
| `smart` | LLM assesses risk; auto-approve low-risk, auto-deny dangerous, escalate uncertain |
| `off` | No approval checks (YOLO mode) |

### Docker Sandbox

```yaml
terminal:
  backend: docker
  docker_image: "nikolaik/python-nodejs:python3.11-nodejs20"
  container_cpu: 1
  container_memory: 5120
  container_persistent: true
```

### Website Blocklist

```yaml
security:
  website_blocklist:
    enabled: true
    domains:
      - "*.internal.company.com"
      - "admin.example.com"
```

### User Authorization (Gateway)

```bash
# ~/.hermes/.env
TELEGRAM_ALLOWED_USERS=123456789,987654321
GATEWAY_ALLOWED_USERS=123456789
```

---

## 7. Install and Manage Skills

```bash
# Browse available skills
hermes skills browse

# Search
hermes skills search kubernetes

# Install
hermes skills install openai/skills/k8s

# Or from URL
hermes skills install https://example.com/SKILL.md

# List installed
hermes skills list

# Update
hermes skills update
```

Skills live in `~/.hermes/skills/` and are available as slash commands: `/k8s`, `/plan`, etc.

---

## 8. Connect Hermes to eyeVesa

This is where Hermes gets cryptographic identity, policy-based authorization, trust scoring, and non-repudiable audit trails.

### 8.1 Start the Gateway

```bash
# Terminal 1: Infrastructure
docker-compose up -d

# Terminal 2: Control Plane (HTTP :8080, gRPC :9090)
cd gateway/control-plane && go run cmd/api/main.go

# Terminal 3: Gateway Core Proxy (HTTP :9443)
cd gateway/core && cargo run
```

### 8.2 Register Hermes as an Agent

```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hermes-ops",
    "owner": "org:devops",
    "capabilities": ["infrastructure_read", "infrastructure_write", "deployment", "code_review"],
    "allowed_tools": ["k8s_deploy", "k8s_scale", "log_search", "incident_create", "github_pr"],
    "max_budget_usd": 500.0,
    "delegation_policy": "single_level",
    "behavioral_tags": ["production", "sre", "high_autonomy"]
  }'
```

Save the `agent_id` and `public_key` from the response.

### 8.3 Register Enterprise Resources

```bash
# Kubernetes API
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "k8s-api",
    "type": "mcp_server",
    "endpoint": "https://k8s-adapter:8443",
    "auth_method": "mTLS+SVID",
    "risk_level": "high",
    "data_sensitivity": "restricted",
    "rate_limit_per_agent": 50
  }'

# Analytics Database
curl -X POST http://localhost:8080/v1/resources/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "analytics-db",
    "type": "mcp_server",
    "endpoint": "https://db-adapter:8443",
    "risk_level": "medium",
    "data_sensitivity": "internal",
    "rate_limit_per_agent": 200
  }'
```

### 8.4 Configure Hermes MCP to Use Gateway

In `~/.hermes/config.yaml`, add eyeVesa as an HTTP MCP server:

```yaml
mcp_servers:
  agentid-gateway:
    url: "https://gateway.yourcompany.com:9443/v1/mcp"
    headers:
      X-Agent-ID: "550e8400-e29b-41d4-a716-446655440000"
    tools:
      include: [tools/list, tools/call, resources/list, prompts/list]
```

Or for local development:

```yaml
mcp_servers:
  agentid-gateway:
    url: "http://localhost:9443/v1/mcp"
    headers:
      X-Agent-ID: "550e8400-e29b-41d4-a716-446655440000"
```

Then reload:

```
/reload-mcp
```

Hermes will discover the gateway's MCP tools and register them as `mcp_agentid_gateway_tools_list`, `mcp_agentid_gateway_tools_call`, etc.

### 8.5 Set Environment Variables

```bash
# ~/.hermes/.env
EYEVESA_AGENT_ID=550e8400-e29b-41d4-a716-446655440000
EYEVESA_AGENT_NAME=hermes-ops
EYEVESA_AGENT_OWNER=org:devops
EYEVESA_GATEWAY=https://gateway.yourcompany.com:9443
EYEVESA_KEY_PATH=/run/secrets/hermes.key
```

---

## 9. How It Works Together

### Decision Flow

```
User sends message to Hermes (Telegram/Discord/CLI)
  │
  ├── Hermes LLM reasons about the request
  │
  ├── Hermes decides to invoke a tool
  │
  ├── Is it a local tool? (terminal, file, web)
  │     └── Execute directly (Hermes approval system applies)
  │
  └── Is it an enterprise resource? (via eyeVesa Gateway MCP)
        │
        ├── Hermes sends MCP tools/call to eyeVesa Gateway
        │
        ├── Gateway verifies Ed25519 identity
        │
        ├── Gateway evaluates OPA policy
        │     ├── AUTO-DENY (trust < 0.1, budget exceeded, never event)
        │     │     └── Return error, trust -= 0.05
        │     │
        │     ├── AUTO-ALLOW (trust > 0.8, low-risk, tool in allowed_tools)
        │     │     └── Execute, trust += 0.01
        │     │
        │     └── HITL (production deploy, bank transfer > $100, restricted data)
        │           └── Pending human approval
        │
        ├── Gateway signs audit log entry
        │
        └── Gateway returns result + trust delta to Hermes
              │
              └── Hermes LLM adapts behavior based on result
                    • Success → continue plan
                    • Denied → find alternative
                    • HITL → summarize for human via messaging platform
```

### Two-Layer Security

Hermes has its own security system. eyeVesa adds a complementary layer:

| Layer | System | What it controls |
|-------|--------|-----------------|
| **Hermes approval** | Hermes Agent | Should this *shell command* run? (local process safety) |
| **eyeVesa authorization** | eyeVesa Gateway | Should this *agent identity* access this *enterprise resource*? (remote access policy) |

They're orthogonal. A `k8s_deploy` command passes Hermes's local approval, then separately passes through eyeVesa's policy engine for enterprise resource authorization.

### HITL Bridge

When eyeVesa requires human approval, Hermes can surface it on any connected platform:

```
eyeVesa: "k8s_deploy to production requires HITL approval"
  │
  └── Hermes receives requires_hitl response
        │
        └── Hermes LLM generates human-readable summary:
              "I need to deploy api-server v2.1.0 to production.
               I've deployed this service 47 times successfully.
               Recommend: APPROVE."
        │
        └── Hermes sends to Slack/Telegram/Discord via messaging gateway
              │
              └── Human taps Approve or Deny
                    │
                    └── eyeVesa records approval, action executes, trust updated
```

### Trust Score Adaptation

Hermes's behavior adapts based on trust score feedback from eyeVesa:

| Trust Score | Behavior |
|-------------|----------|
| 1.0 - 0.8 | Normal operation. Auto-allow for low-risk resources. |
| 0.8 - 0.5 | Restricted. Avoid risky actions. HITL for most enterprise access. |
| 0.5 - 0.1 | Heavily restricted. Only essential operations. Everything requires HITL. |
| < 0.1 | Blocked. Cannot operate. Requires human review. |

Every action adjusts trust:
- Successful action: +0.01
- Policy denied: -0.05
- Budget exceeded: -0.10
- Never event: BLOCKED (auto-deny)
- HITL approved: +0.01
- HITL denied: -0.02
- HITL expired: -0.01

---

## 10. Hermes CLI Quick Reference

```bash
# Setup
hermes setup              # Full setup wizard
hermes model              # Choose LLM provider
hermes tools              # Configure tools per platform
hermes gateway setup      # Configure messaging platforms

# Chat
hermes                    # Start CLI chat
hermes --tui              # Start TUI chat
hermes --continue         # Resume last session
hermes --yolo             # Skip all approval prompts (dangerous!)

# Skills
hermes skills browse
hermes skills search <query>
hermes skills install <skill>
hermes skills list

# Config
hermes config             # View config
hermes config edit        # Open config.yaml in editor
hermes config set KEY VAL # Set a value
hermes config check       # Check for missing options
hermes doctor             # Diagnose issues

# Gateway (messaging)
hermes gateway            # Start messaging gateway
hermes gateway status     # Check gateway status

# Pairing (for DM auth)
hermes pairing list
hermes pairing approve <platform> <code>
hermes pairing revoke <platform> <user_id>
```

---

## 11. Config File Reference

### `~/.hermes/config.yaml`

```yaml
# LLM Provider
model: openrouter/anthropic/claude-sonnet-4

# Terminal backend
terminal:
  backend: docker              # local | docker | ssh | modal | daytona | vercel_sandbox | singularity
  docker_image: "nikolaik/python-nodejs:python3.11-nodejs20"
  container_cpu: 1
  container_memory: 5120
  container_persistent: true

# Security
approvals:
  mode: manual                 # manual | smart | off
  timeout: 60

security:
  website_blocklist:
    enabled: true
    domains:
      - "*.internal.company.com"

# Context compression
compression:
  enabled: true
  threshold: 0.50

# Memory
memory:
  memory_enabled: true
  user_profile_enabled: true

# MCP servers (includes eyeVesa)
mcp_servers:
  agentid-gateway:
    url: "http://localhost:9443/v1/mcp"
    headers:
      X-Agent-ID: "550e8400-e29b-41d4-a716-446655440000"
    tools:
      include: [tools/list, tools/call, resources/list, prompts/list]

  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "ghp_xxx"
    tools:
      include: [list_issues, create_issue, search_code]
```

### `~/.hermes/.env`

```bash
# LLM Provider
OPENROUTER_API_KEY=sk-or-...

# eyeVesa
EYEVESA_AGENT_ID=550e8400-e29b-41d4-a716-446655440000
EYEVESA_AGENT_NAME=hermes-ops
EYEVESA_AGENT_OWNER=org:devops
EYEVESA_GATEWAY=https://gateway.yourcompany.com:9443

# Messaging (if using gateway)
TELEGRAM_BOT_TOKEN=123456:ABC-...
TELEGRAM_ALLOWED_USERS=123456789

# Terminal (if using SSH backend)
TERMINAL_SSH_HOST=agent-worker.local
TERMINAL_SSH_USER=hermes
TERMINAL_SSH_KEY=~/.ssh/hermes_agent_key
```

---

## 12. Directory Structure

```
~/.hermes/
├── config.yaml           # Settings (model, terminal, TTS, compression, etc.)
├── .env                  # API keys and secrets
├── auth.json             # OAuth credentials
├── SOUL.md               # Agent personality
├── memories/             # Persistent memory (MEMORY.md, USER.md)
├── skills/               # Agent-created and installed skills
├── cron/                 # Scheduled automation jobs
├── sessions/             # Session database
└── logs/                  # Logs (errors.log, gateway.log)
```

---

## 13. Troubleshooting

| Problem | Solution |
|---------|----------|
| `hermes: command not found` | `source ~/.bashrc` or check PATH |
| Empty/broken replies | Run `hermes model` and verify provider, model, and auth |
| Gateway won't start | Run `hermes gateway status` and check bot tokens |
| MCP server not connecting | Check `node --version`, verify config, reload with `/reload-mcp` |
| Tools not appearing | Check `enabled: false`, filter config, or server connectivity |
| Docker backend fails | Run `docker version`; fall back with `hermes config set terminal.backend local` |
| API key not set | `hermes config set OPENROUTER_API_KEY sk-or-...` |

### Recovery Toolkit

```bash
hermes doctor              # Diagnose issues
hermes model               # Reconfigure provider
hermes setup               # Full setup wizard
hermes config check        # Check for missing config
hermes gateway status      # Check gateway health
```

---

## 14. Next Steps

- **[eyeVesa README](../README.md)** — Architecture, API reference, and database schema
- **[How to Use](./HOW_TO_USE.md)** — Detailed API usage and flows
- **[Hermes Docs](https://hermes-agent.nousresearch.com/docs/)** — Full Hermes documentation
- **[Hermes MCP Integration](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp)** — MCP client/server setup
- **[Hermes Security](https://hermes-agent.nousresearch.com/docs/user-guide/security)** — Approval modes, container isolation, SSRF protection
- **[Hermes Skills](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills)** — Skill system and Skills Hub