from __future__ import annotations

import base64
import json
import logging
from typing import Any, Optional
from uuid import uuid4

import httpx
from nacl.signing import SigningKey

from .exceptions import (
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
)
from .models import (
    AgentConfig,
    AgentSkill,
    AuthorizeResult,
    CapabilityToken,
    DelegateResult,
    Endorsement,
    HitlApproval,
    InvokeResult,
    McpCapabilities,
    McpTool,
    PtvAttestResult,
    PtvBindResult,
    Skill,
    SkillTrustScore,
    ToolInfo,
    TransactionReceipt,
)

logger = logging.getLogger("agentid_sdk")


class AgentClient:
    def __init__(
        self,
        config: AgentConfig,
        signing_key: Optional[SigningKey] = None,
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> None:
        self._config = config
        self._signing_key = signing_key or SigningKey.generate()
        self._trust_score: float = 1.0
        self._registered: bool = False
        self._api_key = api_key
        self._jwt_token = jwt_token

        headers: dict[str, str] = {"Content-Type": "application/json"}
        if api_key:
            headers["X-API-Key"] = api_key
        if jwt_token:
            headers["Authorization"] = f"Bearer {jwt_token}"

        self._http = httpx.AsyncClient(
            base_url=config.gateway_endpoint.rstrip("/"),
            headers=headers,
            timeout=30.0,
        )

    @classmethod
    def from_env(cls, signing_key: Optional[SigningKey] = None) -> "AgentClient":
        import os

        config = AgentConfig(
            agent_id=os.getenv("AGENT_ID", str(uuid4())),
            name=os.getenv("AGENT_NAME", "python-agent"),
            owner=os.getenv("AGENT_OWNER", "default"),
            gateway_endpoint=os.getenv("GATEWAY_ENDPOINT", "http://localhost:9443"),
        )
        return cls(config, signing_key)

    @property
    def agent_id(self) -> str:
        return self._config.agent_id

    @property
    def name(self) -> str:
        return self._config.name

    @property
    def owner(self) -> str:
        return self._config.owner

    @property
    def trust_score(self) -> float:
        return self._trust_score

    @property
    def is_registered(self) -> bool:
        return self._registered

    @property
    def gateway_endpoint(self) -> str:
        return self._config.gateway_endpoint

    def _url(self, path: str) -> str:
        return f"{self._config.gateway_endpoint.rstrip('/')}{path}"

    async def connect(self) -> "AgentClient":
        logger.info("Connecting to gateway at %s", self._config.gateway_endpoint)

        body = {
            "name": self._config.name,
            "owner": self._config.owner,
            "capabilities": ["mcp"],
            "allowed_tools": ["read", "get_weather", "search_docs"],
        }

        resp = await self._http.post("/v1/register", json=body)

        if resp.status_code != 200 and resp.status_code != 201:
            raise AuthFailedError(f"Registration failed: {resp.status_code} {resp.text}")

        data = resp.json()
        self._trust_score = data.get("trust_score", 1.0)
        self._registered = True
        if "agent_id" in data:
            self._config.agent_id = data["agent_id"]

        logger.info("Agent %s connected (status: %s)", self._config.agent_id, data.get("status", "unknown"))
        return self

    async def discover(self, capability: str = "mcp") -> list[ToolInfo]:
        logger.info("Discovering tools for capability: %s", capability)

        resp = await self._http.get("/v1/resources", params={"capability": capability})

        if resp.status_code != 200:
            raise DiscoverError(f"Discovery failed: {resp.status_code}")

        body = resp.json()
        resources = body.get("resources", [])

        if not resources:
            raise DiscoverError(f"No resources found matching: {capability}")

        tools = []
        for r in resources:
            tools.append(ToolInfo(
                name=r.get("name", ""),
                description=r.get("description", ""),
                resource_id=r.get("resource_id", ""),
                parameters=r.get("capabilities_json", {}),
            ))
        return tools

    async def invoke(
        self,
        resource_id: str,
        tool: str,
        params: Optional[dict[str, Any]] = None,
    ) -> InvokeResult:
        logger.info("Invoking tool %s on resource %s", tool, resource_id)

        auth_body = {
            "agent_id": self._config.agent_id,
            "action": tool,
            "resource_id": resource_id,
        }

        auth_resp = await self._http.post("/v1/auth", json=auth_body)
        if auth_resp.status_code != 200:
            raise InvokeError(f"Auth request failed: {auth_resp.status_code}")

        auth_result = AuthorizeResult(**auth_resp.json())

        if not auth_result.allowed:
            if auth_result.requires_hitl:
                raise HitlRequiredError(auth_result.reason)
            raise NotAuthorizedError(auth_result.reason)

        mcp_body: dict[str, Any] = {
            "jsonrpc": "2.0",
            "method": "tools/call",
            "id": 1,
            "params": {
                "name": tool,
                "arguments": {
                    "agent_id": self._config.agent_id,
                    "resource_id": resource_id,
                    **(params or {}),
                },
            },
        }

        if self._signing_key:
            payload_bytes = json.dumps(mcp_body, sort_keys=True).encode()
            signed = self._signing_key.sign(payload_bytes)
            mcp_body["signature"] = base64.b64encode(signed.signature).decode()

        mcp_resp = await self._http.post("/v1/mcp", json=mcp_body)
        if mcp_resp.status_code != 200:
            raise InvokeError(f"MCP request failed: {mcp_resp.status_code}")

        mcp_result = mcp_resp.json()
        result_data = mcp_result.get("result", {"status": "invoked"})

        return InvokeResult(success=True, data=result_data, trust_score=self._trust_score)

    async def delegate(
        self,
        delegatee_id: str,
        scope: list[str],
        reason: str = "",
    ) -> DelegateResult:
        logger.info("Delegating to agent %s with scope %s", delegatee_id, scope)

        body = {
            "delegator_id": self._config.agent_id,
            "delegatee_id": delegatee_id,
            "scope": scope,
            "reason": reason,
        }

        resp = await self._http.post("/v1/delegate", json=body)

        if resp.status_code != 200 and resp.status_code != 201:
            text = resp.text
            if "max depth" in text.lower() or "depth" in text.lower():
                raise MaxDepthError(text)
            raise DelegateError(f"{resp.status_code}: {text}")

        data = resp.json()
        return DelegateResult(
            delegation_id=data.get("delegation_id", ""),
            status=data.get("status", "unknown"),
        )

    async def attest(self, platform: str, firmware_version: str) -> PtvAttestResult:
        logger.info("Attesting identity for platform: %s", platform)

        body = {
            "agent_id": self._config.agent_id,
            "platform": platform,
            "firmware_version": firmware_version,
        }

        resp = await self._http.post("/v1/ptv/attest", json=body)
        if resp.status_code != 200:
            raise PtvError(f"Attestation failed: {resp.status_code} {resp.text}")

        data = resp.json()
        return PtvAttestResult(
            attestation={
                "agent_id": data.get("agent_id", ""),
                "platform": data.get("platform", ""),
                "nonce": data.get("nonce", ""),
            },
            tpm_signature=data.get("tpm_signature", ""),
            quote=data.get("quote", ""),
        )

    async def bind(
        self,
        attestation: dict[str, Any],
        tpm_signature: str,
        platform: str,
        firmware_version: str,
        agent_id: str = "",
    ) -> PtvBindResult:
        aid = agent_id or self._config.agent_id
        logger.info("Binding identity for agent: %s", aid)

        body = {
            "agent_id": aid,
            "platform": platform,
            "firmware_version": firmware_version,
            "tpm_signature": tpm_signature,
            "attestation": attestation,
        }

        resp = await self._http.post("/v1/ptv/bind", json=body)
        if resp.status_code != 200:
            raise PtvError(f"Bind failed: {resp.status_code} {resp.text}")

        data = resp.json()
        return PtvBindResult(
            binding_id=data.get("binding_id", ""),
            agent_id=data.get("agent_id", ""),
            platform=data.get("platform", ""),
            transformed_at=data.get("transformed_at", 0),
            expires_at=data.get("expires_at", 0),
        )

    async def verify_binding(self, binding_id: str) -> bool:
        resp = await self._http.get(f"/v1/ptv/verify/{binding_id}")
        if resp.status_code != 200:
            raise PtvError(f"Verify binding failed: {resp.status_code}")
        return resp.json().get("valid", False)

    async def request_approval(
        self,
        action: str,
        reason: str = "",
        risk_level: str = "medium",
    ) -> HitlApproval:
        logger.info("Requesting HITL approval for action: %s", action)

        body = {
            "agent_id": self._config.agent_id,
            "action": action,
            "reason": reason,
            "risk_level": risk_level,
        }

        resp = await self._http.post("/v1/hitl/request", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise HitlError(f"Approval request failed: {resp.status_code} {resp.text}")

        data = resp.json()
        return HitlApproval(
            approval_id=data.get("approval_id", ""),
            agent_id=self._config.agent_id,
            action=action,
            status=data.get("status", "pending"),
        )

    async def decide_approval(
        self,
        approval_id: str,
        approved: bool,
        approver_method: str = "manual",
    ) -> str:
        body = {
            "approval_id": approval_id,
            "approved": approved,
            "approver_method": approver_method,
        }

        resp = await self._http.post(f"/v1/hitl/{approval_id}/decide", json=body)
        if resp.status_code != 200:
            raise HitlError(f"Decision failed: {resp.status_code} {resp.text}")

        return resp.json().get("status", "unknown")

    async def get_approval_status(self, approval_id: str) -> str:
        resp = await self._http.get(f"/v1/hitl/{approval_id}")
        if resp.status_code != 200:
            raise HitlError(f"Approval not found: {approval_id}")
        return resp.json().get("status", "unknown")

    async def list_pending_approvals(self) -> list[dict[str, Any]]:
        resp = await self._http.get("/v1/hitl/pending", params={"agent_id": self._config.agent_id})
        if resp.status_code != 200:
            raise HitlError(f"Pending query failed: {resp.status_code}")
        return resp.json().get("approvals", [])

    async def mcp_initialize(self) -> McpCapabilities:
        logger.info("Initializing MCP connection")

        body = {"jsonrpc": "2.0", "method": "initialize", "id": 1}
        resp = await self._http.post("/v1/mcp", json=body)

        if resp.status_code != 200:
            raise McpError(f"MCP initialize failed: {resp.status_code}")

        result = resp.json().get("result", {})
        caps = result.get("capabilities", {})

        return McpCapabilities(
            protocol_version=result.get("protocolVersion", "unknown"),
            tools="tools" in caps,
            resources="resources" in caps,
            prompts="prompts" in caps,
        )

    async def mcp_list_tools(self) -> list[McpTool]:
        body = {"jsonrpc": "2.0", "method": "tools/list", "id": 2}
        resp = await self._http.post("/v1/mcp", json=body)

        if resp.status_code != 200:
            raise McpError(f"MCP list tools failed: {resp.status_code}")

        result = resp.json().get("result", {})
        tools_arr = result.get("tools", [])

        return [
            McpTool(
                name=t.get("name", ""),
                description=t.get("description"),
                input_schema=t.get("inputSchema"),
            )
            for t in tools_arr
        ]

    async def mcp_call_tool(
        self,
        tool_name: str,
        arguments: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        body: dict[str, Any] = {
            "jsonrpc": "2.0",
            "method": "tools/call",
            "id": 3,
            "params": {"name": tool_name, "arguments": arguments or {}},
        }

        resp = await self._http.post("/v1/mcp", json=body)
        if resp.status_code != 200:
            raise McpError(f"MCP call tool failed: {resp.status_code}")

        return resp.json().get("result", {})

    async def verify_signature(
        self,
        agent_id: str,
        message: bytes,
        signature: bytes,
    ) -> bool:
        body = {
            "agent_id": agent_id,
            "message": base64.b64encode(message).decode(),
            "signature": base64.b64encode(signature).decode(),
        }

        resp = await self._http.post("/v1/verify-signature", json=body)
        if resp.status_code == 404:
            raise VerifyError(f"Agent not found: {agent_id}")
        if resp.status_code != 200:
            raise VerifyError(f"Verify failed: {resp.status_code}")

        return resp.json().get("valid", False)

    async def list_skills(self, category: str = "") -> list[Skill]:
        params = {}
        if category:
            params["category"] = category

        resp = await self._http.get("/v1/skills", params=params)
        if resp.status_code != 200:
            raise SkillError(f"List skills failed: {resp.status_code}")

        skills_data = resp.json().get("skills", [])
        return [Skill(**s) for s in skills_data]

    async def create_skill(
        self,
        name: str,
        description: str = "",
        category: str = "",
        risk_level: str = "medium",
        required_trust_min: float = 0.0,
        required_proficiency: int = 0,
    ) -> Skill:
        body: dict[str, Any] = {
            "name": name,
            "description": description,
            "category": category,
            "risk_level": risk_level,
        }
        if required_trust_min > 0:
            body["required_trust_min"] = required_trust_min
        if required_proficiency > 0:
            body["required_proficiency"] = required_proficiency

        resp = await self._http.post("/v1/skills", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise SkillError(f"Create skill failed: {resp.status_code}")

        return Skill(**resp.json())

    async def assign_skill(self, agent_id: str, skill_id: str, proficiency: int = 1) -> AgentSkill:
        body = {"skill_id": skill_id, "proficiency": proficiency}

        resp = await self._http.post(f"/v1/agents/{agent_id}/skills", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise SkillError(f"Assign skill failed: {resp.status_code}")

        return AgentSkill(**resp.json())

    async def endorse_skill(
        self,
        agent_id: str,
        skill_id: str,
        endorser_type: str = "agent",
        endorser_id: str = "",
        comment: str = "",
    ) -> Endorsement:
        body = {
            "endorser_type": endorser_type,
            "endorser_id": endorser_id,
            "comment": comment,
        }

        resp = await self._http.post(f"/v1/agents/{agent_id}/skills/{skill_id}/endorse", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise SkillError(f"Endorse skill failed: {resp.status_code}")

        return Endorsement(**resp.json())

    async def verify_skill(self, agent_id: str, skill_id: str, verified_by: str = "") -> AgentSkill:
        body = {"verified_by": verified_by}

        resp = await self._http.post(f"/v1/agents/{agent_id}/skills/{skill_id}/verify", json=body)
        if resp.status_code != 200:
            raise SkillError(f"Verify skill failed: {resp.status_code}")

        return AgentSkill(**resp.json())

    async def get_skill_trust(self, agent_id: str) -> list[SkillTrustScore]:
        resp = await self._http.get(f"/v1/agents/{agent_id}/skill-trust")
        if resp.status_code != 200:
            raise SkillError(f"Get skill trust failed: {resp.status_code}")

        scores_data = resp.json().get("scores", [])
        return [SkillTrustScore(**s) for s in scores_data]

    async def issue_token(
        self,
        agent_id: str,
        resource_id: str,
        action: str,
        trust_score: float = 1.0,
        scopes: Optional[list[str]] = None,
        skills: Optional[list[dict[str, Any]]] = None,
        params: Optional[dict[str, Any]] = None,
    ) -> CapabilityToken:
        body: dict[str, Any] = {
            "agent_id": agent_id,
            "resource_id": resource_id,
            "action": action,
            "trust_score": trust_score,
        }
        if scopes:
            body["scopes"] = scopes
        if skills:
            body["skills"] = skills
        if params:
            body["params"] = params

        resp = await self._http.post("/v1/tx/issue", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise TxError(f"Issue token failed: {resp.status_code}")

        return CapabilityToken(**resp.json())

    async def verify_token(self, token_id: str) -> CapabilityToken:
        resp = await self._http.post("/v1/tx/verify", json={"token_id": token_id})
        if resp.status_code != 200:
            raise TxError(f"Verify token failed: {resp.status_code}")

        return CapabilityToken(**resp.json())

    async def revoke_token(self, token_id: str, reason: str = "") -> dict[str, Any]:
        resp = await self._http.post(f"/v1/tx/revoke/{token_id}", json={"reason": reason})
        if resp.status_code != 200:
            raise TxError(f"Revoke token failed: {resp.status_code}")

        return resp.json()

    async def list_revoked_tokens(self) -> list[dict[str, Any]]:
        resp = await self._http.get("/v1/tx/revoked")
        if resp.status_code != 200:
            raise TxError(f"List revoked tokens failed: {resp.status_code}")

        return resp.json().get("tokens", [])

    async def issue_receipt(
        self,
        token_id: str,
        allowed: bool = True,
        trust_score: float = 1.0,
        trust_delta: float = 0.0,
    ) -> TransactionReceipt:
        body = {
            "token_id": token_id,
            "allowed": allowed,
            "trust_score": trust_score,
            "trust_delta": trust_delta,
        }

        resp = await self._http.post("/v1/tx/receipt", json=body)
        if resp.status_code != 200 and resp.status_code != 201:
            raise TxError(f"Issue receipt failed: {resp.status_code}")

        return TransactionReceipt(**resp.json())

    async def verify_receipt(self, receipt: TransactionReceipt) -> bool:
        resp = await self._http.post("/v1/tx/receipt/verify", json=receipt.model_dump())
        if resp.status_code != 200:
            raise TxError(f"Verify receipt failed: {resp.status_code}")

        return resp.json().get("valid", False)

    async def airport_heartbeat(
        self,
        status: str = "online",
        metadata: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        body: dict[str, Any] = {
            "agent_id": self.config.agent_id,
            "status": status,
        }
        if metadata is not None:
            body["metadata"] = json.dumps(metadata)

        resp = await self._http.post("/v1/airport/heartbeat", json=body)
        if resp.status_code != 200:
            raise ConnectError(f"Heartbeat failed: {resp.status_code}")

        return resp.json()

    async def airport_update_profile(
        self,
        description: Optional[str] = None,
        services_offered: Optional[list[str]] = None,
        endpoints: Optional[dict[str, str]] = None,
        tags: Optional[list[str]] = None,
        listed: Optional[bool] = None,
    ) -> dict[str, Any]:
        body: dict[str, Any] = {}
        if description is not None:
            body["description"] = description
        if services_offered is not None:
            body["services_offered"] = services_offered
        if endpoints is not None:
            body["endpoints"] = endpoints
        if tags is not None:
            body["tags"] = tags
        if listed is not None:
            body["listed"] = listed

        resp = await self._http.put(
            f"/v1/airport/agents/{self.config.agent_id}", json=body
        )
        if resp.status_code != 200:
            raise ConnectError(f"Profile update failed: {resp.status_code}")

        return resp.json()

    async def airport_search(
        self,
        capability: Optional[str] = None,
        skill: Optional[str] = None,
        min_trust: Optional[float] = None,
        status: Optional[str] = None,
        tag: Optional[str] = None,
        owner: Optional[str] = None,
        limit: int = 50,
        offset: int = 0,
    ) -> dict[str, Any]:
        params: dict[str, Any] = {"limit": limit, "offset": offset}
        if capability is not None:
            params["capability"] = capability
        if skill is not None:
            params["skill"] = skill
        if min_trust is not None:
            params["min_trust"] = min_trust
        if status is not None:
            params["status"] = status
        if tag is not None:
            params["tag"] = tag
        if owner is not None:
            params["owner"] = owner

        resp = await self._http.get("/v1/airport/agents", params=params)
        if resp.status_code != 200:
            raise DiscoverError(f"Airport search failed: {resp.status_code}")

        return resp.json()

    async def airport_get_profile(self, agent_id: str) -> dict[str, Any]:
        resp = await self._http.get(f"/v1/airport/agents/{agent_id}")
        if resp.status_code != 200:
            raise DiscoverError(f"Agent profile not found: {resp.status_code}")

        return resp.json()

    async def airport_list_online(self) -> dict[str, Any]:
        resp = await self._http.get("/v1/airport/online")
        if resp.status_code != 200:
            raise DiscoverError(f"Online list failed: {resp.status_code}")

        return resp.json()

    async def airport_connections(
        self, agent_id: Optional[str] = None, limit: int = 50
    ) -> dict[str, Any]:
        params: dict[str, Any] = {"limit": limit}
        if agent_id is not None:
            params["agent_id"] = agent_id

        resp = await self._http.get("/v1/airport/connections", params=params)
        if resp.status_code != 200:
            raise ConnectError(f"Connections query failed: {resp.status_code}")

        return resp.json()

    async def close(self) -> None:
        await self._http.aclose()

    async def __aenter__(self) -> "AgentClient":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()