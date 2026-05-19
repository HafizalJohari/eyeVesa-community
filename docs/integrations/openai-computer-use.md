# eyeVesa + OpenAI Computer Use Integration Guide

## Overview

This guide shows how to integrate **eyeVesa** with **OpenAI's Computer Use** (CUA — Computer Use Agent) to give OpenAI agents cryptographic identity, policy-based authorization, trust scoring, HITL escalation, and non-repudiable audit trails.

OpenAI's Computer Use uses the **Responses API** with the `computer` tool type, and supports three integration paths:

1. **Built-in `computer` tool** — OpenAI returns screen actions (click, type, scroll, screenshot) and your harness executes them
2. **Custom tool/harness** — Use Playwright, Selenium, or MCP alongside your existing automation
3. **Code-execution harness** — The model writes and runs code in a sandbox that can interact with the UI

eyeVesa integrates as a **policy and identity layer** alongside any of these paths.

### What eyeVesa Provides

| eyeVesa Capability | Benefit for OpenAI Agents |
|---|---|
| **Ed25519 Identity** | Every agent gets a verifiable cryptographic identity |
| **OPA Authorization** | Resource actions are policy-gated before execution |
| **Trust Scoring** | Dynamic scores adjust per-action (allowed: +0.01, denied: -0.05, budget exceeded: -0.1) |
| **HITL Escalation** | High-risk actions require human approval (5-min default expiry) |
| **Audit Trail** | Non-repudiable Ed25519-signed logs of every action |
| **Skills Registry** | Proficiency-verified skill gates for high-risk resources |
| **Delegation** | Scoped agent-to-agent delegation (max depth 3) |
| **PTV Attestation** | Hardware-rooted identity for secure enclaves |
| **Budget Control** | Per-agent spend tracking and rate limiting |

---

## Architecture

```
┌──────────────────┐                                        ┌──────────────────┐
│  OpenAI API       │                                       │  eyeVesa Gateway   │
│  (gpt-5.5/4.1)   │                                       │  Core (Rust :9443) │
└───────┬──────────┘                                       └────────┬─────────┘
        │                                                           │
        │  Responses API                                           │
        │  (computer_call, function_call)                          │
        ▼                                                           ▼
┌──────────────────┐   HTTP/MCP    ┌──────────────────┐    ┌──────┴──────────┐
│  Agent Harness    │ ◀──────────── │  eyeVesa Agent   │───▶│  Control Plane   │
│  (Playwright/VM) │               │  SDK (Python/TS)  │    │  (Go)            │
│                  │ ◀──────────── │                   │    │  OPA · HITL     │
│  Exec actions    │   eyeVesa     │  Authorize · Sign │    │  Audit · PTV     │
│  Capture screen  │   responses   │  Trust · Delegate │    │  Budget · Skills │
└──────────────────┘               └──────────────────┘    └─────────────────┘
```

**Flow**:
1. Agent harness sends task to OpenAI Responses API with `computer` and/or eyeVesa tools
2. OpenAI returns `computer_call` actions (click, type, screenshot) **and/or** `function_call` for eyeVesa tools
3. Harness executes screen actions via Playwright/VM, and eyeVesa calls go through the gateway
4. eyeVesa authorizes via OPA, logs to audit trail, and escalates to HITL when needed
5. Screenshots and eyeVesa results are sent back as `computer_call_output` / `function_call_output`

---

## Method 1: Built-in Computer Use + eyeVesa Function Calling (Python)

Combine OpenAI's built-in `computer` tool with eyeVesa function tools. The agent can both control the screen and access eyeVesa-gated resources.

### Installation

```bash
pip install agentid-sdk openai playwright
npx playwright install chromium
```

### Complete Example

