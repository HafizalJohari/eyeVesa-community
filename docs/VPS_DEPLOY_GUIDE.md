# eyeVesa VPS Deployment Guide

## Prerequisites

- VPS with Docker + Docker Compose v2 (Ubuntu 22.04/24.04 recommended)
- Domain name pointing to your VPS IP
- Open ports: 80, 443

## Quick Deploy (5 minutes)

```bash
# 1. SSH into your VPS
ssh root@your-vps-ip

# 2. Clone eyeVesa
git clone https://github.com/hafizaljohari/eyeVesa.git
cd eyeVesa

# 3. Initialize production config
./deploy/scripts/deploy-vps.sh init

# 4. Edit config — set your domain
nano deploy/.env.prod
# Change: DOMAIN=gateway.yourdomain.com

# 5. Deploy
./deploy/scripts/deploy-vps.sh deploy

# 6. Test
curl https://gateway.yourdomain.com/health
```

## What Gets Deployed

| Service | Port (internal) | Exposed Via |
|---|---|---|
| Caddy (reverse proxy + TLS) | 80, 443 | Public |
| Gateway Core (Rust) | 9443 | Caddy → core |
| Control Plane (Go) | 8080, 9090 | Internal only |
| PostgreSQL + pgvector | 5432 | Internal only |
| OPA Policy Engine | 8181 | Internal only |
| Resource Adapter (Go) | 8443 | Internal only |

**Only Caddy (ports 80/443) is publicly exposed.** All other services are `127.0.0.1` bound or internal-only. Caddy handles TLS via Let's Encrypt automatically.

## Agent Connection

Once deployed, any agent anywhere can connect:

```python
from agentid_sdk import AgentClient, AgentConfig

config = AgentConfig(
    agent_id="",
    name="my-remote-agent",
    owner="my-org",
    gateway_endpoint="https://gateway.yourdomain.com",
)
client = AgentClient(config, api_key="ak_live_abc123")
await client.connect()
```

### Claude Code MCP

```json
{
  "mcpServers": {
    "eyevesa": {
      "type": "http",
      "url": "https://gateway.yourdomain.com/v1/mcp",
      "headers": {"X-API-Key": "ak_live_abc123"}
    }
  }
}
```

### Hermes MCP

```yaml
mcp_servers:
  agentid-gateway:
    url: "https://gateway.yourdomain.com/v1/mcp"
    headers:
      X-API-Key: "ak_live_abc123"
```

### OpenAI Responses API

```python
tools = [{
    "type": "mcp",
    "server_url": "https://gateway.yourdomain.com/v1/mcp",
    "headers": {"X-API-Key": "ak_live_abc123"},
}]
```

## DNS Setup

Point your domain to the VPS:

```
# A record
gateway.yourdomain.com  →  A  →  your-vps-ip

# Or CNAME
gateway.yourdomain.com  →  CNAME  →  your-vps-hostname
```

## TLS

Caddy automatically provisions and renews Let's Encrypt certificates. No manual cert management needed.

For mTLS (mutual TLS — requiring client certificates), change `GATEWAY_MODE` in `.env.prod`:

```bash
GATEWAY_MODE=mtls
```

## VPS Sizing

| Agents | vCPU | RAM | Disk | Est. Cost/mo |
|---|---|---|---|---|
| < 50 | 1 | 1 GB | 25 GB | $5-6 |
| 50-500 | 2 | 4 GB | 50 GB | $12-20 |
| 500+ | 4 | 8 GB | 100 GB | $40-50 |

## Security Checklist

- [ ] Change `DB_PASSWORD` and `JWT_SECRET` in `.env.prod` (auto-generated on `init`)
- [ ] Set `AUTH_ENABLED=true` in `.env.prod`
- [ ] Generate API keys: `curl -X POST http://localhost:8080/v1/api-keys`
- [ ] Configure firewall: only allow ports 22, 80, 443
- [ ] Set `GATEWAY_MODE=tls` or `mtls` for production
- [ ] Enable HITL notifications (Slack/PagerDuty webhook in `.env.prod`)
- [ ] Regular PostgreSQL backups: `pg_dump agentid > backup.sql`

## Cloud Run Deployment (GCP)

For production deployments on Google Cloud Run:

### Architecture
- `eyevesa-core` — Rust gateway (public, port 8080 or $PORT)
- PostgreSQL — Cloud SQL instance
- Caddy not needed — Cloud Run provides automatic TLS

### Quick Deploy

1. Set up Cloud Build trigger pointing to `gateway/core/Dockerfile`
2. The `cloudbuild.yaml` at repo root handles building and pushing
3. Create a separate Cloud Run service for the control plane (internal only)
4. Use Cloud SQL for PostgreSQL

### Environment Variables (Cloud Run)
- `PORT` — Cloud Run sets this to 8080 automatically
- `CONTROL_PLANE_HTTP_ADDR` — Internal URL of the control plane service
- `GATEWAY_MODE` — plaintext
- `AUTH_ENABLED` — true in production

### Firewall
- Only the Rust gateway service needs public ingress
- The control plane service should be internal-only
- Cloud SQL should have private IP only

### Testing the Airport

```bash
# Health check
curl https://your-domain.run.app/v1/airport/health

# Register an agent (auto-creates heartbeat + profile)
curl -X POST https://your-domain.run.app/v1/agents/register \
  -H "Content-Type: application/json" \
  -d '{"name":"hermes","owner":"org:acme","public_key":"BASE64_KEY"}'

# Search for agents
curl https://your-domain.run.app/v1/airport/agents?min_trust=0.5

# See who's online
curl https://your-domain.run.app/v1/airport/online
```

**Note:** Airport browse endpoints (search, online, profile, health) are **public**. Write endpoints (heartbeat, update-profile, connections) require `X-API-Key` or `Authorization: Bearer` header.

## Management Commands

```bash
./deploy/scripts/deploy-vps.sh status    # check service health
./deploy/scripts/deploy-vps.sh logs      # follow logs
./deploy/scripts/deploy-vps.sh restart   # restart all services
./deploy/scripts/deploy-vps.sh stop      # stop everything
./deploy/scripts/deploy-vps.sh register  # register a test agent
```

## Backup & Recovery

```bash
# Backup database
docker exec eyevesa-postgres pg_dump -U agentid agentid > backup_$(date +%Y%m%d).sql

# Restore
cat backup_20260518.sql | docker exec -i eyevesa-postgres psql -U agentid agentid
```