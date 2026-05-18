# AgentID TypeScript SDK

TypeScript SDK for AI agents to connect to the AgentID Gateway with cryptographic identity, policy-based authorization, and framework integrations for LangGraph, CrewAI, and AutoGen.

## Installation

```bash
npm install agentid-sdk
```

**Requirements**: Node.js 18+

## Quick Start

```typescript
import { AgentClient, AgentConfig } from 'agentid-sdk';

const config: AgentConfig = {
  agentId: 'my-agent-001',
  name: 'my-agent',
  owner: 'my-team',
  gatewayEndpoint: 'http://localhost:9443',
};

const client = new AgentClient(config);

// Register with the gateway
await client.connect();
console.log(`Connected! Trust score: ${client.trustScore}`);

// Discover available tools
const tools = await client.discover('mcp');
for (const tool of tools) {
  console.log(`  - ${tool.name}: ${tool.description}`);
}

// Invoke a tool (auto-authorizes via OPA)
const result = await client.invoke(
  'd4385f9f-bcf8-47b9-90f4-b1fce91def59',
  'read',
  { location: 'Kuala Lumpur' }
);
console.log(`Result:`, result.data);
```

### Environment Variables

```typescript
// Set these, then use fromEnv():
// AGENT_ID=my-agent-001
// AGENT_NAME=my-agent
// AGENT_OWNER=my-team
// GATEWAY_ENDPOINT=http://localhost:9443

const client = AgentClient.fromEnv();
```

### Authentication

Pass an API key or JWT token for production gateways:

```typescript
const client = new AgentClient(config, {
  apiKey: 'ak_live_abc123',
  // or
  jwtToken: 'eyJhbGciOiJIUzI1NiIs...',
});
```

## API Reference

### AgentClient

The core client. All methods return `Promise`.

| Method | Returns | Description |
|---|---|---|
| `connect()` | `Promise<AgentClient>` | Register agent with gateway, update trust score |
| `discover(capability)` | `Promise<ToolInfo[]>` | Discover available resources by capability |
| `invoke(resourceId, tool, params)` | `Promise<InvokeResult>` | Authorize + execute a tool via MCP |
| `delegate(delegateeId, scope, reason)` | `Promise<DelegateResult>` | Delegate scoped permissions to another agent |
| `attest(platform, firmwareVersion)` | `Promise<PtvAttestResult>` | PTV hardware identity attestation |
| `bind(attestation, tpmSignature, platform, firmwareVersion, agentId?)` | `Promise<PtvBindResult>` | Bind attestation to agent identity |
| `verifyBinding(bindingId)` | `Promise<boolean>` | Verify a PTV identity binding |
| `requestApproval(action, reason, riskLevel)` | `Promise<HitlApproval>` | Request human-in-the-loop approval |
| `decideApproval(approvalId, approved, approverMethod)` | `Promise<string>` | Approve or reject a HITL request |
| `getApprovalStatus(approvalId)` | `Promise<string>` | Check approval status |
| `listPendingApprovals()` | `Promise<Record<string, unknown>[]>` | List pending HITL approvals |
| `mcpInitialize()` | `Promise<McpCapabilities>` | Initialize MCP connection, get capabilities |
| `mcpListTools()` | `Promise<McpTool[]>` | List available MCP tools |
| `mcpCallTool(toolName, arguments)` | `Promise<Record<string, unknown>>` | Call an MCP tool directly |
| `verifySignature(agentId, message, signature)` | `Promise<boolean>` | Verify an Ed25519 signature |
| `listSkills(category)` | `Promise<Skill[]>` | List registered skills |
| `createSkill(name, ...)` | `Promise<Skill>` | Create a new skill |
| `assignSkill(agentId, skillId, proficiency)` | `Promise<AgentSkill>` | Assign a skill to an agent |
| `endorseSkill(agentId, skillId, endorserType, endorserId, comment)` | `Promise<Endorsement>` | Endorse an agent's skill |
| `verifySkill(agentId, skillId, verifiedBy)` | `Promise<AgentSkill>` | Verify an agent's skill |
| `getSkillTrust(agentId)` | `Promise<SkillTrustScore[]>` | Get per-skill trust scores |
| `issueToken(agentId, resourceId, action, trustScore, scopes, skills, params)` | `Promise<CapabilityToken>` | Issue an Ed25519-signed capability token |
| `verifyToken(tokenId)` | `Promise<CapabilityToken>` | Verify a capability token |
| `revokeToken(tokenId, reason)` | `Promise<Record<string, unknown>>` | Revoke a capability token |
| `listRevokedTokens()` | `Promise<Record<string, unknown>[]>` | List revoked tokens |
| `issueReceipt(tokenId, allowed, trustScore, trustDelta)` | `Promise<TransactionReceipt>` | Issue a transaction receipt |
| `verifyReceipt(receipt)` | `Promise<boolean>` | Verify a transaction receipt |

