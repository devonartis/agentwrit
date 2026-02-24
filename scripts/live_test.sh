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

run_tls() {
  local PORT BROKER_BIN BROKER_PID BASE READY CERT_DIR
  PORT=$((RANDOM % 10000 + 20000))
  BROKER_BIN="${TMPDIR:-/tmp}/agentauth-broker-tls-${PORT}"
  BROKER_PID=""
  BASE="https://127.0.0.1:${PORT}"
  READY=false
  CERT_DIR="$(mktemp -d)"

  cleanup_tls() {
    if [[ -n "${BROKER_PID:-}" ]]; then
      kill "$BROKER_PID" 2>/dev/null || true
      wait "$BROKER_PID" 2>/dev/null || true
    fi
    rm -rf "$CERT_DIR"
  }
  trap cleanup_tls EXIT

  echo "=== Live Test (tls): generating self-signed cert ==="
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$CERT_DIR/key.pem" \
    -out "$CERT_DIR/cert.pem" \
    -days 1 -nodes \
    -subj "/CN=127.0.0.1" \
    -addext "subjectAltName=IP:127.0.0.1" \
    2>/dev/null
  echo "  cert: $CERT_DIR/cert.pem"

  echo "=== Live Test (tls): build broker ==="
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOCACHE="${TMPDIR:-/tmp}/agentauth-go-build" \
    go build -o "$BROKER_BIN" "$PROJECT_ROOT/cmd/broker"

  echo "=== Live Test (tls): start broker with AA_TLS_MODE=tls on port $PORT ==="
  AA_PORT="$PORT" \
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_TLS_MODE=tls \
  AA_TLS_CERT="$CERT_DIR/cert.pem" \
  AA_TLS_KEY="$CERT_DIR/key.pem" \
  AA_LOG_LEVEL=quiet \
    "$BROKER_BIN" >/tmp/agentauth_live_tls.log 2>&1 &
  BROKER_PID=$!

  for _ in $(seq 1 50); do
    if curl -sf --cacert "$CERT_DIR/cert.pem" "$BASE/v1/health" >/dev/null 2>&1; then
      READY=true
      break
    fi
    sleep 0.1
  done

  if [[ "$READY" != "true" ]]; then
    echo "FAIL: broker did not become ready over TLS on port $PORT"
    cat /tmp/agentauth_live_tls.log || true
    exit 1
  fi

  echo "=== Live Test (tls): broker responding over HTTPS — verifying health response ==="
  HEALTH=$(curl -sf --cacert "$CERT_DIR/cert.pem" "$BASE/v1/health")
  echo "  health: $HEALTH"
  if ! echo "$HEALTH" | grep -q '"status"'; then
    echo "FAIL: unexpected health response: $HEALTH"
    exit 1
  fi

  echo ""
  echo "PASS: broker accepted HTTPS connection with AA_TLS_MODE=tls"

  # Story 4: missing cert files should cause broker to fail at startup
  echo ""
  echo "=== Live Test (tls): Story 4 — bad cert path should fail at startup ==="
  local BAD_PID BAD_PORT
  BAD_PORT=$((RANDOM % 10000 + 30000))
  AA_PORT="$BAD_PORT" \
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_TLS_MODE=tls \
  AA_TLS_CERT="/nonexistent/cert.pem" \
  AA_TLS_KEY="/nonexistent/key.pem" \
  AA_LOG_LEVEL=quiet \
    "$BROKER_BIN" >/tmp/agentauth_live_tls_bad.log 2>&1 &
  BAD_PID=$!
  sleep 0.5
  if kill -0 "$BAD_PID" 2>/dev/null; then
    echo "FAIL: broker should have exited with missing cert files but is still running"
    kill "$BAD_PID" 2>/dev/null || true
    exit 1
  fi
  echo "PASS: broker exited on missing cert files (as expected)"

  echo ""
  echo "=== Broker log evidence (last 10 lines) ==="
  tail -10 /tmp/agentauth_live_tls.log || true
}

