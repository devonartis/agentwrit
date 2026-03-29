# P1: Admin Secret (Bcrypt + `aactl init`) — User Stories

**Spec:** `.plans/specs/2026-03-17-p1-admin-secret-spec.md`
**Plan:** `.plans/2026-03-17-p1-admin-secret-plan.md`
**Branch:** `feature/p1-admin-secret`
**Docker stack:** `./scripts/stack_up.sh`

---

## Testing Scenarios — VPS First, Container Second

> **IMPORTANT:** Every Docker story in this file is tested in TWO deployment
> modes. VPS mode runs FIRST to prove the application works. Container mode
> runs SECOND to prove the deployment works. If VPS passes but Container
> fails, the bug is in deployment config. If VPS fails, the bug is in the
> application — don't bother testing Container until VPS passes.

### VPS Mode (Application Test)

Run the compiled broker binary directly on the host machine. No Docker, no
containers, no volume mounts. The binary reads config files and environment
variables from the local filesystem, exactly like it would on a VPS with
systemd or a bare-metal server.

```bash
# Build the broker binary
go build -o bin/broker ./cmd/broker/

# Run it directly (like systemd would)
AA_CONFIG_PATH=/path/to/config \
AA_DB_PATH=/tmp/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/signing.key \
  ./bin/broker
```

**What this proves:** The Go binary works. Config file parsing works. Bcrypt
authentication works. None of these depend on Docker — they're application
logic. If this fails, the bug is in the code.

### Container Mode (Deployment Test)

Run the broker inside a Docker container. Config files are mounted as
volumes. Environment variables are passed via `-e` flags or docker-compose.

```bash
# Docker run (explicit)
docker run -d --name aa-broker \
  -p 8080:8080 \
  -v /path/to/config-dir:/etc/agentauth:ro \
  -e AA_CONFIG_PATH=/etc/agentauth/config \
  agentauth-broker

# Docker Compose (standard stack)
./scripts/stack_up.sh
```

**What this proves:** The Docker image builds correctly. Volume mounts work.
Environment variable passthrough works. Container networking works. If VPS
mode passes but this fails, the bug is in docker-compose.yml, the Dockerfile,
or the volume mount — not in the application.

### Story Categories

| Stories | Mode | Why |
|---------|------|-----|
| S1–S4 | CLI only | Tests `aactl init` — no broker involved |
| S5 | VPS + Container | Config file boot — must work in both |
| S6 | VPS + Container | Env var backward compat — must work in both |
| S7 | VPS + Container | Env override precedence — must work in both |
| S8 | VPS + Container | Dev mode warning — check stdout (VPS) and docker logs (Container) |
| S9 | VPS | Bcrypt timing — behavioral test, deployment mode doesn't matter |

---

## P1-S1: Operator Generates Admin Secret in Dev Mode

**Tracker:** p1-s1
**Plan tasks:** Task 5
**Persona:** Operator
**Tool:** aactl

The operator is setting up AgentAuth on their laptop for the first time.
They run `aactl init` with no flags. Dev mode is the default because most
first-time users are experimenting locally, not deploying to production.
The command generates a strong random secret, writes it to a config file,
and prints it so the operator can copy it to their environment.

In dev mode the plaintext is stored in the config file. This is a
convenience trade-off — the operator can read it back if they forget it.
The security trade-off is acceptable because this is a local dev machine,
not a production server.

**Route:** N/A (local CLI command)
**Steps:**
1. Clean any previous test config: `rm -rf /tmp/aa-test-p1-dev`
2. Run `aactl init --config-path /tmp/aa-test-p1-dev/config`
3. Check the printed output for the secret
4. Read the config file and verify it contains `MODE=development`
5. Verify the config file contains the same plaintext secret that was printed
6. Check file permissions are 0600
7. Check directory permissions are 0700

