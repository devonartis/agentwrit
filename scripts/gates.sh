#!/usr/bin/env bash
set -euo pipefail

# gates.sh — quality gate runner for AgentAuth (M-sec)
#
# Usage:
#   ./scripts/gates.sh task          Fast dev-loop gates (build/vet/lint/format/
#                                    contamination/short tests/security)
#   ./scripts/gates.sh full          Full CI-mirror gates (task + race tests +
#                                    docker-build + smoke-l25 + sbom)
#   ./scripts/gates.sh regression    L4 full regression — iterate tests/*/regression.sh
#   ./scripts/gates.sh --list-gates  Print gate IDs one-per-line for parity test
#
# 'module' is retained as a deprecated alias for 'full'.
#
# Local/CI parity: this script's gate IDs must match ci.yml's GATE_LIST block.
# scripts/test-gate-parity.sh enforces this.

MODE="${1:-}"

# Authoritative gate list — single source of truth.
# scripts/test-gate-parity.sh reads this array; ci.yml's GATE_LIST comment
# block mirrors the same strings. If you add/remove/rename a gate, update BOTH.
GATES_TASK=(
  build
  vet
  lint
  format
  contamination
  unit-tests
  gosec
  govulncheck
  go-mod-verify
)
GATES_FULL=(
  "${GATES_TASK[@]}"
  unit-tests-race
  docker-build
  smoke-l25
  sbom
  no-tracked-ignored
)

if [[ "$MODE" == "--list-gates" ]]; then
  for g in "${GATES_FULL[@]}"; do echo "$g"; done
  exit 0
fi

if [[ -z "$MODE" ]]; then
  echo "Usage: $0 {task|full|regression|--list-gates}"
  exit 1
fi

# Alias: module -> full (deprecated)
if [[ "$MODE" == "module" ]]; then
  echo "NOTE: 'module' is deprecated, use 'full'." >&2
  MODE="full"
fi

if [[ "$MODE" != "task" && "$MODE" != "full" && "$MODE" != "regression" ]]; then
  echo "Error: unknown mode '$MODE'. Use 'task', 'full', 'regression', or --list-gates."
  exit 1
fi

PASS=0
FAIL=0
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

skip_gate() {
  local name="$1"
  local reason="$2"
  echo ""
  echo "=== GATE: $name ==="
  echo "--- SKIP: $reason ---"
  SKIP=$((SKIP + 1))
}

# gosec exclusions — documented in .gosec.yml. Every excluded rule has a
# documented rationale there. Keep the flags here in sync with ci.yml's gosec
# job AND with the linters-settings.gosec.excludes block in .golangci.yml.
GOSEC_EXCLUDE="G117,G304,G101"

# --- TASK gates ---

run_gate "build" go build ./cmd/broker ./cmd/awrit

run_gate "vet" go vet ./...

# Lint: require golangci-lint (no fallback — M-sec policy)
if command -v golangci-lint &>/dev/null; then
  run_gate "lint" golangci-lint run ./...
else
  echo "ERROR: golangci-lint not installed. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
  exit 1
fi

# Format: gofmt -l must return empty
run_gate "format" bash -c 'test -z "$(gofmt -l .)"'

# Contamination: zero enterprise refs in core, zero stale brand strings.
# Brand alignment portion catches the gap that let obs.go metric names and
# problemdetails.go URN namespace survive the first rebrand pass
# (TD-RUNTIME-001 / TD-OBS-001).
run_gate "contamination" bash -c "! grep -ri 'hitl\|approval\|oidc\|federation\|cloud\|sidecar' internal/ cmd/ 2>/dev/null && ! grep -rn 'urn:agentauth\|agentauth_\|github\.com/devonartis/agentauth[^-]' internal/ cmd/ --include='*.go' 2>/dev/null && ! find cmd/ internal/ -name '*.go' -exec sh -c 'head -1 \"\$1\" | grep -qxF \"// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0\" || echo \"\$1\"' _ {} \; | grep ."

run_gate "unit-tests" go test -short -count=1 ./...

# Security: gosec (BLOCKING — flipped from warn per Decision 015)
if command -v gosec &>/dev/null; then
  run_gate "gosec" gosec -quiet -conf .gosec.yml -exclude="$GOSEC_EXCLUDE" -severity=medium ./...
else
  echo "ERROR: gosec not installed. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"
  exit 1
fi

