# AgentID Python SDK

Python SDK for AI agents to connect to the AgentID Gateway with cryptographic identity, policy-based authorization, and framework integrations for LangGraph, CrewAI, and AutoGen.

## Installation

```bash
pip install agentid-sdk
```

With framework extras:

```bash
pip install "agentid-sdk[langchain]"   # LangGraph / LangChain support
pip install "agentid-sdk[crewai]"      # CrewAI support
pip install "agentid-sdk[autogen]"    # AutoGen support
pip install "agentid-sdk[dev]"        # pytest, pytest-asyncio, respx
```

**Requirements**: Python 3.10+, httpx, pydantic 2, PyNaCl

## Quick Start

```python
import asyncio
from agentid_sdk import AgentClient, AgentConfig

async def main():
    config = AgentConfig(
        agent_id="my-agent-001",
        name="my-agent",
        owner="my-team",
        gateway_endpoint="http://localhost:9443",
    )

    async with AgentClient(config) as client:
        # Register with the gateway
        await client.connect()
        print(f"Connected! Trust score: {client.trust_score}")

        # Discover available tools
        tools = await client.discover("mcp")
        for tool in tools:
            print(f"  - {tool.name}: {tool.description}")

        # Invoke a tool (auto-authorizes via OPA)
        result = await client.invoke(
            resource_id="d4385f9f-bcf8-47b9-90f4-b1fce91def59",
            tool="read",
            params={"location": "Kuala Lumpur"},
        )
        print(f"Result: {result.data}")

asyncio.run(main())
```

### Environment Variables

```python
# Set these, then use from_env():
# AGENT_ID=my-agent-001
# AGENT_NAME=my-agent
# AGENT_OWNER=my-team
# GATEWAY_ENDPOINT=http://localhost:9443

client = AgentClient.from_env()
```

### Authentication

Pass an API key or JWT token for production gateways (`AUTH_ENABLED=true`):

```python
client = AgentClient(
    config,
    api_key="ak_live_abc123",
    # or
    jwt_token="eyJhbGciOiJIUzI1NiIs...",
)
```

## API Reference

### AgentClient

The core client. All methods are `async`.

| Method | Returns | Description |
|---|---|---|
| `connect()` | `AgentClient` | Register agent with gateway, update trust score |
| `discover(capability)` | `list[ToolInfo]` | Discover available resources by capability |
| `invoke(resource_id, tool, params)` | `InvokeResult` | Authorize + execute a tool via MCP |
| `delegate(delegatee_id, scope, reason)` | `DelegateResult` | Delegate scoped permissions to another agent |
| `attest(platform, firmware_version)` | `PtvAttestResult` | PTV hardware identity attestation |
| `bind(attestation, tpm_signature, platform, firmware_version, agent_id)` | `PtvBindResult` | Bind attestation to agent identity |
| `verify_binding(binding_id)` | `bool` | Verify a PTV identity binding |
| `request_approval(action, reason, risk_level)` | `HitlApproval` | Request human-in-the-loop approval |
| `decide_approval(approval_id, approved, approver_method)` | `str` | Approve or reject a HITL request |
| `get_approval_status(approval_id)` | `str` | Check approval status |
| `list_pending_approvals()` | `list[dict]` | List pending HITL approvals |
| `mcp_initialize()` | `McpCapabilities` | Initialize MCP connection, get capabilities |
| `mcp_list_tools()` | `list[McpTool]` | List available MCP tools |
| `mcp_call_tool(tool_name, arguments)` | `dict` | Call an MCP tool directly |
| `verify_signature(agent_id, message, signature)` | `bool` | Verify an Ed25519 signature |
| `list_skills(category)` | `list[Skill]` | List registered skills |
| `create_skill(name, ...)` | `Skill` | Create a new skill |
| `assign_skill(agent_id, skill_id, proficiency)` | `AgentSkill` | Assign a skill to an agent |
| `endorse_skill(agent_id, skill_id, endorser_type, endorser_id, comment)` | `Endorsement` | Endorse an agent's skill |
| `verify_skill(agent_id, skill_id, verified_by)` | `AgentSkill` | Verify an agent's skill |
| `get_skill_trust(agent_id)` | `list[SkillTrustScore]` | Get per-skill trust scores |
| `issue_token(agent_id, resource_id, action, trust_score, scopes, skills, params)` | `CapabilityToken` | Issue an Ed25519-signed capability token |
| `verify_token(token_id)` | `CapabilityToken` | Verify a capability token |
| `revoke_token(token_id, reason)` | `dict` | Revoke a capability token |
| `list_revoked_tokens()` | `list[dict]` | List revoked tokens |
| `issue_receipt(token_id, allowed, trust_score, trust_delta)` | `TransactionReceipt` | Issue a transaction receipt |
| `verify_receipt(receipt)` | `bool` | Verify a transaction receipt |