**Expected:**
- Prints a base64url-encoded secret (43+ characters, no special chars)
- Config file exists at `/tmp/aa-test-p1-dev/config`
- Config file contains `MODE=development` and `ADMIN_SECRET=<the printed secret>`
- File permissions are `-rw-------` (0600)
- Directory permissions are `drwx------` (0700)

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-dev
./bin/aactl init --config-path /tmp/aa-test-p1-dev/config
echo "---"
cat /tmp/aa-test-p1-dev/config
echo "---"
ls -la /tmp/aa-test-p1-dev/config
ls -ld /tmp/aa-test-p1-dev/
```

---

## P1-S2: Operator Generates Admin Secret in Prod Mode

**Tracker:** p1-s2
**Plan tasks:** Task 5
**Persona:** Operator
**Tool:** aactl

The operator is deploying AgentAuth to a production VPS. They run
`aactl init --mode=prod`. The command generates the same strong random
secret, but this time it only stores the bcrypt hash on disk. The
plaintext is printed once with a clear warning — if the operator loses
it, they must re-run `aactl init --force` to reset everything.

This is the HashiCorp Vault pattern: `vault operator init` shows the
unseal keys once and never again. The operator is responsible for storing
the secret in their own secrets manager.

**Route:** N/A (local CLI command)
**Steps:**
1. Clean any previous test config: `rm -rf /tmp/aa-test-p1-prod`
2. Run `aactl init --mode=prod --config-path /tmp/aa-test-p1-prod/config`
3. Check the printed output for the secret and the "save now" warning
4. Read the config file and verify it contains `MODE=production`
5. Verify the config file contains a bcrypt hash (`$2a$12$...`), NOT the plaintext
6. Verify the plaintext secret does NOT appear anywhere in the config file
7. Check file permissions are 0600

**Expected:**
- Prints a base64url-encoded secret (43+ characters)
- Prints a warning: "Save this secret now. It will not be shown again."
- Config file contains `MODE=production`
- Config file `ADMIN_SECRET` value starts with `$2a$12$` (bcrypt hash)
- The plaintext secret does NOT appear in the config file
- File permissions are `-rw-------` (0600)

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-prod
./bin/aactl init --mode=prod --config-path /tmp/aa-test-p1-prod/config
echo "---"
cat /tmp/aa-test-p1-prod/config
echo "---"
ls -la /tmp/aa-test-p1-prod/config
```

---

## P1-S3: Operator Cannot Accidentally Overwrite Config

**Tracker:** p1-s3
**Plan tasks:** Task 5
**Persona:** Operator
**Tool:** aactl

The operator has already run `aactl init` and has a working config file
from S1. They accidentally run `aactl init` again — maybe they forgot
they already set it up, or they're running a setup script that calls
init unconditionally. The command must refuse to overwrite, because
doing so would generate a new secret, invalidate the old one, and lock
the operator out of the broker with no way to recover.

**Route:** N/A (local CLI command)
**Steps:**
1. Confirm the config file from S1 still exists
2. Run `aactl init --config-path /tmp/aa-test-p1-dev/config` (same path as S1)
3. Verify the command fails with a clear error
4. Verify the original config file is unchanged

**Expected:**
- Command exits with a non-zero status
- Error message contains "already exists"
- Error message mentions `--force` as the way to override
- Original config file is untouched (same content as after S1)

**Test command:**
```bash
# Save original for comparison
cp /tmp/aa-test-p1-dev/config /tmp/aa-test-p1-dev/config.before
# Try to overwrite
./bin/aactl init --config-path /tmp/aa-test-p1-dev/config 2>&1; echo "exit=$?"
# Verify unchanged
diff /tmp/aa-test-p1-dev/config /tmp/aa-test-p1-dev/config.before
```

---

## P1-S4: Operator Force-Overwrites Config

**Tracker:** p1-s4
**Plan tasks:** Task 5
**Persona:** Operator
**Tool:** aactl

After S3, the operator decides they actually do want a new secret —
maybe the old one was compromised, or they're rotating credentials
manually. They run `aactl init --force` to deliberately reset it.
The old secret is gone forever. This is the recovery path.

**Route:** N/A (local CLI command)
**Steps:**
1. Note the current secret from the config file
2. Run `aactl init --force --config-path /tmp/aa-test-p1-dev/config`
3. Verify the command succeeds
4. Verify the new secret is different from the old one
5. Verify the config file was overwritten with the new secret

**Expected:**
- Command succeeds (exit 0)
- Prints a new admin secret
- New secret is different from the S1 secret
- Config file contains the new secret

