---
description: "Start the AgentAuth broker for testing — Docker or VPS (binary) mode. Use when you need a running broker to test against: after cherry-picks, during development, for smoke tests, or before merging. Trigger on: 'bring up the broker', 'start broker', 'test broker', 'broker-up', 'run smoke test', 'start the stack', 'bring up docker', 'run the broker'."
---

# broker-up — Start Broker for Testing

Brings up the AgentAuth broker in a consistent, repeatable way. Two modes:

- **Docker** — builds image, starts container, runs health check. Production-like.
- **VPS** (binary) — builds Go binary, runs it directly. Faster iteration.

Both modes use the same env vars and verify the broker is healthy before returning control.

---

## Step 0: Ask Mode

Ask the operator which mode to use:

1. **Docker** (recommended for merge verification, cherry-pick testing)
2. **VPS / Binary** (faster, good for development iteration)
3. **Docker + Smoke** (Docker mode + full API smoke test)

If the operator said "smoke test" or "full test", default to option 3.

---

## Step 1: Prerequisites Check

Before starting, verify the environment:

```bash
cd /Users/divineartis/proj/agentauth-core

# Check we're in the right repo
if [ ! -f cmd/broker/main.go ]; then
  echo "ERROR: Not in agentauth-core. cd to /Users/divineartis/proj/agentauth-core"
  exit 1
fi

# Check Go is available (VPS mode)
go version 2>/dev/null || echo "WARN: Go not found — Docker mode only"

# Check Docker is available (Docker mode)
docker compose version 2>/dev/null || echo "WARN: Docker Compose not found — VPS mode only"
```

---

## Step 2a: Docker Mode

### Build and Start

```bash
cd /Users/divineartis/proj/agentauth-core

# Set test env vars
export AA_ADMIN_SECRET="test-secret-minimum-32-characters-long"
export AA_HOST_PORT=8080
export AA_LOG_LEVEL=standard

# Build fresh image (no cache to pick up code changes)
echo "=== Building broker Docker image ==="
docker compose build --no-cache broker

# Stop any existing instance
docker compose down 2>/dev/null || true

# Start broker
echo "=== Starting broker container ==="
docker compose up -d broker

# Wait for health
echo "=== Waiting for health check ==="
for i in $(seq 1 15); do
  if curl -sf http://127.0.0.1:${AA_HOST_PORT}/v1/health > /dev/null 2>&1; then
    echo "Broker is healthy (attempt $i)"
    break
  fi
  if [ $i -eq 15 ]; then
    echo "ERROR: Broker failed to start after 15 attempts"
    docker compose logs broker
    exit 1
  fi
  sleep 1
done

# Show status
echo "=== Broker Running ==="
echo "Health:  http://127.0.0.1:${AA_HOST_PORT}/v1/health"
echo "Metrics: http://127.0.0.1:${AA_HOST_PORT}/v1/metrics"
echo "Admin:   POST http://127.0.0.1:${AA_HOST_PORT}/v1/admin/auth"
echo ""
echo "To stop: docker compose down"
```

### Teardown

```bash
docker compose down
docker volume rm agentauth-core_broker-data 2>/dev/null || true
echo "Stack torn down."
```

---

## Step 2b: VPS / Binary Mode

### Build and Start

```bash
cd /Users/divineartis/proj/agentauth-core

# Set test env vars
export AA_ADMIN_SECRET="test-secret-minimum-32-characters-long"
export AA_PORT=8080
export AA_LOG_LEVEL=standard
export AA_DB_PATH="/tmp/agentauth-test.db"

# Build broker binary
echo "=== Building broker binary ==="
go build -o /tmp/agentauth-broker ./cmd/broker
echo "Build OK"

# Kill any existing instance
pkill -f "agentauth-broker" 2>/dev/null || true
sleep 1

# Start broker in background
echo "=== Starting broker (VPS mode) ==="
/tmp/agentauth-broker &
BROKER_PID=$!
echo "Broker PID: $BROKER_PID"

# Wait for health
echo "=== Waiting for health check ==="
for i in $(seq 1 10); do
  if curl -sf http://127.0.0.1:${AA_PORT}/v1/health > /dev/null 2>&1; then
    echo "Broker is healthy (attempt $i)"
    break
  fi
  if [ $i -eq 10 ]; then
    echo "ERROR: Broker failed to start after 10 attempts"
    kill $BROKER_PID 2>/dev/null
    exit 1
  fi
  sleep 1
done

# Show status
echo "=== Broker Running (PID $BROKER_PID) ==="
echo "Health:  http://127.0.0.1:${AA_PORT}/v1/health"
echo "Metrics: http://127.0.0.1:${AA_PORT}/v1/metrics"
echo "Admin:   POST http://127.0.0.1:${AA_PORT}/v1/admin/auth"
echo ""
echo "To stop: kill $BROKER_PID"
```

