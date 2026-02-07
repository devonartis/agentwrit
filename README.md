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

Module M00 + M01 + M02 + M03 baseline implemented:
- broker entrypoint and health endpoint
- identity challenge endpoint (`GET /v1/challenge`)
- identity register endpoint (`POST /v1/register`)
- token validate endpoint (`POST /v1/token/validate`)
- token renew endpoint (`POST /v1/token/renew`)
- protected customer endpoint (`GET /v1/protected/customers/12345`)
- structured logging framework (`internal/obs`)
- environment configuration loader (`internal/cfg`)
- baseline stores (`internal/store`)
- zero-trust authorization middleware (`internal/authz`)
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
