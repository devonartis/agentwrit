# Getting Started: Operator

> **Target persona:** Platform Operator
>
> **Prerequisite:** Read [Concepts](concepts.md) to understand why AgentAuth exists and what the 7-component security pattern provides.
> If you are a developer integrating an agent, see [Getting Started: Developer](getting-started-developer.md).

This guide walks you through deploying the AgentAuth broker, configuring it, creating launch tokens for developers, deploying sidecars, and monitoring the system.

---

## Quick Start (Docker Compose)

Get a broker and sidecar running in four commands:

```bash
# 1. Set the admin secret (required -- broker exits without it)
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"

# 2. Build and start the stack
./scripts/stack_up.sh

# 3. Verify the broker is healthy
curl http://localhost:8080/v1/health
# {"status":"ok","version":"2.0.0","uptime":5}

# 4. Verify the sidecar is healthy
curl http://localhost:8081/v1/health
# {"status":"ok","broker_connected":true,"healthy":true,...}
```

To tear down the stack:

```bash
./scripts/stack_down.sh
```

### What Docker Compose Deploys

```mermaid
flowchart LR
    subgraph "Docker: agentauth-net (bridge)"
        B["Broker<br/>:8080"]
        S["Sidecar<br/>:8081"]
    end
    S -- "http://broker:8080" --> B
    Dev["Developer App"] -- "http://localhost:8081" --> S
    Op["Operator"] -- "http://localhost:8080" --> B
    Prom["Prometheus"] -- "/v1/metrics" --> B
    Prom -- "/v1/metrics" --> S
```

The `docker-compose.yml` defines two services on a shared bridge network (`agentauth-net`). The sidecar depends on the broker's health check passing before it starts. Both images use multi-stage Alpine builds (golang:1.24 builder, alpine:3.18 runtime).

---

## Broker Configuration

All broker configuration is via environment variables prefixed `AA_`. There are no CLI flags or config files.

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `AA_ADMIN_SECRET` | string | *(none)* | **Yes** | Shared secret for admin authentication. The broker exits immediately on startup if this is unset or empty. Use a strong random value (e.g., `openssl rand -hex 32`). |
| `AA_PORT` | string | `"8080"` | No | HTTP listen port. |
| `AA_LOG_LEVEL` | string | `"verbose"` | No | Log verbosity: `quiet`, `standard`, `verbose`, `trace`. Note: `verbose` currently emits the same output as `standard`. |
| `AA_TRUST_DOMAIN` | string | `"agentauth.local"` | No | SPIFFE trust domain used in agent identity URIs (e.g., `spiffe://agentauth.local/agent/...`). |
| `AA_DEFAULT_TTL` | int | `300` | No | Default token TTL in seconds (5 minutes). |
| `AA_SEED_TOKENS` | bool | `false` | No | Print seed launch and admin tokens to stdout on startup. **Development only** -- never enable in production. |

### Security notes

- **`AA_ADMIN_SECRET`** is the root of trust for the entire system. Anyone who knows this value can create launch tokens, revoke credentials, and read the audit trail. Treat it like a root password.
- **`AA_SEED_TOKENS`** bypasses the normal bootstrap flow by printing tokens to stdout. This is for local development and testing only.
- The broker generates a **fresh Ed25519 signing key pair on every startup**. This means all previously issued tokens become invalid after a broker restart. This is by design -- there is no persistent key material to protect.

---

## Sidecar Configuration

The sidecar is a developer-facing proxy that auto-bootstraps with the broker. It handles Ed25519 key generation, challenge-response registration, and token renewal transparently.

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `AA_ADMIN_SECRET` | string | *(none)* | **Yes** | Must match the broker's `AA_ADMIN_SECRET`. The sidecar exits on startup if unset. |
| `AA_SIDECAR_SCOPE_CEILING` | string | *(none)* | **Yes** | Comma-separated list of maximum scopes this sidecar can issue (e.g., `"read:data:*,write:data:*"`). The sidecar exits on startup if unset. |
| `AA_BROKER_URL` | string | `"http://localhost:8080"` | No | Broker base URL. In Docker Compose, set to `http://broker:8080` (the Docker service name). |
| `AA_SIDECAR_PORT` | string | `"8081"` | No | Sidecar HTTP listen port. |
| `AA_SIDECAR_LOG_LEVEL` | string | `"standard"` | No | Sidecar log level: `quiet`, `standard`, `verbose`, `trace`. |
| `AA_SIDECAR_RENEWAL_BUFFER` | float | `0.8` | No | Fraction of TTL at which the sidecar renews its own bearer token. Valid range: 0.5 to 0.95. At 0.8 with a 900s TTL, renewal happens at 720s. |
| `AA_SIDECAR_CB_WINDOW` | int | `30` | No | Circuit breaker sliding window duration in seconds. |
| `AA_SIDECAR_CB_THRESHOLD` | float | `0.5` | No | Failure rate (0.0--1.0) within the window that trips the circuit breaker. |
| `AA_SIDECAR_CB_PROBE_INTERVAL` | int | `5` | No | Seconds between health probes when the circuit is open. |
| `AA_SIDECAR_CB_MIN_REQUESTS` | int | `5` | No | Minimum requests in the sliding window before the circuit breaker can trip. Prevents tripping on low traffic. |

