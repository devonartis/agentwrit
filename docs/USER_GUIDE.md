# AgentAuth User Guide

## Purpose and audience

This guide is for operators and evaluators running the AgentAuth broker locally.
It explains how to run the service, execute the identity/token flow, validate protected access behavior, and diagnose failures.

## Prerequisites

- Go 1.23+ (module pins secure toolchain `go1.25.7` for gate runs)
- curl
- jq (recommended for response inspection)

## Start the broker

```bash
go run ./cmd/broker
```

Optional runtime settings:

```bash
AA_PORT=8080 AA_TRUST_DOMAIN=agentauth.local AA_DEFAULT_TTL=300 AA_LOG_LEVEL=verbose go run ./cmd/broker
```

## Verify service readiness

```bash
curl -i http://127.0.0.1:8080/v1/health
```

Expected:
- HTTP status `200` when healthy
- HTTP status `503` when degraded or unhealthy
- JSON body with:
  - `status` (`healthy|degraded|unhealthy`)
  - `version`
  - `uptime_seconds`
  - `components` (`sqlite`, `redis`)

## Inspect Prometheus metrics

```bash
curl -sS http://127.0.0.1:8080/v1/metrics | head -40
```

Expected:
- HTTP status `200`
- text exposition containing `aa_` prefixed metrics (for example `aa_validation_decision_total`)

## End-to-end identity and token workflow

This sequence covers the M01-M04 identity, token, authorization, and revocation workflow.

### Step 1: Request challenge nonce

```bash
curl -sS http://127.0.0.1:8080/v1/challenge | jq .
```

Expected fields:
- `nonce` (64 hex chars)
- `expires_at` (RFC3339 timestamp)

### Step 2: Register agent and receive first token

`POST /v1/register` input fields:
- `launch_token`
- `nonce`
- `agent_public_key` (JWK with `kty=OKP`, `crv=Ed25519`)
- `signature` (signature over nonce)
- `orchestration_id`
- `task_id`
- `requested_scope`

Expected success output:
- `agent_instance_id`
- `access_token`
- `expires_in`
- `refresh_after`

Expected failure behavior:
- `400` malformed request
- `401` invalid/expired/reused launch token
- `403` nonce/signature proof failed

### Step 3: Validate token with required scope

```bash
curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"token":"<access_token>","required_scope":"read:Customers:12345"}' \
  http://127.0.0.1:8080/v1/token/validate | jq .
```

Expected:
- `200` with `valid=true` for matching scope
- `403` for scope mismatch
- `401` for invalid token

### Step 4: Renew token

```bash
curl -sS \
  -H 'Content-Type: application/json' \
  -d '{"token":"<access_token>"}' \
  http://127.0.0.1:8080/v1/token/renew | jq .
```

Expected:
- `200` with new `access_token`
- `401` for invalid/expired token

## Token revocation (M04)

Use `POST /v1/revoke` to revoke tokens at one of four levels:

| Level | Target | Effect |
|---|---|---|
| `token` | Single JWT by its `jti` claim | Revokes one specific token |
| `agent` | Agent by SPIFFE ID | Revokes all tokens for that agent |
| `task` | Task by `task_id` | Revokes all tokens issued for that task |
| `delegation_chain` | Chain by SHA-256 hash | Revokes all tokens sharing a delegation chain |

Authorization requirement:
- bearer token with `admin:Broker:*` scope
- for local runs, start broker with `AA_SEED_TOKENS=true` and use `SEED_ADMIN_TOKEN` from startup output

### Revoke a single token

```bash
curl -sS -X POST \
  -H "Authorization: Bearer <admin_token>" \
  -H 'Content-Type: application/json' \
  -d '{"level":"token","target_id":"<jti>","reason":"compromised"}' \
  http://127.0.0.1:8080/v1/revoke | jq .
```

Expected success (`200`):
```json
{
  "revoked": true,
  "level": "token",
  "target_id": "<jti>",
  "revoked_at": "2026-02-07T12:00:00Z"
}
```

### Revoke all tokens for an agent

```bash
curl -sS -X POST \
  -H "Authorization: Bearer <admin_token>" \
  -H 'Content-Type: application/json' \
  -d '{"level":"agent","target_id":"spiffe://agentauth.local/agent/orch-1/task-1/abc123","reason":"agent decommissioned"}' \
  http://127.0.0.1:8080/v1/revoke | jq .
```

### Verify revocation

After revoking, any request using the revoked token (or any token matching the revoked agent/task/chain) receives `401`:

```bash
curl -i -H "Authorization: Bearer <revoked_token>" \
  http://127.0.0.1:8080/v1/protected/customers/12345
# → 401 with {"type":"urn:agentauth:error:token-revoked","title":"token has been revoked","status":401}
```

### Error responses

- `400` — invalid `level` value or missing required fields
- `401` — missing/invalid admin bearer token
- `403` — bearer token lacks `admin:Broker:*` scope
- `405` — non-POST method

## Protected resource authorization workflow (M03)

Protected route:
- `GET /v1/protected/customers/12345`

Behavior:
- no bearer token -> `401`
- bearer token without `read:Customers:12345` -> `403`
- valid bearer token with matching scope -> `200`

Quick unauthorized check:

```bash
curl -i http://127.0.0.1:8080/v1/protected/customers/12345
```

## Delegation workflow (M07)

Use `POST /v1/delegate` to create a narrowed token for another agent.

```bash
curl -sS -X POST \
  -H 'Content-Type: application/json' \
  -d '{
    "delegator_token": "<agent_a_token>",
    "target_agent_id": "spiffe://agentauth.local/agent/orch-1/task-1/agent-b",
    "delegated_scope": ["read:Customers:12345"],
    "max_ttl": 60
  }' \
  http://127.0.0.1:8080/v1/delegate | jq .
```

Expected success (`201`) includes:
- `delegation_token`
- `chain_hash`
- `delegation_depth`

Expected failure behavior:
- `401` invalid delegator token
- `403` scope escalation (requested scope broader than delegator scope)
- `403` depth exceeded (MVP max depth is 3)

## Mutual authentication (M06)

M06 adds agent-to-agent authentication. Unlike the HTTP endpoints in M01-M04, mutual auth operates as a programmatic API between agents sharing the same broker trust domain.

### Components

- **MutAuthHdl**: 3-step handshake — both agents prove identity via broker-issued tokens and Ed25519 nonce signatures
- **DiscoveryRegistry**: Bind agent SPIFFE IDs to network endpoints; verify bindings to prevent MITM
- **HeartbeatMgr**: Track agent liveness; optionally auto-revoke unresponsive agents

### Usage pattern

Agents use the handshake programmatically (no HTTP endpoint — it's a library API):

1. Agent A calls `InitiateHandshake(myToken, targetAgentID)` → gets `HandshakeReq` with nonce
2. Agent B calls `RespondToHandshake(req, myToken, myPrivKey)` → signs nonce, returns `HandshakeResp`
3. Agent A calls `CompleteHandshake(resp, originalNonce)` → verifies signature against registered public key

### Discovery binding

```go
dr := mutauth.NewDiscoveryRegistry()
dr.Bind(agentID, "https://agent-a.internal:8443")
endpoint, _ := dr.Resolve(agentID)
ok, _ := dr.VerifyBinding(agentID, presentedID) // MITM check
```

### Heartbeat monitoring

When wired with `RevSvc`, agents missing 3+ heartbeats are automatically revoked. Without `RevSvc`, missed heartbeats are logged as warnings.

## Run quality gates

Run these before declaring any task/module complete:

```bash
./scripts/gates.sh task
./scripts/gates.sh module
```

Gate levels:
- `task`: gitflow + build + lint + unit + docs
- `module`: task + integration + live + regression
- `milestone`: module + e2e placeholder
- `all`: full run

## Logging model

Format:
- `[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | CONTEXT`

Runtime control:
- `AA_LOG_LEVEL=quiet|standard|verbose|trace`

Example:

```bash
AA_LOG_LEVEL=quiet go run ./cmd/broker
```

Expected quiet behavior:
- suppresses normal OK logs
- keeps failure logs on stderr

## Troubleshooting

Common issues and actions:

- `bind: address already in use`
  - another broker is running on `8080`
  - stop existing process or run with a different `AA_PORT`

- `401` on `/v1/register`
  - launch token was invalid, expired, or already consumed
  - regenerate launch token and retry full challenge/register flow

- `403` on protected route
  - token scope does not match route requirement
  - validate token payload and requested scope

- `401` with `token-revoked` on protected route
  - token (or its agent/task/chain) was revoked via `/v1/revoke`
  - issue a new token through the challenge/register flow

- `400` on `/v1/revoke`
  - `level` must be one of: `token`, `agent`, `task`, `delegation_chain`
  - `target_id` is required; `reason` is optional

- gate failures at `GITFLOW`
  - branch and `.active_module` mismatch
  - align branch naming with active module, then re-run gates