**Test command:**
```bash
OLD_SECRET=$(grep ADMIN_SECRET /tmp/aa-test-p1-dev/config | cut -d= -f2)
./bin/aactl init --force --config-path /tmp/aa-test-p1-dev/config
echo "---"
NEW_SECRET=$(grep ADMIN_SECRET /tmp/aa-test-p1-dev/config | cut -d= -f2)
echo "Old: $OLD_SECRET"
echo "New: $NEW_SECRET"
[ "$OLD_SECRET" != "$NEW_SECRET" ] && echo "DIFFERENT" || echo "SAME (BAD)"
```

---

## P1-S5a: Broker Starts with Config File — VPS Mode

**Tracker:** p1-s5
**Plan tasks:** Task 1, Task 2, Task 4
**Persona:** Operator
**Tool:** aactl + broker binary (VPS)
**Mode:** VPS (binary on host) — runs FIRST

The operator deploys AgentAuth on a VPS (bare-metal server, EC2 instance,
etc.). They ran `aactl init --mode=prod` to generate a config file with a
bcrypt hash. Now they start the broker binary directly — no Docker, no
containers. The binary reads `AA_CONFIG_PATH` from the environment and loads
the admin secret hash from the config file. The operator does NOT set
`AA_ADMIN_SECRET` — the config file is the only source of truth.

This is the simplest production deployment: a compiled binary on a server
reading a config file from disk, exactly like PostgreSQL, Nginx, or any
other server process.

**Route:** GET /v1/health (via curl), admin auth via aactl
**Steps:**
1. Generate a prod config file with `aactl init --mode=prod`
2. Note the printed secret
3. Start the broker binary with `AA_CONFIG_PATH` pointing to the config
   file, `AA_ADMIN_SECRET` unset, and a temp DB + key path
4. Check that the broker is healthy (curl health endpoint)
5. Set `AACTL_ADMIN_SECRET` to the correct secret and run `aactl app list`
6. Set `AACTL_ADMIN_SECRET` to a wrong secret and run `aactl app list`
7. Stop the broker

**Expected:**
- Broker starts and health check returns 200
- `aactl app list` with the correct secret succeeds
- `aactl app list` with a wrong secret fails with an auth error

**Test command:**
```bash
# Step 1-2: Generate config
rm -rf /tmp/aa-test-p1-vps
./bin/aactl init --mode=prod --config-path /tmp/aa-test-p1-vps/config
# (note the secret from output)

# Step 3: Start broker binary directly (VPS mode)
AA_CONFIG_PATH=/tmp/aa-test-p1-vps/config \
AA_DB_PATH=/tmp/aa-test-p1-vps/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/aa-test-p1-vps/signing.key \
AA_ADMIN_SECRET="" \
  ./bin/broker &
BROKER_PID=$!
sleep 2

# Step 4: Health check
curl -s http://127.0.0.1:8080/v1/health

# Step 5: Auth with correct secret via aactl
AACTL_BROKER_URL=http://127.0.0.1:8080 \
  AACTL_ADMIN_SECRET="<SECRET_FROM_STEP_1>" \
  ./bin/aactl app list

# Step 6: Auth with wrong secret via aactl
AACTL_BROKER_URL=http://127.0.0.1:8080 \
  AACTL_ADMIN_SECRET="wrong-secret" \
  ./bin/aactl app list 2>&1; echo "exit=$?"

# Step 7: Cleanup
kill $BROKER_PID
```

---

## P1-S5b: Broker Starts with Config File — Container Mode

**Tracker:** p1-s5
**Plan tasks:** Task 1, Task 2, Task 4
**Persona:** Operator
**Tool:** aactl + Docker
**Mode:** Container (docker run) — runs SECOND, only after S5a passes

The operator deploys AgentAuth as a Docker container (Kubernetes, ECS,
Docker Compose, etc.). The config file was generated on the host by
`aactl init --mode=prod` and is mounted into the container as a read-only
volume. The container reads `AA_CONFIG_PATH` which points to the mounted
file inside the container's filesystem.

This tests the container deployment path: the config file crosses a
filesystem boundary (host → container volume mount). If S5a passes but
this fails, the bug is in the Docker image, the volume mount, or the
path mapping — not in the application.

