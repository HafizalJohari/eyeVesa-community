"""
Hermes Agent (Telegram Bot) — Airport Integration Example

This shows exactly what a Hermes agent running as a Telegram bot
needs to prepare before going to the Airport, and how it accesses
the Airport once connected.

Prerequisites:
    1. eyeVesa gateway running (core + control plane + postgres)
    2. pip install agentid-sdk python-telegram-bot

Step-by-step flow:
    1. PREPARE  — Configure agent identity and gateway endpoint
    2. CONNECT  — Register with eyeVesa, get agent_id + trust_score
    3. ARRIVE   — Send heartbeat, set up Airport profile
    4. DISCOVER — Search for other agents at the Airport
    5. ACT      — Use tools through eyeVesa with authz + audit
    6. STAY     — Periodic heartbeat to maintain presence
"""

import asyncio
import os
import json
import logging
from datetime import datetime

from agentid_sdk import (
    AgentClient,
    AgentConfig,
    HermesIntegration,
)

logger = logging.getLogger("hermes-airport-example")


# =============================================================================
# STEP 1: PREPARE — What the agent needs before going to the Airport
# =============================================================================

# Option A: Environment variables (recommended for production)
#   export AGENT_ID=""           # Leave empty — server assigns on register
#   export AGENT_NAME="hermes-telegram-bot"
#   export AGENT_OWNER="org:my-company"
#   export GATEWAY_ENDPOINT="http://localhost:9443"

# Option B: Explicit config (recommended for development)
HERMES_CONFIG = AgentConfig(
    agent_id="",
    name="hermes-telegram-bot",
    owner="org:my-company",
    gateway_endpoint="http://localhost:9443",
)


# =============================================================================
# STEP 2: CONNECT — Register the agent with eyeVesa gateway
# =============================================================================

async def connect_to_gateway():
    """Register the agent and get an agent_id."""
    client = AgentClient(HERMES_CONFIG)
    result = await client.connect()

    print(f"Connected! agent_id={client.agent_id}")
    print(f"  trust_score={client.trust_score}")
    print(f"  registered={client.is_registered}")
    return client


# =============================================================================
# STEP 3: ARRIVE AT THE AIRPORT — Heartbeat + Profile
# =============================================================================

async def arrive_at_airport(client: AgentClient):
    """Check in at the Airport: announce presence and set up profile."""

    # 3a. Send heartbeat to announce you're online
    heartbeat = await client.airport_heartbeat(
        status="online",
        metadata={
            "framework": "hermes",
            "platform": "telegram",
            "version": "1.0.0",
            "capabilities": ["research", "summarization", "translation"],
            "started_at": datetime.utcnow().isoformat(),
        },
    )
    print(f"Heartbeat sent: {heartbeat}")

    # 3b. Set up your Airport profile so others can find you
    profile = await client.airport_update_profile(
        description=(
            "Hermes Telegram Bot — Research, summarization, and translation "
            "agent. Available for delegation of research tasks."
        ),
        tags=["hermes", "telegram", "research", "summarization", "translation"],
        listed=True,
    )
    print(f"Profile updated: {profile}")

    return heartbeat, profile


# =============================================================================
# STEP 4: DISCOVER — Search for other agents at the Airport
# =============================================================================

async def discover_agents(client: AgentClient):
    """Look for other agents at the Airport."""

    # 4a. Who's online right now?
    online = await client.airport_list_online()
    print(f"\nAgents currently online:")
    for agent in online.get("agents", []):
        print(f"  - {agent.get('name')} (trust: {agent.get('trust_score')}) "
              f"[{agent.get('status')}]")

    # 4b. Search by skill
    researchers = await client.airport_search(skill="research")
    print(f"\nAgents with 'research' skill:")
    for agent in researchers.get("agents", []):
        print(f"  - {agent.get('name')} (trust: {agent.get('trust_score')})")

    # 4c. Search by tag
    tagged = await client.airport_search(tag="data-processing")
    print(f"\nAgents tagged 'data-processing':")
    for agent in tagged.get("agents", []):
        print(f"  - {agent.get('name')} (trust: {agent.get('trust_score')})")

    # 4d. Search with trust threshold (only agents with trust >= 0.8)
    trusted = await client.airport_search(min_trust=0.8, status="online")
    print(f"\nHighly trusted agents (trust >= 0.8, online):")
    for agent in trusted.get("agents", []):
        print(f"  - {agent.get('name')} (trust: {agent.get('trust_score')})")

    return online


