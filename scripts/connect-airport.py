#!/usr/bin/env python3
"""Connect a local Hermes gateway to the Central Airport.

One-shot command that handles:
  1. Generate Ed25519 key pair (saved to ~/.eyevesa/)
  2. Register gateway with Central Airport
  3. Issue passport and sync agent
  4. Start heartbeat loop (Ctrl+C to stop)

Usage:
  python3 scripts/connect-airport.py \
    --url "$CENTRAL_AIRPORT_URL" \
    --api-key "$CENTRAL_AIRPORT_API_KEY" \
    --name "My Hermes"
"""

from __future__ import annotations

import argparse
import asyncio
import base64
import json
import logging
import os
import signal
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger("connect-airport")

EYEVESA_DIR = Path.home() / ".eyevesa"
DEFAULT_KEY_PATH = EYEVESA_DIR / "gateway-ed25519.key"


def generate_ed25519_key(path: Path) -> str:
    path.parent.mkdir(parents=True, exist_ok=True)
    if not path.exists():
        subprocess.run(
            ["openssl", "genpkey", "-algorithm", "ED25519", "-out", str(path)],
            check=True, capture_output=True,
        )
        logger.info("Generated Ed25519 key pair at %s", path)
    else:
        logger.info("Loaded existing key from %s", path)

    pub = subprocess.run(
        ["openssl", "pkey", "-in", str(path), "-pubout"],
        capture_output=True, text=True, check=True,
    )
    
    # OpenSSL outputs PEM with ASN.1 SubjectPublicKeyInfo (44 bytes for Ed25519)
    # The raw Ed25519 public key is exactly the last 32 bytes.
    der_bytes = base64.b64decode(
        "".join(pub.stdout.strip().split("\n")[1:-1])
    )
    raw_32_bytes = der_bytes[-32:]
    pubkey_b64 = base64.b64encode(raw_32_bytes).decode()
    
    return pubkey_b64


def import_sdk():
    try:
        sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "sdk" / "agent-sdk-python" / "src"))
        from agentid_sdk.integrations import HermesIntegration
        return HermesIntegration
    except ImportError:
        pass

    try:
        from agentid_sdk.integrations import HermesIntegration
        return HermesIntegration
    except ImportError:
        logger.error(
            "agentid-sdk not found. Install it:\n"
            "  pip install -e sdk/agent-sdk-python/"
        )
        sys.exit(1)


def parse_args():
    parser = argparse.ArgumentParser(
        description="Connect a Hermes gateway to the Central Airport",
    )
    parser.add_argument(
        "--url", required=True,
        help="Central Airport URL (e.g. https://gateway-control-....run.app)",
    )
    parser.add_argument(
        "--api-key", required=True,
        help="API key for Central Airport authentication",
    )
    parser.add_argument(
        "--name", default="Hermes Local Gateway",
        help="Name for this gateway (default: Hermes Local Gateway)",
    )
    parser.add_argument(
        "--key-path", default=str(DEFAULT_KEY_PATH),
        help=f"Path to Ed25519 key (default: {DEFAULT_KEY_PATH})",
    )
    parser.add_argument(
        "--heartbeat-interval", type=int, default=120,
        help="Heartbeat interval in seconds (default: 120)",
    )
    return parser.parse_args()


async def main():
    args = parse_args()

    print()
    print("╔══════════════════════════════════════════════════╗")
    print("║   eyeVesa — Airport Connection Setup            ║")
    print("╚══════════════════════════════════════════════════╝")
    print()
    logger.info("Central Airport: %s", args.url)
    logger.info("Gateway name:    %s", args.name)
    print()

    HermesIntegration = import_sdk()
    key_path = Path(args.key_path)

    logger.info("Step 1/4: Generating Ed25519 key pair...")
    pubkey_b64 = generate_ed25519_key(key_path)
    logger.info("  Public key: %s", pubkey_b64[:32] + "...")

    logger.info("Step 2/4: Creating local agent...")
    hermes = HermesIntegration.from_config(
        gateway_endpoint="http://localhost:9443",
        agent_name=args.name.lower().replace(" ", "-") + "-agent",
        owner=args.name.lower().replace(" ", "-"),
        api_key=args.api_key,
    )
    await hermes.connect()
    pubkey_raw = hermes.client.public_key_base64
    logger.info("  Agent ID:    %s", hermes.client.agent_id)
    logger.info("  Agent name:  %s", hermes.client.name)

    logger.info("Step 3/4: Registering with Central Airport...")
    reg_result = await hermes.register_gateway(
        name=args.name,
        public_key=pubkey_b64,
        endpoint="http://localhost:9443",
        trust_domain=hermes.client.owner,
        peer_type="remote",
    )
    gateway_id = reg_result.get("gateway_id", "")
    logger.info("  Gateway ID:  %s", gateway_id)

    logger.info("  Syncing agent to Central Airport...")
    agent_reg = await hermes.sync_to_central(
        central_endpoint=args.url,
        description=f"Hermes agent connected via {args.name}",
        tags=["hermes", "federation", "connected"],
        scope="international",
    )
    logger.info("  Agent synced! ID: %s", hermes.client.agent_id)

    print()
    print("  ✅  Gateway registered at Central Airport")
    print("  ✅  Agent synced and discoverable")
    print("  ✅  Heartbeat active — agent is online")
    print()
    logger.info("Gateway ID:   %s", gateway_id)
    logger.info("Agent ID:     %s", hermes.client.agent_id)
    logger.info("Airport URL:  %s", args.url)
    print()
    print("  Press Ctrl+C to disconnect and go offline")
    print()

    heartbeat_count = 0
    running = True

    def _shutdown(sig, frame):
        nonlocal running
        if not running:
            return
        running = False
        logger.info("Shutting down...")

    signal.signal(signal.SIGINT, _shutdown)
    signal.signal(signal.SIGTERM, _shutdown)

    while running:
        try:
            hb = await hermes.federated_heartbeat(
                central_endpoint=args.url,
                status="online",
                gateway_id=gateway_id,
            )
            heartbeat_count += 1
            now = datetime.now(timezone.utc).strftime("%H:%M:%S")
            print(
                f"  [{now}] Heartbeat #{heartbeat_count} — online "
                f"(agent: {hermes.client.agent_id[:8]}...)"
            )

            for _ in range(args.heartbeat_interval):
                if not running:
                    break
                await asyncio.sleep(1)

        except Exception as e:
            logger.warning("Heartbeat failed: %s (retrying in 10s)", e)
            await asyncio.sleep(10)

    logger.info("Sending offline heartbeat...")
    try:
        await hermes.federated_heartbeat(
            central_endpoint=args.url,
            status="offline",
            gateway_id=gateway_id,
        )
        print("  ✅  Offline — agent disconnected")
    except Exception:
        pass

    print()
    print("  👋  Disconnected from Central Airport")
    print()


if __name__ == "__main__":
    asyncio.run(main())
