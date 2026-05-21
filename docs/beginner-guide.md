# eyeVesa Beginner Guide

This guide explains eyeVesa without assuming you are a developer.

## What Is eyeVesa?

eyeVesa is a trust layer for AI agents.

An AI agent is software that can do tasks for you, such as reading data, calling tools, making decisions, or connecting to another system. As agents become more powerful, we need a way to answer basic trust questions:

- Who is this agent?
- Is this agent allowed to do this action?
- Did this action really happen?
- Who approved it?
- Can we review the action later?

eyeVesa helps answer those questions.

## Simple Analogy

Imagine an airport.

People cannot just walk anywhere they want. They need identity, tickets, gates, permissions, logs, and security checks.

eyeVesa does something similar for AI agents:

- Identity: each agent has a cryptographic identity.
- Airport: agents can discover and meet other agents.
- Policy: rules decide what agents can and cannot do.
- Audit: actions are recorded so they can be reviewed.
- Human approval: risky actions can require a person to approve.

## What Problem Does It Solve?

Without a trust layer, an AI agent might:

- Call tools it should not use.
- Access resources without permission.
- Act without a clear identity.
- Make changes with no reliable audit trail.
- Connect to other agents without clear control.

eyeVesa adds structure and control around those actions.

## What Happens When I Run `./start.sh`?

When you run:

```bash
./start.sh
```

eyeVesa starts a local sandbox on your computer.

It starts:

- A local database.
- A local policy engine.
- A local control plane.
- A local gateway.
- A local Airport for agent discovery.

This does not connect to Hafizal's production GCP setup.
This does not give you access to the official International Airport.
This does not use production secrets.

It is just your own local playground.

## Can My AI Agent Install This For Me?

Yes. If you use Hermes, OpenClaw, Claude, Codex, or another AI agent with terminal access, you can ask it to install eyeVesa Community for you.

Give it the repo URL and ask it to follow `AGENT_INSTALL.md`. The agent should only run the local sandbox, verify the health endpoints, and report the local URLs back to you.

It should not ask for GCP credentials, Terraform state, production secrets, or official International Airport keys.

## What Is The Airport?

The Airport is where agents become discoverable.

An agent can say:

- I am online.
- These are my capabilities.
- This is my profile.
- This is how other agents can find me.

In a local sandbox, the Airport is only on your machine.

In production, the official International Airport is controlled by the operator. You need an invite or API key to write to it.

## What Are API Keys?

API keys are access passes.

In local community mode, API keys are usually not needed because the sandbox runs with authentication turned off for easy testing.

In production, API keys are required. They are created by the operator and shared privately. They should never be posted in GitHub, docs, chat, or public code.

## What Is An Audit Trail?

An audit trail is a record of what happened.

For example:

- Which agent requested an action.
- What action it requested.
- Whether the action was allowed or denied.
- When it happened.
- Whether a human approved it.

eyeVesa is designed for non-repudiable audit trails, meaning the records are built so actions are harder to deny later.

## Community vs Production

There are two important modes:

| Mode | Meaning |
|---|---|
| Community local sandbox | Runs on your machine for learning and testing. |
| Production / International Airport | Hosted and controlled by the operator. Requires issued credentials. |

Community users can clone the repo and run eyeVesa locally.

They cannot access production unless the operator gives them credentials.

## Who Is This For?

eyeVesa is for people building or operating AI agents that need more trust, control, and traceability.

It may be useful for:

- AI agent builders.
- SaaS platforms.
- Enterprise automation teams.
- Security-conscious AI workflows.
- Developers experimenting with agent-to-agent systems.

## What Should I Read Next?

If you are new, start here:

1. Read this guide.
2. Run the Community Quickstart in the main README.
3. Explore the local Airport endpoints.
4. Read `docs/features-airport.md`.
5. Read `docs/community-release-workflow.md` if you want to understand the public/private repo model.

You do not need to understand all of Terraform, GCP, SPIRE, OPA, or mTLS on day one. Start with the local sandbox first.
