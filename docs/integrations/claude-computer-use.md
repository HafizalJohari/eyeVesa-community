# eyeVesa + Claude Computer Use Integration Guide

## Overview

This guide shows how to integrate **eyeVesa** with **Claude** (Anthropic) using two integration paths:

1. **Claude Code Computer Use MCP** — Claude Code's built-in `computer-use` MCP server that lets Claude control your screen. eyeVesa acts as an additional MCP server providing identity, authorization, trust scoring, and HITL escalation.

2. **Claude API Tool Calling** — Use the Anthropic Messages API with `tool_use` to give Claude agents eyeVesa-gated capabilities for programmatic access to resources.

### What eyeVesa Provides to Claude

| eyeVesa Capability | Benefit for Claude |
|---|---|
| **Ed25519 Identity** | Every Claude agent gets a verifiable identity passport |
| **OPA Authorization** | All resource actions are policy-gated before execution |
| **Trust Scoring** | Dynamic trust scores adjust per-action based on policy outcomes |
| **HITL Escalation** | High-risk actions require human approval (Telegram, Discord, Slack, PagerDuty) |
| **Audit Trail** | Every action logged with Ed25519 signatures |
| **Skills Registry** | Proficiency-verified skill catalog gates access to high-risk resources |
| **Delegation** | Claude can delegate scoped permissions to sub-agents (max depth 3) |
| **PTV Attestation** | Hardware-rooted identity for agents on secure enclaves |
| **Budget Control** | Per-agent spend tracking and rate limiting |

---

## Architecture

```
┌─────────────────────┐
│  Claude Code CLI     │
│  (computer-use MCP)  │─── screen control (click, type, screenshot)
└──────────┬───────────┘
           │
           │  MCP protocol
           ▼
┌─────────────────────┐     HTTP/MCP      ┌──────────────────┐
│  eyeVesa Gateway     │ ◀─────────────── │  Resource Adapter  │
│  Core (Rust :9443)   │ ──────────────▶ │  (:8443)           │
└──────────┬──────────┘     JSON response └──────────────────┘
           │
    ┌──────▼──────┐
    │ Control     │
    │ Plane (Go)  │
    │ OPA · HITL  │
    │ Audit · PTV │
    └─────────────┘
```

**When Claude needs to access a gated resource** (database, API, file system), it calls through eyeVesa instead of directly. eyeVesa authorizes the action via OPA, logs it with an Ed25519 signature, and escalates to HITL when necessary.

---

## Method 1: Claude Code — MCP Server Integration (Recommended)

Claude Code's `computer-use` is a built-in MCP server for screen control. eyeVesa provides a **second MCP server** that Claude can use alongside `computer-use` for identity-gated resource access.

### How It Works

1. Claude Code enables `computer-use` MCP for screen interaction
2. eyeVesa's MCP endpoint (`/v1/mcp`) is configured as an additional MCP server
3. Claude can both **control the screen** AND **access eyeVesa-gated resources** in the same session
4. When Claude needs to read/write a protected resource, it calls eyeVesa tools instead of raw shell commands
5. Each tool call is authorized by OPA, logged in the audit trail, and trust scores update dynamically

### Setup: Configure eyeVesa as an MCP Server for Claude Code

In your Claude Code project, add eyeVesa as an MCP server. Edit `.opencode.json` or run `/mcp` in Claude Code:

```json
{
  "mcpServers": {
    "computer-use": {
      "disabled": false
    },
    "eyevesa": {
      "type": "http",
      "url": "http://localhost:9443/v1/mcp",
      "headers": {
        "X-API-Key": "ak_live_abc123"
      }
    }
  }
}
```

Or configure via the Claude Code CLI:

```bash
# Enable built-in computer-use
claude mcp enable computer-use

# Add eyeVesa as an HTTP MCP server
claude mcp add eyevesa --transport http --url http://localhost:9443/v1/mcp --header "X-API-Key: ak_live_abc123"
```

