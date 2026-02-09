# AgentAuth User Guide

**Version:** 2.0.0
**Last Updated:** February 2026

---

## Table of Contents

1. [What Is AgentAuth](#what-is-agentauth)
2. [Quick Start](#quick-start)
3. [Installation](#installation)
4. [Configuration](#configuration)
5. [Bootstrap Walkthrough](#bootstrap-walkthrough)
6. [Monitoring](#monitoring)
7. [Security Hardening](#security-hardening)
8. [Troubleshooting](#troubleshooting)
9. [Operational Runbook](#operational-runbook)
10. [Log Format](#log-format)

---

## What Is AgentAuth

AgentAuth is an ephemeral agent credentialing system that issues short-lived, scope-restricted tokens to AI agents through a cryptographic challenge-response identity flow. Traditional IAM systems (OAuth, AWS IAM, service accounts) were designed for long-lived services with persistent identities. AI agents break those assumptions: they are ephemeral (lifetimes measured in minutes), non-deterministic (LLM-driven decisions), and require task-specific permissions at runtime. AgentAuth solves this by giving each agent instance a unique SPIFFE-format identity, scoped credentials that expire with the task, and a tamper-evident audit trail that logs every credential operation. Agents prove their identity with Ed25519 signatures and receive EdDSA-signed JWT tokens that can only narrow in scope through delegation, never expand.

If you are building a new Python or TypeScript agent integration, use the dedicated hands-on guide: [Agent Integration Guide](AGENT_INTEGRATION_GUIDE.md).

---

## Quick Start

Get the broker running and verify it works in under two minutes.

### 1. Build

```bash
cd /path/to/agentauth
go build ./...
```

### 2. Start the broker

```bash
AA_ADMIN_SECRET=my-secret-change-me AA_LOG_LEVEL=standard go run ./cmd/broker
```

You should see:

```
AgentAuth broker v2.0.0 listening on :8080
```

### 3. Verify health

```bash
curl -s http://localhost:8080/v1/health | jq .
```

Expected response:

```json
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 3
}
```

### 4. Authenticate as admin

```bash
curl -s -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"my-secret-change-me"}' | jq .
```

Expected response:

```json
{
  "access_token": "<admin-jwt>",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

You now have a working broker. Continue to the [Bootstrap Walkthrough](#bootstrap-walkthrough) for the full agent registration flow.

---

## Installation

### From Source (recommended)

Requires Go 1.24 or later.

```bash
git clone https://github.com/divineartis/agentauth.git
cd agentauth
go build -o agentauth-broker ./cmd/broker
```

The binary is statically linked (CGO_ENABLED=0 by default on Go 1.24+) and has no runtime dependencies.

### Run Directly

If you prefer not to build a binary:

```bash
go run ./cmd/broker
```

### Verify the Build

```bash
./agentauth-broker &
curl -s http://localhost:8080/v1/health | jq .status
# Expected: "ok"
kill %1
```

---

## Configuration

All configuration is via environment variables prefixed with `AA_`. The broker reads them at startup through `cfg.Load()`.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `AA_PORT` | string | `8080` | HTTP listen port |
| `AA_LOG_LEVEL` | string | `verbose` | Logging verbosity: `quiet`, `standard`, `verbose`, `trace` |
| `AA_TRUST_DOMAIN` | string | `agentauth.local` | SPIFFE trust domain for agent identity URIs |
| `AA_DEFAULT_TTL` | int | `300` | Default token time-to-live in seconds |
| `AA_ADMIN_SECRET` | string | **(required)** | Pre-shared secret for admin authentication. The broker exits on startup if this is not set. |
| `AA_SEED_TOKENS` | bool | `false` | Dev-only: print seed launch and admin tokens to stdout on startup. Not for production use. |

### Configuration Details

**AA_PORT** -- The broker binds to `0.0.0.0:<port>`. Use a reverse proxy (nginx, Caddy) for TLS termination in production.

**AA_LOG_LEVEL** -- Controls which log messages are emitted:

| Level | What It Shows |
|-------|---------------|
| `quiet` | Only FAIL messages (stderr) |
| `standard` | OK + WARN + FAIL |
| `verbose` | Everything except TRACE |
| `trace` | All messages including debug traces |

**AA_TRUST_DOMAIN** -- Appears in all SPIFFE IDs: `spiffe://<trust_domain>/agent/<orch_id>/<task_id>/<instance_id>`. Set this to your organization's domain in production (e.g., `agentauth.example.com`).

**AA_DEFAULT_TTL** -- Tokens issued without an explicit TTL use this value. Applies to agent tokens issued at registration. Admin tokens always use a fixed 300-second TTL.

**AA_ADMIN_SECRET** -- **Required.** The broker exits on startup if this variable is not set (`FATAL: AA_ADMIN_SECRET must be set (non-empty)`). It is compared against the `client_secret` field in `POST /v1/admin/auth` using constant-time comparison (`crypto/subtle.ConstantTimeCompare`) to prevent timing attacks.

### Example: Production Configuration

```bash
export AA_PORT=8080
export AA_LOG_LEVEL=standard
export AA_TRUST_DOMAIN=agentauth.example.com
export AA_DEFAULT_TTL=300
export AA_ADMIN_SECRET=$(openssl rand -hex 32)

./agentauth-broker
```

---

## Bootstrap Walkthrough

This section walks through the complete agent credential lifecycle using curl commands against a running broker. Every command here works against a live instance.

### Prerequisites

Start the broker with an admin secret:

```bash
AA_ADMIN_SECRET=demo-secret-do-not-use-in-prod go run ./cmd/broker &
BROKER=http://localhost:8080
```

### Step 1: Admin Authentication

Exchange the pre-shared secret for a short-lived admin JWT (300 seconds).

```bash
ADMIN_RESP=$(curl -s -X POST $BROKER/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"client_id":"admin","client_secret":"demo-secret-do-not-use-in-prod"}')

echo "$ADMIN_RESP" | jq .

ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.access_token')
```

Response:

```json
{
  "access_token": "eyJhbG...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

The admin JWT contains scopes: `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*`. It cannot issue agent tokens directly or access protected resources.

### Step 2: Create a Launch Token

Create a policy-bound, single-use launch token for an agent. The `allowed_scope` sets the ceiling -- the agent cannot request more than this.

```bash
LT_RESP=$(curl -s -X POST $BROKER/v1/admin/launch-tokens \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "data-reader",
    "allowed_scope": ["read:Customers:*"],
    "max_ttl": 300,
    "single_use": true,
    "ttl": 30
  }')

echo "$LT_RESP" | jq .

LAUNCH_TOKEN=$(echo "$LT_RESP" | jq -r '.launch_token')
```

Response:

```json
{
  "launch_token": "a1b2c3d4...",
  "expires_at": "2026-02-09T12:00:30Z",
  "policy": {
    "allowed_scope": ["read:Customers:*"],
    "max_ttl": 300
  }
}
```

The launch token expires in 30 seconds (`ttl`) and can only be used once (`single_use`).

### Step 3: Get a Challenge Nonce

The agent requests a random nonce for the challenge-response identity flow.

```bash
CHALLENGE_RESP=$(curl -s $BROKER/v1/challenge)

echo "$CHALLENGE_RESP" | jq .

NONCE=$(echo "$CHALLENGE_RESP" | jq -r '.nonce')
```

Response:

```json
{
  "nonce": "a3f2b1c4e5d6...",
  "expires_in": 30
}
```

The nonce is single-use and expires in 30 seconds.

### Step 4: Sign the Nonce with Ed25519

The agent proves identity by signing the nonce with its Ed25519 private key. This step would normally happen inside the agent's code.

Using Python to demonstrate (since Ed25519 signing is not trivial in bash):

```python
import base64, binascii
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

# Generate a key pair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

# Export public key (raw 32 bytes, base64-encoded)
pub_bytes = public_key.public_bytes_raw()
pub_b64 = base64.b64encode(pub_bytes).decode()

# Sign the nonce (hex-decoded to bytes)
nonce_hex = "<NONCE_FROM_STEP_3>"
nonce_bytes = binascii.unhexlify(nonce_hex)
signature = private_key.sign(nonce_bytes)
sig_b64 = base64.b64encode(signature).decode()

print(f"PUBLIC_KEY={pub_b64}")
print(f"SIGNATURE={sig_b64}")
```

Set the values for the next step:

```bash
PUBLIC_KEY="<base64-encoded-ed25519-public-key>"
SIGNATURE="<base64-encoded-signature>"
```

### Step 5: Register the Agent

Register with the launch token, signed nonce, and requested scope. The `requested_scope` must be a subset of the launch token's `allowed_scope`.

```bash
REG_RESP=$(curl -s -X POST $BROKER/v1/register \
  -H "Content-Type: application/json" \
  -d "{
    \"launch_token\": \"$LAUNCH_TOKEN\",
    \"nonce\": \"$NONCE\",
    \"public_key\": \"$PUBLIC_KEY\",
    \"signature\": \"$SIGNATURE\",
    \"orch_id\": \"orch-001\",
    \"task_id\": \"task-read-customers\",
    \"requested_scope\": [\"read:Customers:12345\"]
  }")

echo "$REG_RESP" | jq .

AGENT_ID=$(echo "$REG_RESP" | jq -r '.agent_id')
AGENT_TOKEN=$(echo "$REG_RESP" | jq -r '.access_token')
```

Response:

```json
{
  "agent_id": "spiffe://agentauth.local/agent/orch-001/task-read-customers/a1b2c3d4e5f6",
  "access_token": "eyJhbG...",
  "expires_in": 300
}
```

The agent now has:
- A unique SPIFFE identity
- A scoped JWT token (`read:Customers:12345` -- narrower than the ceiling `read:Customers:*`)
- A 300-second expiry

The launch token is now consumed and cannot be reused.

### Step 6: Validate a Token

Any party can validate a token and inspect its claims without authentication.

```bash
curl -s -X POST $BROKER/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$AGENT_TOKEN\"}" | jq .
```

Response:

```json
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/orch-001/task-read-customers/a1b2c3d4e5f6",
    "exp": 1739100300,
    "nbf": 1739100000,
    "iat": 1739100000,
    "jti": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
    "scope": ["read:Customers:12345"],
    "task_id": "task-read-customers",
    "orch_id": "orch-001"
  }
}
```

### Step 7: Renew a Token

An agent can renew its token before it expires. The new token preserves the same subject, scope, and metadata but gets fresh timestamps and a new JTI.

```bash
RENEW_RESP=$(curl -s -X POST $BROKER/v1/token/renew \
  -H "Authorization: Bearer $AGENT_TOKEN")

echo "$RENEW_RESP" | jq .

AGENT_TOKEN=$(echo "$RENEW_RESP" | jq -r '.access_token')
```

Response:

```json
{
  "access_token": "eyJhbG...",
  "expires_in": 300
}
```

### Step 8: Delegate to Another Agent

An agent can delegate a subset of its scope to another registered agent. Scope can only narrow, never expand. Maximum delegation depth is 5 hops.

First, register a second agent (repeat Steps 3-5 with a new key pair and the same or a new launch token), then:

```bash
curl -s -X POST $BROKER/v1/delegate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  -d "{
    \"delegate_to\": \"$SECOND_AGENT_ID\",
    \"scope\": [\"read:Customers:12345\"],
    \"ttl\": 60
  }" | jq .
```

Response:

```json
{
  "access_token": "eyJhbG...",
  "expires_in": 60,
  "delegation_chain": [
    {
      "agent": "spiffe://agentauth.local/agent/orch-001/task-read-customers/a1b2c3d4e5f6",
      "scope": ["read:Customers:12345"],
      "delegated_at": "2026-02-09T12:01:00Z"
    }
  ]
}
```

### Step 9: Revoke a Credential

Admins can revoke credentials at four levels: `token` (by JTI), `agent` (by SPIFFE ID), `task` (by task ID), or `chain` (by root delegator's agent ID).

Revoke a single token by JTI:

```bash
# First, extract the JTI from the token claims
JTI=$(curl -s -X POST $BROKER/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"$AGENT_TOKEN\"}" | jq -r '.claims.jti')

# Revoke it
curl -s -X POST $BROKER/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"token\", \"target\": \"$JTI\"}" | jq .
```

Response:

```json
{
  "revoked": true,
  "level": "token",
  "target": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4",
  "count": 1
}
```

Revoke all tokens for a task:

```bash
curl -s -X POST $BROKER/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"level": "task", "target": "task-read-customers"}' | jq .
```

Revoke all tokens for an agent:

```bash
curl -s -X POST $BROKER/v1/revoke \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"agent\", \"target\": \"$AGENT_ID\"}" | jq .
```

### Step 10: Query the Audit Trail

All credential operations are recorded in a hash-chain audit log. Query it with admin credentials.

Get all events:

```bash
curl -s "$BROKER/v1/audit/events" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Response:

```json
{
  "events": [
    {
      "id": "evt-000001",
      "timestamp": "2026-02-09T12:00:00Z",
      "event_type": "admin_auth",
      "agent_id": "",
      "task_id": "",
      "orch_id": "",
      "detail": "admin authenticated as admin",
      "hash": "a1b2c3...",
      "prev_hash": "000000..."
    },
    {
      "id": "evt-000002",
      "timestamp": "2026-02-09T12:00:01Z",
      "event_type": "launch_token_issued",
      "detail": "launch token issued for agent=data-reader scope=[read:Customers:*] ...",
      "hash": "d4e5f6...",
      "prev_hash": "a1b2c3..."
    }
  ],
  "total": 5,
  "offset": 0,
  "limit": 100
}
```

Filter by event type:

```bash
curl -s "$BROKER/v1/audit/events?event_type=agent_registered" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Filter by task:

```bash
curl -s "$BROKER/v1/audit/events?task_id=task-read-customers" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

Filter by time range with pagination:

```bash
curl -s "$BROKER/v1/audit/events?since=2026-02-09T00:00:00Z&until=2026-02-09T23:59:59Z&limit=10&offset=0" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq .
```

### Event Types

| Event Type | When It Fires |
|------------|---------------|
| `admin_auth` | Admin authenticates successfully |
| `admin_auth_failed` | Failed admin authentication attempt |
| `launch_token_issued` | Launch token created |
| `launch_token_denied` | Launch token creation rejected |
| `agent_registered` | Agent registered successfully |
| `registration_policy_violation` | Agent registration denied (scope violation) |
| `token_issued` | Token issued (at registration) |
| `token_renewed` | Token renewed successfully via `POST /v1/token/renew` |
| `token_renewal_failed` | Token renewal attempt failed |
| `token_revoked` | Credential revoked |
| `delegation_created` | Delegation token created |
| `resource_accessed` | Resource access logged (emitted by resource server) |

---

## Monitoring

### Health Endpoint

```bash
curl -s http://localhost:8080/v1/health | jq .
```

Returns `{"status":"ok","version":"2.0.0","uptime":<seconds>}`. Use this for load balancer health checks and uptime monitoring.

### Prometheus Metrics

The broker exposes Prometheus-format metrics at `GET /v1/metrics` (no authentication required).

```bash
curl -s http://localhost:8080/v1/metrics
```

### Key Metrics

| Metric | Type | Labels | What to Monitor |
|--------|------|--------|-----------------|
| `agentauth_tokens_issued_total` | counter | `scope` | Token issuance rate. Sudden spikes may indicate automation runaway. |
| `agentauth_tokens_revoked_total` | counter | `level` | Revocation rate by level. High `agent` or `task` revocations may indicate compromise. |
| `agentauth_registrations_total` | counter | `status` | Registration success/failure rate. High failure rate may indicate misconfigured agents or attacks. |
| `agentauth_admin_auth_total` | counter | `status` | Admin authentication attempts. Watch for failed attempts (brute-force). |
| `agentauth_launch_tokens_created_total` | counter | -- | Launch token creation rate. Should correlate with agent spawning. |
| `agentauth_active_agents` | gauge | -- | Current number of registered agents. Useful for capacity planning. |
| `agentauth_request_duration_seconds` | histogram | `endpoint` | Latency by endpoint. Target: <50ms for token validation. |
| `agentauth_clock_skew_total` | counter | -- | Clock skew events during token validation. Non-zero values indicate time sync issues. |

### Alerting Recommendations

| Alert | Condition | Severity |
|-------|-----------|----------|
| Broker down | `up == 0` or health check fails | Critical |
| High auth failures | `rate(agentauth_admin_auth_total{status="failure"}[5m]) > 5` | Warning |
| Registration failure spike | `rate(agentauth_registrations_total{status="failure"}[5m]) > 10` | Warning |
| Revocation spike | `rate(agentauth_tokens_revoked_total[5m]) > 20` | Warning |
| Clock skew | `rate(agentauth_clock_skew_total[5m]) > 0` | Warning |
| High latency | `histogram_quantile(0.99, agentauth_request_duration_seconds) > 0.1` | Warning |

### Prometheus Scrape Configuration

```yaml
scrape_configs:
  - job_name: 'agentauth'
    scrape_interval: 15s
    static_configs:
      - targets: ['broker-host:8080']
    metrics_path: '/v1/metrics'
```

---

## Security Hardening

### TLS Requirement

The broker listens on plain HTTP by default. Production deployments MUST use a TLS-terminating reverse proxy (e.g., nginx, envoy, Caddy) or configure a load balancer with TLS termination. Native TLS support (`AA_TLS_CERT`, `AA_TLS_KEY`) is planned for a future release.

Without TLS, all traffic -- including admin secrets, Bearer tokens, and Ed25519 signatures -- travels in cleartext. This is acceptable only for local development and testing.

### Production Checklist

1. **Use a strong admin secret.** Generate with `openssl rand -hex 32`. Never use default or weak values.

2. **Terminate TLS at a reverse proxy.** The broker serves plain HTTP. Place nginx, Caddy, or a cloud load balancer in front for HTTPS.

   ```nginx
   server {
       listen 443 ssl;
       server_name agentauth.example.com;

       ssl_certificate     /etc/ssl/certs/agentauth.pem;
       ssl_certificate_key /etc/ssl/private/agentauth-key.pem;

       location / {
           proxy_pass http://127.0.0.1:8080;
           proxy_set_header Host $host;
           proxy_set_header X-Real-IP $remote_addr;
       }
   }
   ```

3. **Restrict network access.** The broker should only be reachable by orchestrators, agents, and resource servers. Use firewall rules or network policies to block public access.

4. **Set AA_TRUST_DOMAIN to your real domain.** All SPIFFE IDs contain this value. Use your organization's domain (e.g., `agentauth.example.com`), not the default `agentauth.local`.

5. **Keep AA_DEFAULT_TTL short.** The default 300 seconds (5 minutes) is reasonable. For high-security environments, consider 60-120 seconds with agents using token renewal.

6. **Forward logs to a centralized system.** Broker logs go to stdout (OK, WARN, TRACE) and stderr (FAIL). Ship both to your log aggregation system (ELK, Loki, Splunk).

7. **Do not expose /v1/metrics publicly.** While the metrics endpoint has no authentication, it exposes operational data. Restrict access to your monitoring infrastructure.

8. **Protect the admin secret in transit.** Use environment variables or a secrets manager (Vault, AWS Secrets Manager, K8s Secrets) to inject `AA_ADMIN_SECRET`. Never pass it as a command-line argument (visible in process lists).

9. **Monitor audit events.** Regularly review `admin_auth_failed` and `registration_policy_violation` events for signs of attack.

10. **Rotate the admin secret periodically.** See the [Operational Runbook](#operational-runbook) for the procedure.

### Security Properties

The broker provides these security guarantees:

- **Rate limiting** on `POST /v1/admin/auth` (5 requests/second per IP, burst 10) mitigates brute-force attacks. Exceeding the limit returns `429 Too Many Requests`. Rate limiting uses `X-Forwarded-For` for client IP identification when the header is present. Deploy the broker behind a trusted reverse proxy that sets this header correctly. If the broker is exposed directly to the internet without a proxy, clients can spoof the header to bypass rate limits.
- **Constant-time secret comparison** prevents timing attacks on `POST /v1/admin/auth`.
- **Ed25519 signature verification** at registration prevents identity forgery.
- **Scope attenuation** ensures permissions can only narrow through delegation, never expand.
- **Hash-chain audit log** makes tampering with the event history detectable.
- **PII sanitization** in audit logs redacts secrets, passwords, and private keys from detail fields.
- **Single-use launch tokens** prevent replay attacks on agent registration.
- **Short-lived nonces** (30 seconds) limit the window for challenge-response attacks.

---

## Troubleshooting

### Common Errors

#### 401 Unauthorized on POST /v1/admin/auth

**Cause:** Wrong `client_secret` value, or `AA_ADMIN_SECRET` not set on the broker.

**Fix:** Verify that the secret matches exactly:

```bash
# Check the broker was started with the secret
echo $AA_ADMIN_SECRET
```

If `AA_ADMIN_SECRET` is empty, the broker will reject all admin auth attempts.

#### 401 Unauthorized on POST /v1/register

**Possible causes:**
- Launch token not found (typo or wrong value)
- Launch token expired (default TTL is 30 seconds)
- Launch token already consumed (single-use tokens can only be used once)
- Nonce expired (30-second TTL)
- Nonce already consumed
- Ed25519 signature verification failed

**Fix:** Check the error detail in the response body:

```bash
curl -s -X POST $BROKER/v1/register \
  -H "Content-Type: application/json" \
  -d '...' | jq '.detail'
```

If the launch token expired, create a new one. If the nonce expired, get a fresh challenge.

#### 403 Forbidden -- scope_violation on POST /v1/register

**Cause:** The agent's `requested_scope` is not a subset of the launch token's `allowed_scope`.

**Fix:** Ensure every requested scope entry is covered by the launch token:

| Requested | Allowed | Result |
|-----------|---------|--------|
| `read:Customers:12345` | `read:Customers:*` | OK (wildcard covers specific) |
| `read:Customers:*` | `read:Customers:12345` | DENIED (cannot widen) |
| `write:Customers:*` | `read:Customers:*` | DENIED (different action) |

The launch token is NOT consumed on a scope violation, so you can retry with a narrower scope.

#### 403 Forbidden -- insufficient_scope on admin endpoints

**Cause:** The Bearer token lacks the required admin scope. Agent tokens cannot access admin endpoints.

**Fix:** Use the admin JWT from `POST /v1/admin/auth`, not an agent token.

#### 403 Forbidden -- scope_violation on POST /v1/delegate

**Cause:** The delegated scope exceeds the delegator's current scope, or the delegation depth limit (5 hops) has been reached.

**Fix:** Ensure `scope` in the delegation request is a subset of the delegator's scope. Check the delegation chain depth in the token claims.

#### 404 Not Found -- delegate agent not found

**Cause:** The `delegate_to` SPIFFE ID does not match any registered agent.

**Fix:** The target agent must be registered before delegation. Verify the SPIFFE ID is correct.

#### Token Expired

**Cause:** The JWT's `exp` claim is in the past.

**Fix:** Renew the token before it expires using `POST /v1/token/renew`, or register again with a new launch token.

#### Clock Skew Issues

**Cause:** The broker uses `time.Now().Unix()` for token timestamps. If the broker's clock and the validator's clock are out of sync, tokens may appear expired or "not yet valid."

**Fix:** Ensure all systems use NTP. The broker records `agentauth_clock_skew_total` when it detects skew. Monitor this metric and investigate if non-zero.

---

## Operational Runbook

### Rotate the Admin Secret

The admin secret can be rotated with a brief restart:

1. Generate a new secret:

   ```bash
   NEW_SECRET=$(openssl rand -hex 32)
   ```

2. Update the secret in your secrets manager or environment configuration.

3. Restart the broker with the new secret:

   ```bash
   AA_ADMIN_SECRET=$NEW_SECRET ./agentauth-broker
   ```

4. Update all orchestrators to use the new secret.

5. Existing admin JWTs issued with the old secret will still validate until they expire (300 seconds max), because the JWT signature is verified against the broker's Ed25519 key, not the admin secret. However, no new admin JWTs can be obtained with the old secret after restart.

**Note:** Agent tokens are not affected by admin secret rotation. They are signed with the broker's Ed25519 key, which is generated at startup and stored in memory.

### Respond to Compromise

If you suspect credential compromise:

1. **Identify the scope.** Determine whether a single token, agent, task, or delegation chain is compromised.

2. **Revoke immediately.** Use the appropriate revocation level:

   ```bash
   # Single token
   curl -s -X POST $BROKER/v1/revoke \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -d '{"level":"token","target":"<jti>"}'

   # All tokens for an agent
   curl -s -X POST $BROKER/v1/revoke \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -d '{"level":"agent","target":"<spiffe-id>"}'

   # All tokens for a task
   curl -s -X POST $BROKER/v1/revoke \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -d '{"level":"task","target":"<task-id>"}'

   # Entire delegation chain (target = root delegator's agent SPIFFE ID)
   curl -s -X POST $BROKER/v1/revoke \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -d '{"level":"chain","target":"<root-delegator-spiffe-id>"}'
   ```

3. **Review the audit trail.** Check what the compromised credential accessed:

   ```bash
   curl -s "$BROKER/v1/audit/events?agent_id=<compromised-agent-id>" \
     -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.events[]'
   ```

4. **If the broker signing key is compromised** (worst case), restart the broker. The Ed25519 key pair is generated fresh at startup and stored only in memory. Restarting invalidates all existing tokens.

### Debug Authentication Failures

1. **Check broker logs.** Set `AA_LOG_LEVEL=trace` to see all authentication decisions:

   ```bash
   AA_LOG_LEVEL=trace AA_ADMIN_SECRET=... ./agentauth-broker
   ```

2. **Validate the token directly:**

   ```bash
   curl -s -X POST $BROKER/v1/token/validate \
     -H "Content-Type: application/json" \
     -d "{\"token\": \"$TOKEN\"}" | jq .
   ```

   If `valid` is false, the `error` field explains why (expired, invalid signature, etc.).

3. **Check the audit trail** for `admin_auth_failed` or `registration_policy_violation` events.

### Recover from Broker Restart

The broker stores all state in memory. A restart means:
- All agent registrations are lost
- All launch tokens are invalidated
- All revocations are cleared
- The audit log is reset
- A new Ed25519 signing key pair is generated, invalidating all existing tokens

**Recovery procedure:**
1. Restart the broker.
2. Orchestrators re-authenticate with `POST /v1/admin/auth`.
3. Orchestrators create new launch tokens for agents.
4. Agents re-register through the full challenge-response flow.

This is by design for the MVP. In-memory state means zero persistence surface for attackers. Production deployments requiring persistence across restarts should use external storage (planned for a future release).

### Capacity Planning

The broker is a single-process Go HTTP server with in-memory state. Key limits:
- Memory grows linearly with active agents, launch tokens, and audit events
- Each agent record is approximately 200 bytes
- Each audit event is approximately 300 bytes
- The broker can handle thousands of concurrent agents on modest hardware

For high-availability deployments, run multiple broker instances behind a load balancer. Since state is in-memory and per-instance, agents must connect to the same broker instance for their entire lifecycle (use sticky sessions or agent-to-instance affinity).

---

## Log Format

The broker uses structured logging via the `obs` package. All log lines follow this format:

```
[AA:<MODULE>:<LEVEL>] <TIMESTAMP> | <COMPONENT> | <MESSAGE> | <CONTEXT>
```

### Fields

| Field | Description | Example |
|-------|-------------|---------|
| `MODULE` | Subsystem that produced the log | `BROKER`, `IDENTITY`, `ADMIN`, `TOKEN`, `DELEG` |
| `LEVEL` | Severity: `OK`, `WARN`, `FAIL`, `TRACE` | `OK` |
| `TIMESTAMP` | UTC RFC3339 timestamp | `2026-02-09T12:00:00Z` |
| `COMPONENT` | Specific component within the module | `main`, `Register`, `AdminSvc`, `DelegSvc` |
| `MESSAGE` | Human-readable description | `agent registered` |
| `CONTEXT` | Key-value pairs with additional data | `agent_id=spiffe://..., scope=[read:Customers:*]` |

### Destinations

| Level | Output | When |
|-------|--------|------|
| `OK` | stdout | Standard level and above |
| `WARN` | stdout | Standard level and above |
| `FAIL` | **stderr** | All levels except quiet |
| `TRACE` | stdout | Trace level only |

### Examples

```
[AA:BROKER:OK] 2026-02-09T12:00:00Z | main | starting broker | addr=:8080, version=2.0.0
[AA:ADMIN:OK] 2026-02-09T12:00:01Z | AdminSvc | admin authenticated | client_id=admin
[AA:ADMIN:OK] 2026-02-09T12:00:02Z | AdminSvc | launch token created | agent_name=data-reader, scope=[read:Customers:*]
[AA:IDENTITY:OK] 2026-02-09T12:00:05Z | Register | agent registered | agent_id=spiffe://agentauth.local/agent/orch-001/task-789/a1b2c3d4, scope=[read:Customers:12345]
[AA:ADMIN:WARN] 2026-02-09T12:01:00Z | AdminSvc | authentication failed | client_id=attacker
[AA:IDENTITY:WARN] 2026-02-09T12:01:01Z | Register | scope violation | requested=[write:Customers:*], allowed=[read:Customers:*]
[AA:DELEG:OK] 2026-02-09T12:02:00Z | DelegSvc | delegation created | from=spiffe://..., to=spiffe://..., scope=[read:Customers:12345]
```

### Filtering Logs

Separate OK/WARN/TRACE (stdout) from FAIL (stderr):

```bash
# Only errors
./agentauth-broker 2>errors.log 1>/dev/null

# Only success/warnings
./agentauth-broker 1>access.log 2>/dev/null

# Both to separate files
./agentauth-broker 1>access.log 2>errors.log
```

Filter by module with grep:

```bash
# Only identity-related logs
./agentauth-broker 2>&1 | grep '\[AA:IDENTITY:'

# Only failures
./agentauth-broker 2>&1 | grep ':FAIL\]'

# Only admin operations
./agentauth-broker 2>&1 | grep '\[AA:ADMIN:'
```

---

## Further Reading

- **API Reference:** See `docs/API_REFERENCE.md` for complete endpoint documentation.
- **Developer Guide:** See `docs/DEVELOPER_GUIDE.md` for architecture details and contribution guidelines.
- **Security Pattern:** See `plans/Security-Pattern-That-Is-Why-We-Built-AgentAuth.md` for the security research behind AgentAuth.
- **Technical Specification:** See `plans/AgentAuth-Technical-Spec-v2.0.md` for the authoritative specification.
