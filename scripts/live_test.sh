#!/usr/bin/env bash
set -euo pipefail

# live_test.sh — Docker-based live E2E for broker + sidecar.
#
# This script ALWAYS deploys docker compose stack first, then runs live HTTP
# checks through the sidecar container against broker:8080.
#
# Usage:
#   ./scripts/live_test.sh

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"
PROJECT_NAME="agentauth-live-${RANDOM}"

ADMIN_SECRET="live-test-secret-32bytes-long!!"
HOST_PORT="${AA_HOST_PORT:-18080}"

PASS=0
FAIL=0

cleanup() {
  echo ""
  echo "=== Live Test: docker cleanup ==="
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans) || true
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "FAIL: Docker daemon is not running. Start Docker Desktop or dockerd, then rerun ./scripts/live_test.sh"
  exit 1
fi

echo "=== Live Test: docker compose up (broker + sidecar) ==="
(
  cd "$PROJECT_ROOT"
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_HOST_PORT="$HOST_PORT" \
  AA_SEED_TOKENS=false \
  AA_LOG_LEVEL=standard \
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d --build broker sidecar
)

scurl() {
  (
    cd "$PROJECT_ROOT"
    docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" exec -T sidecar \
      curl -sS "$@"
  )
}

status_check() {
  local name="$1"
  local method="$2"
  local url="$3"
  local expected="$4"
  shift 4

  local status
  status=$(scurl -o /dev/null -w "%{http_code}" -X "$method" "$url" "$@" 2>/dev/null) || true

  if [[ "$status" == "$expected" ]]; then
    echo "  PASS: $name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected HTTP $expected, got $status)"
    FAIL=$((FAIL + 1))
  fi
}

BASE="http://broker:8080"

echo "=== Live Test: wait for broker readiness ==="
READY=false
for i in $(seq 1 60); do
  if scurl -f "$BASE/v1/health" >/dev/null 2>&1; then
    READY=true
    break
  fi
  sleep 0.5
done

if [[ "$READY" != "true" ]]; then
  echo "FAIL: broker did not become ready"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=200) || true
  exit 1
fi

echo "Broker ready (compose project: $PROJECT_NAME)"

echo ""
echo "=== Live Test: smoke + sidecar E2E ==="

# 1. Public endpoints
status_check "GET /v1/health" GET "$BASE/v1/health" 200
status_check "GET /v1/metrics" GET "$BASE/v1/metrics" 200
status_check "GET /v1/challenge" GET "$BASE/v1/challenge" 200

