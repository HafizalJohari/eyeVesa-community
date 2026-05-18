// AgentID Gateway Load Test Suite
// Target: 2000+ RPS across all critical endpoints
// Usage: k6 run load-test.js --env BASE_URL=http://localhost:8080 --env PROXY_URL=http://localhost:9443
// Install: https://k6.io/docs/get-started/installation/

import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const tokenIssueTrend = new Trend('tx_issue_duration', true);
const tokenVerifyTrend = new Trend('tx_verify_duration', true);
const authzTrend = new Trend('authz_duration', true);
const mcpTrend = new Trend('mcp_duration', true);

export const options = {
  scenarios: {
    steady_state: {
      executor: 'constant-arrival-rate',
      rate: 2000,
      timeUnit: '1s',
      duration: '60s',
      preAllocatedVUs: 100,
      maxVUs: 500,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.05'],
    tx_issue_duration: ['p(95)<200'],
    tx_verify_duration: ['p(95)<100'],
    authz_duration: ['p(95)<200'],
    mcp_duration: ['p(95)<300'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const PROXY_URL = __ENV.PROXY_URL || 'http://localhost:9443';

export default function () {
  const scenario = Math.random();

  if (scenario < 0.3) {
    healthCheck();
  } else if (scenario < 0.5) {
    authorizeFlow();
  } else if (scenario < 0.65) {
    mcpInitialize();
  } else if (scenario < 0.8) {
    transactionIssue();
  } else if (scenario < 0.9) {
    transactionVerify();
  } else {
    agentRegistration();
  }

  sleep(0.01);
}

function healthCheck() {
  const res = http.get(`${PROXY_URL}/health`);
  check(res, { 'health ok': (r) => r.status === 200 && r.body === 'ok' }) || errorRate.add(1);
}

function authorizeFlow() {
  const payload = JSON.stringify({
    agent_id: 'load-test-agent',
    action: 'read',
    resource_id: 'doc-load-test',
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const start = Date.now();
  const res = http.post(`${PROXY_URL}/v1/auth`, payload, params);
  authzTrend.add(Date.now() - start);

  check(res, { 'authz response': (r) => r.status === 200 }) || errorRate.add(1);
}

function mcpInitialize() {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    method: 'initialize',
    id: 'load-test',
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const start = Date.now();
  const res = http.post(`${PROXY_URL}/v1/mcp`, payload, params);
  mcpTrend.add(Date.now() - start);

  check(res, { 'mcp init': (r) => r.status === 200 }) || errorRate.add(1);
}

function transactionIssue() {
  const payload = JSON.stringify({
    agent_id: 'load-test-agent',
    resource_id: 'load-test-resource',
    action: 'read',
    scopes: ['read'],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/v1/tx/issue`, payload, params);
  tokenIssueTrend.add(Date.now() - start);

  check(res, { 'tx issue': (r) => r.status === 200 }) || errorRate.add(1);
}

function transactionVerify() {
  const payload = JSON.stringify({
    token: { jti: 'load-test-placeholder' },
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/v1/tx/verify`, payload, params);
  tokenVerifyTrend.add(Date.now() - start);

  // 400 expected for placeholder token, we're measuring latency not correctness
  check(res, { 'tx verify responded': (r) => r.status < 500 }) || errorRate.add(1);
}

function agentRegistration() {
  const agentNum = Math.floor(Math.random() * 100000);
  const payload = JSON.stringify({
    name: `load-agent-${agentNum}`,
    owner: 'load-test-team',
    capabilities: ['mcp'],
    allowed_tools: ['read', 'write'],
  });

  const params = {
    headers: { 'Content-Type': 'application/json' },
  };

  const res = http.post(`${PROXY_URL}/v1/register`, payload, params);
  check(res, { 'agent registered': (r) => r.status === 201 }) || errorRate.add(1);
}