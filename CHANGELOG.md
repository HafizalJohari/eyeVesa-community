# Changelog

All notable changes to this project will be documented in this file.

The format follows the required categories: Added, Changed, Deprecated, Removed, Fixed, and Security.
This project follows Semantic Versioning.

## [Unreleased]

### Added
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

### Changed
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
- Enforced per-tenant agent caps during agent registration (falling back to the license cap when no tenant context is available).
- Fixed `./start.sh` startup in non-interactive agent shells where `TERM` may be unset.
- Updated `./start.sh` to wait on the Compose `postgres` service instead of a fixed global container name.
- Fixed `eyevesa init` so returned registration API keys are saved to config when present, and fixed config saving for JWT-only auth.

### Security
- Restricted autonomous policy generation to detached, validated Rego output and blocked never-event actions such as schema, cluster, policy override, and secret access from promotion.
- Disabled Cloud SQL public IPv4 by default and enabled Cloud SQL deletion protection in the GCP Terraform path.
- Added tenant/owner checks before airport heartbeat and profile update writes.
- Reused existing API key/JWT middleware for A2A routes to keep auth boundaries consistent in the adapter layer.
- Blocked `AUTH_ENABLED=false` when the runtime environment is production.
- Replaced real-looking API key and Central Airport examples with placeholders in public-facing docs and scripts.
- Ignored generated session transcript files to prevent accidental credential capture in source control.

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
