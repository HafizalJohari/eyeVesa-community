import * as crypto from 'crypto';
import type { IncomingMessage, ServerResponse } from 'http';
import type { SecureContextOptions } from 'tls';

import type {
  AgentConfig,
  AuthorizeResult,
  CapabilityToken,
  DelegateResult,
  HitlApproval,
  InvokeResult,
  McpCapabilities,
  McpTool,
  PtvAttestResult,
  PtvBindResult,
  Skill,
  AgentSkill,
  Endorsement,
  SkillTrustScore,
  ToolInfo,
  TransactionReceipt,
} from './models';
import {
  AgentIDError,
  AuthFailedError,
  ConnectError,
  DelegateError,
  DiscoverError,
  HitlError,
  HitlRequiredError,
  InvokeError,
  MaxDepthError,
  McpError,
  NotAuthorizedError,
  PtvError,
  SkillError,
  TxError,
  VerifyError,
} from './exceptions';

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

function uuid(): string {
  return crypto.randomUUID();
}

function base64Encode(data: Uint8Array | string): string {
  const buf = typeof data === 'string' ? Buffer.from(data, 'utf-8') : Buffer.from(data);
  return buf.toString('base64');
}

function snakeToCamel(obj: unknown): unknown {
  if (obj === null || obj === undefined) return obj;
  if (Array.isArray(obj)) return obj.map(snakeToCamel);
  if (typeof obj === 'object') {
    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj as Record<string, unknown>)) {
      const camelKey = key.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
      result[camelKey] = snakeToCamel(value);
    }
    return result;
  }
  return obj;
}

export class AgentClient {
  private _config: AgentConfig;
  private _trustScore: number;
  private _registered: boolean;
  private _apiKey?: string;
  private _jwtToken?: string;

  constructor(
    config: AgentConfig,
    opts?: { apiKey?: string; jwtToken?: string }
  ) {
    this._config = config;
    this._trustScore = 1.0;
    this._registered = false;
    this._apiKey = opts?.apiKey;
    this._jwtToken = opts?.jwtToken;
  }

  static fromEnv(opts?: { apiKey?: string; jwtToken?: string }): AgentClient {
    const config: AgentConfig = {
      agentId: process.env.AGENT_ID || uuid(),
      name: process.env.AGENT_NAME || 'node-agent',
      owner: process.env.AGENT_OWNER || 'default',
      gatewayEndpoint: process.env.GATEWAY_ENDPOINT || 'http://localhost:9443',
    };
    return new AgentClient(config, opts);
  }

  get agentId(): string { return this._config.agentId; }
  get name(): string { return this._config.name; }
  get owner(): string { return this._config.owner; }
  get trustScore(): number { return this._trustScore; }
  get isRegistered(): boolean { return this._registered; }
  get gatewayEndpoint(): string { return this._config.gatewayEndpoint; }

