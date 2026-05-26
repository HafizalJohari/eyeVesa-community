# Changelog

All notable changes to this project will be documented in this file.

The format follows the required categories: Added, Changed, Deprecated, Removed, Fixed, and Security.
This project follows Semantic Versioning.

## [Unreleased]

### Added
- Added invite-only Community Secure Agent Node federation so self-hosted nodes can trust peer nodes and discover signed federated agents.
- Added policy-gated federated invoke authorization via `POST /v1/federation/invoke`, including cross-node connection logging without remote execution.
- Added `federation_peer_invites` storage for one-time hashed peer invite tokens.
- Added `eyevesa federation peers`, `eyevesa federation invite`, `eyevesa federation register`, `eyevesa federation sync`, `eyevesa federation invoke`, and `eyevesa airport search --federated` CLI workflows.
- Added community secure-node onboarding docs with a two-node local federation demo.
- Added `eyevesa agents delete <agent-id>` with interactive confirmation and `--yes` bypass flag.
- Added merchant-as-agent role support with new merchant profile and trust tables (`merchant_profiles`, `merchant_trust_state`, `merchant_trust_events`) and agent `roles`.
- Added merchant endpoints: `POST /v1/merchants`, `GET /v1/merchants`, `GET /v1/merchants/{merchantID}`, and `GET /v1/merchants/{merchantID}/trust`.
- Added merchant trust ingestion endpoints: `POST /v1/merchant-trust/events/outcome` and `POST /v1/merchant-trust/events/feedback`.
- Added the control-plane self-improving loop: successful authorizations now feed behavioral baselines, behavioral drift records anomalies and trust markdowns, and a detached OPA autogen worker compiles learned allow rules into `autogen_compiled.rego`.
- Added `scripts/reset-local.sh` and `scripts/smoke-test.sh` for repeatable community sandbox cleanup and verification.
- Added A2A adapter POC endpoints: `GET /v1/a2a/agents`, `POST /v1/a2a/tasks`, and `GET /v1/a2a/tasks/{taskID}` for interoperability scaffolding.
- Added in-memory A2A task lifecycle service and dedicated handler tests covering discovery, task creation, and task retrieval.
- Added a framework integration kit for Hermes, OpenClaw, and other agentic runtimes covering registration, Airport discovery, authorization, A2A handoff, and monetization positioning.
- Added a Community release workflow note and Terraform GCS backend example for clean public publishing without private history.
- Added a plain-language beginner guide for non-technical readers and new community users.
- Added agent-native onboarding docs so Hermes, OpenClaw, Claude, Codex, and similar agents can install and verify the local community sandbox.
- Added a codebase-backed user flow guide covering platform setup, credential bootstrap, agent registration, Airport discovery, authorization, HITL, A2A, and audit review.
- Added a CLI `quickstart` guide command and `config set` command so first-time users can discover the correct setup path and save gateway credentials without editing TOML manually.
- Added grouped CLI help sections so beginner, core, operator, and advanced commands are easier to scan.
- Added a dedicated Phase 1 security workflow at `.github/workflows/security-phase-1.yml` for PR/push security gating.

### Changed
- Changed the default federation peer type to `community` for community-node registration.
- Restricted federated discovery and invoke authorization to active trusted peers.
- Updated agent registration limit enforcement to apply only with tenant context (centralized Airport path), removing the global community/local cap fallback.
- Extended Airport search to support merchant-focused marketplace filters and ranking (`kind=merchant`, `min_merchant_trust`, `merchant_confidence`, `merchant_category`, `merchant_verification`).
- Changed the CLI module path and imports to `github.com/HafizalJohari/eyeVesa-community/cli` for standalone community builds.
- Updated `./start.sh` to build and install the real `eyevesa` CLI to `~/.local/bin/eyevesa`, show the resolved command path, and include CLI doctor verification in the success screen.
- Updated community install docs and installer defaults to use the `HafizalJohari/eyeVesa-community` repository and lowercase `eyevesa-community` folder examples.
- Hardened the GCP deployment defaults for International Airport by using production-sized regional Cloud SQL settings, private Cloud SQL networking by default, and Secret Manager-backed `DATABASE_URL`/`JWT_SECRET` injection.
- Protected agent registration and airport heartbeat behind authenticated requests.
- Wired control-plane router to expose A2A adapter routes alongside existing Airport/federation surfaces.
- Restricted tenant list/detail routes to admin JWTs.
- Updated `eyevesa connect` to use configured credentials for secure agent registration.
- Expanded ignore rules for local env files, Terraform variables/state/plans, and generated deployment artifacts.
- Changed the community test default gateway to localhost so live production endpoints are not embedded in the repo.
- Clarified the README community quickstart, local sandbox behavior, production API-key flow, and International Airport invite boundary.
- Removed the README link to the missing learning roadmap.
- Clarified that the repository code is Apache 2.0 licensed while hosted services and credentials remain separate.
- Refreshed the README opening with badges, clearer navigation, and visual feature cards for community readers.
- Updated the README and beginner guide to explain the local-only AI-agent installation flow.
- Made Docker Compose services project-scoped so separate checkouts do not fight over fixed container names.
- Clarified CLI root help, doctor guidance, connect examples, and init positioning around the current credential-first onboarding flow.

### Deprecated
- Nothing deprecated.

### Removed
- Removed committed GCP deploy env, Terraform variable, Terraform state, and local session transcript artifacts from the tracked tree.

