#!/usr/bin/env bash
set -euo pipefail

# live_test_sidecar.sh — Docker-based live E2E for the sidecar.
#
# Exercises all 5 sidecar endpoints against a real broker+sidecar stack:
#   1. GET  /v1/health     — bootstrap + broker connectivity
#   2. POST /v1/token      — lazy registration (new agent)
#   3. POST /v1/token      — registry cache hit (same agent)
#   4. POST /v1/token      — scope ceiling denial
#   5. POST /v1/token/renew— token renewal
#   6. GET  /v1/challenge   — challenge proxy
#   7. POST /v1/register   — BYOK registration
#   8. POST /v1/token      — token for BYOK agent
#   9. Broker validate     — confirm sidecar-issued token is real
#
# Usage:
#   ./scripts/live_test_sidecar.sh
#
# Requires: Docker, curl, Python 3 (for Ed25519 BYOK signing)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"
PROJECT_NAME="agentauth-sidecar-live-${RANDOM}"
ADMIN_SECRET="live-test-secret-32bytes-long!!"

# Pick free ports.
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

BROKER_PORT="$(pick_free_port)"
SIDECAR_PORT="$(pick_free_port)"

PASS=0
FAIL=0

cleanup() {
  echo ""
  echo "=== Sidecar Live Test: docker cleanup ==="
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans 2>/dev/null) || true
}
trap cleanup EXIT

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

pass() {
  PASS=$((PASS + 1))
  echo "  PASS: $1"
}

fail() {
  FAIL=$((FAIL + 1))
  echo "  FAIL: $1"
}

json_field() {
  # Extract a string field from JSON. Usage: json_field '{"key":"val"}' key
  local json="$1" field="$2"
  printf '%s' "$json" | sed -n 's/.*"'"$field"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

json_field_raw() {
  # Extract a raw (non-string) field. Usage: json_field_raw '{"key":true}' key
  local json="$1" field="$2"
  printf '%s' "$json" | sed -n 's/.*"'"$field"'"[[:space:]]*:[[:space:]]*\([^,}]*\).*/\1/p'
}

# ---------------------------------------------------------------------------
# Start stack
# ---------------------------------------------------------------------------

if ! docker info >/dev/null 2>&1; then
  echo "FAIL: Docker daemon is not running."
  exit 1
fi

echo "=== Sidecar Live Test: docker compose up (broker:$BROKER_PORT, sidecar:$SIDECAR_PORT) ==="
(
  cd "$PROJECT_ROOT"
  AA_ADMIN_SECRET="$ADMIN_SECRET" \
  AA_HOST_PORT="$BROKER_PORT" \
  AA_SIDECAR_HOST_PORT="$SIDECAR_PORT" \
  AA_SEED_TOKENS=false \
  AA_LOG_LEVEL=standard \
  docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d --build broker sidecar
)

SIDECAR_BASE="http://127.0.0.1:${SIDECAR_PORT}"
BROKER_BASE="http://127.0.0.1:${BROKER_PORT}"

# Wait for sidecar to be healthy (it bootstraps with broker automatically).
echo "=== Waiting for sidecar to be ready ==="
READY=false
for _ in $(seq 1 60); do
  HEALTH=$(curl -sS "$SIDECAR_BASE/v1/health" 2>/dev/null) || true
  if printf '%s' "$HEALTH" | grep -q '"healthy":true'; then
    READY=true
    break
  fi
  sleep 1
done

if [[ "$READY" != "true" ]]; then
  echo "FAIL: sidecar did not become healthy within 60s"
  echo "Last health response: $HEALTH"
  echo "--- Sidecar logs ---"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs sidecar --tail=50) || true
  echo "--- Broker logs ---"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=50) || true
  exit 1
fi

echo "=== Sidecar is ready. Running tests. ==="
echo ""

# ---------------------------------------------------------------------------
# Step 1: GET /v1/health
# ---------------------------------------------------------------------------

echo "--- Step 1: GET /v1/health ---"
HEALTH_RESP=$(curl -sS "$SIDECAR_BASE/v1/health")
HEALTH_STATUS=$(json_field_raw "$HEALTH_RESP" "healthy")
BROKER_CONN=$(json_field_raw "$HEALTH_RESP" "broker_connected")

if [[ "$HEALTH_STATUS" == "true" ]] && [[ "$BROKER_CONN" == "true" ]]; then
  pass "health: healthy=true, broker_connected=true"
else
  fail "health: got healthy=$HEALTH_STATUS, broker_connected=$BROKER_CONN"
