# eyeVesa Framework Integration Kit

## Overview

This kit explains how agent frameworks such as **Hermes**, **OpenClaw**, custom LangGraph agents, CrewAI crews, AutoGen teams, and other agentic runtimes can integrate with eyeVesa.

The positioning is simple:

> Agent frameworks own reasoning and planning. eyeVesa owns identity, policy, approval, and audit before those agents touch real systems.

Frameworks do not need to replace their planner, memory, prompt stack, or tool orchestration. They add eyeVesa as the trust gateway for actions that matter.

## Who This Is For

Use this kit when a framework or agent runtime wants to:

- Register each agent with a verifiable identity.
- Publish agent capability and status to Airport discovery.
- Discover other trusted agents by capability, skill, owner, status, or trust score.
- Gate sensitive tool calls through policy and HITL approval.
- Keep signed audit evidence for every real-world action.
- Expose an enterprise-friendly story for security and compliance teams.

Good fit examples:

| Framework type | How eyeVesa helps |
|---|---|
| Autonomous shopping agents | Budget policy, purchase approval, receipt audit |
| DevOps/SRE agents | Production action approval, incident audit, delegated scope |
| Security response agents | Action gating, trust score, human escalation |
| Data/research agents | Resource authorization and signed data access logs |
| Multi-agent teams | Airport discovery, delegation, connection history |

## Integration Contract

Every framework integration should implement the same five steps:

1. **Register** the agent with eyeVesa.
2. **Heartbeat** to Airport while the agent is active.
3. **Publish profile** with services, tags, endpoints, and capabilities.
4. **Authorize before action** for every meaningful external tool call.
5. **Audit after outcome** through the authorize flow and connection log.

The simplest mental model:

```text
Framework planner -> eyeVesa authorize -> resource adapter -> signed audit
```

For agent-to-agent workflows:

```text
Framework planner -> Airport search -> A2A task/delegation -> policy -> audit
```

## Current eyeVesa Surfaces

These are the endpoints a framework integration should use first.

| Purpose | Endpoint | Notes |
|---|---|---|
| Register agent | `POST /v1/agents/register` | Creates agent identity and returns agent metadata/API key when authorized |
| Authorize action | `POST /v1/authorize` | Policy decision, HITL decision flag, trust delta, and audit path |
| MCP gateway | `POST /v1/mcp` | Tool execution path through the Rust gateway/core proxy |
| Airport heartbeat | `POST /v1/airport/heartbeat` | Requires auth; keeps agent visible as online |
| Airport search | `GET /v1/airport/agents` | Public discovery by capability, skill, trust, tag, owner, status |
| Airport online | `GET /v1/airport/online` | Public list of online agents |
| Airport profile | `GET /v1/airport/agents/{agentID}` | Public profile lookup |
| Update profile | `PUT /v1/airport/agents/{agentID}` | Requires auth; publishes services, endpoints, tags |
| Connections | `GET /v1/airport/connections` | Requires auth; interaction history for audit and graph views |
| A2A discovery | `GET /v1/a2a/agents` | Adapter surface for Agent Card-style discovery |
| A2A task create | `POST /v1/a2a/tasks` | Creates an in-memory task for adapter POC workflows |
| A2A task get | `GET /v1/a2a/tasks/{taskID}` | Reads task state from the adapter POC |

Auth setup:

- Community/local demos can run with local dev auth settings.
- Production integrations should use `X-API-Key`, bearer JWT, or SSO session auth.
- `eyevesa connect` can register an agent and save config once an API key or JWT is already configured locally.

## Minimal Framework Flow

### 1. Register The Agent

```bash
curl -X POST http://localhost:8080/v1/agents/register \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${EYEVESA_API_KEY}" \
  -d '{
    "name": "openclaw-researcher",
    "owner": "org:acme",
    "capabilities": ["research", "read", "summarize"],
    "allowed_tools": ["read", "search", "summarize"],
    "max_budget_usd": 50.0,
    "delegation_policy": "single_level",
    "behavioral_tags": ["research", "low-risk", "human-supervised"]
  }'
```

