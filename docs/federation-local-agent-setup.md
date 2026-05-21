# eyeVesa Federation — Local Agent Setup

Connect your Hermes gateway to the Central Airport.

## Quick Start (one command)

```bash
# 1. Install SDK
pip install -e /path/to/eyeVesa/sdk/agent-sdk-python

# 2. Connect to Central Airport (runs until Ctrl+C)
python3 /path/to/eyeVesa/scripts/connect-airport.py \
  --url "$CENTRAL_AIRPORT_URL" \
  --api-key "$CENTRAL_AIRPORT_API_KEY" \
  --name "My Hermes"
```

That's it. The script handles everything:
- Generates Ed25519 key pair (saved to `~/.eyevesa/`)
- Registers gateway with Central Airport
- Syncs your agent (makes it discoverable)
- Sends heartbeats every 2 minutes
- Sends offline heartbeat on Ctrl+C

## Architecture

```
┌──────────────────┐          ┌──────────────────────────┐
│  Local Hermes     │          │   Central Airport (GCP)    │
│  Gateway (VPS)    │◄────────►│   gateway-control           │
│                   │  HTTPS   │   gateway-core               │
│  connect-airport  │          │   Cloud SQL (PostgreSQL)     │
│  (Python SDK)     │          │                            │
└──────────────────┘          └──────────────────────────┘
```

The Python SDK talks directly to Central Airport via HTTPS — no local gateway changes needed.

## Domestic vs International

| Scope | Meaning |
|-------|---------|
| `domestic` | Agents on the same local gateway |
| `international` | Agents from other gateways (connected via Central Airport) |

The script registers with `peer_type=remote` and `scope=international`.

## What the Script Does

```
Step 1: Generate Ed25519 key pair    (saves to ~/.eyevesa/gateway-ed25519.key)
Step 2: Create local agent            (uses HermesIntegration SDK)
Step 3: Register gateway             (POST /v1/federation/register)
        Sync agent                   (POST /v1/federation/agents/sync)
Step 4: Heartbeat loop               (POST /v1/federation/heartbeat every 2 min)
        Ctrl+C → send offline heartbeat
```

## Output

```
╔══════════════════════════════════════════════════╗
║   eyeVesa — Airport Connection Setup            ║
╚══════════════════════════════════════════════════╝

[INFO] Central Airport: https://gateway-control-....run.app
[INFO] Gateway name:    My Hermes

[INFO] Step 1/4: Generating Ed25519 key pair...
[INFO]   Public key: cMSn+gd7MhbQxrjM3f2s7gwMgcL...
[INFO] Step 2/4: Creating local agent...
[INFO]   Agent ID:    abc123...def456
[INFO]   Agent name:  my-hermes-agent
[INFO] Step 3/4: Registering with Central Airport...
[INFO]   Gateway ID:  56438fa0-1225-443a-9834-f27b9f108dba
[INFO]   Syncing agent to Central Airport...
[INFO]   Agent synced! ID: abc123...def456

  ✅  Gateway registered at Central Airport
  ✅  Agent synced and discoverable
  ✅  Heartbeat active — agent is online

[INFO] Gateway ID:   56438fa0-1225-443a-9834-f27b9f108dba
[INFO] Agent ID:     abc123...def456
[INFO] Airport URL:  https://gateway-control-....run.app

  Press Ctrl+C to disconnect and go offline

  [12:00:00] Heartbeat #1 — online (agent: abc123...)
  [12:02:00] Heartbeat #2 — online (agent: abc123...)
```

## Credentials

Ask your admin for:

| Item | Example |
|------|---------|
| `CENTRAL_AIRPORT_URL` | `https://your-central-airport.example.com` |
| `CENTRAL_AIRPORT_API_KEY` | `eyevesa_REPLACE_WITH_YOUR_ASSIGNED_KEY` |

Each developer or gateway should receive a separate named, revocable key through a password manager or Secret Manager. Do not commit Central Airport credentials to this repository.

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| `agentid-sdk not found` | SDK not installed | Run `pip install -e sdk/agent-sdk-python/` |
| `401 unauthorized` | Wrong API key | Check `--api-key` value |
| Connection refused | Gateway not running | Start local gateway first |
| `400 invalid public_key` | Key issue | Delete `~/.eyevesa/gateway-ed25519.key` and retry |

## Flow Summary

```
1. pip install -e sdk/agent-sdk-python/    (one-time)
2. python3 scripts/connect-airport.py ...   (run and leave running)
3. Ctrl+C to disconnect
```
