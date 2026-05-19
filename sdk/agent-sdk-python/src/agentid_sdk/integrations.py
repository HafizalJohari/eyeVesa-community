from __future__ import annotations

import json
import logging
from typing import Any, Optional

from .client import AgentClient
from .models import AgentConfig, AgentPassport

logger = logging.getLogger("agentid_sdk.integrations")


class LangGraphIntegration:
    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "langgraph-agent",
        owner: str = "langgraph",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "LangGraphIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    async def get_tools(self) -> list[dict[str, Any]]:
        try:
            tools = await self._client.mcp_list_tools()
        except Exception:
            tools = []
        return [
            {
                "type": "function",
                "function": {
                    "name": t.name,
                    "description": t.description or "",
                    "parameters": t.input_schema or {},
                },
            }
            for t in tools
        ]

    async def call_tool(self, tool_name: str, arguments: dict[str, Any]) -> dict[str, Any]:
        return await self._client.mcp_call_tool(tool_name, arguments)

    async def authorize_and_invoke(
        self,
        resource_id: str,
        action: str,
        params: Optional[dict[str, Any]] = None,
    ) -> dict[str, Any]:
        result = await self._client.invoke(resource_id, action, params)
        return result.data

    @property
    def client(self) -> AgentClient:
        return self._client


class CrewAIIntegration:
    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "crewai-agent",
        owner: str = "crewai",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "CrewAIIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    async def create_tool_definition(self, tool_name: str, description: str = "") -> dict[str, Any]:
        return {
            "name": tool_name,
            "description": description or f"AgentID-gated tool: {tool_name}",
            "func": self._make_tool_callable(tool_name),
        }

    def _make_tool_callable(self, tool_name: str):
        async def tool_func(**kwargs: Any) -> Any:
            return await self._client.mcp_call_tool(tool_name, kwargs)
        tool_func.__name__ = tool_name
        return tool_func

    @property
    def client(self) -> AgentClient:
        return self._client


class AutoGenIntegration:
    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "autogen-agent",
        owner: str = "autogen",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "AutoGenIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    async def get_function_definitions(self) -> list[dict[str, Any]]:
        try:
            tools = await self._client.mcp_list_tools()
        except Exception:
            tools = []
        return [
            {
                "name": t.name,
                "description": t.description or "",
                "parameters": t.input_schema or {"type": "object", "properties": {}},
            }
            for t in tools
        ]

    async def execute_function(self, name: str, arguments: dict[str, Any]) -> Any:
        return await self._client.mcp_call_tool(name, arguments)

    @property
    def client(self) -> AgentClient:
        return self._client


EYEVESA_TOOL_DEFINITIONS = [
    {
        "name": "eyevesa_read",
        "description": (
            "Read data from an eyeVesa-gated resource. "
            "Authorization is checked via OPA policy. "
            "High-risk reads may require HITL approval."
        ),
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
        "description": (
            "Write data to an eyeVesa-gated resource. "
            "Writes are typically higher risk and may require HITL approval."
        ),
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
        "description": (
            "Proactively request human-in-the-loop approval for an action. "
            "Use for sensitive operations like bank transfers or data deletion."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "action": {"type": "string", "description": "The action requiring approval"},
                "reason": {"type": "string", "description": "Why this action needs approval"},
                "risk_level": {
                    "type": "string",
                    "enum": ["low", "medium", "high", "critical"],
                    "description": "Risk level of the action",
                },
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
                "capability": {
                    "type": "string",
                    "description": "Filter by capability (e.g., 'mcp', 'database')",
                },
            },
        },
    },
    {
        "name": "eyevesa_delegate",
        "description": "Delegate scoped permissions to another agent. Maximum delegation depth is 3.",
        "input_schema": {
            "type": "object",
            "properties": {
                "delegatee_id": {
                    "type": "string",
                    "description": "The agent ID to delegate to",
                },
                "scope": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "List of permissions to delegate",
                },
                "reason": {
                    "type": "string",
                    "description": "Reason for delegation",
                },
            },
            "required": ["delegatee_id", "scope"],
        },
    },
    {
        "name": "eyevesa_skill_trust",
        "description": (
            "Check per-skill trust scores for an agent. "
            "Use to assess whether an agent has sufficient trust for a particular skill."
        ),
        "input_schema": {
            "type": "object",
            "properties": {
                "agent_id": {
                    "type": "string",
                    "description": "The agent ID to check trust scores for",
                },
            },
            "required": ["agent_id"],
        },
    },
]


