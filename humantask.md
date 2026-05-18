# Human Tasks — Production Readiness

## 1. Environment & Secrets

- [ ] Provision PostgreSQL (RDS recommended) with pgvector extension — get endpoint, master creds
- [ ] Set up AWS Secrets Manager or HashiCorp Vault — move all secrets out of env vars:
  - `JWT_SECRET`, `agentid` DB password, `APNS_KEY_PATH` key file, `FCM_SA_KEY_PATH` service account JSON
- [ ] Reserve 3 internal ports: 8080 (HTTP), 9090 (gRPC), 9443 (proxy)
- [ ] Provision TLS certificates (or use Let's Encrypt / ACM) — need at least 2: one for proxy, one for Go API internal

## 2. Kubernetes / Compute

- [ ] Set up EKS cluster (or equivalent) — Terraform config exists, review instance types
- [ ] Push Docker images to ECR — `docker build` for all 3 components, tag with git SHA
- [ ] Configure SPIRE: deploy SPIRE Agent as DaemonSet, Server as StatefulSet — K8s manifests exist but need testing
- [ ] Add security groups: only proxy (9443) exposed publicly; Go API + DB internal-only

## 3. Database

- [ ] Run all 14 migrations — write a migration job (K8s Job or init container), don't do manual `psql`
- [ ] Set up RDS automated backups (daily snapshot, 7-day retention)
- [ ] Enable Multi-AZ for production RDS
- [ ] Create read replica if you expect heavy audit query load

## 4. Business Decisions

- [ ] Trust score thresholds — currently hardcoded (0.1 deny, 0.8 auto-allow) — are these right for your risk profile?
- [ ] HITL approver team — who are the real human approvers? Need their emails + device tokens
- [ ] Delegation depth — currently max 3 levels — is this sufficient?
- [ ] Budget limits — `max_budget_usd` per agent — needs business input
- [ ] Data residency — Malaysia/SE Asia — confirm AWS region: `ap-southeast-1` (Singapore)?
- [ ] SLA target — what's acceptable downtime? Shapes multi-AZ vs single-AZ decision

## 5. Monitoring & Alerting

- [ ] Set up Prometheus + Grafana (or use CloudWatch)
- [ ] Add alerts: 5xx rate > 1%, DB connection exhaustion, cert expiry < 7 days, SPIRE bundle stale > 15min
- [ ] PagerDuty integration — `PAGERDUTY_INTEGRATION_KEY` is already wired
- [ ] Slack integration — `SLACK_WEBHOOK_URL` already wired

## 6. Pre-Launch Checklist

- [ ] Run `docker-compose up` with all 14 migrations — verify clean start
- [ ] Run `tests/e2e-test.sh` against a staging environment
- [ ] Penetration test — especially the `/v1/authorize` and `/v1/spire/*` endpoints
- [ ] Load test: `k6` or `wrk` at 1K RPS for 10 min against proxy — watch for memory leaks
- [ ] Cert rotation test: deploy new cert, confirm Rust watcher picks it up without restart
- [ ] SIGHUP test: `kill -HUP` the process, verify rate limits and policies actually change
- [ ] Disaster recovery drill: kill the DB pod, confirm app recovers; restore from snapshot

## 7. Documentation for Your Team

- [ ] Runbook: what to do when HITL escalates, when SPIRE bundle is stale, when cert expires
- [ ] On-call rotation: who gets the PagerDuty alerts
- [ ] API key rotation procedure: how to issue new keys, revoke old ones

## Estimated Timeline

| Category | Effort | Owner |
|----------|--------|-------|
| Secrets & TLS | 1-2 days | You |
| Database | 1 day | You |
| K8s deploy | 2-3 days | You |
| Business decisions | 1 day meeting | You + stakeholders |
| Monitoring | 1-2 days | You |
| Pre-launch testing | 2-3 days | You |
| Documentation | 1 day | You |
| **Total** | **~1.5-2 weeks** | |

## Production Blockers (Code Side — Defer to Dev)

These require code changes, not your tasks:
- TLS end-to-end test with cert rotation
- Health/readiness probes that check DB + OPA connectivity
- Migration runner in server startup
- `AUTH_ENABLED=true` as production default
- gRPC server tests (0% coverage)
- OPA federation policy runtime test