**Route:** GET /v1/health (via curl), admin auth via aactl
**Steps:**
1. Reuse the config file from S5a (or generate a new one)
2. Start a Docker container with the config directory mounted and
   `AA_CONFIG_PATH` pointing to the container-internal path
3. Check that the broker is healthy
4. Run `aactl app list` with the correct secret
5. Run `aactl app list` with a wrong secret
6. Stop and remove the container

**Expected:**
- Broker starts and health check returns 200
- `aactl app list` with the correct secret succeeds
- `aactl app list` with a wrong secret fails with an auth error
- Identical behavior to S5a (VPS mode)

**Test command:**
```bash
# Step 1: Reuse config from S5a or generate new
# (config at /tmp/aa-test-p1-vps/config from S5a)

# Step 2: Start container with mounted config
docker run -d --name aa-p1-container \
  -p 8080:8080 \
  -v /tmp/aa-test-p1-vps:/etc/agentauth:ro \
  -e AA_CONFIG_PATH=/etc/agentauth/config \
  -e AA_ADMIN_SECRET="" \
  -e AA_DB_PATH=/tmp/agentauth.db \
  -e AA_SIGNING_KEY_PATH=/tmp/signing.key \
  agentauth-broker
sleep 3

# Step 3: Health check
curl -s http://127.0.0.1:8080/v1/health

# Step 4: Auth with correct secret via aactl
AACTL_BROKER_URL=http://127.0.0.1:8080 \
  AACTL_ADMIN_SECRET="<SECRET_FROM_S5A>" \
  ./bin/aactl app list

# Step 5: Auth with wrong secret via aactl
AACTL_BROKER_URL=http://127.0.0.1:8080 \
  AACTL_ADMIN_SECRET="wrong-secret" \
  ./bin/aactl app list 2>&1; echo "exit=$?"

# Step 6: Cleanup
docker stop aa-p1-container && docker rm aa-p1-container
```

---

## P1-S6: Backward Compatibility — Env Var Still Works

**Tracker:** p1-s6
**Plan tasks:** Task 3, Task 4
**Persona:** Developer
**Tool:** curl + broker binary (VPS), curl + Docker (Container)
**Mode:** VPS first, Container second

The developer has an existing setup that sets
`AA_ADMIN_SECRET=change-me-in-production` as an environment variable.
They have no config file and have never run `aactl init`. This is
the workflow that existed before P1. Everything must work exactly as
before — same env var, same curl commands, same 200 response.

If this story fails, P1 broke backward compatibility and existing
users can't upgrade without changing their deployment.

**Route:** GET /v1/health, POST /v1/admin/auth

### S6 — VPS Mode (runs first)

**Steps:**
1. Start the broker binary with `AA_ADMIN_SECRET=change-me-in-production`,
   no config file, no `AA_CONFIG_PATH`
2. Health check
3. Authenticate with the default admin secret
4. Authenticate with a wrong secret
5. Stop the broker

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-s6-vps
AA_ADMIN_SECRET=change-me-in-production \
AA_DB_PATH=/tmp/aa-test-p1-s6-vps/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/aa-test-p1-s6-vps/signing.key \
  ./bin/broker &
BROKER_PID=$!
sleep 2

curl -s http://127.0.0.1:8080/v1/health

curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"change-me-in-production"}' \
  -w "\nHTTP %{http_code}"

curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"wrong-secret"}' \
  -w "\nHTTP %{http_code}"

kill $BROKER_PID
```

### S6 — Container Mode (runs second)

**Steps:**
1. Start the standard Docker stack with `./scripts/stack_up.sh`
2. Source the environment file
3. Health check, authenticate correct, authenticate wrong

**Test command:**
```bash
./scripts/stack_up.sh
source ./tests/p1-admin-secret/env.sh

curl -s "$BROKER_URL/v1/health"

curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"change-me-in-production"}' \
  -w "\nHTTP %{http_code}"

curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"wrong-secret"}' \
  -w "\nHTTP %{http_code}"