```python
import asyncio
import base64
import json
from openai import OpenAI
from playwright.sync_api import sync_playwright
from agentid_sdk import AgentClient, AgentConfig, NotAuthorizedError, HitlRequiredError


# ── eyeVesa function definitions for OpenAI ──────────────────────────────

EYEVESA_FUNCTIONS = [
    {
        "type": "function",
        "name": "eyevesa_read",
        "description": "Read data from an eyeVesa-gated resource. Authorization via OPA policy. May require HITL approval.",
        "parameters": {
            "type": "object",
            "properties": {
                "resource_id": {"type": "string", "description": "Resource ID to read from"},
                "query": {"type": "string", "description": "Data query or key"},
            },
            "required": ["resource_id"],
        },
    },
    {
        "type": "function",
        "name": "eyevesa_write",
        "description": "Write data to an eyeVesa-gated resource. High-risk; may require HITL approval.",
        "parameters": {
            "type": "object",
            "properties": {
                "resource_id": {"type": "string", "description": "Resource ID to write to"},
                "data": {"type": "string", "description": "Data to write (JSON string)"},
            },
            "required": ["resource_id", "data"],
        },
    },
    {
        "type": "function",
        "name": "eyevesa_request_approval",
        "description": "Request human-in-the-loop approval for a sensitive action (bank transfers, deletions, etc.)",
        "parameters": {
            "type": "object",
            "properties": {
                "action": {"type": "string", "description": "Action requiring approval"},
                "reason": {"type": "string", "description": "Why approval is needed"},
                "risk_level": {"type": "string", "enum": ["low", "medium", "high", "critical"]},
            },
            "required": ["action", "reason", "risk_level"],
        },
    },
    {
        "type": "function",
        "name": "eyevesa_discover",
        "description": "Discover available resources in the eyeVesa gateway.",
        "parameters": {
            "type": "object",
            "properties": {
                "capability": {"type": "string", "description": "Filter by capability (e.g., 'mcp')"},
            },
        },
    },
    {
        "type": "function",
        "name": "eyevesa_delegate",
        "description": "Delegate scoped permissions to another agent. Maximum depth 3.",
        "parameters": {
            "type": "object",
            "properties": {
                "delegatee_id": {"type": "string", "description": "Agent ID to delegate to"},
                "scope": {"type": "array", "items": {"type": "string"}, "description": "Permissions to delegate"},
                "reason": {"type": "string", "description": "Reason for delegation"},
            },
            "required": ["delegatee_id", "scope"],
        },
    },
]


async def handle_eyevesa_call(client: AgentClient, name: str, args: dict) -> str:
    """Route OpenAI function_call to eyeVesa."""
    try:
        if name == "eyevesa_read":
            result = await client.invoke(
                resource_id=args["resource_id"], tool="read",
                params={"query": args.get("query", "")},
            )
            return json.dumps({"success": result.success, "data": result.data, "trust_score": result.trust_score})

        elif name == "eyevesa_write":
            result = await client.invoke(
                resource_id=args["resource_id"], tool="write",
                params={"data": args["data"]},
            )
            return json.dumps({"success": result.success, "data": result.data, "trust_score": result.trust_score})

        elif name == "eyevesa_request_approval":
            approval = await client.request_approval(
                action=args["action"], reason=args["reason"], risk_level=args["risk_level"],
            )
            return json.dumps({"approval_id": approval.approval_id, "status": approval.status})

        elif name == "eyevesa_discover":
            capability = args.get("capability", "mcp")
            tools_info = await client.discover(capability)
            return json.dumps([t.model_dump() for t in tools_info])

        elif name == "eyevesa_delegate":
            result = await client.delegate(
                delegatee_id=args["delegatee_id"], scope=args["scope"],
                reason=args.get("reason", ""),
            )
            return json.dumps({"delegation_id": result.delegation_id, "status": result.status})

        return json.dumps({"error": f"Unknown function: {name}"})

    except NotAuthorizedError as e:
        return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
    except HitlRequiredError as e:
        return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})


# ── Playwright action handlers ────────────────────────────────────────────

def handle_computer_actions(page, actions):
    """Execute OpenAI computer actions via Playwright."""
    for action in actions:
        action_type = action.type if hasattr(action, "type") else action.get("type")

        if action_type == "click":
            page.mouse.click(action.x, action.y, button=action.button if hasattr(action, "button") else "left")
        elif action_type == "double_click":
            page.mouse.dblclick(action.x, action.y)
        elif action_type == "type":
            page.keyboard.type(action.text if hasattr(action, "text") else action["text"])
        elif action_type == "keypress":
            keys = action.keys if hasattr(action, "keys") else action["keys"]
            for key in keys:
                normalized = _normalize_key(key)
                page.keyboard.press(normalized)
        elif action_type == "scroll":
            page.mouse.move(action.x, action.y)
            page.mouse.wheel(action.get("scroll_x", 0), action.get("scroll_y", 0))
        elif action_type == "drag":
            path = action.path if hasattr(action, "path") else action["path"]
            if len(path) >= 2:
                page.mouse.move(path[0]["x"], path[0]["y"])
                page.mouse.down()
                for point in path[1:]:
                    page.mouse.move(point["x"], point["y"])
                page.mouse.up()
        elif action_type in ("wait", "screenshot"):
            pass  # handled by screenshot capture


def _normalize_key(key: str) -> str:
    key_map = {
        "ENTER": "Enter", "RETURN": "Enter", "ESC": "Escape", "ESCAPE": "Escape",
        "TAB": "Tab", "SPACE": "Space", "BACKSPACE": "Backspace",
        "DELETE": "Delete", "DEL": "Delete", "UP": "ArrowUp", "DOWN": "ArrowDown",
        "LEFT": "ArrowLeft", "RIGHT": "ArrowRight", "CTRL": "Control",
        "SHIFT": "Shift", "ALT": "Alt", "META": "Meta", "CMD": "Meta",
    }
    return key_map.get(key, key)


# ── Main loop ─────────────────────────────────────────────────────────────

async def run_openai_with_eyevesa():
    """OpenAI Computer Use agent with eyeVesa-gated resource access."""
    import asyncio

    # 1. Connect to eyeVesa
    config = AgentConfig(
        agent_id="", name="openai-agent", owner="my-team",
        gateway_endpoint="http://localhost:9443",
    )
    client = AgentClient(config, api_key="ak_live_abc123")
    async with client:
        await client.connect()
        print(f"Connected! Agent ID: {client.agent_id}, Trust: {client.trust_score}")

    # 2. Start Playwright
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=False, chromium_sandbox=True)
        page = browser.new_page(viewport={"width": 1280, "height": 720})

        openai_client = OpenAI()

        # 3. First request: combine computer tool + eyeVesa functions
        response = openai_client.responses.create(
            model="gpt-5.5",
            tools=[{"type": "computer"}] + EYEVESA_FUNCTIONS,
            input="Navigate to the dashboard and read the financial data from the eyeVesa-gated resource res-finance-001.",
        )

        # 4. Process response loop
        while True:
            # Check for computer calls
            computer_call = next(
                (item for item in response.output if item.type == "computer_call"), None
            )
            # Check for function calls
            function_calls = [item for item in response.output if item.type == "function_call"]

            if not computer_call and not function_calls:
                # No more actions — print final text
                for item in response.output:
                    if hasattr(item, "content") and isinstance(item.content, list):
                        for block in item.content:
                            if hasattr(block, "text"):
                                print(f"Agent: {block.text}")
                    elif hasattr(item, "text"):
                        print(f"Agent: {item.text}")
                break

            input_items = []

            # Handle computer actions
            if computer_call:
                handle_computer_actions(page, computer_call.actions)
                screenshot_bytes = page.screenshot()
                screenshot_b64 = base64.b64encode(screenshot_bytes).decode("utf-8")
                input_items.append({
                    "type": "computer_call_output",
                    "call_id": computer_call.call_id,
                    "output": {
                        "type": "computer_screenshot",
                        "image_url": f"data:image/png;base64,{screenshot_b64}",
                        "detail": "original",
                    },
                })

            # Handle eyeVesa function calls
            for fc in function_calls:
                result_json = await handle_eyevesa_call(client, fc.name, json.loads(fc.arguments) if isinstance(fc.arguments, str) else fc.arguments)
                input_items.append({
                    "type": "function_call_output",
                    "call_id": fc.call_id,
                    "output": result_json,
                })

            # Send results back
            response = openai_client.responses.create(
                model="gpt-5.5",
                tools=[{"type": "computer"}] + EYEVESA_FUNCTIONS,
                previous_response_id=response.id,
                input=input_items,
            )

        browser.close()


if __name__ == "__main__":
    asyncio.run(run_openai_with_eyevesa())
```

