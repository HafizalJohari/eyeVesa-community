"""
Community Test — eyeVesa Agent Identity & Airport

Self-sovereign identity flow:
1. Generate ed25519 keypair locally (private key never leaves this machine)
2. Register at the Airport (send public_key, keep private_key)
3. Challenge-response login (prove you own the private key)
4. Use the JWT token to access protected endpoints
5. Create API keys for service-to-service auth
6. Airport operations: heartbeat, profile, search
"""

import asyncio
import os
import sys

from nacl.signing import SigningKey

sdk_path = os.path.join(os.path.dirname(__file__), "sdk", "agent-sdk-python", "src")
sys.path.insert(0, sdk_path)

from agentid_sdk import AgentClient, AgentConfig


GATEWAY = os.getenv("GATEWAY_ENDPOINT", "http://localhost:8080")


async def main():
    print("=" * 60)
    print("  eyeVesa Community Test")
    print("  Gateway: " + GATEWAY)
    print("=" * 60)

    # Step 1: Generate ed25519 keypair locally
    print("\n1. Generating ed25519 keypair...")
    signing_key = SigningKey.generate()
    verify_key = signing_key.verify_key
    pub_key_b64 = verify_key.encode().hex()
    print(f"   Public key (hex): {pub_key_b64[:16]}...")
    print(f"   (Private key stays on this machine only)")

    # Step 2: Register at the Airport
    print("\n2. Registering agent at the Airport...")
    config = AgentConfig(
        agent_id="placeholder",
        name="community-test-agent",
        owner="community-tester",
        gateway_endpoint=GATEWAY,
    )
    client = AgentClient(config, signing_key=signing_key)

    await client.register()
    print(f"   Agent ID: {client.agent_id}")
    print(f"   Trust Score: {client.trust_score}")
    print(f"   Registered: {client.is_registered}")

    # Step 3: Challenge-response login
    print("\n3. Authenticating via challenge-response...")
    await client.login()
    print(f"   JWT Token: {client._jwt_token[:40]}...")
    print(f"   (Proved ownership of private key)")

    # Step 4: Create API key
    print("\n4. Creating API key...")
    api_key_resp = await client.create_api_key(name="community-test-key")
    api_key = api_key_resp["api_key"]
    print(f"   API Key: {api_key}")

    # Step 5: Airport heartbeat
    print("\n5. Sending heartbeat...")
    heartbeat = await client.airport_heartbeat(status="online")
    print(f"   Heartbeat: {heartbeat}")

    # Step 6: Update Airport profile
    print("\n6. Updating Airport profile...")
    profile = await client.airport_update_profile(
        description="Community test agent — exploring the Airport!",
        services_offered=["testing", "demo"],
        endpoints={"api": "https://example.com"},
        tags=["community", "test"],
        listed=True,
    )
    print(f"   Profile: {profile}")

    # Step 7: Search for other agents
    print("\n7. Searching Airport for agents...")
    search = await client.airport_search()
    print(f"   Found {search['count']} agent(s):")
    for agent in search.get("agents", []):
        print(f"   - {agent['name']} (trust: {agent.get('trust_score', 'N/A')}, status: {agent.get('status', 'N/A')})")

    # Step 8: List online agents
    print("\n8. Listing online agents...")
    online = await client.airport_list_online()
    print(f"   Online: {online['count']} agent(s)")

    # Step 9: Get own profile
    print("\n9. Getting own profile...")
    profile = await client.airport_get_profile(client.agent_id)
    print(f"   Name: {profile.get('name')}")
    print(f"   Description: {profile.get('description')}")
    print(f"   Tags: {profile.get('tags')}")
    print(f"   Status: {profile.get('status')}")

    # Step 10: List agents via protected endpoint
    print("\n10. Listing all agents (JWT auth)...")
    resp = await client._http.get("/v1/agents")
    if resp.status_code == 200:
        agents = resp.json().get("agents", [])
        print(f"   Found {len(agents)} agent(s)")
    else:
        print(f"   Failed: {resp.status_code}")

    print("\n" + "=" * 60)
    print("  Community test complete!")
    print("=" * 60)

    await client.close()


if __name__ == "__main__":
    asyncio.run(main())
