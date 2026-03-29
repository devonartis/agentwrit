#!/usr/bin/env bash
set -euo pipefail

# stack_down.sh — tears down broker docker stack.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"
docker compose down -v --remove-orphans
echo "Stack is down."
