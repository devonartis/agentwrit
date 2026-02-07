# AgentAuth Developer Guide

## Architecture snapshot (M00-M03)

Implemented packages and responsibilities:

- `cmd/broker`
  - broker process assembly and route wiring
- `internal/cfg` (M00)
  - environment-driven runtime configuration
- `internal/obs` (M00)
  - structured logging and stdout/stderr routing
- `internal/store` (M00-M01)
  - in-memory data backing for launch tokens, nonces, and agent records
  - exported error sentinels:
    - `ErrLaunchTokenNotFound`
    - `ErrLaunchTokenExpired`
    - `ErrLaunchTokenConsumed`
    - `ErrNonceNotFound`
    - `ErrNonceExpired`
    - `ErrAgentExists`
- `internal/identity` (M01)
  - SPIFFE ID generation/parsing/validation
  - launch token create/consume logic
  - Ed25519/JWK key handling
  - `IdSvc.Register` proof-of-possession flow
- `internal/token` (M02)
  - claims model + validation
  - scope parser/matcher/subset logic
  - signed token issue/verify/renew
- `internal/handler` (M01-M02)
  - challenge/register/token validate/token renew HTTP handlers
- `internal/authz` (M03)
  - zero-trust authorization middleware (`ValMw`)
  - required scope context injection and authenticated agent context helper

## Repository layout (current)

```text
cmd/
  broker/
internal/
  authz/
  cfg/
  handler/
  identity/
  obs/
  store/
  token/
docs/
  USER_GUIDE.md
  DEVELOPER_GUIDE.md
  API_REFERENCE.md
  GIT_WORKFLOW.md
  api/openapi.yaml
  dev/
    scaffold.md
    identity.md
    token.md
    authz.md
scripts/
  gates.sh
  doc_check.sh
  gitflow_check.sh
  set_active_module.sh
  integration_test.sh
  live_test.sh
tests/
  integration/
  live/
  e2e/
```

## Development workflow

1. Align module context:
   - `./scripts/set_active_module.sh MNN`
   - work on `feature/mnn-*`
2. Implement one micro-task with tests.
3. Update docs in the same change:
   - `docs/USER_GUIDE.md`
   - `docs/API_REFERENCE.md`
   - `docs/dev/<module>.md`
   - `docs/api/openapi.yaml`
   - `CHANGELOG.md`
4. Run `./scripts/gates.sh task`.
5. At module boundary, run `./scripts/gates.sh module` (includes integration + live).
6. Fix failures before any new module work.

## Coding standards

- Naming follows spec abbreviations where applicable (`IdSvc`, `TknSvc`, `ValMw`, etc.).
- Logging format:
  - `[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | CONTEXT`
- Stream policy:
  - `OK`, `WARN` -> stdout
  - `FAIL` -> stderr
- Authorization policy:
  - fail closed on missing/invalid auth context
  - require explicit route scope for protected resources

## Module documentation map

- M00 scaffold: `docs/dev/scaffold.md`
- M01 identity: `docs/dev/identity.md`
- M02 token: `docs/dev/token.md`
- M03 authorization: `docs/dev/authz.md`

## Documentation policy (done criteria)

Documentation is required for task/module completion:

- User guidance:
  - runtime and troubleshooting in `docs/USER_GUIDE.md`
- Developer guidance:
  - architecture + extension notes in `docs/DEVELOPER_GUIDE.md` and `docs/dev/*.md`
- API guidance:
  - human reference in `docs/API_REFERENCE.md`
  - machine contract in `docs/api/openapi.yaml`
- Change tracking:
  - `CHANGELOG.md` update under `[Unreleased]`
