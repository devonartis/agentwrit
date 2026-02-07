#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-task}"
ACTIVE_MODULE="${AA_ACTIVE_MODULE:-}"

fail() {
  echo "[GITFLOW:FAIL] $1"
  exit 1
}

pass() {
  echo "[GITFLOW:PASS] $1"
}

cd "$ROOT"

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || fail "not a git repository"
branch="$(git symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
[ -n "$branch" ] || fail "detached HEAD is not allowed"

has_head=true
if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
  has_head=false
fi

if [ "$has_head" = true ]; then
  git show-ref --verify --quiet refs/heads/develop || fail "missing local develop branch"
fi

resolve_active_module() {
  if [ -n "$ACTIVE_MODULE" ]; then
    echo "$ACTIVE_MODULE"
    return
  fi

  local module_file="$ROOT/.active_module"
  [ -f "$module_file" ] || fail "missing .active_module (set active module, e.g. M01)"
  local raw
  raw="$(tr -d '[:space:]' <"$module_file")"
  [ -n "$raw" ] || fail ".active_module is empty"
  echo "$raw"
}

check_module_branch_alignment() {
  local module="$1"
  [[ "$module" =~ ^M[0-9]{2}$ ]] || fail "active module must match MNN format (got: $module)"
  local module_branch
  module_branch="$(echo "$module" | tr '[:upper:]' '[:lower:]')"
  [[ "$branch" =~ ^feature/${module_branch}(-.*)?$ ]] || fail "branch/module mismatch: expected feature/${module_branch}-* for active module $module (got: $branch)"
}

case "$MODE" in
  task|module)
    [[ "$branch" =~ ^feature/ ]] || fail "current branch must match feature/* for $MODE checks (got: $branch)"
    active_module="$(resolve_active_module)"
    check_module_branch_alignment "$active_module"
    if [ "$has_head" = true ]; then
      git merge-base --is-ancestor develop HEAD || fail "feature branch must be based on develop"
    fi
    pass "branch=$branch mode=$MODE active_module=$active_module"
    ;;
  milestone|all)
    [[ "$branch" =~ ^(develop|release/.+|hotfix/.+) ]] || fail "branch must be develop, release/*, or hotfix/* for $MODE (got: $branch)"
    pass "branch=$branch mode=$MODE"
    ;;
  *)
    fail "unknown mode: $MODE"
    ;;
esac
