# eyeVesa Examples

This directory contains recipes and code examples to help developers quickly integrate their agents with the eyeVesa identity, trust, and audit network.

## Prerequisites

All examples assume you have the eyeVesa infrastructure running locally.
You can start it in 10 seconds from the root of this repository using:
```bash
./start.sh
```

You must also have Python 3.9+ installed along with the SDK requirements:
```bash
cd sdk/agent-sdk-python
pip install -r requirements.txt
pip install langchain pydantic requests # For Langchain examples
```

## Directory Structure

* **`python/`** - Core Python SDK examples showing identity creation, registration, and challenge-response authentication.
  * [`01_basic_identity.py`](python/01_basic_identity.py): Bootstrapping a vanilla Python agent with an Ed25519 identity and registering it with the Airport.
* **`langchain/`** - Framework-specific integrations.
  * [`01_audited_agent.py`](langchain/01_audited_agent.py): Creating a custom LangChain `BaseTool` that routes LLM network requests through the eyeVesa gateway for immutable auditing and access control.

## Running the Examples

Run any example directly using Python:

```bash
# Run the basic identity workflow
python examples/python/01_basic_identity.py

# Run the LangChain integration example
python examples/langchain/01_audited_agent.py
```
