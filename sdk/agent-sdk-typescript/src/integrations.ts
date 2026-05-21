import type { AgentClient } from './client';
import type { AgentConfig, McpTool } from './models';

export { AgentClient } from './client';
export type {
  AgentConfig,
  ToolInfo,
  InvokeResult,
  AuthorizeResult,
  DelegateResult,
  PtvAttestResult,
  PtvBindResult,
  HitlApproval,
  Skill,
  AgentSkill,
  Endorsement,
  SkillTrustScore,
  McpCapabilities,
  McpTool,
  CapabilityToken,
  TransactionReceipt,
} from './models';
export {
  AgentIDError,
  ConnectError,
  AuthFailedError,
  DiscoverError,
  InvokeError,
  NotAuthorizedError,
  HitlRequiredError,
  DelegateError,
  MaxDepthError,
  PtvError,
  HitlError,
  McpError,
  VerifyError,
  SkillError,
  TxError,
} from './exceptions';

// ── Tool Definitions ──────────────────────────────────────────────────────

const EYEVESA_TOOL_DEFINITIONS: EyevesaToolDefinition[] = [
  {
    name: 'eyevesa_read',
    description: 'Read data from an eyeVesa-gated resource. Authorization is checked via OPA policy. High-risk reads may require HITL approval.',
    input_schema: {
      type: 'object',
      properties: {
        resource_id: { type: 'string', description: 'The resource ID to read from' },
        query: { type: 'string', description: 'The data query or key to read' },
      },
      required: ['resource_id'],
    },
  },
  {
    name: 'eyevesa_write',
    description: 'Write data to an eyeVesa-gated resource. Writes are typically higher risk and may require HITL approval.',
    input_schema: {
      type: 'object',
      properties: {
        resource_id: { type: 'string', description: 'The resource ID to write to' },
        data: { type: 'string', description: 'The data to write (JSON string)' },
      },
      required: ['resource_id', 'data'],
    },
  },
  {
    name: 'eyevesa_request_approval',
    description: 'Proactively request human-in-the-loop approval for an action. Use for sensitive operations.',
    input_schema: {
      type: 'object',
      properties: {
        action: { type: 'string', description: 'The action requiring approval' },
        reason: { type: 'string', description: 'Why this action needs approval' },
        risk_level: { type: 'string', enum: ['low', 'medium', 'high', 'critical'], description: 'Risk level' },
      },
      required: ['action', 'reason', 'risk_level'],
    },
  },
  {
    name: 'eyevesa_discover',
    description: 'Discover available resources registered with the eyeVesa gateway.',
    input_schema: {
      type: 'object',
      properties: {
        capability: { type: 'string', description: 'Filter by capability (e.g., "mcp")' },
      },
    },
  },
  {
    name: 'eyevesa_delegate',
    description: 'Delegate scoped permissions to another agent. Maximum delegation depth is 3.',
    input_schema: {
      type: 'object',
      properties: {
        delegatee_id: { type: 'string', description: 'The agent ID to delegate to' },
        scope: { type: 'array', items: { type: 'string' }, description: 'List of permissions to delegate' },
        reason: { type: 'string', description: 'Reason for delegation' },
      },
      required: ['delegatee_id', 'scope'],
    },
  },
  {
    name: 'eyevesa_skill_trust',
    description: 'Check per-skill trust scores for an agent.',
    input_schema: {
      type: 'object',
      properties: {
        agent_id: { type: 'string', description: 'The agent ID to check trust scores for' },
      },
      required: ['agent_id'],
    },
  },
];

// ── Shared Interface ───────────────────────────────────────────────────────

export interface EyevesaToolDefinition {
  name: string;
  description: string;
  input_schema: Record<string, unknown>;
}

// ── LangGraph Integration ──────────────────────────────────────────────────

export interface LangChainToolDefinition {
  type: 'function';
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  };
}

