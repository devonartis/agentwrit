# P0: Production Foundations — User Stories

**Spec:** `.plans/specs/2026-03-16-p0-production-foundations-spec.md`
**Plan:** `.plans/2026-03-16-p0-production-foundations-plan.md`
**Tracker stories:** p0-k1, p0-k2, p0-k3, p0-k4, p0-k5, p0-s1, p0-s2

**Prerequisites:**
- Docker stack running: `./scripts/stack_up.sh`
- awrit built: `go build -o ./bin/awrit ./cmd/awrit/`
- Source env: `source ./tests/p0-production-foundations/env.sh`
- Broker started with `AA_SEED_TOKENS=true` (default in docker-compose for dev)

---

## P0-K1: Key File Created with Secure Permissions

**Tracker:** p0-k1
**Plan tasks:** Task 2, Task 6
**Persona:** Security reviewer
**Tool:** docker exec + shell

The security reviewer checks that when the broker starts for the first
time, it creates a signing key file on disk. The file must have 0600
permissions so that only the broker process owner can read the private
key material. If the file were world-readable, any process on the same
host could steal the signing key and forge tokens.

**Route:** N/A (filesystem check)
**Steps:**
1. Bring up the Docker stack (broker creates the key on first start)
2. Exec into the broker container
3. Check that `/data/signing.key` exists
4. Check that its permissions are 0600
5. Check that it contains a PEM-encoded private key header

**Expected:**
- File exists at `/data/signing.key`
- Permissions are `-rw-------` (0600)
- File starts with `-----BEGIN PRIVATE KEY-----`

**Test command:**
```bash
docker compose -p agentauth exec broker sh -c \
  'ls -la /data/signing.key && head -1 /data/signing.key'
```

---

## P0-K2: Token Survives Broker Restart

**Tracker:** p0-k2
**Plan tasks:** Task 2, Task 3
**Persona:** Developer
**Tool:** curl

The developer gets an admin token from the broker, then the broker
restarts. After restart, the developer validates the same token. Before
this fix, every restart generated a new signing key, which meant every
token issued before the restart became invalid. Now the key is loaded
from disk, so the token's signature still checks out.

**Route:** POST /v1/admin/auth, POST /v1/token/validate
**Steps:**
1. Authenticate as admin to get a token
2. Validate the token (confirm it works before restart)
3. Restart the broker: `docker compose -p agentauth restart broker`
4. Wait for healthy
5. Validate the same token again

**Expected:**
- Token validates successfully before restart (`valid: true`)
- Token validates successfully after restart (`valid: true`)

**Test command:**
```bash
# Step 1: Get admin token
TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"change-me-in-production"}' | jq -r '.access_token')

# Step 2: Validate before restart
curl -s -X POST "$BROKER_URL/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$TOKEN\"}"

# Step 3: Restart
docker compose -p agentauth restart broker

# Step 4: Wait for healthy
sleep 3 && curl -s "$BROKER_URL/v1/health"

# Step 5: Validate after restart
curl -s -X POST "$BROKER_URL/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$TOKEN\"}"
```

---

## P0-K3: Configurable Key Path via AA_SIGNING_KEY_PATH

**Tracker:** p0-k3
**Plan tasks:** Task 1, Task 6
**Persona:** Operator
**Tool:** docker compose config

The operator wants to control where the signing key is stored. By
default it goes to `./signing.key` (or `/data/signing.key` in Docker).
The operator can override this with the `AA_SIGNING_KEY_PATH` environment
variable. This test verifies that docker-compose.yml passes the variable
through to the container.

**Route:** N/A (configuration check)
**Steps:**
1. Check that `docker-compose.yml` includes `AA_SIGNING_KEY_PATH`
2. Verify the default value resolves to `/data/signing.key`
3. Exec into the container and confirm the key is at that path

**Expected:**
- `docker compose config` shows `AA_SIGNING_KEY_PATH: /data/signing.key`
- The key file exists at `/data/signing.key` inside the container

**Test command:**
```bash
docker compose -p agentauth config | grep SIGNING_KEY_PATH
docker compose -p agentauth exec broker ls -la /data/signing.key
```

---

## P0-K4: Token Renewal Works After Restart

**Tracker:** p0-k4
**Plan tasks:** Task 2, Task 3
**Persona:** Developer
**Tool:** curl

The developer has a long-lived agent that renews its token periodically.
After a broker restart, the renewal must still work because the signing
key is the same. The developer sends a renewal request with a Bearer
token that was issued before the restart. The broker verifies the old
token (using the same key it loaded from disk), issues a fresh token,
and returns it.