---

## Method 2: Responses API — Function Calling Only (No Screen Control)

For back-end agents that don't need screen control, use OpenAI's function calling directly:

```python
import asyncio
import json
from openai import OpenAI
from agentid_sdk import AgentClient, AgentConfig


async def run_openai_functions_only():
    config = AgentConfig(
        agent_id="", name="openai-backend-agent", owner="my-team",
        gateway_endpoint="http://localhost:9443",
    )

    async with AgentClient(config, api_key="ak_live_abc123") as client:
        await client.connect()

        openai_client = OpenAI()

        response = openai_client.responses.create(
            model="gpt-4.1",
            tools=EYEVESA_FUNCTIONS,  # defined above
            input="Check the financial report from resource res-finance-001 and summarize it.",
        )

        # Process function calls
        while True:
            function_calls = [item for item in response.output if item.type == "function_call"]
            if not function_calls:
                for item in response.output:
                    if hasattr(item, "text"):
                        print(f"Agent: {item.text}")
                break

            input_items = []
            for fc in function_calls:
                args = json.loads(fc.arguments) if isinstance(fc.arguments, str) else fc.arguments
                result = await handle_eyevesa_call(client, fc.name, args)
                input_items.append({
                    "type": "function_call_output",
                    "call_id": fc.call_id,
                    "output": result,
                })

            response = openai_client.responses.create(
                model="gpt-4.1",
                tools=EYEVESA_FUNCTIONS,
                previous_response_id=response.id,
                input=input_items,
            )


asyncio.run(run_openai_functions_only())
```

