#!/usr/bin/env bash
set -e

# Start broker in background
export AA_ADMIN_SECRET="test-secret"
go run ./cmd/broker > /tmp/broker_manual.log 2>&1 &
BROKER_PID=$!

# Cleanup on exit
trap "kill $BROKER_PID" EXIT

# Wait for broker
for i in {1..20}; do
  if curl -sf http://localhost:8080/v1/health >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

echo "--- Health Header Check ---"
curl -si http://localhost:8080/v1/health | grep -i '^x-request-id:'

echo -e "\n--- Malformed JSON Check ---"
RESP=$(curl -si -X POST http://localhost:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d '{"bad":"json"')
echo "$RESP"

HID=$(echo "$RESP" | awk -F': ' 'tolower($1)=="x-request-id"{print $2}' | tr -d '\r')
# Extract body (skip headers)
BODY=$(echo "$RESP" | sed -n '/^\r$/,$p')
BID=$(echo "$BODY" | jq -r '.request_id')

if [ -n "$HID" ] && [ "$HID" = "$BID" ]; then
  echo "request_id match: OK ($HID)"
else
  echo "request_id MISMATCH or MISSING: Header=$HID, Body=$BID"
  exit 1
fi

echo -e "\n--- Authz Error Path Check ---"
curl -si -X POST http://localhost:8080/v1/revoke \
  -H "Content-Type: application/json" \
  -d '{"level":"token","target":"x"}' | grep -E "x-request-id:|request_id"

echo -e "\n--- Log Check ---"
# Give a tiny bit of time for logs to flush
sleep 0.1
grep "request completed" /tmp/broker_manual.log | tail -n 3