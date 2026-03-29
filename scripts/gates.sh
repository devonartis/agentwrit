#!/usr/bin/env bash
set -euo pipefail

# gates.sh — quality gate runner for AgentAuth
# Usage: ./scripts/gates.sh task   (build + vet/lint + unit tests + security)
#        ./scripts/gates.sh module (task gates + full test suite + Docker E2E)

MODE="${1:-}"
if [[ -z "$MODE" ]]; then
  echo "Usage: $0 {task|module}"
  exit 1
fi

if [[ "$MODE" != "task" && "$MODE" != "module" ]]; then
  echo "Error: unknown mode '$MODE'. Use 'task' or 'module'."
  exit 1
fi

PASS=0
FAIL=0
WARN=0
SKIP=0

run_gate() {
  local name="$1"
  shift
  echo ""
  echo "=== GATE: $name ==="
  if "$@"; then
    echo "--- PASS: $name ---"
    PASS=$((PASS + 1))
  else
    echo "--- FAIL: $name ---"
    FAIL=$((FAIL + 1))
  fi
}

warn_gate() {
  local name="$1"
  shift
  echo ""
  echo "=== GATE: $name ==="
  if "$@"; then
    echo "--- PASS: $name ---"
    PASS=$((PASS + 1))
  else
    echo "--- WARN: $name (non-blocking) ---"
    WARN=$((WARN + 1))
  fi
}

skip_gate() {
  local name="$1"
  local reason="$2"
  echo ""
  echo "=== GATE: $name ==="
  echo "--- SKIP: $reason ---"
  SKIP=$((SKIP + 1))
}

# --- TASK gates ---

run_gate "build" go build ./...

# Lint: prefer golangci-lint, fall back to go vet
if command -v golangci-lint &>/dev/null; then
  run_gate "lint" golangci-lint run ./...
elif go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest --version &>/dev/null 2>&1; then
  run_gate "lint" go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run ./...
else
  run_gate "lint (vet fallback)" go vet ./...
fi

run_gate "unit tests" go test ./... -short -count=1

# Security: gosec (advisory — warns but does not block)
if command -v gosec &>/dev/null; then
  warn_gate "security (gosec)" gosec -quiet ./...
elif go run github.com/securego/gosec/v2/cmd/gosec@latest -version &>/dev/null 2>&1; then
  warn_gate "security (gosec)" go run github.com/securego/gosec/v2/cmd/gosec@latest -quiet ./...
else
  skip_gate "security (gosec)" "gosec not installed — skipping"
fi

# --- MODULE gates (only if mode is module) ---

if [[ "$MODE" == "module" ]]; then
  run_gate "full tests" go test ./... -count=1

  # Live/E2E: start the broker and run HTTP smoke tests
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  if [[ -x "$SCRIPT_DIR/live_test.sh" ]]; then
    run_gate "live tests (broker)" "$SCRIPT_DIR/live_test.sh"
  else
    skip_gate "live tests (broker)" "scripts/live_test.sh not found or not executable"
  fi

  # Docker live tests: deterministic gates — if Docker is available, these MUST pass.
  if docker info >/dev/null 2>&1; then
    if [[ -x "$SCRIPT_DIR/live_test_docker.sh" ]]; then
      run_gate "live tests (broker docker)" "$SCRIPT_DIR/live_test_docker.sh"
    fi
  else
    skip_gate "live tests (docker)" "Docker daemon not running — skipping Docker E2E gates"
  fi
fi

# --- Summary ---

echo ""
echo "==============================="
echo "  GATE SUMMARY ($MODE mode)"
echo "==============================="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  WARN: $WARN"
echo "  SKIP: $SKIP"
echo "==============================="

if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  exit 1
else
  echo "RESULT: PASSED"
  exit 0
fi