fi

# ---------------------------------------------------------------------------
# Step 2: POST /v1/token — lazy registration (new agent)
# ---------------------------------------------------------------------------

echo "--- Step 2: POST /v1/token (lazy registration) ---"
TOKEN_RESP=$(curl -sS -X POST "$SIDECAR_BASE/v1/token" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"live-agent","task_id":"task-1","scope":["read:data:*"],"ttl":300}')

TOKEN_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$SIDECAR_BASE/v1/token" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"live-agent","task_id":"task-1","scope":["read:data:*"],"ttl":300}')

ACCESS_TOKEN=$(json_field "$TOKEN_RESP" "access_token")
AGENT_ID=$(json_field "$TOKEN_RESP" "agent_id")

if [[ -n "$ACCESS_TOKEN" ]] && [[ -n "$AGENT_ID" ]] && [[ "$AGENT_ID" == spiffe://* ]]; then
  pass "lazy registration: agent_id=$AGENT_ID, token_len=${#ACCESS_TOKEN}"
else
  fail "lazy registration: response=$TOKEN_RESP"
fi

# ---------------------------------------------------------------------------
# Step 3: POST /v1/token — cache hit (same agent)
# ---------------------------------------------------------------------------

echo "--- Step 3: POST /v1/token (cache hit) ---"
TOKEN_RESP2=$(curl -sS -X POST "$SIDECAR_BASE/v1/token" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"live-agent","task_id":"task-1","scope":["read:data:*"]}')

AGENT_ID2=$(json_field "$TOKEN_RESP2" "agent_id")

if [[ "$AGENT_ID2" == "$AGENT_ID" ]]; then
  pass "cache hit: same agent_id=$AGENT_ID2"
else
  fail "cache hit: expected $AGENT_ID, got $AGENT_ID2"
fi

# ---------------------------------------------------------------------------
# Step 4: POST /v1/token — scope ceiling denial
# ---------------------------------------------------------------------------

echo "--- Step 4: POST /v1/token (scope ceiling denial) ---"
ESC_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$SIDECAR_BASE/v1/token" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"evil-agent","scope":["admin:system:*"]}')

if [[ "$ESC_STATUS" == "403" ]]; then
  pass "scope ceiling: 403 for admin:system:*"
else
  fail "scope ceiling: expected 403, got $ESC_STATUS"
fi

# ---------------------------------------------------------------------------
# Step 5: POST /v1/token/renew
# ---------------------------------------------------------------------------

echo "--- Step 5: POST /v1/token/renew ---"
RENEW_RESP=$(curl -sS -X POST "$SIDECAR_BASE/v1/token/renew" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
RENEW_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$SIDECAR_BASE/v1/token/renew" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

RENEWED_TOKEN=$(json_field "$RENEW_RESP" "access_token")

if [[ "$RENEW_STATUS" == "200" ]] && [[ -n "$RENEWED_TOKEN" ]]; then
  pass "renew: got new token (len=${#RENEWED_TOKEN})"
else
  fail "renew: status=$RENEW_STATUS, response=$RENEW_RESP"
fi

# ---------------------------------------------------------------------------
# Step 6: GET /v1/challenge — proxy
# ---------------------------------------------------------------------------

echo "--- Step 6: GET /v1/challenge ---"
CHALLENGE_RESP=$(curl -sS "$SIDECAR_BASE/v1/challenge")
NONCE=$(json_field "$CHALLENGE_RESP" "nonce")

if [[ -n "$NONCE" ]] && [[ ${#NONCE} -ge 32 ]]; then
  pass "challenge proxy: nonce=$NONCE (len=${#NONCE})"
else
  fail "challenge proxy: response=$CHALLENGE_RESP"
fi

# ---------------------------------------------------------------------------
# Step 7: POST /v1/register — BYOK
# ---------------------------------------------------------------------------

echo "--- Step 7: POST /v1/register (BYOK) ---"

# Generate Ed25519 keypair and sign the nonce using Python.
BYOK_RESULT=$(python3 -c "
import hashlib, base64, json, sys
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

nonce_hex = '$NONCE'
nonce_bytes = bytes.fromhex(nonce_hex)

private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

pub_bytes = public_key.public_bytes(Encoding.Raw, PublicFormat.Raw)
signature = private_key.sign(nonce_bytes)

print(json.dumps({
    'public_key': base64.b64encode(pub_bytes).decode(),
    'signature': base64.b64encode(signature).decode()
}))
" 2>/dev/null) || true

if [[ -z "$BYOK_RESULT" ]]; then
  # Fallback: try with nacl if cryptography not installed.
  BYOK_RESULT=$(python3 -c "
import base64, json, os, hashlib

try:
    import nacl.signing
except ImportError:
    # Last resort: use ed25519 from PyNaCl or skip
    print('')
    exit(0)

nonce_hex = '$NONCE'
nonce_bytes = bytes.fromhex(nonce_hex)

signing_key = nacl.signing.SigningKey.generate()
verify_key = signing_key.verify_key

signature = signing_key.sign(nonce_bytes).signature

print(json.dumps({
    'public_key': base64.b64encode(verify_key.encode()).decode(),
    'signature': base64.b64encode(signature).decode()
}))
" 2>/dev/null) || true
fi

if [[ -z "$BYOK_RESULT" ]]; then
  echo "  SKIP: BYOK steps (Python cryptography/nacl not available)"
  echo "  SKIP: BYOK token exchange"
else
  BYOK_PUBKEY=$(json_field "$BYOK_RESULT" "public_key")
  BYOK_SIG=$(json_field "$BYOK_RESULT" "signature")

  REG_RESP=$(curl -sS -X POST "$SIDECAR_BASE/v1/register" \
    -H "Content-Type: application/json" \
    -d '{
      "agent_name": "byok-agent",
      "task_id": "byok-task",
      "public_key": "'"$BYOK_PUBKEY"'",
      "signature": "'"$BYOK_SIG"'",
      "nonce": "'"$NONCE"'"
    }')
  REG_STATUS=$(curl -sS -o /dev/null -w "%{http_code}" -X POST "$SIDECAR_BASE/v1/register" \
    -H "Content-Type: application/json" \
    -d '{
      "agent_name": "byok-agent",
      "task_id": "byok-task",
      "public_key": "'"$BYOK_PUBKEY"'",
      "signature": "'"$BYOK_SIG"'",
      "nonce": "'"$NONCE"'"
    }')

  BYOK_AGENT_ID=$(json_field "$REG_RESP" "agent_id")

  if [[ "$REG_STATUS" == "200" ]] && [[ -n "$BYOK_AGENT_ID" ]] && [[ "$BYOK_AGENT_ID" == spiffe://* ]]; then
    pass "BYOK register: agent_id=$BYOK_AGENT_ID"
  else
    fail "BYOK register: status=$REG_STATUS, response=$REG_RESP"
  fi

  # -------------------------------------------------------------------------
  # Step 8: POST /v1/token — BYOK agent token
  # -------------------------------------------------------------------------

  echo "--- Step 8: POST /v1/token (BYOK agent) ---"
  BYOK_TOKEN_RESP=$(curl -sS -X POST "$SIDECAR_BASE/v1/token" \
    -H "Content-Type: application/json" \
    -d '{"agent_name":"byok-agent","task_id":"byok-task","scope":["read:data:*"]}')

  BYOK_TOKEN=$(json_field "$BYOK_TOKEN_RESP" "access_token")
  BYOK_TOKEN_AGENT=$(json_field "$BYOK_TOKEN_RESP" "agent_id")

  if [[ -n "$BYOK_TOKEN" ]] && [[ "$BYOK_TOKEN_AGENT" == "$BYOK_AGENT_ID" ]]; then
    pass "BYOK token: agent_id matches, token_len=${#BYOK_TOKEN}"
  else
    fail "BYOK token: response=$BYOK_TOKEN_RESP"
  fi
fi

# ---------------------------------------------------------------------------
# Step 9: Validate token at broker
# ---------------------------------------------------------------------------

echo "--- Step 9: Validate sidecar-issued token at broker ---"
VAL_RESP=$(curl -sS -X POST "$BROKER_BASE/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d '{"token":"'"$ACCESS_TOKEN"'"}')
VAL_VALID=$(json_field_raw "$VAL_RESP" "valid")

if [[ "$VAL_VALID" == "true" ]]; then
  pass "broker validates sidecar token as valid"
else
  fail "broker validation: response=$VAL_RESP"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------

echo ""
echo "=== Sidecar Live Test Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"

# Show logs on failure.
if [[ $FAIL -gt 0 ]]; then
  echo ""
  echo "--- Sidecar logs ---"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs sidecar --tail=30) || true
  echo ""
  echo "--- Broker logs ---"
  (cd "$PROJECT_ROOT" && docker compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs broker --tail=30) || true
  echo "RESULT: FAILED"
  exit 1
fi

echo "RESULT: PASSED"
