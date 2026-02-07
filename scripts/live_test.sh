#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/agentauth-broker"
OUT_LOG="/tmp/aa_live_out.log"
ERR_LOG="/tmp/aa_live_err.log"

cd "$ROOT"
go build -o "$BIN" ./cmd/broker

"$BIN" >"$OUT_LOG" 2>"$ERR_LOG" &
PID=$!
trap 'kill $PID 2>/dev/null || true; wait $PID 2>/dev/null || true; rm -f "$BIN"' EXIT

# Wait for health endpoint readiness.
for _ in $(seq 1 20); do
  if curl -sS http://127.0.0.1:8080/v1/health >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

HEALTH="$(curl -sS --max-time 5 http://127.0.0.1:8080/v1/health)"
CHALLENGE="$(curl -sS --max-time 5 http://127.0.0.1:8080/v1/challenge)"
VALIDATE_CODE="$(curl -sS -o /tmp/aa_validate_body.json -w "%{http_code}" -H 'Content-Type: application/json' -d '{"token":"invalid","required_scope":"read:Customers:12345"}' http://127.0.0.1:8080/v1/token/validate)"
RENEW_CODE="$(curl -sS -o /tmp/aa_renew_body.json -w "%{http_code}" -H 'Content-Type: application/json' -d '{"token":"invalid"}' http://127.0.0.1:8080/v1/token/renew)"
PROTECTED_NOAUTH_CODE="$(curl -sS -o /tmp/aa_protected_noauth_body.json -w "%{http_code}" http://127.0.0.1:8080/v1/protected/customers/12345)"

echo "$HEALTH" | grep -q '"status":"healthy"'
echo "$CHALLENGE" | grep -q '"nonce":"'
echo "$CHALLENGE" | grep -q '"expires_at":"'
test "$VALIDATE_CODE" = "401"
test "$RENEW_CODE" = "401"
test "$PROTECTED_NOAUTH_CODE" = "401"

echo "[LIVE:PASS] health/challenge/token/authz error-paths validated"
