#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BROKER_BIN="$ROOT/agentauth-broker"
SMOKE_BIN="$ROOT/agentauth-smoke"
OUT_LOG="/tmp/aa_live_out.log"
ERR_LOG="/tmp/aa_live_err.log"
PORT=18080

cd "$ROOT"
go build -o "$BROKER_BIN" ./cmd/broker
go build -o "$SMOKE_BIN" ./cmd/smoketest

# Start broker with seed tokens on a test port.
AA_SEED_TOKENS=true AA_PORT="$PORT" "$BROKER_BIN" >"$OUT_LOG" 2>"$ERR_LOG" &
PID=$!
trap 'kill $PID 2>/dev/null || true; wait $PID 2>/dev/null || true; rm -f "$BROKER_BIN" "$SMOKE_BIN"' EXIT

# Wait for health endpoint readiness.
for _ in $(seq 1 30); do
  if curl -sS "http://127.0.0.1:${PORT}/v1/health" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

# Extract seed tokens from broker stdout.
SEED_LAUNCH=$(grep 'SEED_LAUNCH_TOKEN=' "$OUT_LOG" | head -1 | sed 's/SEED_LAUNCH_TOKEN=//')
if [ -z "$SEED_LAUNCH" ]; then
  echo "[LIVE:FAIL] seed launch token not found in broker output"
  exit 1
fi
SEED_ADMIN=$(grep 'SEED_ADMIN_TOKEN=' "$OUT_LOG" | head -1 | sed 's/SEED_ADMIN_TOKEN=//')
if [ -z "$SEED_ADMIN" ]; then
  echo "[LIVE:FAIL] seed admin token not found in broker output"
  exit 1
fi

# Run smoke test against the real broker.
SEED_LAUNCH_TOKEN="$SEED_LAUNCH" \
SEED_ADMIN_TOKEN="$SEED_ADMIN" \
AA_BROKER_URL="http://127.0.0.1:${PORT}" \
"$SMOKE_BIN"
SMOKE_EXIT=$?

if [ "$SMOKE_EXIT" -ne 0 ]; then
  echo "[LIVE:FAIL] smoke test exited with code $SMOKE_EXIT"
  exit 1
fi

echo "[LIVE:PASS] full lifecycle validated: health, metrics, register, token ops, authz, admin-gated revoke, single-use launch, delegation"