export class LangGraphIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) {
    this._client = client;
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): LangGraphIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'langgraph-agent',
      owner: opts.owner || 'langgraph',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new LangGraphIntegration(client);
  }

  async connect(): Promise<void> {
    await this._client.connect();
  }

  async getTools(): Promise<LangChainToolDefinition[]> {
    let tools: McpTool[] = [];
    try {
      tools = await this._client.mcpListTools();
    } catch { /* no tools available */ }

    return tools.map((t) => ({
      type: 'function' as const,
      function: {
        name: t.name,
        description: t.description || '',
        parameters: t.inputSchema || {},
      },
    }));
  }

  async callTool(toolName: string, arguments_: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    return this._client.mcpCallTool(toolName, arguments_);
  }

  get client(): AgentClient {
    return this._client;
  }
}

// ── CrewAI Integration ─────────────────────────────────────────────────────

export class CrewAIIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) {
    this._client = client;
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): CrewAIIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'crewai-agent',
      owner: opts.owner || 'crewai',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new CrewAIIntegration(client);
  }

  async connect(): Promise<void> {
    await this._client.connect();
  }

  createToolDefinition(toolName: string, description: string = ''): {
    name: string;
    description: string;
    func: (kwargs: Record<string, unknown>) => Promise<Record<string, unknown>>;
  } {
    return {
      name: toolName,
      description: description || `AgentID-gated tool: ${toolName}`,
      func: async (kwargs: Record<string, unknown>) => this._client.mcpCallTool(toolName, kwargs),
    };
  }

  get client(): AgentClient {
    return this._client;
  }
}

// ── AutoGen Integration ────────────────────────────────────────────────────

export class AutoGenIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) {
    this._client = client;
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): AutoGenIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'autogen-agent',
      owner: opts.owner || 'autogen',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new AutoGenIntegration(client);
  }

  async connect(): Promise<void> {
    await this._client.connect();
  }

  async getFunctionDefinitions(): Promise<Record<string, unknown>[]> {
    let tools: McpTool[] = [];
    try {
      tools = await this._client.mcpListTools();
    } catch { /* no tools available */ }

    return tools.map((t) => ({
      name: t.name,
      description: t.description || '',
      parameters: t.inputSchema || { type: 'object', properties: {} },
    }));
  }

  async executeFunction(name: string, arguments_: Record<string, unknown>): Promise<unknown> {
    return this._client.mcpCallTool(name, arguments_);
  }

  get client(): AgentClient {
    return this._client;
  }
}

// ── Claude Integration ─────────────────────────────────────────────────────

/**
 * Integration with Anthropic Claude (Messages API with tool_use).
 *
 * Usage:
 *   const claude = new ClaudeIntegration(client);
 *   await claude.connect();
 *   const tools = claude.getToolDefinitions();
 *   // Pass tools to Anthropic.messages.create({ tools })
 *   const result = await claude.handleToolCall("eyevesa_read", { resource_id: "..." });
 */
