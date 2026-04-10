#!/usr/bin/env bash
set -euo pipefail

# stack_up.sh — start the broker docker stack.
# Usage:
#   ./scripts/stack_up.sh
#
# Required env:
#   AA_ADMIN_SECRET   (no default — broker rejects weak/empty secrets at startup.
#                     Use `awrit init` to generate, or export a strong value.)
#
# Optional env:
#   AA_HOST_PORT      (default: 8080)
#   AA_SEED_TOKENS    (default: false)
#   AA_LOG_LEVEL      (default: standard)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"
docker compose build --no-cache broker
docker compose up -d broker
echo "Stack is up."
echo "Broker health: curl http://127.0.0.1:${AA_HOST_PORT:-8080}/v1/health"