# Vulnerability check: govulncheck (BLOCKING)
if command -v govulncheck &>/dev/null; then
  run_gate "govulncheck" govulncheck ./...
else
  echo "ERROR: govulncheck not installed. Install: go install golang.org/x/vuln/cmd/govulncheck@latest"
  exit 1
fi

# Module integrity + tidy drift
run_gate "go-mod-verify" bash -c 'go mod verify && go mod tidy && git diff --exit-code go.mod go.sum'

# --- FULL gates (only if mode is full) ---

if [[ "$MODE" == "full" ]]; then
  run_gate "unit-tests-race" go test -race -count=1 -coverprofile=coverage.out ./...

  # Docker build: multi-stage image builds cleanly
  if docker info >/dev/null 2>&1; then
    run_gate "docker-build" docker build -t agentwrit-ci:local .
  else
    skip_gate "docker-build" "Docker daemon not running"
  fi

  # L2.5 smoke: core contract (issue/verify/revoke/deny)
  # Honors BROKER_URL so an operator can point at an already-running
  # broker on a non-default port (e.g. when 8080 is held by another
  # project). Default matches scripts/stack_up.sh default mapping.
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  SMOKE_BROKER_URL="${BROKER_URL:-http://localhost:8080}"
  if [[ -x "$SCRIPT_DIR/smoke/core-contract.sh" ]]; then
    if docker info >/dev/null 2>&1; then
      if curl -sf "$SMOKE_BROKER_URL/v1/health" >/dev/null 2>&1; then
        run_gate "smoke-l25" env BROKER_URL="$SMOKE_BROKER_URL" "$SCRIPT_DIR/smoke/core-contract.sh"
      else
        skip_gate "smoke-l25" "broker not reachable at $SMOKE_BROKER_URL — run scripts/stack_up.sh first"
      fi
    else
      skip_gate "smoke-l25" "Docker daemon not running"
    fi
  else
    skip_gate "smoke-l25" "scripts/smoke/core-contract.sh not found or not executable"
  fi

  # SBOM: syft SPDX output. `syft scan` replaced `syft packages` in 1.x.
  if command -v syft &>/dev/null; then
    run_gate "sbom" syft scan dir:. -o spdx-json=sbom.spdx.json --quiet
  else
    skip_gate "sbom" "syft not installed — brew install syft or https://github.com/anchore/syft"
  fi

  # no-tracked-ignored: fail if any gitignored path is in the git index.
  # Replaces the old main-hygiene + strip_for_main.sh model (Decision 019).
  # Runs on every branch — .gitignore prevents accidental `git add .`, this
  # catches the narrow `git add -f` force-add failure mode everywhere.
  run_gate "no-tracked-ignored" bash -c '
    tracked_ignored=$(git ls-files -c --exclude-standard --ignored 2>/dev/null || true)
    if [[ -n "$tracked_ignored" ]]; then
      echo "FAIL: gitignored paths found in the git index:"
      echo "$tracked_ignored" | sed "s/^/  /"
      exit 1
    fi
    exit 0
  '
fi

# --- REGRESSION gates (only if mode is regression) ---

if [[ "$MODE" == "regression" ]]; then
  echo ""
  echo "=== REGRESSION: Running all previous phase tests ==="
  reg_pass=0
  reg_fail=0
  for test_dir in tests/*/; do
    phase=$(basename "$test_dir")
    runner=""
    if [ -f "$test_dir/regression.sh" ]; then
      runner="$test_dir/regression.sh"
    else
      echo "  SKIP $phase (no regression.sh runner)"
      continue
    fi
    echo "  RUN  $phase ($runner)"
    if bash "$runner"; then
      echo "  PASS $phase"
      reg_pass=$((reg_pass + 1))
    else
      echo "  FAIL $phase"
      reg_fail=$((reg_fail + 1))
    fi
  done
  echo ""
  echo "=== REGRESSION SUMMARY: $reg_pass passed, $reg_fail failed ==="
  if [[ $reg_fail -gt 0 ]]; then
    echo "RESULT: FAILED"
    exit 1
  else
    echo "RESULT: PASSED"
    exit 0
  fi
fi

# --- Summary ---

echo ""
echo "==============================="
echo "  GATE SUMMARY ($MODE mode)"
echo "==============================="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  SKIP: $SKIP"
echo "==============================="

if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  exit 1
else
  echo "RESULT: PASSED"
  exit 0
fi