---

## Method 3: Code-Execution Harness with eyeVesa

OpenAI's code-execution harness lets the model write and run Python/JS code. You can inject eyeVesa SDK calls into the execution environment:

```python
import asyncio
import base64
from openai import OpenAI
from playwright.sync_api import sync_playwright
from agentid_sdk import AgentClient, AgentConfig


async def run_code_execution_with_eyevesa():
    config = AgentConfig(agent_id="", name="code-agent", owner="my-team", gateway_endpoint="http://localhost:9443")
    client = AgentClient(config, api_key="ak_live_abc123")
    await client.connect()

    # Tools: code execution + eyeVesa functions
    tools = EYEVESA_FUNCTIONS + [
        {
            "type": "function",
            "name": "exec_py",
            "description": "Execute Python code in the agent sandbox. Use for UI automation, data analysis, and multi-step workflows.",
            "parameters": {
                "type": "object",
                "properties": {
                    "code": {"type": "string", "description": "Python code to execute"},
                },
                "required": ["code"],
            },
        },
        {
            "type": "function",
            "name": "ask_user",
            "description": "Ask the user a clarification question when stuck.",
            "parameters": {
                "type": "object",
                "properties": {
                    "question": {"type": "string", "description": "Question to ask"},
                },
                "required": ["question"],
            },
        },
    ]

    openai_client = OpenAI()

    # The harness maintains a persistent execution namespace
    namespace = {"client": client}

    response = openai_client.responses.create(
        model="gpt-5.5",
        tools=tools,
        input="Read the Q3 report from eyeVesa resource res-finance-001 and create a summary chart.",
    )

    while True:
        function_calls = [item for item in response.output if item.type == "function_call"]
        if not function_calls:
            for item in response.output:
                if hasattr(item, "text"):
                    print(f"Agent: {item.text}")
            break

        input_items = []
        for fc in function_calls:
            args = json.loads(fc.arguments) if isinstance(fc.arguments, str) else fc.arguments

            if fc.name == "exec_py":
                try:
                    exec_output = []
                    def capture_print(*a, **kw):
                        exec_output.append(" ".join(str(x) for x in a))
                    namespace["print"] = capture_print
                    namespace["eyevesa_client"] = client
                    namespace["asyncio"] = asyncio
                    exec(args["code"], namespace)
                    result = "\n".join(exec_output) if exec_output else "Code executed successfully"
                except Exception as e:
                    result = f"Error: {e}"
            elif fc.name == "ask_user":
                result = input(f"[User prompt] {args['question']}: ")
            else:
                result = await handle_eyevesa_call(client, fc.name, args)

            input_items.append({"type": "function_call_output", "call_id": fc.call_id, "output": result})

        response = openai_client.responses.create(
            model="gpt-5.5", tools=tools,
            previous_response_id=response.id, input=input_items,
        )
```