# =============================================================================
# STEP 5: ACT — Use tools through eyeVesa (with authorization + audit)
# =============================================================================

async def act_via_eyevesa(hermes: HermesIntegration):
    """Execute actions through eyeVesa's authz and audit layer."""

    # 5a. Read a resource (goes through OPA policy check)
    try:
        result = await hermes.handle_action("eyevesa_read", {
            "resource_id": "doc-001",
            "query": "latest financial report",
        })
        print(f"\nRead result: {result}")
    except Exception as e:
        print(f"Not authorized or HITL required: {e}")

    # 5b. Write to a resource (higher risk, may require HITL approval)
    try:
        result = await hermes.handle_action("eyevesa_write", {
            "resource_id": "doc-001",
            "data": json.dumps({"summary": "Q3 results updated by hermes bot"}),
        })
        print(f"Write result: {result}")
    except Exception as e:
        print(f"Not authorized or HITL required: {e}")

    # 5c. Proactively request human approval for a sensitive action
    result = await hermes.handle_action("eyevesa_request_approval", {
        "action": "bank_transfer",
        "reason": "Hermes bot needs to initiate a payment transfer",
        "risk_level": "critical",
    })
    print(f"Approval request: {result}")

    # 5d. Delegate scope to another agent found at the Airport
    result = await hermes.handle_action("eyevesa_delegate", {
        "delegatee_id": "some-other-agent-id",
        "scope": ["read", "search"],
        "reason": "Delegating research task to specialized agent",
    })
    print(f"Delegation result: {result}")


# =============================================================================
# STEP 6: STAY — Periodic heartbeat to maintain Airport presence
# =============================================================================

async def heartbeat_loop(hermes: HermesIntegration, interval_seconds: int = 60):
    """Keep the agent visible at the Airport with periodic heartbeats.

    If heartbeat is not sent within 2 minutes, the Postgres function
    `airport_mark_stale_offline()` will mark the agent as 'offline'.
    """
    import asyncio as _asyncio

    while True:
        try:
            result = await hermes.heartbeat("online")
            print(f"[{datetime.utcnow().isoformat()}] Heartbeat: {result.get('status')}")
        except Exception as e:
            logger.error(f"Heartbeat failed: {e}")
            # Try again with 'busy' status to indicate we're alive but slow
            try:
                await hermes.heartbeat("busy")
            except Exception:
                pass
        await _asyncio.sleep(interval_seconds)


# =============================================================================
# FULL EXAMPLE: Using HermesIntegration directly (recommended)
# =============================================================================

async def full_example_with_hermes_integration():
    """The recommended way — using HermesIntegration wrapper."""

    # PREPARE: Create the integration
    hermes = HermesIntegration.from_config(
        gateway_endpoint="http://localhost:9443",
        agent_name="hermes-telegram-bot",
        owner="org:my-company",
    )

    # CONNECT: Register with gateway
    await hermes.connect()
    print(f"Hermes connected: agent_id={hermes.client.agent_id}")

    # ARRIVE: Announce presence at the Airport
    await hermes.heartbeat("online")
    print("Hermes is now visible at the Airport")

    # ARRIVE: Set up discoverable profile
    await hermes.update_airport_profile(
        description="Hermes Telegram Bot — Research agent with translation capabilities",
        tags=["hermes", "telegram", "research"],
        listed=True,
    )

    # DISCOVER: Find other agents
    peers = await hermes.discover_peers(capability="mcp", status="online")
    print(f"Found {peers.get('count', 0)} online peers")

    online = await hermes.list_online_peers()
    print(f"Currently online: {online.get('count', 0)} agents")

    # ACT: Use eyeVesa tools through the integration
    tools = hermes.get_tool_specs()
    print(f"Available tools ({len(tools)}):")
    for t in tools:
        print(f"  - {t['name']}: {t['description'][:60]}...")

    # ACT: Execute an action
    result = await hermes.handle_action("eyevesa_read", {
        "resource_id": "doc-001",
        "query": "financial summary",
    })
    print(f"Action result: {result}")

    # CONNECT: Look up a specific agent's profile
    if online.get("agents"):
        other_agent_id = online["agents"][0]["agent_id"]
        profile = await hermes.get_peer_profile(other_agent_id)
        print(f"Peer profile: {profile.get('name')} (trust: {profile.get('trust_score')})")

        # CONNECT: View connection history
        connections = await hermes.get_connections(agent_id=other_agent_id)
        print(f"Connection history: {connections.get('count', 0)} past interactions")

    # STAY: Keep heartbeat going (in production, run this in background)
    # asyncio.create_task(heartbeat_loop(hermes, interval_seconds=60))