class ClaudeIntegration:
    """Integration with Anthropic Claude (Messages API with tool_use)."""

    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "claude-agent",
        owner: str = "claude",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "ClaudeIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    def get_tool_definitions(self) -> list[dict[str, Any]]:
        """Get eyeVesa tool definitions in Anthropic Claude tool format."""
        return EYEVESA_TOOL_DEFINITIONS

    async def handle_tool_call(self, tool_name: str, tool_input: dict[str, Any]) -> str:
        """Route a Claude tool_use call through eyeVesa."""
        from .exceptions import NotAuthorizedError, HitlRequiredError

        try:
            if tool_name == "eyevesa_read":
                result = await self._client.invoke(
                    resource_id=tool_input["resource_id"],
                    tool="read",
                    params={"query": tool_input.get("query", "")},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif tool_name == "eyevesa_write":
                result = await self._client.invoke(
                    resource_id=tool_input["resource_id"],
                    tool="write",
                    params={"data": tool_input["data"]},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif tool_name == "eyevesa_request_approval":
                approval = await self._client.request_approval(
                    action=tool_input["action"],
                    reason=tool_input["reason"],
                    risk_level=tool_input["risk_level"],
                )
                return json.dumps({
                    "approval_id": approval.approval_id,
                    "status": approval.status,
                })

            elif tool_name == "eyevesa_discover":
                capability = tool_input.get("capability", "mcp")
                tools_info = await self._client.discover(capability)
                return json.dumps([t.model_dump() for t in tools_info])

            elif tool_name == "eyevesa_delegate":
                result = await self._client.delegate(
                    delegatee_id=tool_input["delegatee_id"],
                    scope=tool_input["scope"],
                    reason=tool_input.get("reason", ""),
                )
                return json.dumps({
                    "delegation_id": result.delegation_id,
                    "status": result.status,
                })

            elif tool_name == "eyevesa_skill_trust":
                scores = await self._client.get_skill_trust(tool_input["agent_id"])
                return json.dumps([s.model_dump() for s in scores])

            return json.dumps({"error": f"Unknown tool: {tool_name}"})

        except NotAuthorizedError as e:
            return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
        except HitlRequiredError as e:
            return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})

    @property
    def client(self) -> AgentClient:
        return self._client


class OpenAIIntegration:
    """Integration with OpenAI (Responses API with function_call and computer_use)."""

    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "openai-agent",
        owner: str = "openai",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "OpenAIIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    def get_function_tools(self) -> list[dict[str, Any]]:
        """Get eyeVesa tools in OpenAI function calling format."""
        return [
            {"type": "function", "function": tool}
            for tool in EYEVESA_TOOL_DEFINITIONS
        ]

    def get_computer_and_function_tools(self) -> list[dict[str, Any]]:
        """Get both computer tool and eyeVesa function tools for combined use."""
        return [{"type": "computer"}] + self.get_function_tools()

    async def handle_function_call(self, function_name: str, arguments: dict[str, Any]) -> str:
        """Route an OpenAI function_call through eyeVesa."""
        from .exceptions import NotAuthorizedError, HitlRequiredError

        try:
            if function_name == "eyevesa_read":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="read",
                    params={"query": arguments.get("query", "")},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif function_name == "eyevesa_write":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="write",
                    params={"data": arguments["data"]},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif function_name == "eyevesa_request_approval":
                approval = await self._client.request_approval(
                    action=arguments["action"],
                    reason=arguments["reason"],
                    risk_level=arguments["risk_level"],
                )
                return json.dumps({
                    "approval_id": approval.approval_id,
                    "status": approval.status,
                })

            elif function_name == "eyevesa_discover":
                capability = arguments.get("capability", "mcp")
                tools_info = await self._client.discover(capability)
                return json.dumps([t.model_dump() for t in tools_info])

            elif function_name == "eyevesa_delegate":
                result = await self._client.delegate(
                    delegatee_id=arguments["delegatee_id"],
                    scope=arguments["scope"],
                    reason=arguments.get("reason", ""),
                )
                return json.dumps({
                    "delegation_id": result.delegation_id,
                    "status": result.status,
                })

            elif function_name == "eyevesa_skill_trust":
                scores = await self._client.get_skill_trust(arguments["agent_id"])
                return json.dumps([s.model_dump() for s in scores])

            return json.dumps({"error": f"Unknown function: {function_name}"})

        except NotAuthorizedError as e:
            return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
        except HitlRequiredError as e:
            return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})

    @property
    def client(self) -> AgentClient:
        return self._client