### Teardown

```bash
pkill -f "agentauth-broker" 2>/dev/null || true
rm -f /tmp/agentauth-test.db /tmp/agentauth-broker
echo "Binary and DB cleaned up."
```

---

## Step 3: Smoke Test (Optional — runs after broker is up)

Only run this if the operator chose "Docker + Smoke" or explicitly asked for a smoke test. This exercises the full core API.

**IMPORTANT:** This is the canonical smoke test. If you need to update it, update BOTH this skill AND `.plans/cherry-pick/TESTING.md` (G6 section).

```bash
PORT="${AA_HOST_PORT:-${AA_PORT:-8080}}"
SECRET="${AA_ADMIN_SECRET:-test-secret-minimum-32-characters-long}"
BASE="http://127.0.0.1:${PORT}"

echo ""
echo "=============================="
echo " AgentAuth Smoke Test"
echo "=============================="
echo ""

PASS=0
FAIL=0

# --- Test 1: Health Check ---
echo "--- [1/7] Health Check ---"
HTTP_CODE=$(curl -sf -o /dev/null -w "%{http_code}" "$BASE/v1/health")
if [ "$HTTP_CODE" = "200" ]; then
  echo "  PASS: Health returned $HTTP_CODE"
  PASS=$((PASS+1))
else
  echo "  FAIL: Health returned $HTTP_CODE (expected 200)"
  FAIL=$((FAIL+1))
fi

# --- Test 2: Admin Auth ---
echo "--- [2/7] Admin Auth ---"
ADMIN_RESP=$(curl -sf -X POST "$BASE/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$SECRET\"}")
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.access_token // empty')
if [ -n "$ADMIN_TOKEN" ]; then
  echo "  PASS: Got admin token (${ADMIN_TOKEN:0:20}...)"
  PASS=$((PASS+1))
else
  echo "  FAIL: No admin token in response: $ADMIN_RESP"
  FAIL=$((FAIL+1))
fi

# --- Test 3: Create Launch Token ---
echo "--- [3/7] Create Launch Token ---"
LT_RESP=$(curl -sf -X POST "$BASE/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"allowed_scope":["read:data","write:data"],"max_uses":5}')
LAUNCH_TOKEN=$(echo "$LT_RESP" | jq -r '.token // empty')
if [ -n "$LAUNCH_TOKEN" ]; then
  echo "  PASS: Got launch token (${LAUNCH_TOKEN:0:20}...)"
  PASS=$((PASS+1))
else
  echo "  FAIL: No launch token in response: $LT_RESP"
  FAIL=$((FAIL+1))
fi

# --- Test 4: Register Agent ---
echo "--- [4/7] Register Agent ---"
REG_RESP=$(curl -sf -X POST "$BASE/v1/register" \
  -H "Authorization: Bearer $LAUNCH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"sub":"spiffe://agentauth/agent/smoke-001","scope":["read:data"],"task_id":"smoke-task","orch_id":"smoke-orch"}')
AGENT_TOKEN=$(echo "$REG_RESP" | jq -r '.access_token // empty')
if [ -n "$AGENT_TOKEN" ]; then
  echo "  PASS: Registered agent (${AGENT_TOKEN:0:20}...)"
  PASS=$((PASS+1))
else
  echo "  FAIL: Agent registration failed: $REG_RESP"
  FAIL=$((FAIL+1))
fi

# --- Test 5: Validate Token ---
echo "--- [5/7] Validate Token ---"
VAL_RESP=$(curl -sf -X POST "$BASE/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$AGENT_TOKEN\"}")
VAL_VALID=$(echo "$VAL_RESP" | jq -r '.valid // empty')
if [ "$VAL_VALID" = "true" ]; then
  echo "  PASS: Token validated"
  PASS=$((PASS+1))
else
  echo "  FAIL: Token validation failed: $VAL_RESP"
  FAIL=$((FAIL+1))
fi

# --- Test 6: Renew Token ---
echo "--- [6/7] Renew Token ---"
RENEW_RESP=$(curl -sf -X POST "$BASE/v1/token/renew" \
  -H "Authorization: Bearer $AGENT_TOKEN")
NEW_TOKEN=$(echo "$RENEW_RESP" | jq -r '.access_token // empty')
if [ -n "$NEW_TOKEN" ]; then
  echo "  PASS: Token renewed (${NEW_TOKEN:0:20}...)"
  PASS=$((PASS+1))
else
  echo "  FAIL: Token renewal failed: $RENEW_RESP"
  FAIL=$((FAIL+1))
fi

# --- Test 7: Audit Trail ---
echo "--- [7/7] Audit Events ---"
AUDIT_RESP=$(curl -sf "$BASE/v1/audit/events" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
EVENT_COUNT=$(echo "$AUDIT_RESP" | jq -r '.events | length // 0')
if [ "$EVENT_COUNT" -gt 0 ] 2>/dev/null; then
  echo "  PASS: $EVENT_COUNT audit events recorded"
  PASS=$((PASS+1))
else
  echo "  FAIL: No audit events found: $AUDIT_RESP"
  FAIL=$((FAIL+1))
fi

# --- Summary ---
echo ""
echo "=============================="
echo " Results: $PASS PASS / $FAIL FAIL / 7 total"
echo "=============================="

if [ "$FAIL" -gt 0 ]; then
  echo "SMOKE TEST FAILED"
  exit 1
else
  echo "SMOKE TEST PASSED"
  exit 0
fi
```

