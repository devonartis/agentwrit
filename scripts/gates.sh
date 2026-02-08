#!/usr/bin/env bash
set -euo pipefail

LEVEL="${1:-task}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

red() { printf "\033[31m%s\033[0m\n" "$1"; }
green() { printf "\033[32m%s\033[0m\n" "$1"; }

run_gate() {
  local name="$1"
  shift
  printf "[GATE] %s\n" "$name"
  if "$@"; then
    green "[GATE:PASS] $name"
  else
    red "[GATE:FAIL] $name"
    return 1
  fi
}

build_gate() {
  (cd "$ROOT" && go build ./...)
}

lint_gate() {
  if command -v golangci-lint >/dev/null 2>&1; then
    (cd "$ROOT" && golangci-lint run ./...)
  else
    echo "[GATE:WARN] golangci-lint not installed; skipping"
  fi

  if command -v ruff >/dev/null 2>&1; then
    (cd "$ROOT" && ruff check demo)
  else
    echo "[GATE:WARN] ruff not installed; skipping"
  fi
}

security_gate() {
  if ! command -v gosec >/dev/null 2>&1; then
    echo "[GATE:FAIL] gosec not installed; install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"
    return 1
  fi
  if ! command -v govulncheck >/dev/null 2>&1; then
    echo "[GATE:FAIL] govulncheck not installed; install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
    return 1
  fi
  (cd "$ROOT" && gosec ./...)
  (cd "$ROOT" && govulncheck ./...)
}

unit_gate() {
  (cd "$ROOT" && go test ./... -short)
}

integration_gate() {
  "$ROOT/scripts/integration_test.sh"
}

live_gate() {
  "$ROOT/scripts/live_test.sh"
}

gitflow_gate() {
  "$ROOT/scripts/gitflow_check.sh" "$LEVEL"
}

doc_gate() {
  "$ROOT/scripts/doc_check.sh"
}

task_level() {
  run_gate GITFLOW gitflow_gate
  run_gate BUILD build_gate
  run_gate LINT lint_gate
  run_gate SECURITY security_gate
  run_gate UNIT unit_gate
  run_gate DOC doc_gate
}

module_level() {
  task_level
  run_gate INTEGRATION integration_gate
  run_gate LIVE live_gate
  run_gate REGRESSION unit_gate
}

milestone_level() {
  module_level
  run_gate E2E test -f "$ROOT/tests/e2e/.keep"
}

case "$LEVEL" in
  task) task_level ;;
  module) module_level ;;
  milestone) milestone_level ;;
  all) milestone_level ;;
  *)
    echo "usage: ./scripts/gates.sh [task|module|milestone|all]"
    exit 1
    ;;
esac
