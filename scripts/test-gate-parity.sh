#!/usr/bin/env bash
set -euo pipefail

# test-gate-parity.sh — enforce local/CI gate list alignment
#
# Reads gate IDs from two sources:
#   A. scripts/gates.sh --list-gates          (single source of truth locally)
#   B. .github/workflows/ci.yml GATE_LIST     (single source of truth in CI)
#
# Fails if the two disagree. Prevents local and CI gate definitions from
# silently drifting — a developer who adds a gate locally but forgets
# ci.yml will see this script fail and know to update both.
#
# Used by:
#   - scripts/gates.sh full (indirectly, via the gate-parity job)
#   - .github/workflows/ci.yml gate-parity job

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

GATES_SH="$SCRIPT_DIR/gates.sh"
CI_YML="$REPO_ROOT/.github/workflows/ci.yml"

if [[ ! -x "$GATES_SH" ]]; then
  echo "FAIL: $GATES_SH not found or not executable"
  exit 1
fi

if [[ ! -f "$CI_YML" ]]; then
  echo "FAIL: $CI_YML not found (run this after ci.yml is created in Task 14)"
  exit 1
fi

# Source A: gates.sh --list-gates
GATES_FROM_SCRIPT=$("$GATES_SH" --list-gates | sort)

# Source B: ci.yml GATE_LIST_START/END block.
# The ci.yml has a comment block listing the canonical gate IDs:
#     # GATE_LIST_START
#     # - build
#     # - vet
#     # ...
#     # GATE_LIST_END
# We parse this block so the list is a single source of truth per file.
GATES_FROM_CI=$(awk '
  /# GATE_LIST_START/ { in_block=1; next }
  /# GATE_LIST_END/   { in_block=0; next }
  in_block && /^# - / { sub(/^# - /, ""); print }
' "$CI_YML" | sort)

if [[ -z "$GATES_FROM_CI" ]]; then
  echo "FAIL: no GATE_LIST_START/END block found in $CI_YML"
  exit 1
fi

# Diff
if diff <(echo "$GATES_FROM_SCRIPT") <(echo "$GATES_FROM_CI") >/dev/null; then
  count=$(echo "$GATES_FROM_SCRIPT" | wc -l | tr -d ' ')
  echo "PASS: gate lists match ($count gates)"
  exit 0
else
  echo "FAIL: gates.sh and ci.yml disagree on the gate list"
  echo ""
  echo "--- gates.sh --list-gates ---"
  echo "$GATES_FROM_SCRIPT"
  echo ""
  echo "--- ci.yml GATE_LIST block ---"
  echo "$GATES_FROM_CI"
  echo ""
  echo "Diff (gates.sh vs ci.yml):"
  diff <(echo "$GATES_FROM_SCRIPT") <(echo "$GATES_FROM_CI") || true
  exit 1
fi
