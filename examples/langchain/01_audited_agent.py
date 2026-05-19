"""
LangChain Audited Agent Integration

This example demonstrates how to integrate eyeVesa with a LangChain agent.
It creates a custom LangChain Tool that routes its external requests through 
the eyeVesa gateway, ensuring that all actions taken by the AI agent are 
cryptographically signed, authenticated, and securely audited.
"""

import asyncio
import os
import sys
import requests
from typing import Optional, Type

# Ensure the SDK can be imported if running from the repo root
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../sdk/agent-sdk-python/src")))

from nacl.signing import SigningKey
from agentid_sdk import AgentClient, AgentConfig

# Langchain imports
from langchain.tools import BaseTool
from pydantic import BaseModel, Field

GATEWAY_URL = os.getenv("GATEWAY_ENDPOINT", "http://localhost:8080")

# 1. Define the LangChain Tool Schema
class SecureApiRequestInput(BaseModel):
    endpoint: str = Field(description="The API endpoint path to query, e.g., /v1/data")
    payload: str = Field(description="Data to send in the request")

# 2. Create a custom LangChain Tool that routes through eyeVesa
class EyeVesaSecureActionTool(BaseTool):
    name = "secure_api_action"
    description = "Executes a secure action via the eyeVesa identity proxy. Use this to perform sensitive network calls."
    args_schema: Type[BaseModel] = SecureApiRequestInput
    
    # We pass the authenticated eyeVesa client into the tool
    client: AgentClient = None

    def __init__(self, client: AgentClient, **kwargs):
        super().__init__(**kwargs)
        self.client = client

    def _run(self, endpoint: str, payload: str) -> str:
        """Synchronous tool execution. In production, use aiohttp with _arun."""
        # The agent uses its cryptographically backed JWT to authorize the request.
        # This request routes THROUGH the eyeVesa gateway, generating an immutable audit log.
        headers = {
            "Authorization": f"Bearer {self.client._jwt_token}",
            "Content-Type": "application/json"
        }
        
        target_url = f"{GATEWAY_URL}{endpoint}"
        
        try:
            print(f"   [LangChain Tool] 🔒 Routing request through eyeVesa gateway to {target_url}...")
            response = requests.post(target_url, headers=headers, json={"payload": payload})
            response.raise_for_status()
            return f"Success: {response.text}"
        except Exception as e:
            return f"Error executing secure action: {str(e)}"

    async def _arun(self, endpoint: str, payload: str) -> str:
        """Async implementation relying on the SDK's internal async HTTP client."""
        headers = {
            "Authorization": f"Bearer {self.client._jwt_token}",
            "Content-Type": "application/json"
        }
        try:
            print(f"   [LangChain Tool] 🔒 Routing ASYNC request through eyeVesa gateway to {endpoint}...")
            # Using the SDK's internal httpx client
            response = await self.client._http.post(endpoint, json={"payload": payload})
            return f"Success: {response.text}"
        except Exception as e:
            return f"Error executing secure async action: {str(e)}"

async def main():
    print("🤖 Booting LangChain eyeVesa Agent...")

    # --- SETUP EYEVESA IDENTITY ---
    signing_key = SigningKey.generate()
    config = AgentConfig(
        agent_id="langchain-agent",
        name="langchain-demo",
        owner="demo-user",
        gateway_endpoint=GATEWAY_URL,
    )
    client = AgentClient(config, signing_key=signing_key)
    
    print("\n📝 Registering LangChain agent identity...")
    await client.register()
    await client.login()
    print("✅ Identity established and authenticated.")

    # --- SETUP LANGCHAIN ---
    # Instantiate our custom tool injected with the authenticated eyeVesa client
    secure_tool = EyeVesaSecureActionTool(client=client)
    
    print("\n🔗 Initializing LangChain Toolkit with eyeVesa Tool...")
    # In a real app, you would pass this tool to initialize_agent()
    # agent = initialize_agent(
    #     tools=[secure_tool],
    #     llm=ChatOpenAI(temperature=0),
    #     agent=AgentType.CHAT_ZERO_SHOT_REACT_DESCRIPTION
    # )
    
    # For demonstration, we manually invoke the tool's async run logic
    # simulating the LLM deciding to use the tool.
    print("🧠 LLM Decision: 'I need to send sensitive data. I will use the secure_api_action tool.'")
    result = await secure_tool._arun(endpoint="/v1/agents", payload="Hello secure world")
    
    print(f"\n✅ Tool Execution Result:\n{result}")
    
    await client.close()

if __name__ == "__main__":
    asyncio.run(main())
