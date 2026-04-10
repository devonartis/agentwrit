#!/usr/bin/env bash
set -euo pipefail

# core-contract.sh — L2.5 smoke test for agentauth-core
#
# Verifies the broker's core contract:
#   1. /v1/health returns 200
#   2. Admin can authenticate (POST /v1/admin/auth)
#   3. Admin can create a launch token (POST /v1/admin/launch-tokens)
#   4. Agent can fetch a challenge nonce (GET /v1/challenge)
#   5. Agent can register via challenge-response (POST /v1/register)
#      — Ed25519 key pair generated, nonce signed, public key presented
#      — returns a short-lived Bearer access token
#   6. Agent token has correct JWT structure (alg=EdDSA, kid, exp > iat)
#   7. /v1/token/validate accepts the token (valid=true)
#   8. Admin can revoke the agent at /v1/revoke
#   9. /v1/token/validate rejects the revoked token (valid=false)
#  10. Registration with an out-of-scope requested_scope is denied
#
# Caller's responsibility: start the broker before running this script.
# This script does NOT start or stop the broker. Use scripts/stack_up.sh
# (Docker) or bin/broker (VPS mode) before calling.
#
# Required env:
#   AA_ADMIN_SECRET     (default: live-test-secret-32bytes-long-ok)
#   BROKER_URL          (default: http://localhost:8080)
#
# Dependencies: curl, jq, python3 with cryptography installed.
# python3 + cryptography is the established pattern for challenge-response
# in this repo — see tests/sec-l2b/integration.sh for prior art.

BROKER_URL="${BROKER_URL:-http://localhost:8080}"
AA_ADMIN_SECRET="${AA_ADMIN_SECRET:-live-test-secret-32bytes-long-ok}"

for dep in curl jq python3; do
  if ! command -v $dep &>/dev/null; then
    echo "FAIL: missing dependency: $dep"
    exit 1
  fi
done
if ! python3 -c 'from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey' &>/dev/null; then
  echo "FAIL: python3 cryptography package not installed (pip install cryptography)"
  exit 1
fi

step=0
pass() { step=$((step+1)); echo "  [$step] PASS: $1"; }
fail() { step=$((step+1)); echo "  [$step] FAIL: $1 — $2"; echo "L2.5 SMOKE: FAIL"; exit 1; }

echo "=== L2.5 Core Contract Smoke ==="
echo "Broker: $BROKER_URL"

# --- Step 1: Health check ---
HEALTH_STATUS=$(curl -so /dev/null -w "%{http_code}" "$BROKER_URL/v1/health" || true)
[[ "$HEALTH_STATUS" == "200" ]] || fail "health check" "expected 200, got $HEALTH_STATUS"
pass "health 200"

# --- Step 2: Admin auth ---
ADMIN_RESP=$(curl -sf -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AA_ADMIN_SECRET\"}" || echo "{}")
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.access_token // empty')
[[ -n "$ADMIN_TOKEN" ]] || fail "admin auth" "no access_token in response: $ADMIN_RESP"
pass "admin authenticated"