### Available MCP Tools

Once connected, Claude has access to these eyeVesa MCP methods:

| MCP Method | Description |
|---|---|
| `initialize` | Handshake, returns capabilities (protocol version `2024-11-05`) |
| `tools/list` | List available eyeVesa-gated tools |
| `tools/call` | Execute a tool through eyeVesa (authorize → execute → audit) |
| `resources/list` | List registered resources |
| `prompts/list` | List available prompts |
| `skills/list` | List skills in the registry |
| `skills/search` | Search skills by query/category |
| `skills/endorse` | Endorse an agent's skill |

### Example: Claude Using Both Computer Use and eyeVesa

```
User: Open the dashboard app, navigate to the financial report, 
      and export the data to the approved database.

Claude: I'll use computer-use to navigate the UI, but I'll route the 
        database write through eyeVesa for authorization.

[Uses computer-use MCP] → Screenshots the dashboard, clicks to financial report
[Uses eyeVesa tools/call] → Authorizes database write via OPA
[If HITL required] → Pauses and notifies user for approval
[Uses eyeVesa tools/call] → Executes the write after authorization
```

### MCP Tool Call Example (JSON-RPC)

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "id": 3,
  "params": {
    "name": "write",
    "arguments": {
      "agent_id": "claude-agent-001",
      "resource_id": "res-financial-db",
      "location": "financial_reports",
      "data": "Q3 earnings report..."
    }
  }
}
```

**Authorized response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [{"type": "text", "text": "Action 'write' authorized for agent claude-agent-001"}],
    "authorization": {"allowed": true, "requires_hitl": false, "trust_delta": 0.01}
  }
}
```

**HITL-required response:**
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "isError": true,
    "content": [{"type": "text", "text": "Action 'bank_transfer' denied: requires HITL approval"}],
    "authorization": {"allowed": false, "requires_hitl": true, "reason": "high-risk action"}
  }
}
```

---

## Method 2: Claude API — Tool Calling (Python SDK)

Use the Anthropic Messages API with `tool_use` for programmatic agent workflows.

### Installation

```bash
pip install agentid-sdk anthropic
```

### Complete Example

```python
import asyncio
import json
from anthropic import Anthropic
from agentid_sdk import AgentClient, AgentConfig, NotAuthorizedError, HitlRequiredError


async def create_eyevesa_tools(client: AgentClient) -> list[dict]:
    """Build Claude-compatible tool definitions from eyeVesa gateway."""
    tools = [
        {
            "name": "eyevesa_read",
            "description": "Read data from an eyeVesa-gated resource. "
                           "Authorization is checked via OPA policy. "
                           "High-risk reads may require HITL approval.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "resource_id": {"type": "string", "description": "The resource ID to read from"},
                    "query": {"type": "string", "description": "The data query or key to read"},
                },
                "required": ["resource_id"],
            },
        },
        {
            "name": "eyevesa_write",
            "description": "Write data to an eyeVesa-gated resource. "
                           "Writes are typically higher risk and may require HITL approval.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "resource_id": {"type": "string", "description": "The resource ID to write to"},
                    "data": {"type": "string", "description": "The data to write (JSON string)"},
                },
                "required": ["resource_id", "data"],
            },
        },
        {
            "name": "eyevesa_request_approval",
            "description": "Proactively request human-in-the-loop approval for an action.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "action": {"type": "string", "description": "The action requiring approval"},
                    "reason": {"type": "string", "description": "Why this action needs approval"},
                    "risk_level": {"type": "string", "enum": ["low", "medium", "high", "critical"]},
                },
                "required": ["action", "reason", "risk_level"],
            },
        },
        {
            "name": "eyevesa_discover",
            "description": "Discover available resources registered with the eyeVesa gateway.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "capability": {"type": "string", "description": "Filter by capability (e.g., 'mcp')"},
                },
            },
        },
        {
            "name": "eyevesa_delegate",
            "description": "Delegate scoped permissions to another agent. Max depth 3.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "delegatee_id": {"type": "string", "description": "The agent ID to delegate to"},
                    "scope": {"type": "array", "items": {"type": "string"}, "description": "Permissions to delegate"},
                    "reason": {"type": "string", "description": "Reason for delegation"},
                },
                "required": ["delegatee_id", "scope"],
            },
        },
    ]
    return tools


