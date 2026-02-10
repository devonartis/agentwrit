#!/usr/bin/env bash
set -euo pipefail

# stack_up.sh — one-command startup for broker + sidecar docker stack.
# Usage:
#   ./scripts/stack_up.sh
#
# Optional env:
#   AA_ADMIN_SECRET   (default: change-me-in-production)
#   AA_HOST_PORT      (default: 8080)
#   AA_SEED_TOKENS    (default: false)
#   AA_LOG_LEVEL      (default: standard)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"
docker compose up -d --build broker sidecar
echo "Stack is up."
echo "Broker health: curl http://127.0.0.1:${AA_HOST_PORT:-8080}/v1/health"