class HermesIntegration:
    """Integration with Hermes agent framework.

    Hermes uses a task/action model where agents declare capabilities
    as structured tool specs and maintain presence at the Airport
    via periodic heartbeat.

    Usage:
        hermes = HermesIntegration.from_config(gateway_endpoint="http://localhost:9443")
        await hermes.connect()
        tools = hermes.get_tool_specs()
        result = await hermes.handle_action("eyevesa_read", {"resource_id": "doc-001"})
        await hermes.heartbeat("online")
        agents = await hermes.discover_peers(capability="mcp")
    """

    def __init__(self, client: AgentClient) -> None:
        self._client = client
        self._heartbeat_status = "idle"

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "hermes-agent",
        owner: str = "hermes",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "HermesIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    def get_tool_specs(self) -> list[dict[str, Any]]:
        """Get eyeVesa tool specs in Hermes action format.

        Hermes tools include an 'action_type' field and wrap
        the standard eyeVesa definitions with additional metadata
        for the Hermes task planning loop.
        """
        return [
            {**tool, "action_type": "eyevesa_gateway"}
            for tool in EYEVESA_TOOL_DEFINITIONS
        ]

    async def handle_action(self, action_name: str, action_input: dict[str, Any]) -> str:
        """Route a Hermes action through eyeVesa.

        In Hermes, actions are dispatched by the task planner and
        routed through this handler which gatekeeps via eyeVesa authz.
        """
        from .exceptions import NotAuthorizedError, HitlRequiredError

        try:
            if action_name == "eyevesa_read":
                result = await self._client.invoke(
                    resource_id=action_input["resource_id"],
                    tool="read",
                    params={"query": action_input.get("query", "")},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif action_name == "eyevesa_write":
                result = await self._client.invoke(
                    resource_id=action_input["resource_id"],
                    tool="write",
                    params={"data": action_input["data"]},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif action_name == "eyevesa_request_approval":
                approval = await self._client.request_approval(
                    action=action_input["action"],
                    reason=action_input["reason"],
                    risk_level=action_input["risk_level"],
                )
                return json.dumps({
                    "approval_id": approval.approval_id,
                    "status": approval.status,
                })

            elif action_name == "eyevesa_discover":
                capability = action_input.get("capability", "mcp")
                tools_info = await self._client.discover(capability)
                return json.dumps([t.model_dump() for t in tools_info])

            elif action_name == "eyevesa_delegate":
                result = await self._client.delegate(
                    delegatee_id=action_input["delegatee_id"],
                    scope=action_input["scope"],
                    reason=action_input.get("reason", ""),
                )
                return json.dumps({
                    "delegation_id": result.delegation_id,
                    "status": result.status,
                })

            elif action_name == "eyevesa_skill_trust":
                scores = await self._client.get_skill_trust(action_input["agent_id"])
                return json.dumps([s.model_dump() for s in scores])

            return json.dumps({"error": f"Unknown action: {action_name}"})

        except NotAuthorizedError as e:
            return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
        except HitlRequiredError as e:
            return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})

    async def heartbeat(self, status: str = "online") -> dict[str, Any]:
        """Send heartbeat to the Airport to maintain presence.

        Hermes agents periodically call this to announce their
        availability. Acceptable statuses: online, offline, busy, idle.
        """
        self._heartbeat_status = status
        return await self._client.airport_heartbeat(status=status)

    async def update_airport_profile(
        self,
        description: Optional[str] = None,
        tags: Optional[list[str]] = None,
        listed: Optional[bool] = None,
    ) -> dict[str, Any]:
        """Update this agent's Airport profile for discoverability."""
        return await self._client.airport_update_profile(
            description=description,
            tags=tags,
            listed=listed,
        )

    async def discover_peers(
        self,
        capability: Optional[str] = None,
        status: Optional[str] = None,
        tag: Optional[str] = None,
        min_trust: Optional[float] = None,
    ) -> dict[str, Any]:
        """Search the Airport for other agents matching criteria."""
        return await self._client.airport_search(
            capability=capability,
            status=status,
            tag=tag,
            min_trust=min_trust,
        )

    async def list_online_peers(self) -> dict[str, Any]:
        """List all agents currently online at the Airport."""
        return await self._client.airport_list_online()

    async def get_peer_profile(self, agent_id: str) -> dict[str, Any]:
        """Get another agent's Airport profile by ID."""
        return await self._client.airport_get_profile(agent_id)

    async def get_connections(
        self, agent_id: Optional[str] = None, limit: int = 50
    ) -> dict[str, Any]:
        """Get connection history for an agent at the Airport."""
        return await self._client.airport_connections(agent_id=agent_id, limit=limit)

    async def register_gateway(
        self,
        name: str,
        public_key: str,
        endpoint: str,
        trust_domain: str = "",
        peer_type: str = "remote",
    ) -> dict[str, Any]:
        """Register this agent's local gateway with Central Airport.

        This is the 'embassy registration' step — your gateway must be registered
        before your passport is accepted at the Central Airport.

        peer_type:
            'self'     = this gateway's own registration
            'domestic' = a peer on the same local network
            'remote'   = a peer at a different Central Airport (default)
        """
        result = await self._client.federation_register_peer(
            name=name,
            public_key=public_key,
            endpoint=endpoint,
            trust_domain=trust_domain or name,
            peer_type=peer_type,
        )
        return result.model_dump()

    async def issue_passport(self) -> AgentPassport:
        """Generate a passport for this agent, signed by the local gateway.

        The passport is the credential that the Central Airport verifies
        before allowing the agent to enter.
        """
        return await self._client.issue_passport()

    async def sync_to_central(
        self,
        central_endpoint: str,
        description: str = "",
        tags: Optional[list[str]] = None,
        scope: str = "international",
    ) -> dict[str, Any]:
        """Sync this agent to a Central Airport.

        Steps:
        1. Register this agent's gateway with Central Airport
        2. Issue a passport (signed by local gateway)
        3. Present passport to Central Airport + create profile

        Requires: central_endpoint (URL of the Central Airport gateway)

        scope:
            'domestic'      = agent belongs to the same organization/network as the Central Airport
            'international' = agent comes from a different gateway/organization (default)
        """
        import httpx as _httpx

        passport = await self._client.issue_passport()

        gateway_pub_key = self._client.public_key_base64

        reg_body = {
            "name": self._client.name,
            "public_key": gateway_pub_key,
            "endpoint": self._client.gateway_endpoint,
            "trust_domain": self._client.owner,
        }

        headers = {"Content-Type": "application/json"}
        api_key = self._client._api_key
        jwt_token = self._client._jwt_token
        if api_key:
            headers["X-API-Key"] = api_key
        if jwt_token:
            headers["Authorization"] = f"Bearer {jwt_token}"

        async with _httpx.AsyncClient(
            base_url=central_endpoint.rstrip("/"),
            headers=headers,
            timeout=30.0,
        ) as central_client:
            reg_resp = await central_client.post("/v1/federation/register", json=reg_body)
            if reg_resp.status_code not in (200, 201):
                raise Exception(f"Gateway registration failed: {reg_resp.status_code} {reg_resp.text}")

            reg_data = reg_resp.json()
            gateway_id = reg_data.get("gateway_id", "")

            passport.gateway_id = gateway_id

            from datetime import datetime as _dt
            import base64 as _b64

            payload = json.dumps({
                "agent_id": passport.agent_id,
                "agent_public_key": passport.agent_public_key,
                "gateway_id": passport.gateway_id,
                "issued_at": passport.issued_at,
            }, sort_keys=True)

            from nacl.signing import SigningKey as _SK
            sig_bytes = self._client._signing_key.sign(payload.encode()).signature
            passport.gateway_signature = _b64.b64encode(sig_bytes).decode()

            sync_body = {
                "passport": passport.model_dump(),
                "name": self._client.name,
                "owner": self._client.owner,
                "trust_score": self._client.trust_score,
                "capabilities": ["mcp"],
                "allowed_tools": ["read"],
                "description": description,
                "tags": tags or [],
                "scope": scope,
            }

            sync_resp = await central_client.post("/v1/federation/agents/sync", json=sync_body)
            if sync_resp.status_code not in (200, 201):
                raise Exception(f"Agent sync failed: {sync_resp.status_code} {sync_resp.text}")

            return sync_resp.json()

    async def federated_heartbeat(
        self,
        central_endpoint: str,
        status: str = "online",
    ) -> dict[str, Any]:
        """Send heartbeat to Central Airport."""
        import httpx as _httpx

        headers = {"Content-Type": "application/json"}
        api_key = self._client._api_key
        jwt_token = self._client._jwt_token
        if api_key:
            headers["X-API-Key"] = api_key
        if jwt_token:
            headers["Authorization"] = f"Bearer {jwt_token}"

        async with _httpx.AsyncClient(
            base_url=central_endpoint.rstrip("/"),
            headers=headers,
            timeout=30.0,
        ) as central_client:
            resp = await central_client.post("/v1/federation/heartbeat", json={
                "agent_id": self._client.agent_id,
                "gateway_id": "",
                "status": status,
            })
            if resp.status_code != 200:
                raise Exception(f"Federated heartbeat failed: {resp.status_code}")
            return resp.json()

    async def discover_federated_agents(
        self,
        central_endpoint: str,
        tag: Optional[str] = None,
        status: Optional[str] = None,
        min_trust: Optional[float] = None,
    ) -> dict[str, Any]:
        """Search for agents at the Central Airport."""
        import httpx as _httpx

        headers = {"Content-Type": "application/json"}
        api_key = self._client._api_key
        jwt_token = self._client._jwt_token
        if api_key:
            headers["X-API-Key"] = api_key
        if jwt_token:
            headers["Authorization"] = f"Bearer {jwt_token}"

        async with _httpx.AsyncClient(
            base_url=central_endpoint.rstrip("/"),
            headers=headers,
            timeout=30.0,
        ) as central_client:
            params: dict[str, Any] = {}
            if tag:
                params["tag"] = tag
            if status:
                params["status"] = status
            if min_trust is not None:
                params["min_trust"] = min_trust

            resp = await central_client.get("/v1/federation/agents", params=params)
            if resp.status_code != 200:
                raise Exception(f"Federated search failed: {resp.status_code}")
            return resp.json()

    @property
    def client(self) -> AgentClient:
        return self._client

    @property
    def heartbeat_status(self) -> str:
        return self._heartbeat_status


