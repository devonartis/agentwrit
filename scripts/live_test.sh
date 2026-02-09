#!/usr/bin/env bash
set -euo pipefail

# live_test.sh — starts the broker and runs live smoke tests against it.
# Usage: ./scripts/live_test.sh
# Exit 0 on success, non-zero on failure.
#
# The script picks a random high port, starts the broker with seed tokens
# enabled, waits for readiness, exercises key endpoints, and tears down.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Pick a random port in the ephemeral range to avoid conflicts.
PORT=$((RANDOM % 10000 + 20000))
GO_TMP_ROOT="${TMPDIR:-/tmp}/agentauth-go"
BROKER_BIN="${TMPDIR:-/tmp}/agentauth-broker-${PORT}"
BROKER_PID=""
PASS=0
FAIL=0

cleanup() {
  if [[ -n "$BROKER_PID" ]]; then
    kill "$BROKER_PID" 2>/dev/null || true
    wait "$BROKER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "=== Live Test: building broker ==="
mkdir -p "$GO_TMP_ROOT/build" "$GO_TMP_ROOT/path/pkg/mod"
GOTOOLCHAIN="${GOTOOLCHAIN:-local}" \
GOCACHE="$GO_TMP_ROOT/build" \
  go build -o "$BROKER_BIN" "$PROJECT_ROOT/cmd/broker"

echo "=== Live Test: starting broker on port $PORT ==="
AA_PORT="$PORT" \
AA_ADMIN_SECRET="live-test-secret-32bytes-long!!" \
AA_SEED_TOKENS=true \
AA_LOG_LEVEL=quiet \
  "$BROKER_BIN" > /tmp/broker_live_test.log 2>&1 &
BROKER_PID=$!

# Wait for broker to become ready (up to 5 seconds).
BASE="http://127.0.0.1:$PORT"
READY=false
for i in $(seq 1 50); do
  if curl -sf "$BASE/v1/health" >/dev/null 2>&1; then
    READY=true
    break
  fi
  sleep 0.1
done

if [[ "$READY" != "true" ]]; then
  echo "FAIL: broker did not become ready within 5 seconds"
  echo "--- Broker log ---"
  cat /tmp/broker_live_test.log || true
  exit 1
fi
echo "Broker ready (PID=$BROKER_PID, port=$PORT)"

# --- Test helpers ---

check() {
  local name="$1"
  local method="$2"
  local url="$3"
  local expected_status="$4"
  shift 4
  # remaining args are extra curl flags

  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" "$@" 2>/dev/null) || true

  if [[ "$status" == "$expected_status" ]]; then
    echo "  PASS: $name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected HTTP $expected_status, got $status)"
    FAIL=$((FAIL + 1))
  fi
}

check_json() {
  local name="$1"
  local method="$2"
  local url="$3"
  local expected_status="$4"
  local body="$5"
  shift 5

  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" \
    -H "Content-Type: application/json" \
    -d "$body" "$@" 2>/dev/null) || true

  if [[ "$status" == "$expected_status" ]]; then
    echo "  PASS: $name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected HTTP $expected_status, got $status)"
    FAIL=$((FAIL + 1))
  fi
}

# --- Live tests ---

echo ""
echo "=== Live Test: endpoint smoke tests ==="

# 1. Health check
check "GET /v1/health" GET "$BASE/v1/health" 200

# 2. Metrics endpoint
check "GET /v1/metrics" GET "$BASE/v1/metrics" 200

# 3. Challenge endpoint returns nonce
check "GET /v1/challenge" GET "$BASE/v1/challenge" 200

# 4. Token validate with empty body returns 400
check_json "POST /v1/token/validate (empty)" POST "$BASE/v1/token/validate" 400 '{}'

# 5. Token validate with bad token returns 200 (valid:false, not HTTP error)
check_json "POST /v1/token/validate (bad token)" POST "$BASE/v1/token/validate" 200 '{"token":"not.a.valid.token"}'

# 6. Delegate without auth returns 401
check_json "POST /v1/delegate (no auth)" POST "$BASE/v1/delegate" 401 '{"delegate_to":"x","scope":["read:data:*"]}'

# 7. Revoke without auth returns 401
check_json "POST /v1/revoke (no auth)" POST "$BASE/v1/revoke" 401 '{"level":"token","target":"x"}'

# 8. Audit without auth returns 401
check "GET /v1/audit/events (no auth)" GET "$BASE/v1/audit/events" 401

# 9. Admin auth with correct credentials returns 200
AUTH_RESP=$(curl -sS -X POST "$BASE/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"live-test-secret-32bytes-long!!"}' 2>/dev/null || true)
if [[ "$AUTH_RESP" == *'"access_token"'* ]]; then
  echo "  PASS: POST /v1/admin/auth (valid) returned access token"
  PASS=$((PASS + 1))
  ADMIN_TOKEN=$(printf '%s' "$AUTH_RESP" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
else
  echo "  FAIL: POST /v1/admin/auth (valid) did not return access token"
  FAIL=$((FAIL + 1))
  ADMIN_TOKEN=""
fi

# 10. Admin auth with wrong secret returns 401
check_json "POST /v1/admin/auth (invalid)" POST "$BASE/v1/admin/auth" 401 \
  '{"client_id":"admin","client_secret":"wrong-secret"}'

# 11. Launch token creation with admin token returns 201
if [[ -n "$ADMIN_TOKEN" ]]; then
  check_json "POST /v1/admin/launch-tokens (auth)" POST "$BASE/v1/admin/launch-tokens" 201 \
    '{"agent_name":"live-smoke","allowed_scope":["read:Customers:*"],"max_ttl":300,"ttl":120}' \
    -H "Authorization: Bearer $ADMIN_TOKEN"
else
  echo "  FAIL: POST /v1/admin/launch-tokens (auth) skipped, missing admin token"
  FAIL=$((FAIL + 1))
fi

# 12. Audit query with admin token returns 200
if [[ -n "$ADMIN_TOKEN" ]]; then
  check "GET /v1/audit/events (auth)" GET "$BASE/v1/audit/events" 200 \
    -H "Authorization: Bearer $ADMIN_TOKEN"
else
  echo "  FAIL: GET /v1/audit/events (auth) skipped, missing admin token"
  FAIL=$((FAIL + 1))
fi

# --- Summary ---

echo ""
echo "=== Live Test Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"

if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  exit 1
fi

echo "RESULT: PASSED"
exit 0