# =============================================================================
# FULL EXAMPLE: Using AgentClient directly (lower-level)
# =============================================================================

async def full_example_with_raw_client():
    """The manual way — using AgentClient directly for maximum control."""

    config = AgentConfig(
        agent_id="",
        name="hermes-telegram-bot",
        owner="org:my-company",
        gateway_endpoint="http://localhost:9443",
    )
    client = AgentClient(config)

    # CONNECT
    await client.connect()
    print(f"Connected: agent_id={client.agent_id}, trust={client.trust_score}")

    # ARRIVE
    await client.airport_heartbeat("online")
    await client.airport_update_profile(
        description="Hermes Telegram Bot",
        tags=["hermes", "telegram"],
        listed=True,
    )

    # DISCOVER
    agents = await client.airport_search(min_trust=0.5, status="online")
    print(f"Found {agents['count']} agents")

    # ACT
    try:
        result = await client.invoke("doc-001", "read", {"query": "summary"})
        print(f"Invoke result: {result}")
    except Exception as e:
        print(f"Action blocked: {e}")

    # STAY — send heartbeat every 60 seconds
    # (in production: use asyncio.create_task with a loop)


# =============================================================================
# TELEGRAM BOT INTEGRATION PATTERN
# =============================================================================
#
# In a real Telegram bot, the flow looks like this:
#
#   import telegram
#   from agentid_sdk import HermesIntegration
#
#   bot = telegram.Bot(token="YOUR_TELEGRAM_BOT_TOKEN")
#   hermes = HermesIntegration.from_config(gateway_endpoint="http://localhost:9443")
#
#   async def start():
#       await hermes.connect()            # Register + get agent_id
#       await hermes.heartbeat("online")  # Check in at Airport
#       await hermes.update_airport_profile(
#           description="Hermes Telegram research bot",
#           tags=["telegram", "research"],
#           listed=True,
#       )
#       # Start heartbeat background task
#       asyncio.create_task(heartbeat_loop(hermes, 60))
#       # Start polling for Telegram messages
#       await bot.start_polling()
#
#   async def handle_message(update, context):
#       user_msg = update.message.text
#       result = await hermes.handle_action("eyevesa_read", {
#           "resource_id": "knowledge-base",
#           "query": user_msg,
#       })
#       await update.message.reply_text(result)
#
# =============================================================================


if __name__ == "__main__":
    print("=" * 60)
    print("Hermes Agent — Airport Integration Example")
    print("=" * 60)
    print()
    print("This example requires eyeVesa gateway to be running.")
    print("Start it with:")
    print("  docker-compose up -d")
    print("  cd gateway/control-plane && go run cmd/api/main.go")
    print("  cd gateway/core && cargo run")
    print()
    print("PREPARATION CHECKLIST:")
    print("  1. Gateway endpoint configured (GATEWAY_ENDPOINT env var)")
    print("  2. Agent name and owner decided")
    print("  3. Ed25519 signing key (auto-generated if not provided)")
    print("  4. Optional: API key or JWT token for authenticated access")
    print("  5. Optional: Tags, description, and services for Airport profile")
    print()

    choice = input("Run full example with HermesIntegration? (y/n): ")
    if choice.lower() == "y":
        asyncio.run(full_example_with_hermes_integration())
    else:
        print("Skipping live example. See the code above for the integration pattern.")