Save:

- `agent_id`
- `api_key` if returned
- public identity metadata

### 2. Heartbeat To Airport

```bash
curl -X POST http://localhost:8080/v1/airport/heartbeat \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${AGENT_API_KEY}" \
  -d '{
    "agent_id": "'${AGENT_ID}'",
    "status": "online",
    "metadata": {
      "framework": "openclaw",
      "runtime": "python",
      "version": "0.1.0"
    }
  }'
```

Frameworks should send this on startup and then repeat on a timer while the agent is active.

### 3. Publish Airport Profile

```bash
curl -X PUT http://localhost:8080/v1/airport/agents/${AGENT_ID} \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${AGENT_API_KEY}" \
  -d '{
    "description": "Research agent that can search approved sources and summarize findings.",
    "services_offered": ["research", "summarization"],
    "endpoints": {
      "mcp": "http://localhost:9443/v1/mcp",
      "a2a": "http://localhost:8080/v1/a2a"
    },
    "tags": ["openclaw", "research", "safe-tools"],
    "listed": true
  }'
```

### 4. Discover Trusted Peers

```bash
curl "http://localhost:8080/v1/airport/agents?capability=research&min_trust=0.7&status=online&limit=10"
```

For A2A-style discovery:

```bash
curl http://localhost:8080/v1/a2a/agents \
  -H "X-API-Key: ${AGENT_API_KEY}"
```

### 5. Authorize Before External Action

```bash
curl -X POST http://localhost:8080/v1/authorize \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${AGENT_API_KEY}" \
  -d '{
    "agent_id": "'${AGENT_ID}'",
    "resource_id": "res-internal-api",
    "action": "read",
    "params": {
      "path": "/customers/summary",
      "estimated_cost": 0.01,
      "reason": "Summarize customer health for account review"
    }
  }'
```

Expected decision shape:

```json
{
  "allowed": true,
  "requires_hitl": false,
  "reason": "allowed",
  "trust_delta": 0.01
}
```

If `requires_hitl` is true, the framework should pause the action and wait for the human approval workflow instead of executing directly.

## Framework Wrapper Pattern

Most integrations only need a small wrapper around tool execution.

```python
class EyeVesaGuard:
    def __init__(self, base_url, agent_id, api_key):
        self.base_url = base_url.rstrip("/")
        self.agent_id = agent_id
        self.headers = {
            "Content-Type": "application/json",
            "X-API-Key": api_key,
        }

    async def before_tool_call(self, resource_id, action, params):
        response = await http_post(
            f"{self.base_url}/v1/authorize",
            headers=self.headers,
            json={
                "agent_id": self.agent_id,
                "resource_id": resource_id,
                "action": action,
                "params": params,
            },
        )
        decision = await response.json()
        if not decision.get("allowed"):
            raise PermissionError(decision.get("reason", "eyeVesa denied action"))
        if decision.get("requires_hitl"):
            raise RuntimeError("HITL approval required before execution")
        return decision
```

Framework use:

```python
decision = await guard.before_tool_call(
    resource_id="res-internal-api",
    action="write",
    params={"path": "/deployments/prod", "reason": "roll forward hotfix"},
)
result = await actual_tool_call()
```

## Hermes Integration Shape

Hermes can treat eyeVesa as the governance layer for action specs:

```yaml
tools:
  - name: eyevesa_authorize
    description: Authorize a real-world action before execution.
    endpoint: http://localhost:8080/v1/authorize
  - name: eyevesa_airport_search
    description: Discover trusted peer agents.
    endpoint: http://localhost:8080/v1/airport/agents
  - name: eyevesa_mcp
    description: Execute approved MCP tools through eyeVesa.
    endpoint: http://localhost:9443/v1/mcp
```

