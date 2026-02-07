#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE="${1:-}"

if [[ ! "$MODULE" =~ ^M[0-9]{2}$ ]]; then
  echo "usage: ./scripts/set_active_module.sh MNN"
  exit 1
fi

printf "%s\n" "$MODULE" >"$ROOT/.active_module"
echo "[MODULE] active module set to $MODULE"

