from __future__ import annotations

import json
from uuid import uuid4

import pytest

from agentid_sdk import (
    AgentClient,
    AgentConfig,
    AuthorizeResult,
    DelegateResult,
    HitlApproval,
    InvokeResult,
    McpCapabilities,
    McpTool,
    PtvAttestResult,
    PtvBindResult,
    Skill,
    SkillTrustScore,
    ToolInfo,
)
from agentid_sdk.exceptions import (
    AuthFailedError,
    HitlRequiredError,
    MaxDepthError,
    NotAuthorizedError,
)


def test_agent_config_creation():
    config = AgentConfig(
        agent_id=str(uuid4()),
        name="test-agent",
        owner="test-team",
        gateway_endpoint="http://localhost:9443",
    )
    assert config.name == "test-agent"
    assert config.owner == "test-team"
    assert config.gateway_endpoint == "http://localhost:9443"


def test_agent_config_serialization():
    config = AgentConfig(
        agent_id=str(uuid4()),
        name="test-agent",
        owner="test-team",
        gateway_endpoint="http://localhost:9443",
    )
    json_str = config.model_dump_json()
    restored = AgentConfig.model_validate_json(json_str)
    assert restored.name == config.name
    assert restored.agent_id == config.agent_id


def test_agent_client_initialization():
    config = AgentConfig(
        agent_id=str(uuid4()),
        name="test-agent",
        owner="test-team",
        gateway_endpoint="http://localhost:9443",
    )
    client = AgentClient(config)
    assert client.name == "test-agent"
    assert client.owner == "test-team"
    assert client.trust_score == 1.0
    assert not client.is_registered
    assert client.gateway_endpoint == "http://localhost:9443"


def test_agent_client_with_signing_key():
    from nacl.signing import SigningKey

    config = AgentConfig(
        agent_id=str(uuid4()),
        name="test-agent",
        owner="test-team",
        gateway_endpoint="http://localhost:9443",
    )
    key = SigningKey.generate()
    client = AgentClient(config, signing_key=key)
    assert client._signing_key == key


def test_invoke_result_deserialization():
    data = {"success": True, "data": {"result": "ok"}, "trust_score": 0.95}
    result = InvokeResult(**data)
    assert result.success
    assert abs(result.trust_score - 0.95) < 1e-9
    assert result.data == {"result": "ok"}


def test_authorize_result_deserialization():
    data = {"allowed": True, "requires_hitl": False, "reason": "ok", "trust_delta": 0.1}
    result = AuthorizeResult(**data)
    assert result.allowed
    assert not result.requires_hitl
    assert abs(result.trust_delta - 0.1) < 1e-9


def test_delegate_result_deserialization():
    data = {"delegation_id": str(uuid4()), "status": "active"}
    result = DelegateResult(**data)
    assert result.status == "active"


def test_hitl_approval_deserialization():
    data = {
        "approval_id": str(uuid4()),
        "agent_id": "agent-1",
        "action": "bank_transfer",
        "status": "pending",
    }
    result = HitlApproval(**data)
    assert result.status == "pending"
    assert result.action == "bank_transfer"


def test_ptv_attest_result_deserialization():
    data = {
        "attestation": {"agent_id": "agent-1"},
        "tpm_signature": "c2ln",
        "quote": "cXVvdGU=",
    }
    result = PtvAttestResult(**data)
    assert result.tpm_signature == "c2ln"
    assert result.quote == "cXVvdGU="


def test_ptv_bind_result_deserialization():
    data = {
        "binding_id": str(uuid4()),
        "agent_id": "agent-1",
        "platform": "linux-tpm2",
        "transformed_at": 1700000000,
        "expires_at": 1700086400,
    }
    result = PtvBindResult(**data)
    assert result.agent_id == "agent-1"
    assert result.platform == "linux-tpm2"


def test_tool_info_deserialization():
    data = {
        "name": "get_weather",
        "description": "Get weather info",
        "resource_id": str(uuid4()),
        "parameters": {"type": "object"},
    }
    result = ToolInfo(**data)
    assert result.name == "get_weather"


def test_mcp_capabilities_deserialization():
    data = {"protocol_version": "2024-11-05", "tools": True, "resources": False, "prompts": True}
    result = McpCapabilities(**data)
    assert result.protocol_version == "2024-11-05"
    assert result.tools is True
    assert result.prompts is True


def test_mcp_tool_deserialization():
    data = {"name": "read", "description": "Read data", "input_schema": {"type": "object"}}
    result = McpTool(**data)
    assert result.name == "read"
    assert result.input_schema is not None


def test_skill_deserialization():
    data = {
        "skill_id": str(uuid4()),
        "name": "python-coding",
        "description": "Python programming",
        "category": "programming",
        "risk_level": "low",
        "required_trust_min": 0.5,
        "required_proficiency": 3,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
    }
    result = Skill(**data)
    assert result.name == "python-coding"
    assert result.required_proficiency == 3


def test_skill_trust_score_deserialization():
    data = {
        "agent_id": "agent-1",
        "skill_id": str(uuid4()),
        "skill_name": "python-coding",
        "trust_score": 0.85,
        "updated_at": "2026-01-01T00:00:00Z",
    }
    result = SkillTrustScore(**data)
    assert abs(result.trust_score - 0.85) < 1e-9


def test_from_env(monkeypatch):
    monkeypatch.setenv("AGENT_ID", "test-id")
    monkeypatch.setenv("AGENT_NAME", "env-agent")
    monkeypatch.setenv("AGENT_OWNER", "env-owner")
    monkeypatch.setenv("GATEWAY_ENDPOINT", "http://gateway:9443")

    client = AgentClient.from_env()
    assert client.agent_id == "test-id"
    assert client.name == "env-agent"
    assert client.owner == "env-owner"
    assert client.gateway_endpoint == "http://gateway:9443"


def test_exception_hierarchy():
    from agentid_sdk.exceptions import AgentIDError

    assert issubclass(AuthFailedError, AgentIDError)
    assert issubclass(NotAuthorizedError, AgentIDError)
    assert issubclass(HitlRequiredError, AgentIDError)
    assert issubclass(MaxDepthError, AgentIDError)