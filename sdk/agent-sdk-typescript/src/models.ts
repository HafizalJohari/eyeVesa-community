export interface AgentConfig {
  agentId: string;
  name: string;
  owner: string;
  gatewayEndpoint: string;
}

export interface ToolInfo {
  name: string;
  description: string;
  resourceId: string;
  parameters: Record<string, unknown>;
}

export interface InvokeResult {
  success: boolean;
  data: Record<string, unknown>;
  trustScore: number;
}

export interface AuthorizeResult {
  allowed: boolean;
  requiresHitl: boolean;
  reason: string;
  trustDelta: number;
}

export interface DelegateResult {
  delegationId: string;
  status: string;
}

export interface PtvAttestResult {
  attestation: Record<string, unknown>;
  tpmSignature: string;
  quote: string;
}

export interface PtvBindResult {
  bindingId: string;
  agentId: string;
  platform: string;
  transformedAt: number;
  expiresAt: number;
}

export interface HitlApproval {
  approvalId: string;
  agentId: string;
  action: string;
  status: string;
  expiresAt?: string;
}

export interface Skill {
  skillId: string;
  name: string;
  description: string;
  category: string;
  riskLevel: string;
  requiredTrustMin: number;
  requiredProficiency: number;
  createdAt: string;
  updatedAt: string;
}

export interface AgentSkill {
  agentId: string;
  skillId: string;
  skillName: string;
  proficiency: number;
  verified: boolean;
  verifiedBy: string;
  verifiedAt?: string;
  endorsementsCount: number;
  acquiredAt: string;
}

export interface Endorsement {
  endorsementId: string;
  agentId: string;
  skillId: string;
  endorserType: string;
  endorserId: string;
  comment: string;
  createdAt: string;
}

export interface SkillTrustScore {
  agentId: string;
  skillId: string;
  skillName: string;
  trustScore: number;
  updatedAt: string;
}

export interface McpCapabilities {
  protocolVersion: string;
  tools: boolean;
  resources: boolean;
  prompts: boolean;
}

export interface McpTool {
  name: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
}

export interface CapabilityToken {
  id: string;
  issuer: string;
  subject: string;
  resourceId: string;
  action: string;
  scopes: string[];
  trustScore: number;
  agentSkills: Record<string, unknown>[];
  params?: Record<string, unknown>;
  issuedAt: number;
  expiresAt: number;
  nonce: string;
  signature: string;
}

export interface TransactionReceipt {
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
  signature: string;
}