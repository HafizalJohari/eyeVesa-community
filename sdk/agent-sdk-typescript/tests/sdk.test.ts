import {
  AgentClient,
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
} from '../src/index';
import type { AgentConfig } from '../src/models';

function makeConfig(): AgentConfig {
  return {
    agentId: crypto.randomUUID(),
    name: 'test-agent',
    owner: 'test-team',
    gatewayEndpoint: 'http://localhost:9443',
  };
}

describe('AgentClient', () => {
  test('initializes with config', () => {
    const config = makeConfig();
    const client = new AgentClient(config);
    expect(client.name).toBe('test-agent');
    expect(client.owner).toBe('test-team');
    expect(client.trustScore).toBe(1.0);
    expect(client.isRegistered).toBe(false);
    expect(client.gatewayEndpoint).toBe('http://localhost:9443');
  });

  test('initializes with api key', () => {
    const config = makeConfig();
    const client = new AgentClient(config, { apiKey: 'test-key' });
    expect(client).toBeDefined();
  });

  test('fromEnv creates client from env vars', () => {
    process.env.AGENT_ID = 'env-id';
    process.env.AGENT_NAME = 'env-agent';
    process.env.AGENT_OWNER = 'env-owner';
    process.env.GATEWAY_ENDPOINT = 'http://gateway:9443';

    const client = AgentClient.fromEnv();
    expect(client.agentId).toBe('env-id');
    expect(client.name).toBe('env-agent');
    expect(client.owner).toBe('env-owner');
    expect(client.gatewayEndpoint).toBe('http://gateway:9443');

    delete process.env.AGENT_ID;
    delete process.env.AGENT_NAME;
    delete process.env.AGENT_OWNER;
    delete process.env.GATEWAY_ENDPOINT;
  });
});

describe('Exception hierarchy', () => {
  test('AuthFailedError extends ConnectError', () => {
    const err = new AuthFailedError('test');
    expect(err).toBeInstanceOf(ConnectError);
    expect(err).toBeInstanceOf(AgentIDError);
    expect(err).toBeInstanceOf(Error);
  });

  test('NotAuthorizedError extends InvokeError', () => {
    const err = new NotAuthorizedError('denied');
    expect(err).toBeInstanceOf(InvokeError);
    expect(err).toBeInstanceOf(AgentIDError);
  });

  test('HitlRequiredError extends InvokeError', () => {
    const err = new HitlRequiredError('need approval');
    expect(err).toBeInstanceOf(InvokeError);
    expect(err).toBeInstanceOf(AgentIDError);
  });

  test('MaxDepthError extends DelegateError', () => {
    const err = new MaxDepthError('too deep');
    expect(err).toBeInstanceOf(DelegateError);
    expect(err).toBeInstanceOf(AgentIDError);
  });

  test('all errors extend AgentIDError', () => {
    const errors = [
      new ConnectError(''),
      new DiscoverError(''),
      new InvokeError(''),
      new DelegateError(''),
      new PtvError(''),
      new HitlError(''),
      new McpError(''),
      new VerifyError(''),
      new SkillError(''),
      new TxError(''),
    ];
    for (const err of errors) {
      expect(err).toBeInstanceOf(AgentIDError);
    }
  });
});