# --- Step 3: Create launch token ---
LT_RESP=$(curl -sf -X POST "$BROKER_URL/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"smoke-l25-agent","allowed_scope":["read:data:*"],"max_ttl":300}' || echo "{}")
LAUNCH_TOKEN=$(echo "$LT_RESP" | jq -r '.launch_token // empty')
[[ -n "$LAUNCH_TOKEN" ]] || fail "launch token" "no launch_token in response: $LT_RESP"
pass "launch token issued"

# --- Step 4: Challenge ---
CHALLENGE_RESP=$(curl -sf "$BROKER_URL/v1/challenge" || echo "{}")
NONCE=$(echo "$CHALLENGE_RESP" | jq -r '.nonce // empty')
[[ -n "$NONCE" ]] || fail "challenge" "no nonce in response: $CHALLENGE_RESP"
pass "challenge nonce fetched"

# --- Step 5: Register agent (Ed25519 challenge-response) ---
# Python generates a keypair, signs the hex-decoded nonce, and POSTs to /v1/register.
REG_RESP=$(python3 <<PYEOF
import json, base64, urllib.request, sys
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

key = Ed25519PrivateKey.generate()
pub_bytes = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
sig = key.sign(bytes.fromhex("${NONCE}"))

body = json.dumps({
    "launch_token": "${LAUNCH_TOKEN}",
    "nonce": "${NONCE}",
    "public_key": base64.b64encode(pub_bytes).decode(),
    "signature": base64.b64encode(sig).decode(),
    "orch_id": "smoke-orch",
    "task_id": "smoke-task",
    "requested_scope": ["read:data:*"],
}).encode()
req = urllib.request.Request("${BROKER_URL}/v1/register", data=body,
    headers={"Content-Type": "application/json"})
try:
    print(urllib.request.urlopen(req).read().decode())
except urllib.error.HTTPError as e:
    print(json.dumps({"error": e.code, "body": e.read().decode()}), file=sys.stderr)
    sys.exit(1)
PYEOF
)
AGENT_TOKEN=$(echo "$REG_RESP" | jq -r '.access_token // empty')
AGENT_ID=$(echo "$REG_RESP" | jq -r '.agent_id // empty')
[[ -n "$AGENT_TOKEN" ]] || fail "agent registration" "no access_token in response: $REG_RESP"
pass "agent registered ($AGENT_ID)"

# --- Step 6: JWT structure ---
decode_jwt_part() {
  local part=$1
  local padded
  padded=$(echo -n "$part" | tr '_-' '/+')
  while [ $((${#padded} % 4)) -ne 0 ]; do padded="${padded}="; done
  echo "$padded" | base64 -d 2>/dev/null
}
HEADER=$(decode_jwt_part "$(echo "$AGENT_TOKEN" | cut -d'.' -f1)")
PAYLOAD=$(decode_jwt_part "$(echo "$AGENT_TOKEN" | cut -d'.' -f2)")
ALG=$(echo "$HEADER" | jq -r '.alg // empty')
KID=$(echo "$HEADER" | jq -r '.kid // empty')
EXP=$(echo "$PAYLOAD" | jq -r '.exp // 0')
IAT=$(echo "$PAYLOAD" | jq -r '.iat // 0')
JTI=$(echo "$PAYLOAD" | jq -r '.jti // empty')
[[ "$ALG" == "EdDSA" ]] || fail "jwt alg" "expected EdDSA, got $ALG"
[[ -n "$KID" ]] || fail "jwt kid" "kid missing"
[[ $EXP -gt $IAT ]] || fail "jwt exp" "exp ($EXP) must be > iat ($IAT)"
[[ -n "$JTI" ]] || fail "jwt jti" "jti missing"
pass "JWT structure valid (alg=EdDSA, kid, exp>iat, jti)"

# --- Step 7: Token validate (accepted) ---
VAL_RESP=$(curl -sf -X POST "$BROKER_URL/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}" || echo "{}")
# Note: jq's `//` operator treats `false` as empty, so use plain .valid.
VAL_VALID=$(echo "$VAL_RESP" | jq -r '.valid')
[[ "$VAL_VALID" == "true" ]] || fail "token validate accepted" "expected valid=true, got: $VAL_RESP"
pass "token validate accepted"

# --- Step 8: Revoke the agent ---
REV_STATUS=$(curl -so /tmp/rev_resp.json -w "%{http_code}" -X POST "$BROKER_URL/v1/revoke" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"level\":\"agent\",\"target\":\"$AGENT_ID\"}")
[[ "$REV_STATUS" == "200" ]] || fail "revocation" "expected 200, got $REV_STATUS (body: $(cat /tmp/rev_resp.json))"
pass "agent revoked ($AGENT_ID)"

# --- Step 9: Validate revoked token is rejected ---
REV_VAL_RESP=$(curl -sf -X POST "$BROKER_URL/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}" || echo "{}")
REV_VAL_VALID=$(echo "$REV_VAL_RESP" | jq -r '.valid')
[[ "$REV_VAL_VALID" == "false" ]] || fail "revocation enforced" "expected valid=false after revoke, got: $REV_VAL_RESP"
pass "revoked token rejected (valid=false)"

# --- Step 10: Out-of-scope registration is denied ---
# Fresh launch token with the SAME narrow ceiling, then try to register
# requesting a scope that's NOT in the ceiling. Broker must refuse.
LT2_RESP=$(curl -sf -X POST "$BROKER_URL/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"smoke-oos-agent","allowed_scope":["read:data:*"],"max_ttl":300}')
LT2=$(echo "$LT2_RESP" | jq -r '.launch_token // empty')
[[ -n "$LT2" ]] || fail "oos setup" "could not mint second launch token"
NONCE2=$(curl -sf "$BROKER_URL/v1/challenge" | jq -r '.nonce // empty')
[[ -n "$NONCE2" ]] || fail "oos setup" "could not fetch second nonce"

OOS_STATUS=$(python3 <<PYEOF
import json, base64, urllib.request, urllib.error
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

key = Ed25519PrivateKey.generate()
pub_bytes = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
sig = key.sign(bytes.fromhex("${NONCE2}"))

body = json.dumps({
    "launch_token": "${LT2}",
    "nonce": "${NONCE2}",
    "public_key": base64.b64encode(pub_bytes).decode(),
    "signature": base64.b64encode(sig).decode(),
    "orch_id": "smoke-orch",
    "task_id": "smoke-oos",
    "requested_scope": ["admin:all:*"],
}).encode()
req = urllib.request.Request("${BROKER_URL}/v1/register", data=body,
    headers={"Content-Type": "application/json"})
try:
    urllib.request.urlopen(req)
    print("200")  # This is a FAILURE — registration should have been denied
except urllib.error.HTTPError as e:
    print(e.code)
PYEOF
)
if [[ "$OOS_STATUS" == "403" || "$OOS_STATUS" == "400" ]]; then
  pass "out-of-scope registration denied ($OOS_STATUS)"
else
  fail "out-of-scope registration denied" "expected 400/403, got $OOS_STATUS"
fi

echo ""
echo "L2.5 SMOKE: PASS"
exit 0