class OpenClawIntegration:
    """Integration with OpenClaw agent framework.

    OpenClaw uses a tool registry pattern where tools are discovered
    dynamically and registered with the agent's runtime. This integration
    provides tool registration specs and an execution dispatcher.

    Usage:
        claw = OpenClawIntegration.from_config(gateway_endpoint="http://localhost:9443")
        await claw.connect()
        specs = claw.get_tool_specs()
        result = await claw.execute_tool("eyevesa_read", {"resource_id": "doc-001"})
        profile = await claw.register_at_airport(
            description="OpenClaw data processor",
            tags=["openclaw", "data", "research"],
        )
    """

    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "openclaw-agent",
        owner: str = "openclaw",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "OpenClawIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    def get_tool_specs(self) -> list[dict[str, Any]]:
        """Get eyeVesa tool specs in OpenClaw registry format.

        OpenClaw tools carry a 'handler' field pointing to the
        dispatch method and a 'source' field identifying the gateway.
        """
        return [
            {
                **tool,
                "handler": "eyevesa_gateway",
                "source": "eyevesa",
                "permissions": ["read", "write"],
            }
            for tool in EYEVESA_TOOL_DEFINITIONS
        ]

    async def execute_tool(self, tool_name: str, arguments: dict[str, Any]) -> str:
        """Execute a tool call through the OpenClaw dispatcher routed via eyeVesa."""
        from .exceptions import NotAuthorizedError, HitlRequiredError

        try:
            if tool_name == "eyevesa_read":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="read",
                    params={"query": arguments.get("query", "")},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif tool_name == "eyevesa_write":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="write",
                    params={"data": arguments["data"]},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif tool_name == "eyevesa_request_approval":
                approval = await self._client.request_approval(
                    action=arguments["action"],
                    reason=arguments["reason"],
                    risk_level=arguments["risk_level"],
                )
                return json.dumps({
                    "approval_id": approval.approval_id,
                    "status": approval.status,
                })

            elif tool_name == "eyevesa_discover":
                capability = arguments.get("capability", "mcp")
                tools_info = await self._client.discover(capability)
                return json.dumps([t.model_dump() for t in tools_info])

            elif tool_name == "eyevesa_delegate":
                result = await self._client.delegate(
                    delegatee_id=arguments["delegatee_id"],
                    scope=arguments["scope"],
                    reason=arguments.get("reason", ""),
                )
                return json.dumps({
                    "delegation_id": result.delegation_id,
                    "status": result.status,
                })

            elif tool_name == "eyevesa_skill_trust":
                scores = await self._client.get_skill_trust(arguments["agent_id"])
                return json.dumps([s.model_dump() for s in scores])

            return json.dumps({"error": f"Unknown tool: {tool_name}"})

        except NotAuthorizedError as e:
            return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
        except HitlRequiredError as e:
            return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})

    async def register_at_airport(
        self,
        description: str = "",
        tags: Optional[list[str]] = None,
        listed: bool = True,
    ) -> dict[str, Any]:
        """Register this agent at the Airport with a profile and set online."""
        await self._client.airport_heartbeat(status="online")
        return await self._client.airport_update_profile(
            description=description,
            tags=tags or ["openclaw"],
            listed=listed,
        )

    async def discover_agents(
        self,
        capability: Optional[str] = None,
        tag: Optional[str] = None,
        min_trust: Optional[float] = None,
    ) -> dict[str, Any]:
        """Discover other agents at the Airport."""
        return await self._client.airport_search(
            capability=capability,
            tag=tag,
            min_trust=min_trust,
        )

    async def list_online_agents(self) -> dict[str, Any]:
        """List all currently online agents."""
        return await self._client.airport_list_online()

    @property
    def client(self) -> AgentClient:
        return self._client