export class ClaudeIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) {
    this._client = client;
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): ClaudeIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'claude-agent',
      owner: opts.owner || 'claude',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new ClaudeIntegration(client);
  }

  async connect(): Promise<void> {
    await this._client.connect();
  }

  /** Get eyeVesa tool definitions in Anthropic Claude tool format. */
  getToolDefinitions(): EyevesaToolDefinition[] {
    return EYEVESA_TOOL_DEFINITIONS;
  }

  /** Route a Claude tool_use call through eyeVesa. */
  async handleToolCall(toolName: string, toolInput: Record<string, unknown>): Promise<string> {
    try {
      if (toolName === 'eyevesa_read') {
        const result = await this._client.invoke(
          toolInput.resource_id as string, 'read',
          { query: toolInput.query ?? '' },
        );
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }

      if (toolName === 'eyevesa_write') {
        const result = await this._client.invoke(
          toolInput.resource_id as string, 'write',
          { data: toolInput.data },
        );
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }

      if (toolName === 'eyevesa_request_approval') {
        const approval = await this._client.requestApproval(
          toolInput.action as string,
          toolInput.reason as string,
          toolInput.risk_level as string,
        );
        return JSON.stringify({ approval_id: approval.approvalId, status: approval.status });
      }

      if (toolName === 'eyevesa_discover') {
        const capability = (toolInput.capability as string) || 'mcp';
        const toolsInfo = await this._client.discover(capability);
        return JSON.stringify(toolsInfo);
      }

      if (toolName === 'eyevesa_delegate') {
        const result = await this._client.delegate(
          toolInput.delegatee_id as string,
          toolInput.scope as string[],
          (toolInput.reason as string) || '',
        );
        return JSON.stringify({ delegation_id: result.delegationId, status: result.status });
      }

      if (toolName === 'eyevesa_skill_trust') {
        const scores = await this._client.getSkillTrust(toolInput.agent_id as string);
        return JSON.stringify(scores);
      }

      return JSON.stringify({ error: `Unknown tool: ${toolName}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('not authorized') || msg.includes('NotAuthorized')) {
        return JSON.stringify({ error: 'NOT_AUTHORIZED', reason: msg });
      }
      if (msg.includes('HITL') || msg.includes('hitl')) {
        return JSON.stringify({ error: 'HITL_REQUIRED', reason: msg });
      }
      return JSON.stringify({ error: msg });
    }
  }

  get client(): AgentClient {
    return this._client;
  }
}

// ── OpenAI Integration ──────────────────────────────────────────────────────

/**
 * Integration with OpenAI (Responses API with function_call and computer_use).
 *
 * Usage:
 *   const openai = new OpenAIIntegration(client);
 *   await openai.connect();
 *   const functionTools = openai.getFunctionTools();
 *   const allTools = openai.getComputerAndFunctionTools();
 *   const result = await openai.handleFunctionCall("eyevesa_read", { resource_id: "..." });
 */
export class OpenAIIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) {
    this._client = client;
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): OpenAIIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'openai-agent',
      owner: opts.owner || 'openai',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new OpenAIIntegration(client);
  }

  async connect(): Promise<void> {
    await this._client.connect();
  }

  /** Get eyeVesa tools in OpenAI function calling format. */
  getFunctionTools(): Record<string, unknown>[] {
    return EYEVESA_TOOL_DEFINITIONS.map((tool) => ({
      type: 'function',
      function: {
        name: tool.name,
        description: tool.description,
        parameters: tool.input_schema,
      },
    }));
  }

  /** Get both computer tool and eyeVesa function tools for combined use. */
  getComputerAndFunctionTools(): Record<string, unknown>[] {
    return [{ type: 'computer' }, ...this.getFunctionTools()];
  }

  /** Route an OpenAI function_call through eyeVesa. */
  async handleFunctionCall(functionName: string, args: Record<string, unknown>): Promise<string> {
    try {
      if (functionName === 'eyevesa_read') {
        const result = await this._client.invoke(
          args.resource_id as string, 'read',
          { query: args.query ?? '' },
        );
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }

      if (functionName === 'eyevesa_write') {
        const result = await this._client.invoke(
          args.resource_id as string, 'write',
          { data: args.data },
        );
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }

      if (functionName === 'eyevesa_request_approval') {
        const approval = await this._client.requestApproval(
          args.action as string,
          args.reason as string,
          args.risk_level as string,
        );
        return JSON.stringify({ approval_id: approval.approvalId, status: approval.status });
      }

      if (functionName === 'eyevesa_discover') {
        const capability = (args.capability as string) || 'mcp';
        const toolsInfo = await this._client.discover(capability);
        return JSON.stringify(toolsInfo);
      }

      if (functionName === 'eyevesa_delegate') {
        const result = await this._client.delegate(
          args.delegatee_id as string,
          args.scope as string[],
          (args.reason as string) || '',
        );
        return JSON.stringify({ delegation_id: result.delegationId, status: result.status });
      }

      if (functionName === 'eyevesa_skill_trust') {
        const scores = await this._client.getSkillTrust(args.agent_id as string);
        return JSON.stringify(scores);
      }

      return JSON.stringify({ error: `Unknown function: ${functionName}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('not authorized') || msg.includes('NotAuthorized')) {
        return JSON.stringify({ error: 'NOT_AUTHORIZED', reason: msg });
      }
      if (msg.includes('HITL') || msg.includes('hitl')) {
        return JSON.stringify({ error: 'HITL_REQUIRED', reason: msg });
      }
      return JSON.stringify({ error: msg });
    }
  }

  get client(): AgentClient {
    return this._client;
  }
}

