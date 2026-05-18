# eyeVesa Integration Guide

Yes, **Agentic AI from Claude, OpenAI, Anthropic, Grok, Gemini, Llama, and other major LLM providers can integrate with eyeVesa**.

## How Integration Works

eyeVesa provides a **standardized SDK + MCP (Model Context Protocol)** interface that any agentic system can use:

1. **Rust SDK** (`sdk/agent-sdk-rust/`) — Production-ready, used by Rust-based agents
2. **MCP Protocol** — The gateway exposes a standard `/v1/mcp` endpoint that follows the emerging MCP standard
3. **HTTP + JWT/API Key Auth** — Any language that can make HTTP requests can integrate

## Current Integration Status by Provider

| Provider                  | Integration Feasibility | Method |
|---------------------------|-------------------------|--------|
| **Claude (Anthropic)**    | High                    | Use Claude + Computer Use + custom MCP tool calling to eyeVesa |
| **OpenAI**                | High                 | Use OpenAI Agents + function calling to eyeVesa MCP endpoint |
| **Grok (xAI)**            | High                    | Native Rust support via the Rust SDK |
| **Gemini (Google)**       | Medium-High             | Via HTTP + MCP or custom tool calling |
| **Llama (Meta)**          | Medium                  | Via Llama.cpp tool calling or custom agent framework |
| **LangGraph / CrewAI / AutoGen** | High              | All support custom tool calling to external MCP servers |

## Integration Methods Available

1. **Direct SDK** (Recommended for Rust-based agents)
2. **MCP Protocol** (`/v1/mcp`) — Standard JSON-RPC interface
3. **REST API** (`/v1/authorize`, `/v1/ptv/*`, `/v1/hitl/*`, etc.)
4. **CLI Integration** (`eyevesa mcp call ...`)

## What eyeVesa Provides to LLMs

- **Identity** (Digital Agent Passport + SPIRE SVID)
- **Authorization** (FGA with delegation chains)
- **Runtime Policy Enforcement** (OPA)
- **Behavioral Monitoring + Trust Scoring**
- **HITL Escalation** (with Telegram, Discord, Push, Slack, PagerDuty)
- **Non-repudiable Audit Trail**
- **Budget Control**
- **PTV Hardware Attestation**

---

**Bottom line**: Any sufficiently advanced agentic framework (Claude Computer Use, OpenAI Swarm, LangGraph, etc.) can integrate with eyeVesa today using either the Rust SDK or the MCP/REST API.

Would you like me to create a specific integration guide for **Claude Computer Use** or **OpenAI Swarm**?