#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────
# SEC-L2b: Error Handling & Headers — Acceptance + Regression Tests
# Adapted for agentauth-core (no OIDC, no HITL, no sidecar, no cloud)
# ─────────────────────────────────────────────────────────────────
#
# Usage:
#   1. Start broker:  AA_ADMIN_SECRET=live-test-secret-32bytes-long-ok ./scripts/stack_up.sh
#   2. Run:           AA_ADMIN_SECRET=live-test-secret-32bytes-long-ok ./tests/sec-l2b/integration.sh
#
# Required env vars:
#   AA_ADMIN_SECRET  — admin secret (plaintext, for auth endpoint)
#
# Optional env vars:
#   BROKER_URL       — broker base URL (default: http://127.0.0.1:8443)
#
# Evidence is printed to stdout with banners. Redirect to file for records.
set -euo pipefail

BROKER_URL="${BROKER_URL:-http://127.0.0.1:8443}"
AA_ADMIN_SECRET="${AA_ADMIN_SECRET:?AA_ADMIN_SECRET must be set}"
PASS=0
FAIL=0
SKIP=0

banner() {
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  $1"
  echo "═══════════════════════════════════════════════════════════════"
}

section() {
  echo ""
  echo "── $1 ──"
}

pass() {
  echo "  ✅ PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "  ❌ FAIL: $1"
  FAIL=$((FAIL + 1))
}

skip() {
  echo "  ⏭  SKIP: $1"
  SKIP=$((SKIP + 1))
}

banner "SEC-L2b Acceptance + Regression Tests (agentauth-core)"
echo "  Broker: ${BROKER_URL}"
echo "  Date:   $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ─────────────────────────────────────────────────────────────────
# Setup: get admin token + register agent
# ─────────────────────────────────────────────────────────────────
section "Setup: Admin auth + Agent registration"

# Admin auth
AUTH_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"${AA_ADMIN_SECRET}\"}")
ADMIN_TOKEN=$(echo "$AUTH_RESP" | jq -r .token)
if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
  echo "FATAL: Could not get admin token. Response: $AUTH_RESP"
  exit 1
fi
echo "  Admin token: ${ADMIN_TOKEN:0:20}..."

# Create launch token
LT_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/admin/launch-tokens" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"allowed_scopes":["read:data:*"],"max_ttl":300,"single_use":false}')
LAUNCH_TOKEN=$(echo "$LT_RESP" | jq -r .launch_token)
if [ -z "$LAUNCH_TOKEN" ] || [ "$LAUNCH_TOKEN" = "null" ]; then
  echo "FATAL: Could not create launch token. Response: $LT_RESP"
  exit 1
fi
echo "  Launch token: ${LAUNCH_TOKEN:0:20}..."

# Register agent with launch token
REG_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/register" \
  -H "Authorization: Bearer ${LAUNCH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"l2b-test-agent","scopes":["read:data:*"]}')
AGENT_TOKEN=$(echo "$REG_RESP" | jq -r .token)
AGENT_ID=$(echo "$REG_RESP" | jq -r .agent_id)
if [ -z "$AGENT_TOKEN" ] || [ "$AGENT_TOKEN" = "null" ]; then
  echo "FATAL: Could not register agent. Response: $REG_RESP"
  exit 1
fi
echo "  Agent token:  ${AGENT_TOKEN:0:20}..."
echo "  Agent ID:     $AGENT_ID"

# ─────────────────────────────────────────────────────────────────
# B5 Acceptance Stories
# ─────────────────────────────────────────────────────────────────

banner "B5 (SEC-L2b) Acceptance Stories"

# ── S1: Generic error on invalid token (H3) ──
section "S1: Validate generic error on invalid token (H3)"
S1_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d '{"token": "not-a-valid-token"}')
echo "  Response: $S1_RESP"
S1_ERR=$(echo "$S1_RESP" | jq -r .error)
if [ "$S1_ERR" = "token is invalid or expired" ]; then
  pass "S1: Generic error message returned"
else
  fail "S1: Expected 'token is invalid or expired', got '$S1_ERR'"
fi

# ── S2: Generic error on revoked token (H3) ──
section "S2: Validate generic error on revoked token (H3)"
# Register a throw-away agent to revoke
LT2_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/admin/launch-tokens" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"allowed_scopes":["read:data:*"],"max_ttl":300,"single_use":false}')
LT2=$(echo "$LT2_RESP" | jq -r .launch_token)
REV_REG=$(curl -sf -X POST "${BROKER_URL}/v1/register" \
  -H "Authorization: Bearer ${LT2}" \
  -H "Content-Type: application/json" \
  -d '{"name":"l2b-revoke-test","scopes":["read:data:*"]}')
REV_TOKEN=$(echo "$REV_REG" | jq -r .token)
REV_ID=$(echo "$REV_REG" | jq -r .agent_id)
echo "  Revoking agent: $REV_ID"

# Revoke
REVOKE_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/revoke" \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"level\": \"agent\", \"target\": \"${REV_ID}\"}")
echo "  Revoke response: $REVOKE_RESP"

