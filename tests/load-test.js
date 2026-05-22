// eyeVesa Gateway Load Test Suite
// Target: 2000+ RPS across all critical endpoints
// Usage: k6 run load-test.js --env BASE_URL=http://localhost:8080 --env PROXY_URL=http://localhost:9443
// Install: https://k6.io/docs/get-started/installation/

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const tokenIssueTrend = new Trend('tx_issue_duration', true);
const tokenVerifyTrend = new Trend('tx_verify_duration', true);
const authzTrend = new Trend('authz_duration', true);
const mcpTrend = new Trend('mcp_duration', true);
const healthTrend = new Trend('health_duration', true);
const skillsTrend = new Trend('skills_duration', true);

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
    health_duration: ['p(95)<50'],
    skills_duration: ['p(95)<100'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const PROXY_URL = __ENV.PROXY_URL || 'http://localhost:9443';
const AGENT_ID = __ENV.AGENT_ID || '66a62aaa-53a8-472c-8973-f3ec15ae8b32';

export default function () {
  const scenario = Math.random();

  if (scenario < 0.20) {
    healthCheck();
  } else if (scenario < 0.40) {
    authorizeViaProxy();
  } else if (scenario < 0.55) {
    authorizeDirect();
  } else if (scenario < 0.65) {
    mcpInitialize();
  } else if (scenario < 0.75) {
    transactionIssue();
  } else if (scenario < 0.82) {
    transactionVerify();
  } else if (scenario < 0.90) {
    skillsList();
  } else if (scenario < 0.95) {
    agentGet();
  } else {
    budgetCheck();
  }

  sleep(0.01);
}

function healthCheck() {
  const start = Date.now();
  const res = http.get(`${PROXY_URL}/health`);
  healthTrend.add(Date.now() - start);
  check(res, { 'health ok': (r) => r.status === 200 }) || errorRate.add(1);
}

function authorizeViaProxy() {
  const payload = JSON.stringify({
    agent_id: AGENT_ID,
    action: 'read',
    resource_id: 'doc-load-test',
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const start = Date.now();
  const res = http.post(`${PROXY_URL}/v1/auth`, payload, params);
  authzTrend.add(Date.now() - start);

  check(res, { 'authz via proxy': (r) => r.status === 200 }) || errorRate.add(1);
}

function authorizeDirect() {
  const payload = JSON.stringify({
    agent_id: AGENT_ID,
    action: 'search',
    params: {},
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/v1/authorize`, payload, params);
  authzTrend.add(Date.now() - start);

  check(res, { 'authz direct': (r) => r.status === 200 }) || errorRate.add(1);
}

function mcpInitialize() {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    method: 'initialize',
    id: 'load-test',
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const start = Date.now();
  const res = http.post(`${PROXY_URL}/v1/mcp`, payload, params);
  mcpTrend.add(Date.now() - start);

  check(res, { 'mcp init': (r) => r.status === 200 }) || errorRate.add(1);
}

function transactionIssue() {
  const payload = JSON.stringify({
    agent_id: AGENT_ID,
    resource_id: 'load-test-resource',
    action: 'read',
    scopes: ['read'],
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/v1/tx/issue`, payload, params);
  tokenIssueTrend.add(Date.now() - start);

  check(res, { 'tx issue': (r) => r.status === 200 || r.status === 403 }) || errorRate.add(1);
}

function transactionVerify() {
  const payload = JSON.stringify({
    token: { jti: 'load-test-placeholder' },
  });
  const params = { headers: { 'Content-Type': 'application/json' } };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/v1/tx/verify`, payload, params);
  tokenVerifyTrend.add(Date.now() - start);

  check(res, { 'tx verify responded': (r) => r.status < 500 }) || errorRate.add(1);
}

function skillsList() {
  const start = Date.now();
  const res = http.get(`${BASE_URL}/v1/skills`);
  skillsTrend.add(Date.now() - start);

  check(res, { 'skills list': (r) => r.status === 200 }) || errorRate.add(1);
}

function agentGet() {
  const res = http.get(`${BASE_URL}/v1/agents/${AGENT_ID}`);
  check(res, { 'agent get': (r) => r.status === 200 }) || errorRate.add(1);
}

function budgetCheck() {
  const res = http.get(`${BASE_URL}/v1/budget/check?agent_id=${AGENT_ID}`);
  check(res, { 'budget check': (r) => r.status === 200 || r.status === 402 }) || errorRate.add(1);
}
