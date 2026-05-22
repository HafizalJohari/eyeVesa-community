# AGENT_INSTALL.md

This file is for AI agents helping a user install eyeVesa Community.

Your job is to install and verify a local community sandbox. Do not ask for production credentials. Do not connect to the official International Airport unless the user explicitly provides an invite or API key.

## User Intent

Install and run eyeVesa from:

```text
https://github.com/HafizalJohari/eyeVesa-community.git
```

Use local community mode only.

## Safety Rules

- Do not ask for GCP credentials.
- Do not ask for Terraform state.
- Do not request production database passwords, JWT secrets, gateway private keys, or API keys.
- Do not commit `.env`, `.tfvars`, `.tfstate`, private keys, or generated secrets.
- Do not connect to production or the official International Airport unless the user explicitly provides scoped credentials.
- If credentials are needed for a production task, tell the user to use a password manager or secret manager, not git.

## Prerequisites

Check that the machine has:

- Git
- Docker
- Docker Compose
- A shell that can run `./start.sh`

If Docker is not running, ask the user to start Docker Desktop or the Docker daemon.

## Install Steps

```bash
git clone https://github.com/HafizalJohari/eyeVesa-community.git
cd eyevesa-community
./start.sh
```

If the repo already exists:

```bash
cd eyevesa-community
git pull --ff-only
./start.sh
```

## Verify Setup

Run these checks:

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:9443/health
curl -fsS http://localhost:8080/v1/airport/health
eyevesa doctor --gateway http://localhost:8080
```

Expected success signals:

- `./start.sh` completes without fatal errors.
- Docker containers are running.
- `http://localhost:8080/health` responds successfully.
- `http://localhost:9443/health` responds successfully.
- `http://localhost:8080/v1/airport/health` responds successfully.
- `eyevesa doctor --gateway http://localhost:8080` can reach the local gateway.
- The user can access the local services listed below.

## Report Back To The User

When setup succeeds, report:

```text
eyeVesa Community is running locally.

Control plane: http://localhost:8080
Gateway proxy: http://localhost:9443
OPA policy server: http://localhost:8181
Health: http://localhost:8080/health
Gateway health: http://localhost:9443/health
Airport health: http://localhost:8080/v1/airport/health
CLI doctor: eyevesa doctor --gateway http://localhost:8080

This is a local sandbox only. It is not connected to the official International Airport.
```

## Troubleshooting

Check container status:

```bash
docker compose ps
```

Read recent logs:

```bash
docker compose logs --tail=80
```

Stop the sandbox:

```bash
docker compose down
```

If a port is already in use, tell the user which port is blocked and ask whether they want to stop the conflicting process or change the local port mapping.

If a health check fails, report the failing command, the response or error, and the most relevant recent Docker logs.

## Boundaries

Community setup is for learning, local testing, and agent integration experiments. Official International Airport access is operator-controlled and invite/API-key gated.