run_mtls() {
  local PORT BROKER_BIN BROKER_PID BASE READY CERT_DIR CLIENT_CERT_DIR
  PORT=$((RANDOM % 10000 + 20000))
  BROKER_BIN="${TMPDIR:-/tmp}/agentauth-broker-mtls-${PORT}"
  BROKER_PID=""
  BASE="https://127.0.0.1:${PORT}"
  READY=false
  CERT_DIR="$(mktemp -d)"
  CLIENT_CERT_DIR="$(mktemp -d)"

  cleanup_mtls() {
    if [[ -n "${BROKER_PID:-}" ]]; then
      kill "$BROKER_PID" 2>/dev/null || true
      wait "$BROKER_PID" 2>/dev/null || true
    fi
    rm -rf "$CERT_DIR" "$CLIENT_CERT_DIR"
  }
  trap cleanup_mtls EXIT

  echo "=== Live Test (mtls): generating CA, server cert, and client cert ==="
  # CA
  openssl req -x509 -newkey rsa:2048 \
    -keyout "$CERT_DIR/ca-key.pem" -out "$CERT_DIR/ca-cert.pem" \
    -days 1 -nodes -subj "/CN=test-ca" 2>/dev/null
  # Server cert signed by CA
  openssl req -newkey rsa:2048 \
    -keyout "$CERT_DIR/server-key.pem" -out "$CERT_DIR/server-csr.pem" \
    -nodes -subj "/CN=127.0.0.1" 2>/dev/null
  openssl x509 -req -in "$CERT_DIR/server-csr.pem" \
    -CA "$CERT_DIR/ca-cert.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial \
    -out "$CERT_DIR/server-cert.pem" -days 1 \
    -extfile <(echo "subjectAltName=IP:127.0.0.1") 2>/dev/null
  # Client cert signed by same CA
  openssl req -newkey rsa:2048 \
    -keyout "$CLIENT_CERT_DIR/client-key.pem" -out "$CLIENT_CERT_DIR/client-csr.pem" \
    -nodes -subj "/CN=test-client" 2>/dev/null
  openssl x509 -req -in "$CLIENT_CERT_DIR/client-csr.pem" \
    -CA "$CERT_DIR/ca-cert.pem" -CAkey "$CERT_DIR/ca-key.pem" -CAcreateserial \
    -out "$CLIENT_CERT_DIR/client-cert.pem" -days 1 2>/dev/null
  echo "  CA, server cert, and client cert generated"

  echo "=== Live Test (mtls): build broker ==="
  GOTOOLCHAIN="${GOTOOLCHAIN:-local}" GOCACHE="${TMPDIR:-/tmp}/agentauth-go-build" \
    go build -o "$BROKER_BIN" "$PROJECT_ROOT/cmd/broker"

  echo "=== Live Test (mtls): start broker with AA_TLS_MODE=mtls on port $PORT ==="
  AA_PORT="$PORT" \
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_TLS_MODE=mtls \
  AA_TLS_CERT="$CERT_DIR/server-cert.pem" \
  AA_TLS_KEY="$CERT_DIR/server-key.pem" \
  AA_TLS_CLIENT_CA="$CERT_DIR/ca-cert.pem" \
  AA_LOG_LEVEL=quiet \
    "$BROKER_BIN" >/tmp/agentauth_live_mtls.log 2>&1 &
  BROKER_PID=$!

  for _ in $(seq 1 50); do
    if curl -sf \
        --cacert "$CERT_DIR/ca-cert.pem" \
        --cert "$CLIENT_CERT_DIR/client-cert.pem" \
        --key "$CLIENT_CERT_DIR/client-key.pem" \
        "$BASE/v1/health" >/dev/null 2>&1; then
      READY=true
      break
    fi
    sleep 0.1
  done

  if [[ "$READY" != "true" ]]; then
    echo "FAIL: broker did not become ready over mTLS on port $PORT"
    cat /tmp/agentauth_live_mtls.log || true
    exit 1
  fi

  echo "=== Live Test (mtls): Story 2 — client WITH cert should succeed ==="
  HEALTH=$(curl -sf \
    --cacert "$CERT_DIR/ca-cert.pem" \
    --cert "$CLIENT_CERT_DIR/client-cert.pem" \
    --key "$CLIENT_CERT_DIR/client-key.pem" \
    "$BASE/v1/health")
  if ! echo "$HEALTH" | grep -q '"status"'; then
    echo "FAIL: unexpected health response with valid client cert: $HEALTH"
    exit 1
  fi
  echo "PASS: valid client cert accepted"

  echo "=== Live Test (mtls): Story 3 — client WITHOUT cert should be rejected ==="
  HTTP_CODE=$(curl -sk --cacert "$CERT_DIR/ca-cert.pem" \
    -o /dev/null -w "%{http_code}" "$BASE/v1/health" 2>/dev/null || echo "000")
  if [[ "$HTTP_CODE" == "200" ]]; then
    echo "FAIL: broker accepted connection without client cert (expected TLS handshake failure)"
    exit 1
  fi
  echo "PASS: connection without client cert rejected (http_code=$HTTP_CODE)"

  echo ""
  echo "PASS: broker enforces mTLS with AA_TLS_MODE=mtls"
  echo ""
  echo "=== Broker log evidence (last 10 lines) ==="
  tail -10 /tmp/agentauth_live_mtls.log || true
}

if [[ "${1:-}" == "--docker" ]]; then
  run_docker
elif [[ "${1:-}" == "--self-host" ]]; then
  run_self_host
elif [[ "${1:-}" == "--tls" ]]; then
  run_tls
elif [[ "${1:-}" == "--mtls" ]]; then
  run_mtls
else
  run_external
fi
