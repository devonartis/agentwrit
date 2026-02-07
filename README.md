# AgentAuth

AgentAuth is an ephemeral agent credentialing system with a Go broker and Python demo application.

## Documentation

- User guide: `docs/USER_GUIDE.md`
- Developer guide: `docs/DEVELOPER_GUIDE.md`
- API reference: `docs/API_REFERENCE.md`
- OpenAPI spec: `docs/api/openapi.yaml`
- Git workflow: `docs/GIT_WORKFLOW.md`
- Changelog: `CHANGELOG.md`
- Active module marker: `.active_module`

## Current module status

Module M00 + M01 + M02 + M03 + M04 baseline implemented:
- broker entrypoint and health endpoint
- identity challenge endpoint (`GET /v1/challenge`)
- identity register endpoint (`POST /v1/register`)
- token validate endpoint (`POST /v1/token/validate`)
- token renew endpoint (`POST /v1/token/renew`)
- token revocation endpoint (`POST /v1/revoke`)
- protected customer endpoint (`GET /v1/protected/customers/12345`)
- structured logging framework (`internal/obs`)
- environment configuration loader (`internal/cfg`)
- baseline stores (`internal/store`)
- zero-trust authorization middleware (`internal/authz`)
- 4-level token revocation service (`internal/revoke`)
- quality gate runner (`scripts/gates.sh`)

## Local development

### Run tests and gates

```bash
go test ./...
./scripts/gates.sh task
```

### Run the broker

```bash
go run ./cmd/broker
```

### Health check

```bash
curl -i http://127.0.0.1:8080/v1/health
```

## Run the demo

End-to-end flow demonstrating challenge-response registration, token usage, and revocation:

```bash
# 1. Start the broker
go run ./cmd/broker &

# 2. Get a challenge nonce
CHALLENGE=$(curl -sS http://127.0.0.1:8080/v1/challenge)
echo "$CHALLENGE"

# 3. Register an agent (requires launch token + Ed25519 signed nonce)
#    See tests/integration/ for programmatic examples with key generation
curl -sS -X POST http://127.0.0.1:8080/v1/register \
  -H 'Content-Type: application/json' \
  -d '{
    "launch_token": "<token>",
    "nonce": "<nonce-from-step-2>",
    "agent_public_key": {"kty":"OKP","crv":"Ed25519","x":"<base64url-pubkey>"},
    "signature": "<base64url-sig>",
    "orchestration_id": "orch-1",
    "task_id": "task-1",
    "requested_scope": ["read:Customers:12345"]
  }'
# Returns: agent_instance_id + access_token

# 4. Use token on a protected endpoint
curl -sS http://127.0.0.1:8080/v1/protected/customers/12345 \
  -H 'Authorization: Bearer <access_token>'
# Returns: {"customer_id":"12345","message":"protected customer data"}

# 5. Revoke the token
curl -sS -X POST http://127.0.0.1:8080/v1/revoke \
  -H 'Content-Type: application/json' \
  -d '{"level":"token","target_id":"<jti>","reason":"demo revocation"}'
# Returns: {"revoked":true,...}

# 6. Verify revocation — same token now returns 401
curl -sS http://127.0.0.1:8080/v1/protected/customers/12345 \
  -H 'Authorization: Bearer <access_token>'
# Returns: 401 token-revoked
```