**Route:** POST /v1/admin/auth, POST /v1/token/renew
**Steps:**
1. Get an admin token before restart
2. Restart the broker
3. Wait for healthy
4. Renew the token using the pre-restart Bearer token

**Expected:**
- Renewal returns HTTP 200 with a new `access_token` and `expires_in`
- The new token is different from the old one (fresh JTI and timestamps)

**Test command:**
```bash
# Step 1: Get token
TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"change-me-in-production"}' | jq -r '.access_token')

# Step 2-3: Restart and wait
docker compose -p agentauth restart broker && sleep 3

# Step 4: Renew
curl -s -X POST "$BROKER_URL/v1/token/renew" \
  -H "Authorization: Bearer $TOKEN" \
  -w "\nHTTP %{http_code}"
```

---

## P0-K5: Corrupt Key File Fails Fast

**Tracker:** p0-k5
**Plan tasks:** Task 2
**Persona:** Security reviewer
**Tool:** docker exec + shell

The security reviewer corrupts the signing key file to simulate disk
damage or tampering. When the broker restarts, it must refuse to start
and print a clear error message. It must NOT silently generate a new
key, because that would invalidate all existing tokens without the
operator knowing why.

**Route:** N/A (startup behavior)
**Steps:**
1. Stop the broker
2. Overwrite the key file with garbage: `echo "corrupted" > /data/signing.key`
3. Try to start the broker
4. Check the container logs for the error message

**Expected:**
- Broker exits with a non-zero status
- Logs contain "FATAL" and a message about the signing key
- Broker does NOT become healthy

**Test command:**
```bash
# Step 1: Stop
docker compose -p agentauth stop broker

# Step 2: Corrupt the key file (write garbage into the volume)
docker compose -p agentauth run --rm --no-deps broker \
  sh -c 'echo "corrupted" > /data/signing.key'

# Step 3-4: Try to start and check logs
docker compose -p agentauth up -d broker && sleep 2
docker compose -p agentauth logs broker --tail 20
docker compose -p agentauth ps broker

# Cleanup: Remove corrupt key so next restart generates fresh
docker compose -p agentauth run --rm --no-deps broker \
  sh -c 'rm /data/signing.key'
docker compose -p agentauth up -d broker
```

---

## P0-S1: Graceful Shutdown on SIGTERM

**Tracker:** p0-s1
**Plan tasks:** Task 4
**Persona:** Operator
**Tool:** docker compose + logs

The operator stops the broker using `docker compose stop`, which sends
SIGTERM. The broker must log that it received the signal, finish any
work in progress, close the database, and exit cleanly. Before this
fix, the broker had no signal handler and just died immediately when
killed.

**Route:** N/A (shutdown behavior)
**Steps:**
1. Make sure the broker is running and healthy
2. Stop the broker with `docker compose stop`
3. Check the logs for shutdown messages

**Expected:**
- Logs contain "signal received" or "Shutting down gracefully"
- Logs contain "database closed" or "clean exit"
- Container exits with status 0

**Test command:**
```bash
# Step 1: Confirm healthy
curl -s "$BROKER_URL/v1/health"

# Step 2: Stop
docker compose -p agentauth stop broker

# Step 3: Check logs
docker compose -p agentauth logs broker --tail 10
```

---

## P0-S2: SQLite Closed on Shutdown

**Tracker:** p0-s2
**Plan tasks:** Task 4, Task 5
**Persona:** Operator
**Tool:** docker compose + logs

The operator wants to be sure that when the broker shuts down, the
SQLite database is properly closed. An improperly closed database can
leave WAL journals behind or corrupt data. This test checks the broker
logs for the "database closed" message that the shutdown handler prints
after calling `sqlStore.Close()`.

**Route:** N/A (shutdown behavior, verified via logs)
**Steps:**
1. Bring up the broker and do something that writes to the database
   (e.g., hit the health endpoint, which triggers audit)
2. Stop the broker
3. Check logs for the database close confirmation

**Expected:**
- Logs contain "database closed"
- No SQLite error messages in logs

**Test command:**
```bash
# Step 1: Generate some DB activity
curl -s "$BROKER_URL/v1/health" > /dev/null

# Step 2: Stop
docker compose -p agentauth stop broker

# Step 3: Check logs
docker compose -p agentauth logs broker --tail 10 | grep -i "database\|sqlite\|close"
```