```

**Expected (both modes):**
- Broker starts and health check returns 200
- `POST /v1/admin/auth` with `change-me-in-production` returns 200 + `access_token`
- `POST /v1/admin/auth` with wrong secret returns 401
- Identical behavior to pre-P1

---

## P1-S7: Env Var Overrides Config File

**Tracker:** p1-s7
**Plan tasks:** Task 2
**Persona:** Developer
**Tool:** curl + broker binary (VPS), curl + Docker (Container)
**Mode:** VPS first, Container second

The developer has both a config file and the `AA_ADMIN_SECRET` env var
set to different values. This happens when the operator generates a config
file with `aactl init` but then overrides the secret via an environment
variable (on a VPS via systemd override, or in Kubernetes via a Secret
mounted as an env var). The env var must win — this is the standard
override pattern (env > file > defaults).

If the config file wins, operators can't override secrets without
modifying the config file, which breaks environment-based configuration.

**Route:** POST /v1/admin/auth

### S7 — VPS Mode (runs first)

**Steps:**
1. Generate a dev config file with a known secret
2. Start the broker binary with both `AA_CONFIG_PATH` and `AA_ADMIN_SECRET`
   set to a different value
3. Authenticate with the env var secret — should succeed
4. Authenticate with the config file secret — should fail
5. Stop the broker

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-s7-vps
./bin/aactl init --config-path /tmp/aa-test-p1-s7-vps/config
CONFIG_SECRET=$(grep ADMIN_SECRET /tmp/aa-test-p1-s7-vps/config | cut -d= -f2)
ENV_SECRET="env-override-secret-for-testing"

AA_CONFIG_PATH=/tmp/aa-test-p1-s7-vps/config \
AA_ADMIN_SECRET="$ENV_SECRET" \
AA_DB_PATH=/tmp/aa-test-p1-s7-vps/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/aa-test-p1-s7-vps/signing.key \
  ./bin/broker &
BROKER_PID=$!
sleep 2

# Env var secret (should work)
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$ENV_SECRET\"}" \
  -w "\nHTTP %{http_code}"

# Config file secret (should fail)
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$CONFIG_SECRET\"}" \
  -w "\nHTTP %{http_code}"

kill $BROKER_PID
```

### S7 — Container Mode (runs second)

**Steps:**
1. Reuse the config file from S7 VPS
2. Start a Docker container with both the mounted config and a different
   `AA_ADMIN_SECRET` env var
3. Authenticate with the env var secret — should succeed
4. Authenticate with the config file secret — should fail

**Test command:**
```bash
docker run -d --name aa-p1-override \
  -p 8080:8080 \
  -v /tmp/aa-test-p1-s7-vps:/etc/agentauth:ro \
  -e AA_CONFIG_PATH=/etc/agentauth/config \
  -e AA_ADMIN_SECRET="$ENV_SECRET" \
  -e AA_DB_PATH=/tmp/agentauth.db \
  -e AA_SIGNING_KEY_PATH=/tmp/signing.key \
  agentauth-broker
sleep 3

curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$ENV_SECRET\"}" \
  -w "\nHTTP %{http_code}"

curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$CONFIG_SECRET\"}" \
  -w "\nHTTP %{http_code}"

docker stop aa-p1-override && docker rm aa-p1-override
```

**Expected (both modes):**
- `POST /v1/admin/auth` with the env var secret returns 200
- `POST /v1/admin/auth` with the config file secret returns 401
- The env var won

---

## P1-S8: Dev Mode Startup Warning

**Tracker:** p1-s8
**Plan tasks:** Task 4
**Persona:** Operator
**Tool:** broker binary stdout (VPS), Docker logs (Container)
**Mode:** VPS first, Container second

The operator starts the broker in development mode. The broker must log
a warning so the operator knows the admin secret is stored as plaintext
on disk. This is a safety net — if someone accidentally runs a dev
config in production, the warning shows up in the logs and monitoring
systems can alert on it.

Without this warning, an operator might not realize their production
broker is running with plaintext secrets on disk until a security audit
catches it.

**Route:** N/A (log inspection)

### S8 — VPS Mode (runs first)

In VPS mode the warning goes to stdout/stderr, which is what systemd
or any process manager captures into the journal.

**Steps:**
1. Generate a dev-mode config file
2. Start the broker binary with `AA_CONFIG_PATH` pointing to the dev config
3. Check stdout for the development mode warning
4. Stop the broker

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-s8-vps
./bin/aactl init --config-path /tmp/aa-test-p1-s8-vps/config

