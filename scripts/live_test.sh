#!/usr/bin/env bash
set -euo pipefail

# live_test.sh
#
# Default: external broker smoke test (does NOT start/stop backend).
#   Runs the Go-based smoketest binary that exercises the full sidecar
#   lifecycle (12 steps including challenge-response, activation, exchange,
#   replay denial, scope escalation denial, and token validation).
#
# Optional: --self-host builds + starts broker locally, then runs smoketest.
# Optional: --docker delegates to live_test_docker.sh.
#
# Usage:
#   ./scripts/live_test.sh                       # external (broker at $AA_LIVE_BASE_URL)
#   ./scripts/live_test.sh --self-host            # build + start broker, then test
#   ./scripts/live_test.sh --docker               # docker compose mode
# Env:
#   AA_LIVE_BASE_URL   (default: http://127.0.0.1:8080)
#   AA_ADMIN_SECRET    (default: live-test-secret-32bytes-long!!)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BASE_URL="${AA_LIVE_BASE_URL:-http://127.0.0.1:8080}"
ADMIN_SECRET="${AA_ADMIN_SECRET:-live-test-secret-32bytes-long!!}"

build_smoketest() {
  local bin="$1"
  echo "=== Build smoketest binary ==="
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOCACHE="${TMPDIR:-/tmp}/agentauth-go-build" \
    go build -o "$bin" "$PROJECT_ROOT/cmd/smoketest"
}

run_external() {
  local SMOKETEST_BIN="${TMPDIR:-/tmp}/agentauth-smoketest-ext"
  build_smoketest "$SMOKETEST_BIN"

  echo "=== Live Test (external): broker must already be running at $BASE_URL ==="
  "$SMOKETEST_BIN" "$BASE_URL" "$ADMIN_SECRET"
}

run_self_host() {
  local PORT BROKER_BIN SMOKETEST_BIN BROKER_PID BASE READY
  PORT=$((RANDOM % 10000 + 20000))
  BROKER_BIN="${TMPDIR:-/tmp}/agentauth-broker-${PORT}"
  SMOKETEST_BIN="${TMPDIR:-/tmp}/agentauth-smoketest-${PORT}"
  BROKER_PID=""
  BASE="http://127.0.0.1:${PORT}"
  READY=false

  cleanup_local() {
    if [[ -n "${BROKER_PID:-}" ]]; then
      kill "$BROKER_PID" 2>/dev/null || true
      wait "$BROKER_PID" 2>/dev/null || true
    fi
  }
  trap cleanup_local EXIT

  echo "=== Live Test (self-host): build broker + smoketest ==="
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOCACHE="${TMPDIR:-/tmp}/agentauth-go-build" \
    go build -o "$BROKER_BIN" "$PROJECT_ROOT/cmd/broker"
  build_smoketest "$SMOKETEST_BIN"

  echo "=== Live Test (self-host): start broker on port $PORT ==="
  AA_PORT="$PORT" \
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_SEED_TOKENS=true \
  AA_LOG_LEVEL=quiet \
    "$BROKER_BIN" >/tmp/agentauth_live_local.log 2>&1 &
  BROKER_PID=$!

  for _ in $(seq 1 50); do
    if curl -sf "$BASE/v1/health" >/dev/null 2>&1; then
      READY=true
      break
    fi
    sleep 0.1
  done

  if [[ "$READY" != "true" ]]; then
    echo "FAIL: broker did not become ready on port $PORT"
    cat /tmp/agentauth_live_local.log || true
    exit 1
  fi

  echo "=== Live Test (self-host): run smoketest ==="
  "$SMOKETEST_BIN" "$BASE" "$ADMIN_SECRET"

  echo ""
  echo "=== Broker log evidence (last 30 lines) ==="
  tail -30 /tmp/agentauth_live_local.log || true
}

run_docker() {
  "$PROJECT_ROOT/scripts/live_test_docker.sh"
}

if [[ "${1:-}" == "--docker" ]]; then
  run_docker
elif [[ "${1:-}" == "--self-host" ]]; then
  run_self_host
else
  run_external
fi
