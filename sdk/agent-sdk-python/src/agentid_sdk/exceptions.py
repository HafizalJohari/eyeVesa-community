from __future__ import annotations


class AgentIDError(Exception):
    pass


class ConnectError(AgentIDError):
    pass


class AuthFailedError(ConnectError):
    pass


class DiscoverError(AgentIDError):
    pass


class InvokeError(AgentIDError):
    pass


class NotAuthorizedError(InvokeError):
    pass


class HitlRequiredError(InvokeError):
    pass


class DelegateError(AgentIDError):
    pass


class MaxDepthError(DelegateError):
    pass


class PtvError(AgentIDError):
    pass


class HitlError(AgentIDError):
    pass


class McpError(AgentIDError):
    pass


class VerifyError(AgentIDError):
    pass


class SkillError(AgentIDError):
    pass


class TxError(AgentIDError):
    pass