---

## Method 4: Code-Execution Harness (TypeScript/Node.js)

```typescript
import OpenAI from 'openai';
import { AgentClient, AgentConfig, NotAuthorizedError, HitlRequiredError } from 'agentid-sdk';

async function runOpenAIWithEyevesa() {
  // Connect to eyeVesa
  const config: AgentConfig = {
    agentId: '', name: 'openai-agent', owner: 'my-team',
    gatewayEndpoint: 'http://localhost:9443',
  };
  const client = new AgentClient(config, { apiKey: 'ak_live_abc123' });
  await client.connect();
  console.log(`Connected! Trust: ${client.trustScore}`);

  const openai = new OpenAI();

  // Define eyeVesa tools alongside the computer tool
  const tools: OpenAI.Responses.ResponseTool[] = [
    { type: 'computer' },  // OpenAI built-in screen control
    {
      type: 'function',
      name: 'eyevesa_read',
      description: 'Read data from an eyeVesa-gated resource.',
      parameters: {
        type: 'object',
        properties: {
          resource_id: { type: 'string', description: 'Resource ID to read from' },
          query: { type: 'string', description: 'Data query' },
        },
        required: ['resource_id'],
      },
    } as any,
    {
      type: 'function',
      name: 'eyevesa_request_approval',
      description: 'Request human-in-the-loop approval.',
      parameters: {
        type: 'object',
        properties: {
          action: { type: 'string', description: 'Action requiring approval' },
          reason: { type: 'string', description: 'Why approval is needed' },
          risk_level: { type: 'string', enum: ['low', 'medium', 'high', 'critical'] },
        },
        required: ['action', 'reason', 'risk_level'],
      },
    } as any,
  ];

  const response = await openai.responses.create({
    model: 'gpt-5.5',
    tools,
    input: 'Navigate to the dashboard and read financial data from the eyeVesa resource res-finance-001.',
  });

  // Process loop — handle both computer_call and function_call
  // (see Python Method 1 for full loop implementation)
  // Computer actions execute via Playwright, eyeVesa calls via SDK
}
```

---

## Advanced: HITL Escalation with OpenAI Agents SDK

Using OpenAI's Agents SDK with guardrails:

