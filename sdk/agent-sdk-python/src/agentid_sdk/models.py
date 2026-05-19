from __future__ import annotations

from typing import Any, Optional

from pydantic import BaseModel, Field


class AgentConfig(BaseModel):
    agent_id: str
    name: str
    owner: str
    gateway_endpoint: str


class ToolInfo(BaseModel):
    name: str
    description: str = ""
    resource_id: str
    parameters: dict[str, Any] = Field(default_factory=dict)


class InvokeResult(BaseModel):
    success: bool
    data: dict[str, Any] = Field(default_factory=dict)
    trust_score: float


class AuthorizeResult(BaseModel):
    allowed: bool
    requires_hitl: bool
    reason: str = ""
    trust_delta: float = 0.0


class DelegateResult(BaseModel):
    delegation_id: str
    status: str


class PtvAttestResult(BaseModel):
    attestation: dict[str, Any] = Field(default_factory=dict)
    tpm_signature: str = ""
    quote: str = ""


class PtvBindResult(BaseModel):
    binding_id: str
    agent_id: str
    platform: str
    transformed_at: int
    expires_at: int


class HitlApproval(BaseModel):
    approval_id: str
    agent_id: str
    action: str
    status: str
    expires_at: Optional[str] = None


class Skill(BaseModel):
    skill_id: str
    name: str
    description: str = ""
    category: str = ""
    risk_level: str = "medium"
    required_trust_min: float = 0.0
    required_proficiency: int = 0
    created_at: str = ""
    updated_at: str = ""


class AgentSkill(BaseModel):
    agent_id: str
    skill_id: str
    skill_name: str = ""
    proficiency: int = 0
    verified: bool = False
    verified_by: str = ""
    verified_at: Optional[str] = None
    endorsements_count: int = 0
    acquired_at: str = ""


class Endorsement(BaseModel):
    endorsement_id: str
    agent_id: str
    skill_id: str
    endorser_type: str
    endorser_id: str
    comment: str = ""
    created_at: str = ""


class SkillTrustScore(BaseModel):
    agent_id: str
    skill_id: str
    skill_name: str = ""
    trust_score: float
    updated_at: str = ""


class McpCapabilities(BaseModel):
    protocol_version: str = "unknown"
    tools: bool = False
    resources: bool = False
    prompts: bool = False


class McpTool(BaseModel):
    name: str
    description: Optional[str] = None
    input_schema: Optional[dict[str, Any]] = None


class CapabilityToken(BaseModel):
    id: str = ""
    issuer: str = "agentid-gateway"
    subject: str = ""
    resource_id: str = ""
    action: str = ""
    scopes: list[str] = Field(default_factory=list)
    trust_score: float = 0.0
    agent_skills: list[dict[str, Any]] = Field(default_factory=list)
    params: Optional[dict[str, Any]] = None
    issued_at: int = 0
    expires_at: int = 0
    nonce: str = ""
    signature: str = ""


class TransactionReceipt(BaseModel):
    receipt_id: str = ""
    token_id: str = ""
    agent_id: str = ""
    resource_id: str = ""
    action: str = ""
    allowed: bool = False
    trust_score: float = 0.0
    trust_delta: float = 0.0
    token_issued_at: int = 0
    token_expires: int = 0
    issued_at: str = ""
    signature: str = ""


class AgentPassport(BaseModel):
    agent_id: str
    agent_public_key: str
    gateway_id: str
    gateway_signature: str
    issued_at: str = ""


class FederationPeer(BaseModel):
    gateway_id: str
    name: str
    public_key: str
    endpoint: str
    trust_domain: str = ""
    status: str = "active"
    trust_score: float = 1.0
    agent_count: int = 0
    last_sync_at: Optional[str] = None
    registered_at: str = ""


class FederatedAgent(BaseModel):
    agent_id: str
    gateway_id: str
    name: str = ""
    owner: str = ""
    public_key: str = ""
    trust_score: float = 1.0
    capabilities: list[str] = Field(default_factory=list)
    allowed_tools: list[str] = Field(default_factory=list)
    passport_issued_at: str = ""
    status: str = "active"
    description: str = ""
    tags: list[str] = Field(default_factory=list)
    heartbeat_status: str = "offline"
    last_heartbeat: str = ""