### Scope ceiling design

The scope ceiling is the most important sidecar configuration decision. It defines the maximum permissions any agent token issued through this sidecar can have. Developers can request scopes within this ceiling but never exceed it.

Design principles:

1. **One sidecar per trust boundary.** Deploy separate sidecars for teams or services that need different scope ceilings.
2. **Least privilege.** Set the ceiling to the narrowest set of scopes the developers behind this sidecar actually need.
3. **Use wildcards carefully.** `read:data:*` allows reading any data resource. `read:data:users` restricts to a specific resource.

Scope format: `action:resource:identifier` (e.g., `read:data:*`, `write:config:app1`).

---

## The Bootstrap Flow

Before any agent can get a token, the operator must complete the bootstrap chain: authenticate as admin, create a launch token (or activate a sidecar), and hand credentials to the agent or sidecar.

```mermaid
sequenceDiagram
    participant Op as Operator
    participant B as Broker
    participant S as Sidecar
    participant A as Agent

    Note over Op,B: Step 1: Admin Authentication
    Op->>B: POST /v1/admin/auth<br/>{client_id, client_secret}
    B-->>Op: {access_token} (TTL 300s)

    Note over Op,B: Step 2: Create Launch Token
    Op->>B: POST /v1/admin/launch-tokens<br/>Bearer: admin_token
    B-->>Op: {launch_token, policy}

    Note over Op,A: Step 3: Distribute
    Op->>A: Hand launch_token to agent<br/>(or configure sidecar)

    Note over A,B: Step 4: Agent Registers
    A->>B: GET /v1/challenge
    B-->>A: {nonce}
    A->>B: POST /v1/register<br/>{launch_token, signed_nonce, public_key}
    B-->>A: {agent_id, access_token}
```

### Step 1: Authenticate as Admin

```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"client_id\": \"admin\", \"client_secret\": \"$AA_ADMIN_SECRET\"}" \
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
| `agent_name` | Descriptive name for the agent (used in SPIFFE ID generation). |
| `allowed_scope` | Maximum scopes the agent can request during registration. |
| `max_ttl` | Maximum token TTL (seconds) the agent can request. Default: 300. |
| `single_use` | If `true`, the launch token is consumed after one successful registration. If `false`, it can be reused until it expires. |
| `ttl` | Lifetime of the launch token itself (seconds). Default: 30. |

### Step 3: Create a Sidecar Activation

If you are deploying a sidecar instead of handing launch tokens directly to developers:

```bash
curl -s -X POST http://localhost:8080/v1/admin/sidecar-activations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "allowed_scopes": ["read:data:*", "write:data:*"],
    "ttl": 600
  }'
```

Response (201 Created):

```json
{
  "activation_token": "eyJhbGciOiJFZERTQSIs...",
  "expires_at": "2026-02-15T12:10:00Z",
  "scope": "sidecar:activate:read:data:* sidecar:activate:write:data:*"
}
```

The sidecar uses this activation token in a single-use exchange at `POST /v1/sidecar/activate` to obtain its own bearer token.

---

## Deploying a Sidecar for Developers

In production, sidecars auto-bootstrap -- you configure them via environment variables, and they handle the rest. The sequence below shows what happens automatically on sidecar startup:

```mermaid
sequenceDiagram
    participant S as Sidecar
    participant B as Broker

    Note over S: Start HTTP server<br/>(health + metrics only)

    loop Bootstrap retry (1s → 60s backoff)
        S->>B: GET /v1/health
        B-->>S: {status: ok}
    end

    S->>B: POST /v1/admin/auth<br/>{client_id, client_secret}
    B-->>S: {access_token} (admin JWT)

    S->>B: POST /v1/admin/sidecar-activations<br/>Bearer: admin_token
    B-->>S: {activation_token}

    S->>B: POST /v1/sidecar/activate<br/>{sidecar_activation_token}
    B-->>S: {access_token, sidecar_id} (TTL 900s)

    Note over S: Register /v1/token, /v1/token/renew,<br/>/v1/challenge, /v1/register routes

    Note over S: Start renewal goroutine<br/>(renew at 80% TTL)

    Note over S: Start circuit breaker<br/>health probe goroutine
```

### Docker Compose configuration

The `docker-compose.yml` in the repository already configures a sidecar. To customize:

```yaml
sidecar:
  build:
    context: .
    target: sidecar
  ports:
    - "${AA_SIDECAR_HOST_PORT:-8081}:8081"
  environment:
    - AA_BROKER_URL=http://broker:8080
    - AA_ADMIN_SECRET=${AA_ADMIN_SECRET}
    - AA_SIDECAR_SCOPE_CEILING=read:data:*,write:data:*
    - AA_SIDECAR_PORT=8081
  depends_on:
    broker:
      condition: service_healthy
  networks:
    - agentauth-net