# Validate revoked token
REV_VAL=$(curl -sf -X POST "${BROKER_URL}/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"${REV_TOKEN}\"}")
echo "  Validate revoked: $REV_VAL"
S2_ERR=$(echo "$REV_VAL" | jq -r .error)
if [ "$S2_ERR" = "token is invalid or expired" ]; then
  pass "S2: Revoked token returns generic error"
else
  fail "S2: Expected 'token is invalid or expired', got '$S2_ERR'"
fi

# ── S3: Renew rejects tampered token (H4) ──
section "S3: Renew rejects tampered token without leaking details (H4)"
TAMPERED="${AGENT_TOKEN}tampered"
S3_CODE=$(curl -s -o /tmp/s3_body.txt -w "%{http_code}" -X POST "${BROKER_URL}/v1/token/renew" \
  -H "Authorization: Bearer ${TAMPERED}")
S3_BODY=$(cat /tmp/s3_body.txt)
echo "  HTTP status: $S3_CODE"
echo "  Body: $S3_BODY"
if [ "$S3_CODE" = "401" ]; then
  if echo "$S3_BODY" | grep -qiE "signature|segment|malformed"; then
    fail "S3: Response leaks error details"
  else
    pass "S3: Tampered token rejected with generic error"
  fi
else
  fail "S3: Expected 401, got $S3_CODE"
fi

# ── S4: Security headers present (H1) ──
section "S4: Security headers on all responses (H1)"
S4_FAIL=0
for EP in "/v1/health" "/v1/metrics" "/v1/token/validate"; do
  if [ "$EP" = "/v1/token/validate" ]; then
    HEADERS=$(curl -sI -X POST "${BROKER_URL}${EP}" \
      -H "Content-Type: application/json" \
      -d '{"token":"x"}')
  else
    HEADERS=$(curl -sI "${BROKER_URL}${EP}")
  fi
  echo "  --- ${EP} ---"
  echo "$HEADERS" | grep -iE "x-content-type|x-frame|cache-control|strict-transport" || echo "  (none found)"
  if ! echo "$HEADERS" | grep -qi "X-Content-Type-Options: nosniff"; then
    echo "  MISSING: X-Content-Type-Options: nosniff on ${EP}"
    S4_FAIL=1
  fi
  if ! echo "$HEADERS" | grep -qi "X-Frame-Options: DENY"; then
    echo "  MISSING: X-Frame-Options: DENY on ${EP}"
    S4_FAIL=1
  fi
done
if [ "$S4_FAIL" -eq 0 ]; then
  pass "S4: Security headers present on all endpoints"
else
  fail "S4: Missing security headers"
fi

# ── S5: HSTS — SKIP (requires TLS cert) ──
section "S5: HSTS present when TLS enabled (H1)"
skip "S5: Requires TLS cert — not available in Docker test mode"

# ── S6: Oversized body returns 413 (H7) ──
section "S6: Oversized body returns 413 (H7)"
S6_CODE=$(python3 -c "import sys; sys.stdout.buffer.write(b'{' + b'x' * (1024*1024) + b'}')" | \
  curl -s -o /dev/null -w "%{http_code}" -X POST "${BROKER_URL}/v1/token/validate" \
  -H "Content-Type: application/json" \
  --data-binary @-)
echo "  HTTP status: $S6_CODE"
if [ "$S6_CODE" = "413" ]; then
  pass "S6: Oversized body returns 413"
else
  fail "S6: Expected 413, got $S6_CODE"
fi

# ─────────────────────────────────────────────────────────────────
# Regression Tests (B0-B4 must still work)
# ─────────────────────────────────────────────────────────────────

banner "Regression Tests (B0-B4)"

# ── R1: Admin auth (B2) ──
section "R1: Admin auth works (B2)"
echo "  Already tested in setup — admin token obtained."
pass "R1: Admin auth works with hashed secret"

# ── R2: Agent registration (B0) ──
section "R2: Agent registration works (B0)"
echo "  Already tested in setup — agent registered via launch token."
pass "R2: Agent registration works"

# ── R3: Token renewal (B1) ──
section "R3: Token renewal works (B1)"
R3_RESP=$(curl -sf -X POST "${BROKER_URL}/v1/token/renew" \
  -H "Authorization: Bearer ${AGENT_TOKEN}")
R3_TOKEN=$(echo "$R3_RESP" | jq -r .token)
echo "  Renew response: ${R3_RESP:0:200}..."
if [ -n "$R3_TOKEN" ] && [ "$R3_TOKEN" != "null" ]; then
  pass "R3: Token renewal works"
  # Update agent token for subsequent tests
  AGENT_TOKEN="$R3_TOKEN"
else
  fail "R3: Token renewal failed: $R3_RESP"
fi

# ── R4: Token revocation + validate (B4) ──
section "R4: Token revocation + validate (B4)"
REV_VALID=$(echo "$REV_VAL" | jq -r .valid)
if [ "$REV_VALID" = "false" ]; then
  pass "R4: Revoked token correctly rejected"
else
  fail "R4: Revoked token still valid"
fi

# ─────────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────────
banner "Results"
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
echo "  SKIP: $SKIP"
echo "  TOTAL: $((PASS + FAIL + SKIP))"

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "  ❌ SOME TESTS FAILED"
  exit 1
else
  echo ""
  echo "  ✅ ALL TESTS PASSED"
  exit 0
fi