# 2. Admin auth
AUTH_RESP=$(scurl -X POST "$BASE/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"'"$ADMIN_SECRET"'"}') || true

if [[ "$AUTH_RESP" == *'"access_token"'* ]]; then
  echo "  PASS: POST /v1/admin/auth returned access_token"
  PASS=$((PASS + 1))
  ADMIN_TOKEN=$(printf '%s' "$AUTH_RESP" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
else
  echo "  FAIL: POST /v1/admin/auth did not return access_token"
  FAIL=$((FAIL + 1))
  ADMIN_TOKEN=""
fi

# 3. Sidecar activation token issuance
if [[ -n "$ADMIN_TOKEN" ]]; then
  ACT_RESP=$(scurl -X POST "$BASE/v1/admin/sidecar-activations" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"allowed_scope_prefix":"read:data:*","ttl":120}') || true

  if [[ "$ACT_RESP" == *'"activation_token"'* ]]; then
    echo "  PASS: POST /v1/admin/sidecar-activations returned activation_token"
    PASS=$((PASS + 1))
    ACTIVATION_TOKEN=$(printf '%s' "$ACT_RESP" | sed -n 's/.*"activation_token":"\([^"]*\)".*/\1/p')
  else
    echo "  FAIL: POST /v1/admin/sidecar-activations did not return activation_token"
    FAIL=$((FAIL + 1))
    ACTIVATION_TOKEN=""
  fi
else
  echo "  FAIL: activation issuance skipped, missing admin token"
  FAIL=$((FAIL + 1))
  ACTIVATION_TOKEN=""
fi

# 4. Activate sidecar + replay denial
if [[ -n "$ACTIVATION_TOKEN" ]]; then
  SIDECAR_RESP=$(scurl -X POST "$BASE/v1/sidecar/activate" \
    -H "Content-Type: application/json" \
    -d '{"sidecar_activation_token":"'"$ACTIVATION_TOKEN"'"}') || true

  if [[ "$SIDECAR_RESP" == *'"access_token"'* ]]; then
    echo "  PASS: POST /v1/sidecar/activate returned sidecar access_token"
    PASS=$((PASS + 1))
    SIDECAR_TOKEN=$(printf '%s' "$SIDECAR_RESP" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
  else
    echo "  FAIL: POST /v1/sidecar/activate did not return access_token"
    FAIL=$((FAIL + 1))
    SIDECAR_TOKEN=""
  fi

  REPLAY_STATUS=$(scurl -o /tmp/replay_body.json -w "%{http_code}" -X POST "$BASE/v1/sidecar/activate" \
    -H "Content-Type: application/json" \
    -d '{"sidecar_activation_token":"'"$ACTIVATION_TOKEN"'"}' 2>/dev/null) || true
  if [[ "$REPLAY_STATUS" == "401" ]] && grep -q 'activation_token_replayed' /tmp/replay_body.json; then
    echo "  PASS: sidecar activation replay denied (401 activation_token_replayed)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: sidecar activation replay behavior unexpected (status=$REPLAY_STATUS)"
    FAIL=$((FAIL + 1))
  fi
else
  echo "  FAIL: sidecar activation checks skipped, missing activation token"
  FAIL=$((FAIL + 1))
  SIDECAR_TOKEN=""
fi

# 5. Token exchange negative checks (enforced sidecar authority)
if [[ -n "$SIDECAR_TOKEN" ]]; then
  # 5a. Scope escalation should be denied before agent lookup.
  ESC_STATUS=$(scurl -o /tmp/exchange_escalation.json -w "%{http_code}" -X POST "$BASE/v1/token/exchange" \
    -H "Authorization: Bearer $SIDECAR_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"agent_id":"spiffe://agentauth.local/agent/o/t/i","scope":["write:data:*"]}' 2>/dev/null) || true

  if [[ "$ESC_STATUS" == "403" ]] && grep -q 'scope_escalation_denied' /tmp/exchange_escalation.json; then
    echo "  PASS: token exchange scope escalation denied"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: token exchange escalation check unexpected (status=$ESC_STATUS)"
    FAIL=$((FAIL + 1))
  fi

  # 5b. Allowed scope but unknown agent -> 404 not_found.
  NF_STATUS=$(scurl -o /tmp/exchange_not_found.json -w "%{http_code}" -X POST "$BASE/v1/token/exchange" \
    -H "Authorization: Bearer $SIDECAR_TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"agent_id":"spiffe://agentauth.local/agent/o/t/i","scope":["read:data:*"]}' 2>/dev/null) || true

  if [[ "$NF_STATUS" == "404" ]]; then
    echo "  PASS: token exchange not_found behavior confirmed"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: token exchange expected 404 for unknown agent (got $NF_STATUS)"
    FAIL=$((FAIL + 1))
  fi
else
  echo "  FAIL: token exchange checks skipped, missing sidecar token"
  FAIL=$((FAIL + 1))
fi

# 6. Audit query includes sidecar activation events
if [[ -n "$ADMIN_TOKEN" ]]; then
  AUDIT_RESP=$(scurl -X GET "$BASE/v1/audit/events?limit=200" -H "Authorization: Bearer $ADMIN_TOKEN") || true
  if [[ "$AUDIT_RESP" == *'sidecar_activated'* ]] && [[ "$AUDIT_RESP" == *'sidecar_activation_failed'* ]]; then
    echo "  PASS: audit contains sidecar activation success/failure events"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: audit missing expected sidecar activation events"
    FAIL=$((FAIL + 1))
  fi
else
  echo "  FAIL: audit check skipped, missing admin token"
  FAIL=$((FAIL + 1))
fi

# 7. HTTP request logging evidence check
BROKER_LOGS=$(cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=200 || true)
if printf '%s' "$BROKER_LOGS" | grep -q 'request completed'; then
  echo "  PASS: broker HTTP request logs present"
  PASS=$((PASS + 1))
else
  echo "  FAIL: broker HTTP request logs not detected"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Live Test Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"

if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  echo "--- Broker logs (tail) ---"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=200) || true
  exit 1
fi

echo "RESULT: PASSED"
exit 0
