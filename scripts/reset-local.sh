#!/usr/bin/env bash
set -euo pipefail

PURGE_ENV=false
for arg in "$@"; do
  case "$arg" in
    --purge-env) PURGE_ENV=true ;;
    -h|--help)
      echo "Usage: ./scripts/reset-local.sh [--purge-env]"
      echo "Stops the local eyeVesa compose stack and removes its volumes."
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      exit 1
      ;;
  esac
done

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

echo "Stopping local eyeVesa Docker Compose stack and removing volumes..."
docker compose down -v
echo "Removed Docker Compose containers, network, and volumes for this project."

if [ "$PURGE_ENV" = true ]; then
  if [ -f .env ]; then
    rm .env
    echo "Removed local .env."
  else
    echo "No local .env found."
  fi
else
  echo "Kept local .env. Pass --purge-env to remove it."
fi

echo "Local sandbox reset complete. Production resources were not touched."
