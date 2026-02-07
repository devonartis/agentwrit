# AgentAuth User Guide

## Purpose and audience

This guide is for operators and evaluators running the AgentAuth broker locally.
It explains how to run the service, execute the identity/token flow, validate protected access behavior, and diagnose failures.

## Prerequisites

- Go 1.22+
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
- HTTP status `200`
- JSON body `{"status":"healthy"}`

## End-to-end identity and token workflow

This sequence covers the M01-M03 identity, token, and authorization workflow.

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

- gate failures at `GITFLOW`
  - branch and `.active_module` mismatch
  - align branch naming with active module, then re-run gates
