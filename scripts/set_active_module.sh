#!/usr/bin/env bash
set -euo pipefail

# set_active_module.sh — sets the active module context
# Usage: ./scripts/set_active_module.sh M02

MODULE="${1:-}"
if [[ -z "$MODULE" ]]; then
  echo "Usage: $0 <MODULE>"
  echo "Example: $0 M02"
  exit 1
fi

# Validate format: M followed by two digits
if [[ ! "$MODULE" =~ ^M[0-9]{2}$ ]]; then
  echo "Error: module must match pattern MXX (e.g., M00, M02, M14)"
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
echo -n "$MODULE" > "$REPO_ROOT/.active_module"
echo "Active module set to: $MODULE"
