"""
Basic eyeVesa Identity Example

This example demonstrates how to:
1. Generate an Ed25519 cryptographic identity for an agent.
2. Register the agent with the eyeVesa control plane (Airport).
3. Authenticate via a cryptographic challenge-response.
4. Interact with the protected API securely.
"""

import asyncio
import os
import sys

# Ensure the SDK can be imported if running from the repo root
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../sdk/agent-sdk-python/src")))

from nacl.signing import SigningKey
from agentid_sdk import AgentClient, AgentConfig

# The URL of your local or production eyeVesa Gateway
GATEWAY_URL = os.getenv("GATEWAY_ENDPOINT", "http://localhost:8080")

async def run_basic_agent():
    print("🤖 Booting eyeVesa Basic Agent...")

    # 1. Generate local cryptographic keypair
    # This private key never leaves the agent's environment!
    signing_key = SigningKey.generate()
    print(f"🔑 Generated Ed25519 Public Key: {signing_key.verify_key.encode().hex()[:16]}...")

    # 2. Configure the agent
    config = AgentConfig(
        agent_id="agent-" + signing_key.verify_key.encode().hex()[:8],
        name="basic-example-agent",
        owner="demo-user",
        gateway_endpoint=GATEWAY_URL,
    )
    
    # 3. Initialize client and register
    client = AgentClient(config, signing_key=signing_key)
    
    print("\n📝 Registering with eyeVesa Gateway...")
    await client.register()
    print(f"✅ Registered! Assigned Agent ID: {client.agent_id}")
    print(f"📈 Initial Trust Score: {client.trust_score}")

    # 4. Authenticate (Challenge-Response)
    print("\n🔐 Authenticating via challenge-response to obtain JWT...")
    await client.login()
    print("✅ Authenticated successfully.")

    # 5. Broadcast status to the Airport
    print("\n📡 Broadcasting online status heartbeat...")
    await client.airport_heartbeat(status="online")
    
    # 6. Fetch profiles of other agents in the network
    print("\n🔍 Discovering other agents in the network...")
    online_agents = await client.airport_list_online()
    print(f"🌐 Found {online_agents.get('count', 0)} agents online.")

    print("\n🎉 Basic Agent operations complete!")
    await client.close()

if __name__ == "__main__":
    asyncio.run(run_basic_agent())