```

### Verify sidecar health

```bash
curl -s http://localhost:8081/v1/health | python3 -m json.tool
```

A healthy sidecar returns:

```json
{
  "status": "ok",
  "broker_connected": true,
  "healthy": true,
  "scope_ceiling": ["read:data:*", "write:data:*"],
  "agents_registered": 0,
  "last_renewal": "2026-02-15T12:01:00Z",
  "uptime_seconds": 120.5
}
```

Key fields to check:
- `broker_connected`: `true` means the sidecar has a valid bearer token for broker communication.
- `healthy`: `true` means the sidecar is fully operational.
- `status`: `"ok"` (healthy), `"degraded"` (broker connection lost), or `"bootstrapping"` (startup in progress).

---

## Monitoring

### Health Endpoints

| Endpoint | Port | Description |
|----------|------|-------------|
| `GET /v1/health` (broker) | 8080 | Returns `{"status":"ok","version":"2.0.0","uptime":N}`. Used by Docker health checks and load balancers. |
| `GET /v1/health` (sidecar) | 8081 | Returns status, broker connectivity, scope ceiling, agent count, last renewal time, and uptime. |
| `GET /v1/metrics` (broker) | 8080 | Prometheus metrics exposition endpoint. |
| `GET /v1/metrics` (sidecar) | 8081 | Prometheus metrics exposition endpoint. |

### Broker Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agentauth_tokens_issued_total` | counter | `scope` | Tokens issued, labeled by primary scope. |
| `agentauth_tokens_revoked_total` | counter | `level` | Revocation operations by level (`token`, `agent`, `task`, `chain`). |
| `agentauth_registrations_total` | counter | `status` | Agent registration attempts (`success`, `failure`). |
| `agentauth_admin_auth_total` | counter | `status` | Admin authentication attempts (`success`, `failure`). |
| `agentauth_launch_tokens_created_total` | counter | -- | Total launch tokens created. |
| `agentauth_active_agents` | gauge | -- | Currently registered agents. |
| `agentauth_request_duration_seconds` | histogram | `endpoint` | HTTP request latency by endpoint. |
| `agentauth_clock_skew_total` | counter | -- | Clock skew events detected during token validation. |

### Sidecar Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `agentauth_sidecar_bootstrap_total` | counter | `status` | Bootstrap attempts (`success`, `failure`). |
| `agentauth_sidecar_renewals_total` | counter | `status` | Sidecar bearer token renewal attempts. |
| `agentauth_sidecar_token_exchanges_total` | counter | `status` | Agent token exchanges via broker. |
| `agentauth_sidecar_scope_denials_total` | counter | -- | Requests denied by scope ceiling enforcement. |
| `agentauth_sidecar_agents_registered` | gauge | -- | Agents currently in sidecar memory. |
| `agentauth_sidecar_request_duration_seconds` | histogram | `endpoint` | Sidecar HTTP request latency. |
| `agentauth_sidecar_circuit_state` | gauge | -- | Circuit breaker state: 0 = closed, 1 = open, 2 = probing. |
| `agentauth_sidecar_circuit_trips_total` | counter | -- | Number of times the circuit breaker has tripped open. |
| `agentauth_sidecar_cached_tokens_served_total` | counter | -- | Tokens served from cache during open circuit. |

### Key metrics to alert on

- `agentauth_admin_auth_total{status="failure"}` -- repeated failures may indicate a brute-force attempt or misconfigured secret.
- `agentauth_sidecar_circuit_state > 0` -- sidecar has lost broker connectivity.
- `agentauth_sidecar_circuit_trips_total` increasing -- broker reliability issue.
- `agentauth_tokens_revoked_total` spike -- potential security incident in progress.
- `agentauth_sidecar_bootstrap_total{status="failure"}` increasing -- sidecar unable to bootstrap (check `AA_ADMIN_SECRET` and `AA_BROKER_URL`).

### Log Format

All logs follow this format:

```
[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | context1, context2
```

Example:

```
[AA:BROKER:OK] 2026-02-15T12:00:00Z | main | starting broker | addr=:8080, version=2.0.0
[AA:SIDECAR:WARN] 2026-02-15T12:00:05Z | BOOTSTRAP | failed, retrying | attempt=1, retry_in=1s
```

- `FAIL` level logs go to stderr; all others go to stdout.
- Log levels control verbosity: `quiet` (errors only), `standard` (normal operations), `verbose` (same as standard currently), `trace` (detailed debugging).

---

## Next Steps

- [Common Tasks: Operator](common-tasks.md#operator-tasks) -- revocation, audit queries, launch token management
- [Architecture](architecture.md) -- how the broker works internally
- [Troubleshooting](troubleshooting.md#operator-errors) -- operational issues and fixes
- [API Reference](api.md) -- complete endpoint documentation
