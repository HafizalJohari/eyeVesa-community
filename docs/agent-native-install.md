# Agent-Native Install

Install eyeVesa by asking your AI agent.

This guide is for people who want Hermes, OpenClaw, Claude, Codex, or another terminal-capable AI agent to install eyeVesa Community for them.

The agent should set up only the local community sandbox. It should not ask for production credentials, GCP access, Terraform state, or official International Airport keys.

## Copy-Paste Prompt

Send this to your AI agent:

```text
Install and run eyeVesa from https://github.com/HafizalJohari/eyeVesa-community.git.
Use the local community sandbox only. Do not request production credentials.
Clone the repo, run ./start.sh, verify health endpoints, and report the local URLs back to me.
```

## What The Agent Should Do

The agent should:

1. Check that Git and Docker are installed.
2. Check that Docker is running.
3. Clone the community repo.
4. Run `./start.sh`.
5. Verify `http://localhost:8080/health`.
6. Verify `http://localhost:8080/v1/airport/health`.
7. Explain the local endpoints back to you.

## Expected Local URLs

| Local URL | Purpose |
|---|---|
| `http://localhost:8080` | Go control-plane API |
| `http://localhost:9443` | Rust gateway proxy |
| `http://localhost:8181` | OPA policy server |

If those services are running and the health checks pass, the local sandbox is ready.

## Safety Boundary

The community setup is local-only by default.

Your agent should not:

- Ask for GCP credentials.
- Ask for Terraform state.
- Ask for production database credentials.
- Ask for JWT secrets, gateway private keys, or official API keys.
- Commit `.env`, `.tfvars`, `.tfstate`, private keys, or generated secrets.
- Connect to the official International Airport unless you intentionally provide a scoped invite or API key.

Public code access is not production access. The official International Airport remains operator-controlled.

## For AI Agents

AI agents should follow the root [AGENT_INSTALL.md](../AGENT_INSTALL.md) file. It contains the machine-readable install checklist, verification commands, success message, troubleshooting commands, and safety rules.