---

## Step 4: Report

After the broker is up (and optionally smoke tested), report:

1. **Mode**: Docker or VPS
2. **Branch**: current git branch name
3. **Health**: URL and response
4. **Smoke result**: PASS/FAIL count (if run)
5. **How to stop**: exact teardown command

If anything failed, show the relevant error output and suggest next steps.

---

## Environment Variable Reference

| Variable | Default | What |
|----------|---------|------|
| `AA_ADMIN_SECRET` | `test-secret-minimum-32-characters-long` | Admin auth secret (min 32 chars) |
| `AA_PORT` | `8080` | Broker listen port (VPS mode) |
| `AA_HOST_PORT` | `8080` | Host port mapping (Docker mode) |
| `AA_LOG_LEVEL` | `standard` | Log verbosity: `quiet`, `standard`, `verbose` |
| `AA_DB_PATH` | `/data/agentauth.db` (Docker) or `/tmp/agentauth-test.db` (VPS) | SQLite database path |
| `AA_TLS_MODE` | `none` | Transport: `none`, `tls`, `mtls` |
| `AA_TLS_CERT` | — | TLS certificate path (required if TLS enabled) |
| `AA_TLS_KEY` | — | TLS private key path (required if TLS enabled) |
| `AA_TLS_CLIENT_CA` | — | Client CA path (required for mTLS) |
| `AA_AUDIENCE` | — | Expected JWT audience claim |
| `AA_APP_TOKEN_TTL` | — | Default app token TTL override |

---

## TLS Modes (Docker Only)

For TLS testing, use overlay compose files:

```bash
# Generate test certs first
./scripts/gen_test_certs.sh

# One-way TLS
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d broker

# Mutual TLS
docker compose -f docker-compose.yml -f docker-compose.mtls.yml up -d broker
```

Health check with TLS:
```bash
curl -sf --cacert /tmp/agentauth-certs/ca.pem https://127.0.0.1:8080/v1/health
```