async def handle_tool_call(client: AgentClient, tool_name: str, tool_input: dict) -> str:
    """Route Claude tool_use calls through eyeVesa."""
    try:
        if tool_name == "eyevesa_read":
            result = await client.invoke(
                resource_id=tool_input["resource_id"],
                tool="read",
                params={"query": tool_input.get("query", "")},
            )
            return json.dumps({"success": result.success, "data": result.data, "trust_score": result.trust_score})

        elif tool_name == "eyevesa_write":
            result = await client.invoke(
                resource_id=tool_input["resource_id"],
                tool="write",
                params={"data": tool_input["data"]},
            )
            return json.dumps({"success": result.success, "data": result.data, "trust_score": result.trust_score})

        elif tool_name == "eyevesa_request_approval":
            approval = await client.request_approval(
                action=tool_input["action"],
                reason=tool_input["reason"],
                risk_level=tool_input["risk_level"],
            )
            return json.dumps({"approval_id": approval.approval_id, "status": approval.status})

        elif tool_name == "eyevesa_discover":
            capability = tool_input.get("capability", "mcp")
            tools_info = await client.discover(capability)
            return json.dumps([t.model_dump() for t in tools_info])

        elif tool_name == "eyevesa_delegate":
            result = await client.delegate(
                delegatee_id=tool_input["delegatee_id"],
                scope=tool_input["scope"],
                reason=tool_input.get("reason", ""),
            )
            return json.dumps({"delegation_id": result.delegation_id, "status": result.status})

        return json.dumps({"error": f"Unknown tool: {tool_name}"})

    except NotAuthorizedError as e:
        return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
    except HitlRequiredError as e:
        return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})


async def run_claude_with_eyevesa():
    """Main loop: Claude agent with eyeVesa-gated tool use."""
    config = AgentConfig(
        agent_id="",
        name="claude-agent",
        owner="my-team",
        gateway_endpoint="http://localhost:9443",
    )

    async with AgentClient(config, api_key="ak_live_abc123") as client:
        await client.connect()
        print(f"Connected! Agent ID: {client.agent_id}, Trust: {client.trust_score}")

        tools = await create_eyevesa_tools(client)
        claude = Anthropic()

        messages = [{"role": "user", "content": "Read the financial report from resource res-finance-001."}]

        while True:
            response = claude.messages.create(
                model="claude-sonnet-4-20250514",
                max_tokens=4096,
                tools=tools,
                messages=messages,
            )

            messages.append({"role": "assistant", "content": response.content})

            if response.stop_reason == "end_turn":
                for block in response.content:
                    if block.type == "text":
                        print(f"Claude: {block.text}")
                break

            if response.stop_reason == "tool_use":
                tool_results = []
                for block in response.content:
                    if block.type == "tool_use":
                        print(f"Tool call: {block.name}({block.input})")
                        result = await handle_tool_call(client, block.name, block.input)
                        tool_results.append({
                            "type": "tool_result",
                            "tool_use_id": block.id,
                            "content": result,
                        })
                messages.append({"role": "user", "content": tool_results})

if __name__ == "__main__":
    asyncio.run(run_claude_with_eyevesa())
```

---

## Method 3: Claude API — TypeScript SDK

```typescript
import Anthropic from '@anthropic-ai/sdk';
import { AgentClient, AgentConfig, NotAuthorizedError, HitlRequiredError } from 'agentid-sdk';