```python
from openai import Agent, Runner
from agentid_sdk import AgentClient, AgentConfig, HitlRequiredError

# Define guardrail: high-risk actions must go through eyeVesa HITL
async def eyevesa_guardrail(ctx, agent, input_data):
    """Guardrail that routes high-risk actions through eyeVesa."""
    client = ctx.get("eyevesa_client")

    # Actions that require HITL
    HIGH_RISK_ACTIONS = {"bank_transfer", "delete", "deploy", "shutdown"}

    if input_data.action in HIGH_RISK_ACTIONS:
        try:
            result = await client.invoke(input_data.resource_id, input_data.action)
        except HitlRequiredError as e:
            # Escalate to human via HITL
            approval = await client.request_approval(
                action=input_data.action,
                reason=f"Guardrail: {e}",
                risk_level="high",
            )
            return {"approved": False, "approval_id": approval.approval_id, "status": "pending"}

    return {"approved": True}
```

---

## Advanced: MCP Integration via eyeVesa Gateway

OpenAI supports MCP connectors. You can connect eyeVesa's MCP endpoint directly:

```python
from openai import OpenAI

client = OpenAI()

response = client.responses.create(
    model="gpt-4.1",
    tools=[{
        "type": "mcp",
        "server_url": "http://localhost:9443/v1/mcp",
        "headers": {"X-API-Key": "ak_live_abc123"},
        "allowed_tools": ["read", "write", "search_docs"],
    }],
    input="What resources are available?",
)
```

Or via the OpenAI Agents SDK MCP integration:

```python
from agents import Agent, Runner, MCPServerHTTP

mcp_server = MCPServerHTTP(
    name="eyevesa",
    url="http://localhost:9443/v1/mcp",
    headers={"X-API-Key": "ak_live_abc123"},
)

agent = Agent(
    name="eyevesa-agent",
    instructions="You can access eyeVesa-gated resources. Always check authorization before writing.",
    mcp_servers=[mcp_server],
)

result = await Runner.run(agent, "Read the Q3 report from res-finance-001")
```

---

## Airport Integration

OpenAI agents can discover and connect with other agents at the Airport:

```python
from agentid_sdk import AgentClient, AgentConfig, OpenAIIntegration

# After connecting
integration = OpenAIIntegration(client)

# Heartbeat — announce presence
await client.airport_heartbeat(status="online", metadata={"framework": "openai"})

# Update profile — become discoverable
await client.airport_update_profile(
    description="OpenAI data analysis agent",
    services_offered=["data_analysis", "summarization"],
    tags=["openai", "gpt", "analysis"],
    listed=True,
)

# Search for agents
results = await client.airport_search(skill="compliance_check", min_trust=0.9)

# Who's online
online = await client.airport_list_online()

# Peer profile
profile = await client.airport_get_profile("other-agent-uuid")

# Connection history
conns = await client.airport_connections(agent_id="my-agent-uuid")
```

---

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GATEWAY_ENDPOINT` | No | eyeVesa gateway URL (default: `http://localhost:9443`) |
| `AGENT_NAME` | No | Agent display name (default: `openai-agent`) |
| `AGENT_OWNER` | No | Agent owner/org (default: `default`) |
| `OPENAI_API_KEY` | Yes | Your OpenAI API key |

---

## Key Considerations

1. **Computer Use + eyeVesa**: The `computer` tool handles screen actions. eyeVesa functions handle identity-gated resource access. Use both in the same `tools` array.

2. **`computer_call` vs `function_call`**: OpenAI Responses API returns `computer_call` for screen actions and `function_call` for eyeVesa tools. Handle both in your loop.

3. **Previous Response ID**: Use `previous_response_id` for multi-turn conversations with OpenAI's Responses API.

4. **Screenshot Handling**: Use `detail: "original"` for computer screenshots to maintain accuracy. Downscale to ~1440x900 for optimal performance.

5. **Trust Scores**: Start at 1.0. Allowed: +0.01, denied: -0.05, budget exceeded: -0.1.

6. **HITL Integration**: When `HITL_REQUIRED` is returned, automatically request approval and poll for the human decision.

7. **Multiple Agents**: Each OpenAI invocation can use a different eyeVesa identity with separate trust scores and delegation scopes.

8. **MCP Protocol**: eyeVesa gateway supports MCP version `2024-11-05` with capabilities: tools, resources, prompts, skills.