class NanoClawIntegration:
    """Integration with NanoClaw agent framework.

    NanoClaw is a lightweight claw-based agent framework that uses
    compact tool definitions with guardrails metadata and trust-gated
    execution. This integration provides NanoClaw-compatible function
    specs, execution routing, and Airport-based agent discovery with
    trust verification.

    Usage:
        claw = NanoClawIntegration.from_config(gateway_endpoint="http://localhost:9443")
        await claw.connect()
        funcs = claw.get_function_definitions()
        result = await claw.execute_function("eyevesa_read", {"resource_id": "doc-001"})
        trust_ok = await claw.check_trust("agent-456", min_trust=0.7)
    """

    def __init__(self, client: AgentClient) -> None:
        self._client = client

    @classmethod
    def from_config(
        cls,
        gateway_endpoint: str = "http://localhost:9443",
        agent_name: str = "nanoclaw-agent",
        owner: str = "nanoclaw",
        api_key: Optional[str] = None,
        jwt_token: Optional[str] = None,
    ) -> "NanoClawIntegration":
        config = AgentConfig(
            agent_id="",
            name=agent_name,
            owner=owner,
            gateway_endpoint=gateway_endpoint,
        )
        client = AgentClient(config, api_key=api_key, jwt_token=jwt_token)
        return cls(client)

    async def connect(self) -> None:
        await self._client.connect()

    def get_function_definitions(self) -> list[dict[str, Any]]:
        """Get eyeVesa function definitions in NanoClaw-compatible format.

        NanoClaw functions include a 'guardrails' metadata field for
        integration with NanoClaw guardrails and a 'trust_requirement'
        indicating the minimum trust score needed to execute.
        """
        return [
            {
                "name": tool["name"],
                "description": tool["description"],
                "parameters": tool["input_schema"],
                "guardrails": {
                    "input_validation": True,
                    "output_validation": True,
                },
                "trust_requirement": 0.5 if "read" in tool["name"] else 0.7,
            }
            for tool in EYEVESA_TOOL_DEFINITIONS
        ]

    async def execute_function(self, function_name: str, arguments: dict[str, Any]) -> str:
        """Route a NanoClaw function call through eyeVesa with trust gating."""
        from .exceptions import NotAuthorizedError, HitlRequiredError

        try:
            if function_name == "eyevesa_read":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="read",
                    params={"query": arguments.get("query", "")},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif function_name == "eyevesa_write":
                result = await self._client.invoke(
                    resource_id=arguments["resource_id"],
                    tool="write",
                    params={"data": arguments["data"]},
                )
                return json.dumps({
                    "success": result.success,
                    "data": result.data,
                    "trust_score": result.trust_score,
                })

            elif function_name == "eyevesa_request_approval":
                approval = await self._client.request_approval(
                    action=arguments["action"],
                    reason=arguments["reason"],
                    risk_level=arguments["risk_level"],
                )
                return json.dumps({
                    "approval_id": approval.approval_id,
                    "status": approval.status,
                })

            elif function_name == "eyevesa_discover":
                capability = arguments.get("capability", "mcp")
                tools_info = await self._client.discover(capability)
                return json.dumps([t.model_dump() for t in tools_info])

            elif function_name == "eyevesa_delegate":
                result = await self._client.delegate(
                    delegatee_id=arguments["delegatee_id"],
                    scope=arguments["scope"],
                    reason=arguments.get("reason", ""),
                )
                return json.dumps({
                    "delegation_id": result.delegation_id,
                    "status": result.status,
                })

            elif function_name == "eyevesa_skill_trust":
                scores = await self._client.get_skill_trust(arguments["agent_id"])
                return json.dumps([s.model_dump() for s in scores])

            return json.dumps({"error": f"Unknown function: {function_name}"})

        except NotAuthorizedError as e:
            return json.dumps({"error": "NOT_AUTHORIZED", "reason": str(e)})
        except HitlRequiredError as e:
            return json.dumps({"error": "HITL_REQUIRED", "reason": str(e)})

    async def check_trust(self, agent_id: str, min_trust: float = 0.5) -> bool:
        """Check if an agent's trust score meets the minimum threshold.

        Uses the Airport profile to get the agent's current trust score
        and compares against the required minimum.
        """
        profile = await self._client.airport_get_profile(agent_id)
        trust_score = profile.get("trust_score", 0.0)
        return trust_score >= min_trust

    async def heartbeat(self, status: str = "online") -> dict[str, Any]:
        """Send heartbeat to maintain Airport presence."""
        return await self._client.airport_heartbeat(status=status)

    async def update_airport_profile(
        self,
        description: Optional[str] = None,
        tags: Optional[list[str]] = None,
        listed: Optional[bool] = None,
    ) -> dict[str, Any]:
        """Update this agent's Airport profile."""
        return await self._client.airport_update_profile(
            description=description,
            tags=tags,
            listed=listed,
        )

    async def discover_agents(
        self,
        capability: Optional[str] = None,
        skill: Optional[str] = None,
        min_trust: Optional[float] = None,
    ) -> dict[str, Any]:
        """Search the Airport for agents matching criteria with optional trust gating."""
        return await self._client.airport_search(
            capability=capability,
            skill=skill,
            min_trust=min_trust,
        )

    async def list_online_agents(self) -> dict[str, Any]:
        """List all agents currently online at the Airport."""
        return await self._client.airport_list_online()

    async def get_agent_profile(self, agent_id: str) -> dict[str, Any]:
        """Get another agent's Airport profile with trust details."""
        return await self._client.airport_get_profile(agent_id)

    @property
    def client(self) -> AgentClient:
        return self._client