### Properties

| Property | Type | Description |
|---|---|---|
| `agentId` | `string` | Agent UUID |
| `name` | `string` | Agent display name |
| `owner` | `string` | Agent owner/team |
| `trustScore` | `number` | Current trust score (starts at 1.0) |
| `isRegistered` | `boolean` | Whether agent has connected to gateway |
| `gatewayEndpoint` | `string` | Gateway URL |

## Interfaces

### AgentConfig

```typescript
interface AgentConfig {
  agentId: string;
  name: string;
  owner: string;
  gatewayEndpoint: string;
}
```

### InvokeResult

```typescript
interface InvokeResult {
  success: boolean;
  data: Record<string, unknown>;
  trustScore: number;
}
```

### AuthorizeResult

```typescript
interface AuthorizeResult {
  allowed: boolean;
  requiresHitl: boolean;
  reason: string;
  trustDelta: number;
}
```

### CapabilityToken

```typescript
interface CapabilityToken {
  id: string;                    // jti (token UUID)
  issuer: string;                // "agentid-gateway"
  subject: string;               // agent_id
  resourceId: string;
  action: string;
  scopes: string[];
  trustScore: number;
  agentSkills: Record<string, unknown>[];
  params?: Record<string, unknown>;
  issuedAt: number;               // Unix timestamp
  expiresAt: number;              // Unix timestamp (default 5 min)
  nonce: string;
  signature: string;              // Ed25519 base64
}
```

### TransactionReceipt

```typescript
interface TransactionReceipt {
  receiptId: string;
  tokenId: string;
  agentId: string;
  resourceId: string;
  action: string;
  allowed: boolean;
  trustScore: number;
  trustDelta: number;
  tokenIssuedAt: number;
  tokenExpires: number;
  issuedAt: string;
  signature: string;               // Ed25519 base64
}
```

### Skill / AgentSkill / SkillTrustScore

```typescript
interface Skill {
  skillId: string;
  name: string;
  description: string;
  category: string;
  riskLevel: string;              // "low" | "medium" | "high" | "critical"
  requiredTrustMin: number;
  requiredProficiency: number;
  createdAt: string;
  updatedAt: string;
}

interface AgentSkill {
  agentId: string;
  skillId: string;
  skillName: string;
  proficiency: number;             // 1-5
  verified: boolean;              // auto-verified at 3 endorsements
  verifiedBy: string;
  verifiedAt?: string;
  endorsementsCount: number;
  acquiredAt: string;
}

interface SkillTrustScore {
  agentId: string;
  skillId: string;
  skillName: string;
  trustScore: number;
  updatedAt: string;
}
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

```typescript
import { NotAuthorizedError, HitlRequiredError } from 'agentid-sdk';

try {
  const result = await client.invoke(resourceId, 'delete', {});
} catch (e) {
  if (e instanceof NotAuthorizedError) {
    console.log(`Not allowed: ${e.message}`);
  } else if (e instanceof HitlRequiredError) {
    console.log(`Needs human approval: ${e.message}`);
  }
}
```

## Framework Integrations

### LangGraph

```typescript
import { LangGraphIntegration } from 'agentid-sdk';

const lang = LangGraphIntegration.fromConfig({
  gatewayEndpoint: 'http://localhost:9443',
  agentName: 'langgraph-agent',
  owner: 'my-team',
  apiKey: 'ak_live_abc123',
});

await lang.connect();

// Get tools in LangChain function-calling format
const tools = await lang.getTools();
// [{ type: "function", function: { name: "read", description: "...", parameters: {...} } }]

