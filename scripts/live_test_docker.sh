#!/usr/bin/env bash
set -euo pipefail

# live_test_docker.sh — Docker-based live E2E for broker + sidecar.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"
PROJECT_NAME="agentauth-live-${RANDOM}"
ADMIN_SECRET="live-test-secret-32bytes-long!!"

pick_free_port() {
  local candidate
  while :; do
    candidate=$((RANDOM % 10000 + 20000))
    if ! lsof -iTCP:"$candidate" -sTCP:LISTEN >/dev/null 2>&1; then
      echo "$candidate"
      return 0
    fi
  done
}

if [[ -n "${AA_HOST_PORT:-}" ]]; then
  HOST_PORT="${AA_HOST_PORT}"
else
  HOST_PORT="$(pick_free_port)"
fi

PASS=0
FAIL=0

cleanup() {
  echo ""
  echo "=== Live Test: docker cleanup ==="
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans) || true
}
trap cleanup EXIT

if ! docker info >/dev/null 2>&1; then
  echo "FAIL: Docker daemon is not running. Start Docker Desktop or dockerd, then rerun ./scripts/live_test_docker.sh"
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
  local name="$1" method="$2" url="$3" expected="$4"
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
READY=false
for _ in $(seq 1 60); do
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

echo "=== Live Test: smoke + sidecar E2E ==="
status_check "GET /v1/health" GET "$BASE/v1/health" 200
status_check "GET /v1/metrics" GET "$BASE/v1/metrics" 200
status_check "GET /v1/challenge" GET "$BASE/v1/challenge" 200

AUTH_RESP=$(scurl -X POST "$BASE/v1/admin/auth" -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"'"$ADMIN_SECRET"'"}') || true
if [[ "$AUTH_RESP" == *'"access_token"'* ]]; then
  PASS=$((PASS + 1)); echo "  PASS: POST /v1/admin/auth returned access_token"
  ADMIN_TOKEN=$(printf '%s' "$AUTH_RESP" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
else
  FAIL=$((FAIL + 1)); echo "  FAIL: POST /v1/admin/auth did not return access_token"; ADMIN_TOKEN=""
fi

if [[ -n "$ADMIN_TOKEN" ]]; then
  ACT_RESP=$(scurl -X POST "$BASE/v1/admin/sidecar-activations" \
    -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" \
    -d '{"allowed_scope_prefix":"read:data:*","ttl":120}') || true
  if [[ "$ACT_RESP" == *'"activation_token"'* ]]; then
    PASS=$((PASS + 1)); echo "  PASS: activation token issued"
    ACTIVATION_TOKEN=$(printf '%s' "$ACT_RESP" | sed -n 's/.*"activation_token":"\([^"]*\)".*/\1/p')
  else
    FAIL=$((FAIL + 1)); echo "  FAIL: activation token missing"; ACTIVATION_TOKEN=""
  fi
else
  FAIL=$((FAIL + 1)); echo "  FAIL: activation issuance skipped"; ACTIVATION_TOKEN=""
fi

if [[ -n "$ACTIVATION_TOKEN" ]]; then
  SIDECAR_RESP=$(scurl -X POST "$BASE/v1/sidecar/activate" -H "Content-Type: application/json" \
    -d '{"sidecar_activation_token":"'"$ACTIVATION_TOKEN"'"}') || true
  if [[ "$SIDECAR_RESP" == *'"access_token"'* ]]; then
    PASS=$((PASS + 1)); echo "  PASS: sidecar activated"
    SIDECAR_TOKEN=$(printf '%s' "$SIDECAR_RESP" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')
  else
    FAIL=$((FAIL + 1)); echo "  FAIL: sidecar activation failed"; SIDECAR_TOKEN=""
  fi

  REPLAY_BODY=$(scurl -X POST "$BASE/v1/sidecar/activate" -H "Content-Type: application/json" \
    -d '{"sidecar_activation_token":"'"$ACTIVATION_TOKEN"'"}' 2>/dev/null || true)
  REPLAY_STATUS=$(scurl -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/sidecar/activate" \
    -H "Content-Type: application/json" -d '{"sidecar_activation_token":"'"$ACTIVATION_TOKEN"'"}' 2>/dev/null || true)
  if [[ "$REPLAY_STATUS" == "401" ]] && [[ "$REPLAY_BODY" == *"activation_token_replayed"* ]]; then
    PASS=$((PASS + 1)); echo "  PASS: replay denied"
  else
    FAIL=$((FAIL + 1)); echo "  FAIL: replay behavior unexpected"
  fi
else
  FAIL=$((FAIL + 1)); echo "  FAIL: sidecar checks skipped"; SIDECAR_TOKEN=""
fi

if [[ -n "$SIDECAR_TOKEN" ]]; then
  ESC_BODY=$(scurl -X POST "$BASE/v1/token/exchange" -H "Authorization: Bearer $SIDECAR_TOKEN" \
    -H "Content-Type: application/json" -d '{"agent_id":"spiffe://agentauth.local/agent/o/t/i","scope":["write:data:*"]}' 2>/dev/null || true)
  ESC_STATUS=$(scurl -o /dev/null -w "%{http_code}" -X POST "$BASE/v1/token/exchange" \
    -H "Authorization: Bearer $SIDECAR_TOKEN" -H "Content-Type: application/json" \
    -d '{"agent_id":"spiffe://agentauth.local/agent/o/t/i","scope":["write:data:*"]}' 2>/dev/null || true)
  if [[ "$ESC_STATUS" == "403" ]] && [[ "$ESC_BODY" == *"scope_escalation_denied"* ]]; then
    PASS=$((PASS + 1)); echo "  PASS: exchange scope escalation denied"
  else
    FAIL=$((FAIL + 1)); echo "  FAIL: exchange escalation behavior unexpected"
  fi
fi

BROKER_LOGS=$(cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=200 || true)
if printf '%s' "$BROKER_LOGS" | grep -q 'request completed'; then
  PASS=$((PASS + 1)); echo "  PASS: broker HTTP request logs present"
else
  FAIL=$((FAIL + 1)); echo "  FAIL: broker HTTP request logs missing"
fi

echo "=== Live Test Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
if [[ $FAIL -gt 0 ]]; then
  echo "RESULT: FAILED"
  exit 1
fi

echo "RESULT: PASSED"