async function runClaudeWithEyevesa() {
  const config: AgentConfig = {
    agentId: '',
    name: 'claude-agent',
    owner: 'my-team',
    gatewayEndpoint: 'http://localhost:9443',
  };

  const client = new AgentClient(config, { apiKey: 'ak_live_abc123' });
  await client.connect();
  console.log(`Connected! Trust: ${client.trustScore}`);

  const tools: Anthropic.Tool[] = [
    {
      name: 'eyevesa_read',
      description: 'Read data from an eyeVesa-gated resource. Authorization via OPA policy.',
      input_schema: {
        type: 'object' as const,
        properties: {
          resource_id: { type: 'string', description: 'Resource ID to read from' },
          query: { type: 'string', description: 'Data query or key' },
        },
        required: ['resource_id'],
      },
    },
    {
      name: 'eyevesa_request_approval',
      description: 'Request human-in-the-loop approval for a sensitive action.',
      input_schema: {
        type: 'object' as const,
        properties: {
          action: { type: 'string', description: 'Action requiring approval' },
          reason: { type: 'string', description: 'Why approval is needed' },
          risk_level: { type: 'string', enum: ['low', 'medium', 'high', 'critical'] },
        },
        required: ['action', 'reason', 'risk_level'],
      },
    },
  ];

  const anthropic = new Anthropic();
  const messages: Anthropic.MessageParam[] = [
    { role: 'user', content: 'Read the financial report from resource res-finance-001.' },
  ];

  while (true) {
    const response = await anthropic.messages.create({
      model: 'claude-sonnet-4-20250514',
      max_tokens: 4096,
      tools,
      messages,
    });

    messages.push({ role: 'assistant', content: response.content });

    if (response.stop_reason === 'end_turn') {
      for (const block of response.content) {
        if (block.type === 'text') console.log(`Claude: ${block.text}`);
      }
      break;
    }

    if (response.stop_reason === 'tool_use') {
      const toolResults: Anthropic.ToolResultBlockParam[] = [];
      for (const block of response.content) {
        if (block.type === 'tool_use') {
          let result: Record<string, unknown>;
          try {
            if (block.name === 'eyevesa_read') {
              const r = await client.invoke(
                (block.input as Record<string, unknown>).resource_id as string,
                'read',
                { query: (block.input as Record<string, unknown>).query }
              );
              result = { success: r.success, data: r.data, trustScore: r.trustScore };
            } else if (block.name === 'eyevesa_request_approval') {
              const a = await client.requestApproval(
                (block.input as Record<string, unknown>).action as string,
                (block.input as Record<string, unknown>).reason as string,
                (block.input as Record<string, unknown>).risk_level as string,
              );
              result = { approvalId: a.approvalId, status: a.status };
            } else {
              result = { error: `Unknown tool: ${block.name}` };
            }
          } catch (e) {
            if (e instanceof NotAuthorizedError) result = { error: 'NOT_AUTHORIZED', reason: e.message };
            else if (e instanceof HitlRequiredError) result = { error: 'HITL_REQUIRED', reason: e.message };
            else result = { error: String(e) };
          }
          toolResults.push({ type: 'tool_result', tool_use_id: block.id, content: JSON.stringify(result) });
        }
      }
      messages.push({ role: 'user', content: toolResults });
    }
  }
}

runClaudeWithEyevesa().catch(console.error);
```

---

## Advanced: HITL Escalation Loop

When an action requires human approval, Claude can automatically escalate and poll:

```python
import asyncio
from agentid_sdk import AgentClient, HitlRequiredError

HITL_POLL_INTERVAL = 5
HITL_MAX_WAIT = 300