// Call a gated tool
const result = await lang.callTool('read', { location: 'Kuala Lumpur' });

// Access underlying client for advanced operations
const trustScores = await lang.client.getSkillTrust('agent-001');
```

### CrewAI

```typescript
import { CrewAIIntegration } from 'agentid-sdk';

const crew = CrewAIIntegration.fromConfig({
  gatewayEndpoint: 'http://localhost:9443',
  agentName: 'crewai-agent',
});

await crew.connect();

// Create tool definitions for CrewAI agents
const toolDef = crew.createToolDefinition('read', 'Read data from resource');
// { name: "read", description: "...", func: [async callable] }
```

### AutoGen

```typescript
import { AutoGenIntegration } from 'agentid-sdk';

const autogen = AutoGenIntegration.fromConfig({
  gatewayEndpoint: 'http://localhost:9443',
  agentName: 'autogen-agent',
});

await autogen.connect();

// Get function definitions in AutoGen format
const functions = await autogen.getFunctionDefinitions();
// [{ name: "read", description: "...", parameters: {...} }]

// Execute a function through the gateway
const result = await autogen.executeFunction('read', { location: 'Kuala Lumpur' });
```

## Transaction Protocol

The transaction protocol provides Ed25519-signed capability tokens and receipts for non-repudiable audit trails.

```typescript
// Issue a capability token
const token = await client.issueToken(
  'agent-001',
  'res-abc',
  'read',
  0.85,                            // trustScore
  ['read', 'list'],                 // scopes
  [{ skill_id: 's1', proficiency: 3 }], // skills
);

console.log(token.id);          // jti UUID
console.log(token.signature);   // Ed25519 base64
console.log(token.expiresAt);   // Unix timestamp (5 min default)

// Verify a token
const verified = await client.verifyToken(token.id);

// Issue a receipt after execution
const receipt = await client.issueReceipt(
  token.id,
  true,     // allowed
  0.87,     // trustScore
  0.02,     // trustDelta
);

// Revoke a token
await client.revokeToken(token.id, 'compromised');

// Verify a receipt
const valid = await client.verifyReceipt(receipt);
```

## Skills System

Skills are proficiency-verified capabilities that gate resource access beyond basic trust scores.

```typescript
// Create a skill
const skill = await client.createSkill(
  'python-coding',
  'Python programming proficiency',
  'programming',
  'low',
  0.5,     // requiredTrustMin
  3,       // requiredProficiency
);

// Assign to an agent
const agentSkill = await client.assignSkill('agent-001', skill.skillId, 3);

// Endorse (3 endorsements auto-verifies)
const endorsement = await client.endorseSkill(
  'agent-001',
  skill.skillId,
  'agent',        // endorserType
  'agent-002',     // endorserId
  'Solid Python skills',
);

// Manual verification
const verified = await client.verifySkill('agent-001', skill.skillId, 'admin');

// Check per-skill trust (falls back to global trustScore if no skill-specific score)
const scores = await client.getSkillTrust('agent-001');
for (const s of scores) {
  console.log(`  ${s.skillName}: ${s.trustScore}`);
}
```

## HITL (Human-in-the-Loop)

```typescript
// Request approval for high-risk action
const approval = await client.requestApproval(
  'bank_transfer',
  'Transfer $10K externally',
  'high',
);

// Check status
const status = await client.getApprovalStatus(approval.approvalId);

// List pending
const pending = await client.listPendingApprovals();

// Decide (approver side)
const resultStatus = await client.decideApproval(
  approval.approvalId,
  true,
  'faceid',
);
```

## PTV (Prove-Transform-Verify)

Hardware-rooted identity attestation:

```typescript
// Attest platform identity
const attestResult = await client.attest(
  'linux-tpm2',
  '2.0.0',
);

// Bind attestation to agent
const bindResult = await client.bind(
  attestResult.attestation,
  attestResult.tpmSignature,
  'linux-tpm2',
  '2.0.0',
);

// Verify binding
const isValid = await client.verifyBinding(bindResult.bindingId);
```

## Building from Source

```bash
npm install
npm run build       # Compiles to dist/
npm test            # Run tests
```

## Running Tests

```bash
npm test
```

## License

Apache-2.0