Recommended Hermes policy:

- Browse/search actions can be low risk.
- Purchase/payment/deployment/delete actions should require HITL.
- The agent should record `reason`, `estimated_cost`, and `target_resource` in every authorization request.

## OpenClaw Integration Shape

OpenClaw-style agent runtimes can add eyeVesa as middleware around their tool executor:

```text
OpenClaw planner
  -> select tool
  -> eyeVesa authorize(action, resource, params)
  -> execute tool only when allowed
  -> publish heartbeat/profile while running
  -> use Airport/A2A to discover peers
```

Recommended OpenClaw policy:

- All tools that touch external systems go through `POST /v1/authorize`.
- Agent startup sends Airport heartbeat.
- Agent profile includes framework name, model/runtime, capabilities, endpoint, and tags.
- Multi-agent handoff uses Airport search first, then A2A task creation or delegation.

## A2A Adapter Flow

Use this flow when one framework wants to expose agent cards or tasks to another framework:

```bash
curl http://localhost:8080/v1/a2a/agents \
  -H "X-API-Key: ${AGENT_API_KEY}"
```

```bash
curl -X POST http://localhost:8080/v1/a2a/tasks \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${AGENT_API_KEY}" \
  -d '{
    "from_agent_id": "'${AGENT_ID}'",
    "to_agent_id": "'${PEER_AGENT_ID}'",
    "action": "summarize_incident_logs",
    "scope": ["read:logs", "summarize"],
    "duration": "1h",
    "input": {
      "priority": "normal",
      "reason": "Incident review support"
    }
  }'
```

The current A2A surface is a POC. Treat it as the interoperability entry point, while Airport remains the source of truth for registered identity, profile, trust, and connection history.

## What Framework Partners Should Build

Minimum partner integration:

- `eyevesa register` or API registration command.
- Heartbeat loop with framework/runtime metadata.
- Tool execution middleware that calls `/v1/authorize`.
- Airport profile publishing on startup.
- A sample policy pack for low, medium, high, and critical actions.
- A demo showing one allowed action, one denied action, and one HITL action.

Better partner integration:

- Native SDK package or plugin.
- Config file support for `EYEVESA_GATEWAY`, `EYEVESA_API_KEY`, and `EYEVESA_AGENT_ID`.
- Built-in Airport search for agent discovery.
- A2A task support for agent-to-agent handoff.
- Audit report link in task output.

## Partner Pitch

Use this short pitch when approaching framework maintainers:

> You own agent intelligence. eyeVesa gives your agents enterprise-grade identity, policy, human approval, and signed audit without forcing you to rebuild governance from scratch.

Use this buyer-facing pitch:

> Your agents can keep using Hermes, OpenClaw, LangGraph, CrewAI, or custom runtimes. eyeVesa sits between the agent and real systems so every risky action is identified, authorized, approved when needed, and auditable later.

## Demo Checklist

A strong integration demo should show:

- Agent starts and registers identity.
- Agent appears in Airport search as online.
- Agent calls a read-only tool and gets allowed.
- Agent tries a risky write/delete/purchase/deploy action and triggers HITL.
- Human approval changes the outcome.
- Audit/connection history proves what happened.
- Another agent discovers the first agent through Airport or A2A.

## Monetization Tie-In

Framework integrations should keep Community adoption easy and make Pro/Cloud upgrades obvious.

Community-friendly:

- Local gateway.
- Five-agent pilot.
- Basic Airport discovery.
- Basic policy and audit demo.

Pro/Cloud upgrade triggers:

- More agents or tenants.
- SSO/JWT/SAML enforcement.
- Slack/Teams/PagerDuty HITL.
- Hosted audit retention and compliance export.
- Managed Airport federation.
- Enterprise support and deployment help.

This keeps the open source story honest while making the paid value live where customers actually need reliability, compliance, and support.
