# eyeVesa CLI Reference

## Global Flags

| Flag | Shorthand | Default | Description |
|------|-----------|---------|-------------|
| `--config` | `-c` | `~/.eyevesa/config.toml` | Config file path |
| `--gateway` | `-g` | `http://localhost:8080` | Gateway endpoint URL |
| `--output` | `-o` | `text` | Output format: text, json |

---

## Commands

### `eyevesa init`

Register a new agent with eyeVesa, generate an Ed25519 keypair, and save configuration to `~/.eyevesa/`.

```
eyevesa init --name <name> --owner <owner> [flags]
```

| Flag | Shorthand | Default | Description |
|------|-----------|---------|-------------|
| `--name` | `-n` | | Agent name (required) |
| `--owner` | | | Agent owner (required) |
| `--capabilities` | | | Comma-separated capabilities |
| `--allowed-tools` | | | Comma-separated allowed tools |
| `--max-budget` | | `0` | Maximum budget in USD |
| `--delegation-policy` | | `no_chain` | Delegation policy: `no_chain`, `single_level` |
| `--behavioral-tags` | | | Comma-separated behavioral tags |
| `--gateway` | `-g` | | Gateway endpoint URL |

**Examples:**
```
eyevesa init --name hermes-ops --owner org:devops
eyevesa init --name my-agent --owner org:eng --capabilities "read,write" --max-budget 500
```

---

### `eyevesa agents`

List, get, and inspect registered agents.

#### `agents list`

```
eyevesa agents list
```

#### `agents get`

```
eyevesa agents get <agent-id>
```

#### `agents trust`

```
eyevesa agents trust <agent-id>
```

---

### `eyevesa resources`

List, get, and register enterprise resources.

#### `resources list`

```
eyevesa resources list
```

#### `resources get`

```
eyevesa resources get <resource-id>
```

#### `resources register`

```
eyevesa resources register --name <name> --type <type> --endpoint <url> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | | Resource name (required) |
| `--type` | | Resource type (required) |
| `--endpoint` | | Resource endpoint URL (required) |
| `--auth-method` | | Authentication method |
| `--risk-level` | | Risk level (low, medium, high, critical) |
| `--data-sensitivity` | | Data sensitivity classification |
| `--rate-limit` | `0` | Rate limit per agent |
| `--capabilities` | | JSON string of resource capabilities |

---

### `eyevesa authorize`

Check whether an agent is authorized to perform an action. Alias: `auth`.

```
eyevesa authorize --agent-id <id> --action <action> [flags]
```

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--agent-id` | | Agent ID (required) |
| `--action` | | Action/tool name (required) |
| `--resource-id` | | Resource ID |
| `--params` | `-p` | Action parameters (JSON or key=val) |

**Examples:**
```
eyevesa authorize --agent-id <id> --action database_query
eyevesa authorize --agent-id <id> --action bank_transfer --resource-id <id> --params '{"amount": 250}'
```

---

### `eyevesa hitl`

List, approve, and deny human-in-the-loop approval requests.

#### `hitl list`

```
eyevesa hitl list
```

#### `hitl approve`

```
eyevesa hitl approve <approval-id> [--method <method>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--method` | `cli` | Approval method |

#### `hitl deny`

```
eyevesa hitl deny <approval-id> [--method <method>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--method` | `cli` | Approval method |

---

### `eyevesa delegate`

Create, validate, and revoke agent-to-agent delegations.

#### `delegate create`

```
eyevesa delegate create --parent <id> --child <id> --scope <capabilities> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--parent` | | Parent agent ID (required) |
| `--child` | | Child agent ID (required) |
| `--scope` | | Comma-separated capabilities to delegate |
| `--depth` | `1` | Maximum delegation depth |
| `--duration` | `2h` | Delegation duration (e.g. 30m, 2h, 24h) |

#### `delegate validate`

```
eyevesa delegate validate --parent <id> --child <id>
```

#### `delegate list`

```
eyevesa delegate list <agent-id>
```

#### `delegate revoke`

```
eyevesa delegate revoke <delegation-id>
```

---

### `eyevesa audit`

View the audit trail for an agent.

```
eyevesa audit <agent-id> [--limit <n>] [--offset <n>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `50` | Maximum number of entries |
| `--offset` | `0` | Offset for pagination |

---

### `eyevesa mcp`

Interact with the Model Context Protocol endpoint.

#### `mcp init`

Initialize MCP connection.

```
eyevesa mcp init
```

#### `mcp tools`

List available MCP tools.

```
eyevesa mcp tools
```

---

### `eyevesa discover`

Discover available tools and resources registered with the gateway.

```
eyevesa discover [capability]
```

**Examples:**
```
eyevesa discover
eyevesa discover deployment
eyevesa discover database
```

---

### `eyevesa doctor`

Diagnose configuration and connectivity. Checks config file, keypair, gateway health, and agent identity.

```
eyevesa doctor
```

Output:
- Config file existence and validity
- Agent ID configuration
- Keypair presence
- Gateway health endpoint
- SPIFFE identity

---

### `eyevesa config`

View and manage configuration.

#### `config show`

```
eyevesa config show
```

Displays current config file path, gateway endpoint, agent ID, keypair status, and public key.

---

## Configuration

Config file: `~/.eyevesa/config.toml`

Keypair directory: `~/.eyevesa/keys/`

### Config Fields

| Field | Description |
|-------|-------------|
| `gateway_endpoint` | Gateway server URL |
| `agent_id` | Registered agent ID |
| `key_path` | Path to Ed25519 private key |
| `timeout_secs` | HTTP request timeout |

### Example

```toml
gateway_endpoint = "http://localhost:8080"
agent_id = "agent_abc123"
key_path = "/home/user/.eyevesa/keys/agent_abc123.key"
timeout_secs = 30
```