AA_CONFIG_PATH=/tmp/aa-test-p1-s8-vps/config \
AA_DB_PATH=/tmp/aa-test-p1-s8-vps/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/aa-test-p1-s8-vps/signing.key \
AA_ADMIN_SECRET="" \
  ./bin/broker &
BROKER_PID=$!
sleep 2

# The warning should have appeared in stdout above
# Verify by checking if the broker is running (it should be)
curl -s http://127.0.0.1:8080/v1/health

kill $BROKER_PID
```

### S8 — Container Mode (runs second)

In container mode the warning goes to docker logs.

**Steps:**
1. Reuse the dev config from S8 VPS
2. Start a Docker container with `AA_CONFIG_PATH`
3. Check `docker logs` for the warning

**Test command:**
```bash
docker run -d --name aa-p1-devwarn \
  -p 8080:8080 \
  -v /tmp/aa-test-p1-s8-vps:/etc/agentauth:ro \
  -e AA_CONFIG_PATH=/etc/agentauth/config \
  -e AA_ADMIN_SECRET="" \
  -e AA_DB_PATH=/tmp/agentauth.db \
  -e AA_SIGNING_KEY_PATH=/tmp/signing.key \
  agentauth-broker
sleep 3

docker logs aa-p1-devwarn 2>&1 | grep -i "development mode"

docker stop aa-p1-devwarn && docker rm aa-p1-devwarn
```

**Expected (both modes):**
- Broker logs contain a warning about development mode and plaintext secrets

---

## P1-S9: Security — Bcrypt Timing Resistance

**Tracker:** p1-s9
**Plan tasks:** Task 3
**Persona:** Security reviewer
**Tool:** curl + broker binary (VPS)
**Mode:** VPS only — this is a behavioral test of the application's auth
logic, not a deployment test. Bcrypt works identically in VPS and Container
mode because it's pure Go code with no external dependencies.

The security reviewer verifies that the admin auth endpoint behaves
consistently regardless of how wrong the submitted secret is. With
the old plaintext comparison, a timing side-channel could theoretically
reveal how many bytes of the secret matched — an attacker could try
progressively longer prefixes and measure response time to narrow down
the secret. With bcrypt, the full hash computation runs regardless of
input, so all wrong secrets take the same amount of time.

This test checks the behavioral side: all wrong secrets return the same
HTTP status and response shape. The timing guarantee comes from bcrypt's
internal constant-time comparison.

**Route:** POST /v1/admin/auth
**Steps:**
1. Start the broker binary with `AA_ADMIN_SECRET=change-me-in-production`
2. Send a wrong secret of the same length as the real one
3. Send a wrong secret of a different (shorter) length
4. Send a wrong secret of a different (longer) length
5. Send an empty secret
6. Verify all responses are consistent
7. Stop the broker

**Expected:**
- Same-length wrong secret: 401, `application/problem+json`
- Shorter wrong secret: 401, `application/problem+json`
- Longer wrong secret: 401, `application/problem+json`
- Empty secret: 400 (validation error, not auth failure)
- All 401 response bodies have the same structure (no info leakage)

**Test command:**
```bash
rm -rf /tmp/aa-test-p1-s9
AA_ADMIN_SECRET=change-me-in-production \
AA_DB_PATH=/tmp/aa-test-p1-s9/agentauth.db \
AA_SIGNING_KEY_PATH=/tmp/aa-test-p1-s9/signing.key \
  ./bin/broker &
BROKER_PID=$!
sleep 2

# Same length (26 chars like "change-me-in-production")
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"AAAAAAAAAAAAAAAAAAAAAAAAAA"}' \
  -w "\nHTTP %{http_code}"

echo "---"

# Shorter
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"short"}' \
  -w "\nHTTP %{http_code}"

echo "---"

# Longer
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"this-is-a-very-long-secret-that-is-much-longer-than-the-real-one-to-test-timing"}' \
  -w "\nHTTP %{http_code}"

echo "---"

# Empty (should be 400 not 401)
curl -s -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":""}' \
  -w "\nHTTP %{http_code}"

kill $BROKER_PID
```