async def invoke_with_hitl_fallback(
    client: AgentClient, resource_id: str, tool: str, params: dict | None = None,
) -> dict:
    """Try to invoke; if HITL required, escalate and poll."""
    try:
        result = await client.invoke(resource_id, tool, params)
        return result.data
    except HitlRequiredError as e:
        approval = await client.request_approval(
            action=tool, reason=f"Auto-escalated: {e}", risk_level="high",
        )
        print(f"HITL approval requested: {approval.approval_id}")
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
```

---

## Advanced: PTV Hardware Attestation

For agents running on secure hardware:

```python
async def attest_and_bind(client: AgentClient):
    attest_result = await client.attest(platform="macos-secure-enclave", firmware_version="1.0.0")
    bind_result = await client.bind(
        attestation=attest_result.attestation,
        tpm_signature=attest_result.tpm_signature,
        platform="macos-secure-enclave",
        firmware_version="1.0.0",
    )
    is_valid = await client.verify_binding(bind_result.binding_id)
    return bind_result
```

---

## Advanced: Skills-Based Authorization

```python
async def invoke_with_skill_check(
    client: AgentClient, agent_id: str, resource_id: str,
    tool: str, required_skill: str, min_proficiency: int = 3,
) -> dict:
    scores = await client.get_skill_trust(agent_id)
    skill_match = [s for s in scores if s.skill_name == required_skill]
    if not skill_match or skill_match[0].trust_score < 0.5:
        approval = await client.request_approval(
            action=f"{tool} (skill: {required_skill})",
            reason=f"Agent lacks sufficient {required_skill} proficiency",
            risk_level="high",
        )
        return {"status": "pending_approval", "approval_id": approval.approval_id}
    result = await client.invoke(resource_id, tool)
    return result.data
```

---

## Airport Integration

Claude agents can discover and connect with other agents at the Airport:

```python
from agentid_sdk import AgentClient, AgentConfig, ClaudeIntegration

# After connecting
integration = ClaudeIntegration(client)

# Heartbeat — announce presence
await client.airport_heartbeat(status="online", metadata={"framework": "claude"})

# Update profile — become discoverable
await client.airport_update_profile(
    description="Claude code review agent",
    services_offered=["code_review", "security_analysis"],
    tags=["claude", "code-review", "security"],
    listed=True,
)

# Search for agents with specific capabilities
results = await client.airport_search(capability="weather", min_trust=0.8, status="online")

# Get a specific agent's profile
profile = await client.airport_get_profile("agent-uuid-here")

# See who's online
online = await client.airport_list_online()

# View connection history
connections = await client.airport_connections(agent_id="agent-uuid-here", limit=20)
```

### Auth Policy
Airport browse endpoints (search, online, profile, health) are **public** — no API key needed. Write endpoints (heartbeat, update-profile, connections) require `X-API-Key` header.

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GATEWAY_ENDPOINT` | No | eyeVesa gateway URL (default: `http://localhost:9443`) |
| `AGENT_NAME` | No | Agent display name (default: `claude-agent`) |
| `AGENT_OWNER` | No | Agent owner/org (default: `default`) |
| `ANTHROPIC_API_KEY` | Yes | Your Anthropic API key |

---

## Key Considerations

1. **Trust Scores**: Start at 1.0. Allowed actions get `+0.01`, denied get `-0.05`, budget-exceeding get `-0.1`.

2. **HITL Notifications**: Sent via Slack, PagerDuty, Telegram, Discord, or Push. 5-minute default expiry.

3. **Delegation Depth**: Max depth 3. Each delegation is scoped and audit-logged.

4. **MCP Protocol**: eyeVesa gateway supports MCP version `2024-11-05` with capabilities: tools, resources, prompts, skills.

5. **Claude Code Computer Use**: The `computer-use` MCP server handles screen control. eyeVesa MCP handles identity-gated resource access. They work side by side.

6. **API Key Auth**: For production, set `AUTH_ENABLED=true` on the gateway and pass `api_key` or `jwt_token` to the SDK.

7. **Multiple Agents**: Each Claude invocation can use a different eyeVesa identity with its own trust score, skills, and delegation scope.