// ── Hermes Integration ────────────────────────────────────────────────────

/**
 * Integration with Hermes agent framework.
 *
 * Hermes uses a task/action model where agents declare capabilities
 * as structured tool specs and maintain presence at the Airport
 * via periodic heartbeat.
 */
export class HermesIntegration {
  private _client: AgentClient;
  private _heartbeatStatus: string;

  constructor(client: AgentClient) {
    this._client = client;
    this._heartbeatStatus = 'idle';
  }

  static fromConfig(opts: {
    gatewayEndpoint?: string;
    agentName?: string;
    owner?: string;
    apiKey?: string;
    jwtToken?: string;
  }): HermesIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '',
      name: opts.agentName || 'hermes-agent',
      owner: opts.owner || 'hermes',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new HermesIntegration(client);
  }

  async connect(): Promise<void> { await this._client.connect(); }

  getToolSpecs(): Record<string, unknown>[] {
    return EYEVESA_TOOL_DEFINITIONS.map((tool) => ({ ...tool, action_type: 'eyevesa_gateway' }));
  }

  async handleAction(actionName: string, actionInput: Record<string, unknown>): Promise<string> {
    try {
      if (actionName === 'eyevesa_read') {
        const result = await this._client.invoke(actionInput.resource_id as string, 'read', { query: actionInput.query ?? '' });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (actionName === 'eyevesa_write') {
        const result = await this._client.invoke(actionInput.resource_id as string, 'write', { data: actionInput.data });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (actionName === 'eyevesa_request_approval') {
        const approval = await this._client.requestApproval(actionInput.action as string, actionInput.reason as string, actionInput.risk_level as string);
        return JSON.stringify({ approval_id: approval.approvalId, status: approval.status });
      }
      if (actionName === 'eyevesa_discover') {
        const toolsInfo = await this._client.discover((actionInput.capability as string) || 'mcp');
        return JSON.stringify(toolsInfo);
      }
      if (actionName === 'eyevesa_delegate') {
        const result = await this._client.delegate(actionInput.delegatee_id as string, actionInput.scope as string[], (actionInput.reason as string) || '');
        return JSON.stringify({ delegation_id: result.delegationId, status: result.status });
      }
      if (actionName === 'eyevesa_skill_trust') {
        const scores = await this._client.getSkillTrust(actionInput.agent_id as string);
        return JSON.stringify(scores);
      }
      return JSON.stringify({ error: `Unknown action: ${actionName}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('not authorized') || msg.includes('NotAuthorized')) return JSON.stringify({ error: 'NOT_AUTHORIZED', reason: msg });
      if (msg.includes('HITL') || msg.includes('hitl')) return JSON.stringify({ error: 'HITL_REQUIRED', reason: msg });
      return JSON.stringify({ error: msg });
    }
  }

  async heartbeat(status: string = 'online'): Promise<Record<string, unknown>> { this._heartbeatStatus = status; return this._client.airportHeartbeat(status); }
  async updateAirportProfile(opts: { description?: string; tags?: string[]; listed?: boolean }): Promise<Record<string, unknown>> { return this._client.airportUpdateProfile(opts); }
  async discoverPeers(opts: { capability?: string; status?: string; tag?: string; minTrust?: number } = {}): Promise<Record<string, unknown>> { return this._client.airportSearch(opts); }
  async listOnlinePeers(): Promise<Record<string, unknown>> { return this._client.airportListOnline(); }
  async getPeerProfile(agentId: string): Promise<Record<string, unknown>> { return this._client.airportGetProfile(agentId); }
  async getConnections(agentId?: string, limit: number = 50): Promise<Record<string, unknown>> { return this._client.airportConnections(agentId, limit); }

  get client(): AgentClient { return this._client; }
  get heartbeatStatus(): string { return this._heartbeatStatus; }
}

// ── OpenClaw Integration ──────────────────────────────────────────────────

/**
 * Integration with OpenClaw agent framework.
 *
 * OpenClaw uses a tool registry pattern where tools are discovered
 * dynamically and registered with the agent's runtime.
 */
export class OpenClawIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) { this._client = client; }

  static fromConfig(opts: {
    gatewayEndpoint?: string; agentName?: string; owner?: string; apiKey?: string; jwtToken?: string;
  }): OpenClawIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '', name: opts.agentName || 'openclaw-agent', owner: opts.owner || 'openclaw',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new OpenClawIntegration(client);
  }

  async connect(): Promise<void> { await this._client.connect(); }

  getToolSpecs(): Record<string, unknown>[] {
    return EYEVESA_TOOL_DEFINITIONS.map((tool) => ({ ...tool, handler: 'eyevesa_gateway', source: 'eyevesa', permissions: ['read', 'write'] }));
  }

  async executeTool(toolName: string, arguments_: Record<string, unknown>): Promise<string> {
    try {
      if (toolName === 'eyevesa_read') {
        const result = await this._client.invoke(arguments_.resource_id as string, 'read', { query: arguments_.query ?? '' });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (toolName === 'eyevesa_write') {
        const result = await this._client.invoke(arguments_.resource_id as string, 'write', { data: arguments_.data });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (toolName === 'eyevesa_request_approval') {
        const approval = await this._client.requestApproval(arguments_.action as string, arguments_.reason as string, arguments_.risk_level as string);
        return JSON.stringify({ approval_id: approval.approvalId, status: approval.status });
      }
      if (toolName === 'eyevesa_discover') {
        const toolsInfo = await this._client.discover((arguments_.capability as string) || 'mcp');
        return JSON.stringify(toolsInfo);
      }
      if (toolName === 'eyevesa_delegate') {
        const result = await this._client.delegate(arguments_.delegatee_id as string, arguments_.scope as string[], (arguments_.reason as string) || '');
        return JSON.stringify({ delegation_id: result.delegationId, status: result.status });
      }
      if (toolName === 'eyevesa_skill_trust') {
        const scores = await this._client.getSkillTrust(arguments_.agent_id as string);
        return JSON.stringify(scores);
      }
      return JSON.stringify({ error: `Unknown tool: ${toolName}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('not authorized') || msg.includes('NotAuthorized')) return JSON.stringify({ error: 'NOT_AUTHORIZED', reason: msg });
      if (msg.includes('HITL') || msg.includes('hitl')) return JSON.stringify({ error: 'HITL_REQUIRED', reason: msg });
      return JSON.stringify({ error: msg });
    }
  }

  async registerAtAirport(opts: { description?: string; tags?: string[]; listed?: boolean }): Promise<Record<string, unknown>> {
    await this._client.airportHeartbeat('online');
    return this._client.airportUpdateProfile({ description: opts.description, tags: opts.tags || ['openclaw'], listed: opts.listed });
  }

  async discoverAgents(opts: { capability?: string; tag?: string; minTrust?: number } = {}): Promise<Record<string, unknown>> { return this._client.airportSearch(opts); }
  async listOnlineAgents(): Promise<Record<string, unknown>> { return this._client.airportListOnline(); }

  get client(): AgentClient { return this._client; }
}

// ── NanoClaw Integration ──────────────────────────────────────────────────

/**
 * Integration with NanoClaw agent framework.
 *
 * NanoClaw is a lightweight claw-based agent framework that uses compact
 * tool definitions with guardrails metadata and trust-gated execution.
 */
export class NanoClawIntegration {
  private _client: AgentClient;

  constructor(client: AgentClient) { this._client = client; }

  static fromConfig(opts: {
    gatewayEndpoint?: string; agentName?: string; owner?: string; apiKey?: string; jwtToken?: string;
  }): NanoClawIntegration {
    const { AgentClient: AC } = require('./client');
    const config: AgentConfig = {
      agentId: '', name: opts.agentName || 'nanoclaw-agent', owner: opts.owner || 'nanoclaw',
      gatewayEndpoint: opts.gatewayEndpoint || 'http://localhost:9443',
    };
    const client = new AC(config, { apiKey: opts.apiKey, jwtToken: opts.jwtToken });
    return new NanoClawIntegration(client);
  }

  async connect(): Promise<void> { await this._client.connect(); }

  getFunctionDefinitions(): Record<string, unknown>[] {
    return EYEVESA_TOOL_DEFINITIONS.map((tool) => ({
      name: tool.name, description: tool.description, parameters: tool.input_schema,
      guardrails: { input_validation: true, output_validation: true },
      trust_requirement: tool.name.includes('read') ? 0.5 : 0.7,
    }));
  }

  async executeFunction(functionName: string, args: Record<string, unknown>): Promise<string> {
    try {
      if (functionName === 'eyevesa_read') {
        const result = await this._client.invoke(args.resource_id as string, 'read', { query: args.query ?? '' });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (functionName === 'eyevesa_write') {
        const result = await this._client.invoke(args.resource_id as string, 'write', { data: args.data });
        return JSON.stringify({ success: result.success, data: result.data, trust_score: result.trustScore });
      }
      if (functionName === 'eyevesa_request_approval') {
        const approval = await this._client.requestApproval(args.action as string, args.reason as string, args.risk_level as string);
        return JSON.stringify({ approval_id: approval.approvalId, status: approval.status });
      }
      if (functionName === 'eyevesa_discover') {
        const toolsInfo = await this._client.discover((args.capability as string) || 'mcp');
        return JSON.stringify(toolsInfo);
      }
      if (functionName === 'eyevesa_delegate') {
        const result = await this._client.delegate(args.delegatee_id as string, args.scope as string[], (args.reason as string) || '');
        return JSON.stringify({ delegation_id: result.delegationId, status: result.status });
      }
      if (functionName === 'eyevesa_skill_trust') {
        const scores = await this._client.getSkillTrust(args.agent_id as string);
        return JSON.stringify(scores);
      }
      return JSON.stringify({ error: `Unknown function: ${functionName}` });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('not authorized') || msg.includes('NotAuthorized')) return JSON.stringify({ error: 'NOT_AUTHORIZED', reason: msg });
      if (msg.includes('HITL') || msg.includes('hitl')) return JSON.stringify({ error: 'HITL_REQUIRED', reason: msg });
      return JSON.stringify({ error: msg });
    }
  }

  async checkTrust(agentId: string, minTrust: number = 0.5): Promise<boolean> {
    const profile = await this._client.airportGetProfile(agentId) as Record<string, unknown>;
    const trustScore = (profile.trust_score as number) || 0;
    return trustScore >= minTrust;
  }

  async heartbeat(status: string = 'online'): Promise<Record<string, unknown>> { return this._client.airportHeartbeat(status); }
  async updateAirportProfile(opts: { description?: string; tags?: string[]; listed?: boolean }): Promise<Record<string, unknown>> { return this._client.airportUpdateProfile(opts); }
  async discoverAgents(opts: { capability?: string; skill?: string; minTrust?: number } = {}): Promise<Record<string, unknown>> { return this._client.airportSearch(opts); }
  async listOnlineAgents(): Promise<Record<string, unknown>> { return this._client.airportListOnline(); }
  async getAgentProfile(agentId: string): Promise<Record<string, unknown>> { return this._client.airportGetProfile(agentId); }

  get client(): AgentClient { return this._client; }
}