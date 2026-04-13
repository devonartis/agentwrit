# Getting Started: Operator

Deploy AgentWrit in production. This guide covers broker startup, TLS configuration, app registration, scope ceilings, monitoring, and everything an operator needs to run the system securely.

**Prerequisites:** Familiarity with Linux administration, Docker, and TLS certificates.

---

## Deployment Options

There are three ways to deploy the broker, in order of how much you need to build locally:

| Option | When to use |
|---|---|
| **1. Pre-built Docker Hub image** | Production, fastest onboarding, no source checkout needed. Multi-arch (amd64 + arm64), signed with cosign. |
| **2. Docker Compose with local build** | You want to run unreleased code from `main` or `develop`, or you need to modify the Dockerfile. |
| **3. Native binary (VPS mode)** | Systemd-managed deployments, bare-metal hosts, no Docker available. |

---

## Option 1: Pre-built image from Docker Hub

The signed multi-arch broker image is published to [`devonartis/agentwrit`](https://hub.docker.com/r/devonartis/agentwrit) on every push to `main` and on release tags. This is the recommended operator onboarding path — no source checkout, no local build toolchain required.

### Available tags

| Tag | When it moves | Use for |
|---|---|---|
| `latest` | Every push to `main` | Quick evaluation, lab environments |
| `main-<sha>` | Every push to `main` | Reproducible deployments, pinning to a specific commit |
| `v2.0.0`, `v2.0`, `v2` | Release tags | **Production** — pin to an exact semver release |

Never pin production to `latest` — it moves with every `main` push and can introduce unexpected changes. Pin to a `v<semver>` tag or a `main-<sha>` digest and upgrade on a schedule you control.

### Pull and run

```bash
# 1. Set the admin secret (required — broker exits without it)
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

# 2. Pull a specific version (recommended over :latest for production)
docker pull devonartis/agentwrit:v2.0.0

# 3. Run with a persistent volume
docker run -d --name agentwrit \
  -p 8080:8080 \
  -e AA_ADMIN_SECRET \
  -e AA_BIND_ADDRESS=0.0.0.0 \
  -e AA_DB_PATH=/data/data.db \
  -e AA_SIGNING_KEY_PATH=/data/signing.key \
  -v agentwrit-data:/data \
  devonartis/agentwrit:v2.0.0

# 4. Verify
curl -s http://localhost:8080/v1/health | jq .
```

### Verifying the image signature

The release workflow signs every published image with [cosign](https://github.com/sigstore/cosign) keyless mode, using a short-lived Sigstore certificate issued via GitHub Actions OIDC. There is no long-lived signing key to rotate. Verify before running in production:

```bash
cosign verify devonartis/agentwrit:v2.0.0 \
  --certificate-identity-regexp='^https://github.com/devonartis/agentwrit/\.github/workflows/release\.yml@' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

If verification fails, **do not run the image** — it was not produced by this project's release pipeline. Report the discrepancy.

### What's in the image

- Multi-stage build (Alpine 3.21 runtime, ~12 MB compressed per arch)
- Platforms: `linux/amd64`, `linux/arm64`
- Binary: `/broker` (entry point)
- Ports exposed: `8080`
- OCI labels: `org.opencontainers.image.source`, `revision`, `created`, `title`, `description`, `licenses=PolyForm-Internal-Use-1.0.0`

### Docker Hub listing

- **Repository:** <https://hub.docker.com/r/devonartis/agentwrit>
- **Source:** <https://github.com/devonartis/agentwrit>
- **Issues and support:** <https://github.com/devonartis/agentwrit/issues>

---

## Option 2: Docker Compose (local build)

If you're working from a source checkout — either to track unreleased changes or to modify the Dockerfile — use Docker Compose:

```bash
# 1. Clone the repo
git clone https://github.com/devonartis/agentwrit.git
cd agentwrit

# 2. Set the admin secret (required -- broker exits without it)
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

# 3. Build and start the stack
./scripts/stack_up.sh

# 4. Verify the broker is healthy
curl http://localhost:8080/v1/health
# {"status":"ok","version":"2.0.0","uptime":5,"db_connected":true,"audit_events_count":0}
```

To swap the local build for the pre-built image without forking `docker-compose.yml`, edit the file and change `build: .` to `image: devonartis/agentwrit:v2.0.0` — the commented-out `image:` line at the top of the `broker:` service shows exactly where.

To tear down the stack:

```bash
./scripts/stack_down.sh
```

### What Docker Compose Deploys

```mermaid
flowchart LR
    subgraph "Docker: agentwrit-net (bridge)"
        B["Broker<br/>:8080"]
    end
    Dev["Developer App"] -- "http://localhost:8080" --> B
    Op["Operator"] -- "http://localhost:8080" --> B
    Prom["Prometheus"] -- "/v1/metrics" --> B

    classDef broker fill:#e3f2fd,stroke:#42a5f5,color:#0d47a1
    classDef client fill:#e8f5e9,stroke:#66bb6a,color:#1b5e20
    classDef obs fill:#f3e5f5,stroke:#ba68c8,color:#4a148c

    class B broker
    class Dev,Op client
    class Prom obs
```

The `docker-compose.yml` defines the broker service on a bridge network (`agentwrit-net`). The image uses a multi-stage Alpine build (golang:1.24 builder, alpine:3.18 runtime).

---

## Operator CLI (awrit)

`awrit` is the operator command-line tool for AgentWrit. It reads broker connection details from environment variables and handles authentication automatically.

### Install / build

```bash
go build -o awrit ./cmd/awrit/
```

### Initialize

Use `awrit init` to set up configuration. This creates a config file with broker URL and secret:

```bash
# Dev mode (creates ~/.broker/config)
awrit init --mode dev --force

# Prod mode with custom path
awrit init --mode prod --config-path /etc/broker/config
```

### Configure

```bash
export AACTL_BROKER_URL=http://localhost:8080
export AACTL_ADMIN_SECRET=my-secure-admin-secret-here
```

### Quick reference

```bash
# App management
awrit app register [--name NAME] [--scopes SCOPES]
awrit app list
awrit app get <app-id>
awrit app update --id <app-id> [--scopes SCOPES] [--token-ttl N]
awrit app remove --id <app-id>

# Revocation
awrit revoke --level token  --target <jti>
awrit revoke --level agent  --target spiffe://...
awrit revoke --level task   --target task-001
awrit revoke --level chain  --target spiffe://...

# Token Release
awrit token release --token <jwt>

# Audit
awrit audit events
awrit audit events --event-type token_revoked
awrit audit events --agent-id spiffe://...
awrit audit events --outcome success
awrit audit events --since 2026-03-29T00:00:00Z --limit 50
awrit audit events --json
```

---

## Broker Configuration

All broker configuration is via environment variables prefixed `AA_`. Configuration can also be loaded from a config file generated by `awrit init` (see [Operator CLI](#operator-cli-awrit) above). Environment variables always override config file values.

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `AA_ADMIN_SECRET` | string | *(none)* | **Yes** | Shared secret for admin authentication. The broker exits immediately on startup if this is unset or empty. Use a strong random value (e.g., `openssl rand -hex 32`). |
| `AA_PORT` | string | `"8080"` | No | HTTP listen port. |
| `AA_LOG_LEVEL` | string | `"verbose"` | No | Log verbosity: `quiet`, `standard`, `verbose`, `trace`. Note: `verbose` currently emits the same output as `standard`. |
| `AA_TRUST_DOMAIN` | string | `"agentwrit.local"` | No | SPIFFE trust domain used in agent identity URIs (e.g., `spiffe://agentwrit.local/agent/...`). |
| `AA_DEFAULT_TTL` | int | `300` | No | Default token TTL in seconds (5 minutes). |
| `AA_MAX_TTL` | int | `86400` | No | Maximum token TTL ceiling in seconds (24 hours). Tokens requesting longer TTL are clamped to this value. Set to `0` to disable the ceiling entirely. |
| `AA_BIND_ADDRESS` | string | `"127.0.0.1"` | No | Bind address for the HTTP listener. Use `"0.0.0.0"` for Docker or to accept external connections. |
| `AA_SIGNING_KEY_PATH` | string | `"./signing.key"` | No | Path to Ed25519 private key for token signing. If the file does not exist, a fresh key is generated and saved to this path on startup. |
| `AA_AUDIENCE` | string | *(empty)* | No | Expected `aud` claim in JWTs. Unset or empty skips audience validation. Set to a non-empty value to enforce it. |
| `AA_APP_TOKEN_TTL` | int | `1800` | No | TTL for app JWTs in seconds (30 minutes). Controls how long app-authenticated tokens last. |
| `AA_SEED_TOKENS` | bool | `false` | No | Print seed launch and admin tokens to stdout on startup. **Development only** -- never enable in production. |
| `AA_DB_PATH` | string | `"./data.db"` | No | Path to the SQLite database file for audit event persistence. The broker creates the file and table on first startup. Leaving this unset uses the default `./data.db`. See [Audit Persistence](#audit-persistence-aa_db_path) below. |
| `AA_TLS_MODE` | string | `"none"` | No | Transport security mode: `none` (plain HTTP), `tls` (one-way TLS), `mtls` (mutual TLS). See [TLS/mTLS Configuration](#tlsmtls-configuration) below. |
| `AA_TLS_CERT` | string | *(none)* | If TLS | Path to the broker's TLS certificate PEM file. Required when `AA_TLS_MODE` is `tls` or `mtls`. |
| `AA_TLS_KEY` | string | *(none)* | If TLS | Path to the broker's TLS private key PEM file. Required when `AA_TLS_MODE` is `tls` or `mtls`. |
| `AA_TLS_CLIENT_CA` | string | *(none)* | If mTLS | Path to the client CA certificate PEM file used to verify client certificates. Required when `AA_TLS_MODE` is `mtls`. |

### Security notes

- **`AA_ADMIN_SECRET`** is the root of trust for the entire system. Anyone who knows this value can create launch tokens, revoke credentials, and read the audit trail. Treat it like a root password.
- **`AA_SEED_TOKENS`** bypasses the normal bootstrap flow by printing tokens to stdout. This is for local development and testing only.
- The broker **persists its Ed25519 signing key** to disk at `AA_SIGNING_KEY_PATH` (default `./signing.key`). A new key is generated only on first startup. Tokens remain valid across restarts. To rotate the key, delete the file and restart — all previously issued tokens become invalid. Protect the key file as you would any private key.
- **Security headers** are set on ALL responses automatically: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, `X-Frame-Options: DENY`. When `AA_TLS_MODE` is `tls` or `mtls`, `Strict-Transport-Security` (HSTS) is also included. No configuration is needed -- these are always active.
- **Request body limit** of 1 MB is enforced globally on all endpoints (not just POST). Requests exceeding this limit are rejected before reaching any handler.
- **Error messages are sanitized** for security-sensitive endpoints. Token validation, renewal, and auth middleware return generic messages (e.g., `"token is invalid or expired"`) to prevent leaking internal state to clients.

---

## TLS/mTLS Configuration

By default the broker listens on plain HTTP (`AA_TLS_MODE=none`). For production deployments, enable TLS or mutual TLS to encrypt traffic and optionally require client certificates.

### Mode: tls (one-way TLS)

The broker presents a certificate. Clients verify the broker's identity but do not present their own certificate.

```bash
export AA_TLS_MODE=tls
export AA_TLS_CERT=/etc/broker/certs/broker.crt
export AA_TLS_KEY=/etc/broker/certs/broker.key
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

go run ./cmd/broker
# AgentWrit broker v2.0.0 listening on :8080 (TLS)
```

Clients connect with HTTPS:

```bash
curl --cacert /etc/broker/certs/ca.crt https://localhost:8080/v1/health
```

### Mode: mtls (mutual TLS)

Both broker and client present certificates. The broker verifies client certificates against the configured CA. This is the recommended mode for production — only authorized clients with valid certificates can connect.

```bash
export AA_TLS_MODE=mtls
export AA_TLS_CERT=/etc/broker/certs/broker.crt
export AA_TLS_KEY=/etc/broker/certs/broker.key
export AA_TLS_CLIENT_CA=/etc/broker/certs/client-ca.crt
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

go run ./cmd/broker
```

Clients must present a certificate signed by the configured client CA:

```bash
curl \
  --cacert /etc/broker/certs/ca.crt \
  --cert /etc/broker/certs/client.crt \
  --key /etc/broker/certs/client.key \
  https://localhost:8080/v1/health
```

Clients without a valid certificate are rejected at the TLS handshake — they never reach the HTTP layer.

### Docker Compose with TLS

Three compose files ship with the repo — a base plus two overlays:

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Base — broker on HTTP (no TLS). Used by `scripts/stack_up.sh`. |
| `docker-compose.tls.yml` | Overlay — adds one-way TLS (broker serves HTTPS) |
| `docker-compose.mtls.yml` | Overlay — adds mutual TLS (broker requires client cert) |

The overlays expect certs in `/tmp/agentwrit-certs/`. Generate them first:

```bash
./scripts/gen_test_certs.sh
```

This creates `broker.pem`, `broker-key.pem`, `ca.pem`, `client.pem`, `client-key.pem` using EC P-256 keys. **Test certs only** — use a proper CA in production.

**One-way TLS (broker serves HTTPS):**

```bash
docker compose -f docker-compose.yml -f docker-compose.tls.yml up -d
```

**Mutual TLS (broker requires client cert):**

```bash
docker compose -f docker-compose.yml -f docker-compose.mtls.yml up -d
```

Stop with the matching overlays:

```bash
docker compose -f docker-compose.yml -f docker-compose.tls.yml down -v
```

### Rolling your own TLS compose configuration

If you need a custom cert layout (different paths, production CA, Kubernetes secret mounts), extend `docker-compose.yml` directly:

```yaml
broker:
  environment:
    - AA_TLS_MODE=${AA_TLS_MODE:-none}
    - AA_TLS_CERT=${AA_TLS_CERT:-}
    - AA_TLS_KEY=${AA_TLS_KEY:-}
    - AA_TLS_CLIENT_CA=${AA_TLS_CLIENT_CA:-}
  volumes:
    - /etc/broker/certs:/certs:ro
```

Then export the env vars before bringing up the stack:

```bash
export AA_TLS_MODE=tls
export AA_TLS_CERT=/certs/broker.crt
export AA_TLS_KEY=/certs/broker.key
./scripts/stack_up.sh
```

> **Warning:** `scripts/gen_test_certs.sh` generates self-signed certs for development and testing only. Use a proper CA in production.

---

## Audit Persistence (AA_DB_PATH)

The broker persists audit events and revocations to SQLite. To configure the database path:

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `AA_DB_PATH` | string | `"./data.db"` | No | Path to the SQLite database file. The broker creates the file if it does not exist. The directory must be writable by the broker process. |

Set `AA_DB_PATH` to a stable location on the host:

```bash
export AA_DB_PATH="/var/lib/broker/data.db"
AA_ADMIN_SECRET="..." go run ./cmd/broker
```

In Docker Compose, mount a volume so the database survives container replacement:

```yaml
broker:
  environment:
    - AA_DB_PATH=/data/data.db
  volumes:
    - agentwrit-data:/data

volumes:
  agentwrit-data:
```

On startup, the broker loads all existing audit events from SQLite to rebuild the hash chain in memory. The number of events loaded is logged and exposed as the `agentwrit_audit_events_loaded` Prometheus gauge.

**Note:** The broker persists its Ed25519 signing key to `AA_SIGNING_KEY_PATH`. Both audit events and signing keys survive restarts, so previously issued tokens remain valid. To force key rotation, delete the signing key file and restart.

---

## The Bootstrap Flow

Before any agent can get a token, the operator must complete the bootstrap chain: authenticate as admin, create a launch token, and hand it to the agent.

```mermaid
sequenceDiagram
    participant Op as Operator
    participant B as Broker
    participant A as Agent

    Note over Op,B: Step 1: Admin Authentication
    Op->>B: POST /v1/admin/auth<br/>{secret}
    B-->>Op: {access_token} (TTL 300s)

    Note over Op,B: Step 2: Create Launch Token
    Op->>B: POST /v1/admin/launch-tokens<br/>Bearer: admin_token
    B-->>Op: {launch_token, policy}

    Note over Op,A: Step 3: Distribute
    Op->>A: Hand launch_token to agent

    Note over A,B: Step 4: Agent Registers
    A->>B: GET /v1/challenge
    B-->>A: {nonce}
    A->>B: POST /v1/register<br/>{launch_token, nonce, public_key,<br/>signature, orch_id, task_id, requested_scope}
    B-->>A: {agent_id, access_token, expires_in}
```

### Step 1: Authenticate as Admin

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
```

The admin token has a 300-second TTL and includes three scopes: `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*`. Cache and reuse it within its TTL rather than re-authenticating for every operation.

The admin auth endpoint is rate-limited to 5 requests/second with a burst of 10 per IP address.

### Step 2: Create a Launch Token

```bash
curl -s -X POST http://localhost:8080/v1/admin/launch-tokens \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "data-processor",
    "allowed_scope": ["read:data:*", "write:data:*"],
    "max_ttl": 300,
    "single_use": true,
    "ttl": 30
  }'
```

Response (201 Created):

```json
{
  "launch_token": "a1b2c3d4...64-hex-characters",
  "expires_at": "2026-02-15T12:00:30Z",
  "policy": {
    "allowed_scope": ["read:data:*", "write:data:*"],
    "max_ttl": 300
  }
}
```

Launch token fields:

| Field | Description |
|-------|-------------|
| `agent_name` | Descriptive label stored with the launch token for operator context and auditability. |
| `allowed_scope` | Maximum scopes the agent can request during registration. |
| `max_ttl` | Maximum token TTL (seconds) the agent can request. Default: 300. |
| `single_use` | If `true`, the launch token is consumed after one successful registration. If `false`, it can be reused until it expires. |
| `ttl` | Lifetime of the launch token itself (seconds). Default: 30. |

## Token Release (Task Completion Signal)

When an agent completes its task, it can optionally signal task completion by releasing its token. This triggers audit logging and allows the broker to perform cleanup operations.

```bash
# Agent releases its token when task is complete
curl -s -X POST "http://localhost:8080/v1/token/release" \
  -H "Authorization: Bearer <agent-token>"
```

Response (204 No Content):

```
HTTP/1.1 204 No Content
```

This endpoint records the token release in the audit trail and marks the token as explicitly completed. Token release is optional -- tokens are also automatically recorded as expired when their TTL elapses. Token release can be used for:

- **Task completion audit trails** -- mark exactly when a task finished
- **Cleanup operations** -- trigger resource deallocation
- **Billing/metering** -- record precise duration of task execution
- **Compliance** -- demonstrate explicit credential deactivation

---

---

## Monitoring

### Health Endpoints

| Endpoint | Port | Description |
|----------|------|-------------|
| `GET /v1/health` (broker) | 8080 | Returns `{"status":"ok","version":"2.0.0","uptime":N,"db_connected":true,"audit_events_count":N}`. Used by Docker health checks and load balancers. |
| `GET /v1/metrics` (broker) | 8080 | Prometheus metrics exposition endpoint. |

### Broker Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agentwrit_tokens_issued_total` | counter | `scope` | Tokens issued, labeled by primary scope. |
| `agentwrit_tokens_revoked_total` | counter | `level` | Revocation operations by level (`token`, `agent`, `task`, `chain`). |
| `agentwrit_registrations_total` | counter | `status` | Agent registration attempts (`success`, `failure`). |
| `agentwrit_admin_auth_total` | counter | `status` | Admin authentication attempts (`success`, `failure`). |
| `agentwrit_launch_tokens_created_total` | counter | -- | Total launch tokens created. |
| `agentwrit_active_agents` | gauge | -- | Currently registered agents. |
| `agentwrit_request_duration_seconds` | histogram | `endpoint` | HTTP request latency by endpoint. |
| `agentwrit_clock_skew_total` | counter | -- | Clock skew events detected during token validation. |


### Key metrics to alert on

- `agentwrit_admin_auth_total{status="failure"}` -- repeated failures may indicate a brute-force attempt or misconfigured secret.
- `agentwrit_tokens_revoked_total` spike -- potential security incident in progress.

### Log Format

All logs follow this format:

```
[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | context1, context2
```

Example:

```
[AA:BROKER:OK] 2026-02-15T12:00:00Z | main | starting broker | addr=:8080, version=2.0.0
[AA:BROKER:WARN] 2026-02-15T12:00:05Z | CFG | AA_DEFAULT_TTL exceeds AA_MAX_TTL | default_ttl=600 max_ttl=300
```

- `FAIL` level logs go to stderr; all others go to stdout.
- Log levels control verbosity: `quiet` (errors only), `standard` (normal operations), `verbose` (same as standard currently), `trace` (detailed debugging).

### Background Maintenance

The broker automatically runs two cleanup tasks every 60 seconds:

- **JTI pruning** — removes expired JWT ID entries from the database to prevent unbounded growth
- **Agent expiration** — marks inactive agents as expired when they exceed inactivity thresholds

These run silently in the background. You'll see log entries like `pruned expired JTIs | count=42` when records are cleaned up. No operator action is required — this is fully automatic.

---

---

## What's Next?

The broker is running in production. Now learn the day-to-day operations:

**[Common Tasks →](common-tasks.md)**
App management, token revocation, audit queries, and other everyday operations.

Or explore related topics:

| If you want to... | Read this |
|-------------------|-----------|
| Understand the internal architecture | [Architecture](architecture.md) |
| Use the CLI for all operator tasks | [CLI Reference (awrit)](awrit-reference.md) |
| Debug issues | [Troubleshooting](troubleshooting.md) |
| See all configuration options | [API Reference](api.md) |

---

*Previous: [Getting Started: Developer](getting-started-developer.md) · Next: [Common Tasks](common-tasks.md)*
