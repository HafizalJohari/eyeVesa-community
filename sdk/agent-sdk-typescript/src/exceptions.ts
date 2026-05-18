export class AgentIDError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'AgentIDError';
  }
}

export class ConnectError extends AgentIDError {}
export class AuthFailedError extends ConnectError {}
export class DiscoverError extends AgentIDError {}
export class InvokeError extends AgentIDError {}
export class NotAuthorizedError extends InvokeError {}
export class HitlRequiredError extends InvokeError {}
export class DelegateError extends AgentIDError {}
export class MaxDepthError extends DelegateError {}
export class PtvError extends AgentIDError {}
export class HitlError extends AgentIDError {}
export class McpError extends AgentIDError {}
export class VerifyError extends AgentIDError {}
export class SkillError extends AgentIDError {}
export class TxError extends AgentIDError {}