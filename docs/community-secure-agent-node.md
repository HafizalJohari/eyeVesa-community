# Community Secure Agent Node

The community version can run as a secure local Airport node for AI agents.
Each node registers local agents, publishes only listed profiles, and can
federate with other community nodes through explicit invites.

## Security Boundary

Community federation starts with secure discovery only:

- Local agents remain local unless their profile is listed.
- Remote nodes must be invited before they can register as trusted peers.
- Federated agents must sync with a signed passport from their gateway.
- Suspended peers are excluded from federated search and online results.
- Remote tool execution is not enabled by this milestone.

Think of it like airport flight boards. A trusted terminal can share which
agents are online and reachable, but an agent still cannot board, invoke tools,
or access resources without later policy-gated authorization.

## Two-Node Demo

Start two local community nodes on different ports, then create an invite on
node A for node B:

```bash
eyevesa --gateway http://localhost:8080 federation invite \
  --name node-b \
  --endpoint http://localhost:8081 \
  --trust-domain community-b
```

Register node B with the invite token returned by node A:

```bash
eyevesa --gateway http://localhost:8080 federation register \
  --name node-b \
  --endpoint http://localhost:8081 \
  --public-key <base64-ed25519-public-key> \
  --trust-domain community-b \
  --invite-token <invite-token>
```

List trusted peers:

```bash
eyevesa --gateway http://localhost:8080 federation peers
```

Sync a signed agent passport into node A:

```bash
eyevesa --gateway http://localhost:8080 federation sync \
  --payload '{"passport":{"agent_id":"<agent-id>","agent_public_key":"<base64-agent-public-key>","gateway_id":"<gateway-id>","gateway_signature":"<base64-signature>","issued_at":"<rfc3339>"},"name":"research-agent","owner":"community-b","trust_score":0.8,"capabilities":["research"],"allowed_tools":["read"],"description":"Research agent from node B","tags":["research"],"scope":"community"}'
```

Search trusted federated agents:

```bash
eyevesa --gateway http://localhost:8080 airport search --federated --status online
```

If node B is suspended, its agents disappear from federated discovery.