### Properties

| Property | Type | Description |
|---|---|---|
| `agent_id` | `str` | Agent UUID |
| `name` | `str` | Agent display name |
| `owner` | `str` | Agent owner/team |
| `trust_score` | `float` | Current trust score (starts at 1.0) |
| `is_registered` | `bool` | Whether agent has connected to gateway |
| `gateway_endpoint` | `str` | Gateway URL |

## Models

All models use Pydantic v2 `BaseModel`:

### AgentConfig

```python
class AgentConfig(BaseModel):
    agent_id: str
    name: str
    owner: str
    gateway_endpoint: str
```

### InvokeResult

```python
class InvokeResult(BaseModel):
    success: bool
    data: dict[str, Any]
    trust_score: float
```

### AuthorizeResult

```python
class AuthorizeResult(BaseModel):
    allowed: bool
    requires_hitl: bool
    reason: str
    trust_delta: float
```

### CapabilityToken

```python
class CapabilityToken(BaseModel):
    id: str                      # jti (token UUID)
    issuer: str                  # "agentid-gateway"
    subject: str                 # agent_id
    resource_id: str
    action: str
    scopes: list[str]
    trust_score: float
    agent_skills: list[dict]
    params: dict | None
    issued_at: int               # Unix timestamp
    expires_at: int               # Unix timestamp (default 5 min)
    nonce: str
    signature: str                # Ed25519 base64
```

### TransactionReceipt

```python
class TransactionReceipt(BaseModel):
    receipt_id: str
    token_id: str
    agent_id: str
    resource_id: str
    action: str
    allowed: bool
    trust_score: float
    trust_delta: float
    token_issued_at: int
    token_expires: int
    issued_at: str
    signature: str                # Ed25519 base64
```

### Skill / AgentSkill / SkillTrustScore

```python
class Skill(BaseModel):
    skill_id: str
    name: str
    description: str
    category: str
    risk_level: str               # "low", "medium", "high", "critical"
    required_trust_min: float
    required_proficiency: int
    created_at: str
    updated_at: str

class AgentSkill(BaseModel):
    agent_id: str
    skill_id: str
    skill_name: str
    proficiency: int              # 1-5
    verified: bool               # auto-verified at 3 endorsements
    verified_by: str
    verified_at: str | None
    endorsements_count: int
    acquired_at: str

class SkillTrustScore(BaseModel):
    agent_id: str
    skill_id: str
    skill_name: str
    trust_score: float
    updated_at: str
```

## Exception Hierarchy

```
AgentIDError
  +-- ConnectError
  |     +-- AuthFailedError
  +-- DiscoverError
  +-- InvokeError
  |     +-- NotAuthorizedError
  |     +-- HitlRequiredError
  +-- DelegateError
  |     +-- MaxDepthError
  +-- PtvError
  +-- HitlError
  +-- McpError
  +-- VerifyError
  +-- SkillError
  +-- TxError
```

Catch specific errors:

```python
from agentid_sdk import AgentClient, NotAuthorizedError, HitlRequiredError

try:
    result = await client.invoke(resource_id, "delete", {})
except NotAuthorizedError as e:
    print(f"Not allowed: {e}")
except HitlRequiredError as e:
    print(f"Needs human approval: {e}")
```

## Framework Integrations

### LangGraph

```python
from agentid_sdk import LangGraphIntegration

lang = LangGraphIntegration.from_config(
    gateway_endpoint="http://localhost:9443",
    agent_name="langgraph-agent",
    owner="my-team",
    api_key="ak_live_abc123",
)

await lang.connect()

# Get tools in LangChain function-calling format
tools = await lang.get_tools()
# [{"type": "function", "function": {"name": "read", "description": "...", "parameters": {...}}}]

# Call a gated tool
result = await lang.call_tool("read", {"location": "Kuala Lumpur"})

# Or use authorize + invoke
data = await lang.authorize_and_invoke(resource_id, "read", {"key": "value"})
```

