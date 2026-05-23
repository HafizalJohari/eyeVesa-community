# Security Harness Incident Runbook

## Scope
This runbook covers security harness failures from:
- `Security Phase 1`
- `Security Phase 3 - Container Scan Gate`
- `Security Phase 4 - Post Deploy Smoke`

## Triage Flow
1. Open the failed workflow run URL from the alert payload.
2. Identify failing job and step.
3. Classify impact:
- `critical`: secret leak, high/critical container vulnerability, post-deploy auth boundary or core health failure.
- `warning`: transient or flaky check that recovered on rerun.
4. Decide mitigation:
- block merge/deploy, or
- rollback (if production impact is confirmed), or
- patch and rerun.

## Playbooks

### Secret scan failure (Phase 1)
1. Rotate exposed credentials immediately.
2. Remove secret from code/history as needed.
3. Re-run workflow and confirm pass.

### Go vulnerability gate failure (Phase 1)
1. Upgrade vulnerable module(s).
2. Rebuild and run targeted tests.
3. Re-run workflow and confirm pass.

### IaC/config scan failure (Phase 1)
1. Inspect flagged Terraform/config path.
2. Apply least-privilege and secure-default fixes.
3. Re-run workflow and confirm pass.

### Container scan failure (Phase 3)
1. Identify vulnerable package from Trivy output.
2. Update base image and dependencies.
3. Rebuild image and re-run scan.

### Post-deploy smoke failure (Phase 4)
1. Validate `/health` and Airport endpoints.
2. Validate `/v1/authorize` auth boundary (`401/403` without auth).
3. If auth boundary or critical health fails in production, rollback.

## Escalation
1. Incident commander: release owner on duty.
2. Security reviewer: code owner for affected surface.
3. Communication: send status updates in the incident channel every 15 minutes until mitigated.