### Fixed
- Fixed stale AgentID Gateway CLI branding in user-facing CLI help text.
- Fixed installer handling for stale Hermes/profile-wrapper `eyevesa` commands by backing up and replacing the wrapper with the eyeVesa CLI.
- Fixed `deploy-gcp.sh build` Docker contexts so local GCP image builds match the root-context Dockerfiles used by Cloud Build.
- Fixed gateway-core forwarding to accept full HTTPS Cloud Run control-plane URLs instead of forcing an `http://` scheme when backend TLS is disabled.
- Persisted `tenant_id` on agent registration when tenant context is present.
- Enforced per-tenant agent caps during agent registration without applying a global fallback cap to community/local registrations.
- Fixed `./start.sh` startup in non-interactive agent shells where `TERM` may be unset.
- Updated `./start.sh` to wait on the Compose `postgres` service instead of a fixed global container name.
- Fixed `eyevesa init` so returned registration API keys are saved to config when present, and fixed config saving for JWT-only auth.

### Security
- Required federation peer registration to use an invite token unless explicitly admin-approved.
- Hardened federated passport verification with required fields, signature checks, active peer checks, and 24-hour freshness enforcement.
- Rate-limited federation registration, agent sync, and heartbeat routes.
- Logged federated invoke decisions to `federated_connections` for cross-node audit visibility.
- Excluded suspended peers from federated agent search, online lists, and federated agent detail reads.
- Added marketplace guardrails in merchant trust state (`risk_flags`, `hitl_only`, `suspended`) so low-confidence or low-trust merchants can be rate-limited before checkout.
- Restricted autonomous policy generation to detached, validated Rego output and blocked never-event actions such as schema, cluster, policy override, and secret access from promotion.
- Disabled Cloud SQL public IPv4 by default and enabled Cloud SQL deletion protection in the GCP Terraform path.
- Added tenant/owner checks before airport heartbeat and profile update writes.
- Reused existing API key/JWT middleware for A2A routes to keep auth boundaries consistent in the adapter layer.
- Blocked `AUTH_ENABLED=false` when the runtime environment is production.
- Replaced real-looking API key and Central Airport examples with placeholders in public-facing docs and scripts.
- Ignored generated session transcript files to prevent accidental credential capture in source control.
- Added secret-leak blocking in CI using Gitleaks on every push and pull request to `main`.
- Added High/Critical filesystem and IaC risk blocking in CI using Trivy (`scan-type=fs`) across repository content.
- Added explicit Go vulnerability scanning gates with `govulncheck` for `gateway/control-plane` and `adapter/resource-adapter-go`.
- Added an auth/policy regression CI gate that explicitly runs `internal/auth`, `internal/policy`, and `cmd/api/handlers` test suites to block authorization boundary regressions.
- Added Phase 3 container-image vulnerability gates that build control-plane, core, and adapter images, then block CI on Trivy High/Critical findings.
- Added Phase 4 post-deploy smoke gate with public health checks plus protected-route auth boundary verification (`401/403` without auth, authenticated path check with smoke API key).
- Added Phase 5 alerting and incident routing for failed security workflows with severity mapping, branch-scoped dedupe, cooldown guard, webhook delivery, and runbook-linked remediation context.
- Added alert delivery test workflow (`security-alert-delivery-test.yml`) to validate webhook routing and payload formatting on demand.
- Added automatic GitHub incident issue creation for failed critical security workflows with duplicate-open-issue suppression.
- Added weekly security digest reporting workflow that summarizes failed security runs over the last 7 days.
- Added branch-protection setup guidance for enforcing security harness checks on `main`.
- Added a TUI Security view that shows latest Phase 1/3/4/5 GitHub Actions security harness run statuses (success/failure, branch, timestamp, and run URL).

## [0.1.1] - 2026-05-20

### Added
- Added automatic API key creation in agent registration responses.
- Added `eyevesa connect` for register, save API key, and heartbeat onboarding.
- Added public `GET /v1/airport/stats` for landing page health metrics.
- Added this changelog as the release history source of truth.
- Added an Astro Starlight documentation site under `docs/` with overview, quickstart, architecture, Airport, CLI, and expanded SDK documentation pages.

### Changed
- Airport listing and online endpoints now include federated agents from `federated_heartbeats`.
- API key migrations now repair `api_keys.tenant_id` to `TEXT` and add hash lookup support.

### Deprecated
- Nothing deprecated.

### Removed
- Nothing removed.

### Fixed
- Fixed community heartbeat onboarding by making `POST /v1/airport/heartbeat` public.
- Fixed production API key creation against the new text tenant schema.

### Security
- Restricted API key creation/revocation, key rotation, and tenant creation to admin JWTs.
- Stored new API keys as SHA-256 hashes and kept legacy plaintext lookup compatibility.
- Wrapped authorize trust-score updates in a row-locking transaction.
- Added tenant filtering to agent, resource, and API-key query paths when tenant context is present.

## [0.1.0] - 2026-05-19

### Added
- Initial EyeVesa platform baseline with Go control plane APIs, Rust gateway core, Go CLI, resource adapter, SDKs, PostgreSQL migrations, OPA policies, SPIRE/SPIFFE support, Docker/Kubernetes deployment assets, and static site assets.
- Added agent identity, authorization, audit, delegation, HITL, PTV, Airport discovery, MCP, API key, and SDK integration surfaces.

### Changed
- Nothing changed.

### Deprecated
- Nothing deprecated.

### Removed
- Nothing removed.

### Fixed
- Nothing fixed.

### Security
- Added cryptographic identity, policy-based authorization, non-repudiable audit logging, mTLS/SPIFFE support, and API key/JWT authentication primitives.