  private buildHeaders(): Record<string, string> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this._apiKey) headers['X-API-Key'] = this._apiKey;
    if (this._jwtToken) headers['Authorization'] = `Bearer ${this._jwtToken}`;
    return headers;
  }

  private async request(
    method: 'GET' | 'POST' | 'PUT' | 'DELETE',
    path: string,
    body?: unknown
  ): Promise<unknown> {
    const url = `${this._config.gatewayEndpoint.replace(/\/$/, '')}${path}`;
    const init: RequestInit = {
      method,
      headers: this.buildHeaders(),
    };
    if (body !== undefined) {
      init.body = JSON.stringify(body);
    }

    const resp = await fetch(url, init);
    return resp;
  }

  private async jsonRequest(
    method: 'GET' | 'POST' | 'PUT' | 'DELETE',
    path: string,
    body?: unknown,
    queryParams?: Record<string, string>
  ): Promise<unknown> {
    let url = `${this._config.gatewayEndpoint.replace(/\/$/, '')}${path}`;
    if (queryParams) {
      const params = new URLSearchParams(queryParams);
      url += `?${params.toString()}`;
    }

    const init: RequestInit = {
      method,
      headers: this.buildHeaders(),
    };
    if (body !== undefined) {
      init.body = JSON.stringify(body);
    }

    const resp = await fetch(url, init);
    if (!resp.ok) {
      const text = await resp.text().catch(() => '');
      throw new AgentIDError(`${method} ${path}: ${resp.status} ${text}`);
    }
    return resp.json();
  }

  async connect(): Promise<AgentClient> {
    const body = {
      name: this._config.name,
      owner: this._config.owner,
      capabilities: ['mcp'],
      allowed_tools: ['read', 'get_weather', 'search_docs'],
    };

    const data = await this.jsonRequest('POST', '/v1/register', body) as Record<string, unknown>;
    this._trustScore = (data.trust_score as number) ?? 1.0;
    this._registered = true;
    if (data.agent_id) {
      this._config.agentId = data.agent_id as string;
    }
    return this;
  }

  async discover(capability: string = 'mcp'): Promise<ToolInfo[]> {
    const data = await this.jsonRequest('GET', '/v1/resources', undefined, { capability }) as Record<string, unknown>;
    const resources = (data.resources as Record<string, unknown>[]) || [];

    if (!resources.length) {
      throw new DiscoverError(`No resources found matching: ${capability}`);
    }

    return resources.map((r: Record<string, unknown>) => ({
      name: (r.name as string) || '',
      description: (r.description as string) || '',
      resourceId: (r.resource_id as string) || '',
      parameters: (r.capabilities_json as Record<string, unknown>) || {},
    }));
  }

  async invoke(resourceId: string, tool: string, params?: Record<string, unknown>): Promise<InvokeResult> {
    const authBody = {
      agent_id: this._config.agentId,
      action: tool,
      resource_id: resourceId,
    };

    const authData = await this.jsonRequest('POST', '/v1/auth', authBody) as Record<string, unknown>;
    const authResult: AuthorizeResult = {
      allowed: authData.allowed as boolean,
      requiresHitl: authData.requires_hitl as boolean,
      reason: (authData.reason as string) || '',
      trustDelta: (authData.trust_delta as number) || 0,
    };

    if (!authResult.allowed) {
      if (authResult.requiresHitl) {
        throw new HitlRequiredError(authResult.reason);
      }
      throw new NotAuthorizedError(authResult.reason);
    }

    const mcpBody = {
      jsonrpc: '2.0',
      method: 'tools/call',
      id: 1,
      params: {
        name: tool,
        arguments: {
          agent_id: this._config.agentId,
          resource_id: resourceId,
          ...params,
        },
      },
    };

    const mcpData = await this.jsonRequest('POST', '/v1/mcp', mcpBody) as Record<string, unknown>;
    const resultData = (mcpData.result as Record<string, unknown>) || { status: 'invoked' };

    return { success: true, data: resultData, trustScore: this._trustScore };
  }

  async delegate(delegateeId: string, scope: string[], reason: string = ''): Promise<DelegateResult> {
    const body = {
      delegator_id: this._config.agentId,
      delegatee_id: delegateeId,
      scope,
      reason,
    };

    try {
      const data = await this.jsonRequest('POST', '/v1/delegate', body) as Record<string, unknown>;
      return {
        delegationId: (data.delegation_id as string) || '',
        status: (data.status as string) || 'unknown',
      };
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.toLowerCase().includes('max depth') || msg.toLowerCase().includes('depth')) {
        throw new MaxDepthError(msg);
      }
      throw new DelegateError(msg);
    }
  }

  async attest(platform: string, firmwareVersion: string): Promise<PtvAttestResult> {
    const body = {
      agent_id: this._config.agentId,
      platform,
      firmware_version: firmwareVersion,
    };

    const data = await this.jsonRequest('POST', '/v1/ptv/attest', body) as Record<string, unknown>;
    return {
      attestation: {
        agent_id: data.agent_id || '',
        platform: data.platform || '',
        nonce: data.nonce || '',
      },
      tpmSignature: (data.tpm_signature as string) || '',
      quote: (data.quote as string) || '',
    };
  }

  async bind(
    attestation: Record<string, unknown>,
    tpmSignature: string,
    platform: string,
    firmwareVersion: string,
    agentId?: string
  ): Promise<PtvBindResult> {
    const body = {
      agent_id: agentId || this._config.agentId,
      platform,
      firmware_version: firmwareVersion,
      tpm_signature: tpmSignature,
      attestation,
    };

    const data = await this.jsonRequest('POST', '/v1/ptv/bind', body) as Record<string, unknown>;
    return {
      bindingId: (data.binding_id as string) || '',
      agentId: (data.agent_id as string) || '',
      platform: (data.platform as string) || '',
      transformedAt: (data.transformed_at as number) || 0,
      expiresAt: (data.expires_at as number) || 0,
    };
  }

  async verifyBinding(bindingId: string): Promise<boolean> {
    const data = await this.jsonRequest('GET', `/v1/ptv/verify/${bindingId}`) as Record<string, unknown>;
    return (data.valid as boolean) || false;
  }

  async requestApproval(action: string, reason: string = '', riskLevel: string = 'medium'): Promise<HitlApproval> {
    const body = {
      agent_id: this._config.agentId,
      action,
      reason,
      risk_level: riskLevel,
    };

    const data = await this.jsonRequest('POST', '/v1/hitl/request', body) as Record<string, unknown>;
    return {
      approvalId: (data.approval_id as string) || '',
      agentId: this._config.agentId,
      action,
      status: (data.status as string) || 'pending',
    };
  }

  async decideApproval(approvalId: string, approved: boolean, approverMethod: string = 'manual'): Promise<string> {
    const body = { approval_id: approvalId, approved, approver_method: approverMethod };
    const data = await this.jsonRequest('POST', `/v1/hitl/${approvalId}/decide`, body) as Record<string, unknown>;
    return (data.status as string) || 'unknown';
  }

  async getApprovalStatus(approvalId: string): Promise<string> {
    const data = await this.jsonRequest('GET', `/v1/hitl/${approvalId}`) as Record<string, unknown>;
    return (data.status as string) || 'unknown';
  }

  async listPendingApprovals(): Promise<Record<string, unknown>[]> {
    const data = await this.jsonRequest('GET', '/v1/hitl/pending', undefined, { agent_id: this._config.agentId }) as Record<string, unknown>;
    return (data.approvals as Record<string, unknown>[]) || [];
  }

  async mcpInitialize(): Promise<McpCapabilities> {
    const body = { jsonrpc: '2.0', method: 'initialize', id: 1 };
    const data = await this.jsonRequest('POST', '/v1/mcp', body) as Record<string, unknown>;
    const result = (data.result as Record<string, unknown>) || {};
    const caps = (result.capabilities as Record<string, unknown>) || {};

    return {
      protocolVersion: (result.protocolVersion as string) || 'unknown',
      tools: 'tools' in caps,
      resources: 'resources' in caps,
      prompts: 'prompts' in caps,
    };
  }

  async mcpListTools(): Promise<McpTool[]> {
    const body = { jsonrpc: '2.0', method: 'tools/list', id: 2 };
    const data = await this.jsonRequest('POST', '/v1/mcp', body) as Record<string, unknown>;
    const result = (data.result as Record<string, unknown>) || {};
    const toolsArr = (result.tools as Record<string, unknown>[]) || [];

    return toolsArr.map((t: Record<string, unknown>) => ({
      name: (t.name as string) || '',
      description: t.description as string | undefined,
      inputSchema: t.inputSchema as Record<string, unknown> | undefined,
    }));
  }

  async mcpCallTool(toolName: string, arguments_: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
    const body = {
      jsonrpc: '2.0',
      method: 'tools/call',
      id: 3,
      params: { name: toolName, arguments: arguments_ },
    };
    const data = await this.jsonRequest('POST', '/v1/mcp', body) as Record<string, unknown>;
    return (data.result as Record<string, unknown>) || {};
  }

  async verifySignature(agentId: string, message: Uint8Array, signature: Uint8Array): Promise<boolean> {
    const body = {
      agent_id: agentId,
      message: base64Encode(message),
      signature: base64Encode(signature),
    };

    try {
      const data = await this.jsonRequest('POST', '/v1/verify-signature', body) as Record<string, unknown>;
      return (data.valid as boolean) || false;
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (msg.includes('404')) throw new VerifyError(`Agent not found: ${agentId}`);
      throw new VerifyError(msg);
    }
  }

  async listSkills(category: string = ''): Promise<Skill[]> {
    const queryParams = category ? { category } : undefined;
    const data = await this.jsonRequest('GET', '/v1/skills', undefined, queryParams) as Record<string, unknown>;
    const skills = (data.skills as Record<string, unknown>[]) || [];
    return skills.map((s: Record<string, unknown>) => snakeToCamel(s) as Skill);
  }

  async createSkill(
    name: string,
    description: string = '',
    category: string = '',
    riskLevel: string = 'medium',
    requiredTrustMin: number = 0,
    requiredProficiency: number = 0
  ): Promise<Skill> {
    const body: Record<string, unknown> = { name, description, category, risk_level: riskLevel };
    if (requiredTrustMin > 0) body.required_trust_min = requiredTrustMin;
    if (requiredProficiency > 0) body.required_proficiency = requiredProficiency;

    const data = await this.jsonRequest('POST', '/v1/skills', body) as Record<string, unknown>;
    return snakeToCamel(data) as Skill;
  }

  async assignSkill(agentId: string, skillId: string, proficiency: number = 1): Promise<AgentSkill> {
    const body = { skill_id: skillId, proficiency };
    const data = await this.jsonRequest('POST', `/v1/agents/${agentId}/skills`, body) as Record<string, unknown>;
    return snakeToCamel(data) as AgentSkill;
  }

  async endorseSkill(
    agentId: string,
    skillId: string,
    endorserType: string = 'agent',
    endorserId: string = '',
    comment: string = ''
  ): Promise<Endorsement> {
    const body = { endorser_type: endorserType, endorser_id: endorserId, comment };
    const data = await this.jsonRequest('POST', `/v1/agents/${agentId}/skills/${skillId}/endorse`, body) as Record<string, unknown>;
    return snakeToCamel(data) as Endorsement;
  }

  async verifySkill(agentId: string, skillId: string, verifiedBy: string = ''): Promise<AgentSkill> {
    const body = { verified_by: verifiedBy };
    const data = await this.jsonRequest('POST', `/v1/agents/${agentId}/skills/${skillId}/verify`, body) as Record<string, unknown>;
    return snakeToCamel(data) as AgentSkill;
  }

  async getSkillTrust(agentId: string): Promise<SkillTrustScore[]> {
    const data = await this.jsonRequest('GET', `/v1/agents/${agentId}/skill-trust`) as Record<string, unknown>;
    const scores = (data.scores as Record<string, unknown>[]) || [];
    return scores.map((s: Record<string, unknown>) => snakeToCamel(s) as SkillTrustScore);
  }

  async issueToken(
    agentId: string,
    resourceId: string,
    action: string,
    trustScore: number = 1.0,
    scopes: string[] = [],
    skills: Record<string, unknown>[] = [],
    params?: Record<string, unknown>
  ): Promise<CapabilityToken> {
    const body: Record<string, unknown> = {
      agent_id: agentId,
      resource_id: resourceId,
      action,
      trust_score: trustScore,
      scopes,
      skills,
    };
    if (params) body.params = params;
    const data = await this.jsonRequest('POST', '/v1/tx/issue', body) as Record<string, unknown>;
    return snakeToCamel(data) as CapabilityToken;
  }

  async verifyToken(tokenId: string): Promise<CapabilityToken> {
    const data = await this.jsonRequest('POST', '/v1/tx/verify', { token_id: tokenId }) as Record<string, unknown>;
    return snakeToCamel(data) as CapabilityToken;
  }

  async revokeToken(tokenId: string, reason: string = ''): Promise<Record<string, unknown>> {
    return this.jsonRequest('POST', `/v1/tx/revoke/${tokenId}`, { reason }) as Promise<Record<string, unknown>>;
  }

  async listRevokedTokens(): Promise<Record<string, unknown>[]> {
    const data = await this.jsonRequest('GET', '/v1/tx/revoked') as Record<string, unknown>;
    return (data.tokens as Record<string, unknown>[]) || [];
  }

  async issueReceipt(
    tokenId: string,
    allowed: boolean = true,
    trustScore: number = 1.0,
    trustDelta: number = 0.0
  ): Promise<TransactionReceipt> {
    const body = { token_id: tokenId, allowed, trust_score: trustScore, trust_delta: trustDelta };
    const data = await this.jsonRequest('POST', '/v1/tx/receipt', body) as Record<string, unknown>;
    return snakeToCamel(data) as TransactionReceipt;
  }

  async verifyReceipt(receipt: TransactionReceipt): Promise<boolean> {
    const data = await this.jsonRequest('POST', '/v1/tx/receipt/verify', receipt) as Record<string, unknown>;
    return (data.valid as boolean) || false;
  }

  async airportHeartbeat(status: string = 'online', metadata?: Record<string, unknown>): Promise<Record<string, unknown>> {
    const body: Record<string, unknown> = { agent_id: this._config.agentId, status };
    if (metadata) body.metadata = metadata;
    return await this.jsonRequest('POST', '/v1/airport/heartbeat', body) as Record<string, unknown>;
  }

  async airportUpdateProfile(opts: {
    description?: string;
    servicesOffered?: string[];
    endpoints?: Record<string, string>;
    tags?: string[];
    listed?: boolean;
  }): Promise<Record<string, unknown>> {
    const body: Record<string, unknown> = {};
    if (opts.description !== undefined) body.description = opts.description;
    if (opts.servicesOffered !== undefined) body.services_offered = opts.servicesOffered;
    if (opts.endpoints !== undefined) body.endpoints = opts.endpoints;
    if (opts.tags !== undefined) body.tags = opts.tags;
    if (opts.listed !== undefined) body.listed = opts.listed;
    return await this.jsonRequest('PUT', `/v1/airport/agents/${this._config.agentId}`, body) as Record<string, unknown>;
  }

  async airportSearch(opts: {
    capability?: string;
    skill?: string;
    minTrust?: number;
    status?: string;
    tag?: string;
    owner?: string;
    limit?: number;
    offset?: number;
  } = {}): Promise<Record<string, unknown>> {
    const params: Record<string, string> = {};
    if (opts.capability) params.capability = opts.capability;
    if (opts.skill) params.skill = opts.skill;
    if (opts.minTrust !== undefined) params.min_trust = String(opts.minTrust);
    if (opts.status) params.status = opts.status;
    if (opts.tag) params.tag = opts.tag;
    if (opts.owner) params.owner = opts.owner;
    if (opts.limit !== undefined) params.limit = String(opts.limit);
    if (opts.offset !== undefined) params.offset = String(opts.offset);
    return await this.jsonRequest('GET', `/v1/airport/agents?${new URLSearchParams(params)}`) as Record<string, unknown>;
  }

  async airportGetProfile(agentId: string): Promise<Record<string, unknown>> {
    return await this.jsonRequest('GET', `/v1/airport/agents/${agentId}`) as Record<string, unknown>;
  }

  async airportListOnline(): Promise<Record<string, unknown>> {
    return await this.jsonRequest('GET', '/v1/airport/online') as Record<string, unknown>;
  }

  async airportConnections(agentId?: string, limit: number = 50): Promise<Record<string, unknown>> {
    const params = new URLSearchParams({ limit: String(limit) });
    if (agentId) params.set('agent_id', agentId);
    return await this.jsonRequest('GET', `/v1/airport/connections?${params}`) as Record<string, unknown>;
  }
}