### CrewAI

```python
from agentid_sdk import CrewAIIntegration

crew = CrewAIIntegration.from_config(
    gateway_endpoint="http://localhost:9443",
    agent_name="crewai-agent",
)

await crew.connect()

# Create tool definitions for CrewAI agents
tool_def = crew.create_tool_definition("read", description="Read data from resource")
# tool_def = {"name": "read", "description": "...", "func": <async callable>}

# Access underlying client for advanced operations
trust_scores = await crew.client.get_skill_trust("agent-001")
```

### AutoGen

```python
from agentid_sdk import AutoGenIntegration

autogen = AutoGenIntegration.from_config(
    gateway_endpoint="http://localhost:9443",
    agent_name="autogen-agent",
)

await autogen.connect()

# Get function definitions in AutoGen format
functions = await autogen.get_function_definitions()
# [{"name": "read", "description": "...", "parameters": {...}}]

# Execute a function through the gateway
result = await autogen.execute_function("read", {"location": "Kuala Lumpur"})
```

## Transaction Protocol

The transaction protocol provides Ed25519-signed capability tokens and receipts for non-repudiable audit trails.

```python
# Issue a capability token
token = await client.issue_token(
    agent_id="agent-001",
    resource_id="res-abc",
    action="read",
    trust_score=0.85,
    scopes=["read", "list"],
    skills=[{"skill_id": "s1", "proficiency": 3}],
)

print(token.id)           # jti UUID
print(token.signature)    # Ed25519 base64
print(token.expires_at)   # Unix timestamp (5 min default)

# Verify a token
verified = await client.verify_token(token.id)

# Issue a receipt after execution
receipt = await client.issue_receipt(
    token_id=token.id,
    allowed=True,
    trust_score=0.87,
    trust_delta=0.02,
)

# Revoke a token
await client.revoke_token(token.id, reason="compromised")

# Verify a receipt
valid = await client.verify_receipt(receipt)
```

## Skills System

Skills are proficiency-verified capabilities that gate resource access beyond basic trust scores.

```python
# Create a skill
skill = await client.create_skill(
    name="python-coding",
    description="Python programming proficiency",
    category="programming",
    risk_level="low",
    required_trust_min=0.5,
    required_proficiency=3,
)

# Assign to an agent
agent_skill = await client.assign_skill("agent-001", skill.skill_id, proficiency=3)

# Endorse (3 endorsements auto-verifies)
endorsement = await client.endorse_skill(
    "agent-001",
    skill.skill_id,
    endorser_type="agent",
    endorser_id="agent-002",
    comment="Solid Python skills",
)

# Manual verification
verified = await client.verify_skill("agent-001", skill.skill_id, verified_by="admin")

# Check per-skill trust (falls back to global trust_score if no skill-specific score)
scores = await client.get_skill_trust("agent-001")
for s in scores:
    print(f"  {s.skill_name}: {s.trust_score}")
```

## HITL (Human-in-the-Loop)

```python
# Request approval for high-risk action
approval = await client.request_approval(
    action="bank_transfer",
    reason="Transfer $10K externally",
    risk_level="high",
)

# Check status
status = await client.get_approval_status(approval.approval_id)

# List pending
pending = await client.list_pending_approvals()

# Decide (approver side)
result_status = await client.decide_approval(
    approval.approval_id,
    approved=True,
    approver_method="faceid",
)
```

## PTV (Prove-Transform-Verify)

Hardware-rooted identity attestation:

```python
# Attest platform identity
attest_result = await client.attest(
    platform="linux-tpm2",
    firmware_version="2.0.0",
)

# Bind attestation to agent
bind_result = await client.bind(
    attestation=attest_result.attestation,
    tpm_signature=attest_result.tpm_signature,
    platform="linux-tpm2",
    firmware_version="2.0.0",
)

# Verify binding
is_valid = await client.verify_binding(bind_result.binding_id)
```

## Context Manager

`AgentClient` supports `async with` for automatic cleanup:

```python
async with AgentClient(config) as client:
    await client.connect()
    tools = await client.discover()
    # client._http auto-closes on exit
```

## Running Tests

```bash
pip install -e ".[dev]"
pytest tests/ -v
```

## License

Apache-2.0