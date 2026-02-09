# AgentAuth Developer Guide

## Architecture snapshot (M00-M08, M05, M11-M12)

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
- `internal/audit` (M05)
  - immutable hash-chain audit event storage (`AuditLog`)
  - PII sanitization (email, phone, customer ID hashing)
  - read aggregation for repeated access events
  - chain integrity verification (`VerifyChain`)
- `internal/handler` (M01-M05)
  - challenge/register/token validate/token renew/revoke/audit HTTP handlers
- `internal/authz` (M03)
  - zero-trust authorization middleware (`ValMw`)
  - required scope context injection and authenticated agent context helper
- `internal/revoke` (M04)
  - 4-level revocation service (token/agent/task/delegation_chain)
  - `RevChecker` interface for pluggable backends
  - integrated into `ValMw` for real-time revocation enforcement
- `internal/mutauth` (M06)
  - 3-step mutual authentication handshake (`MutAuthHdl`)
  - discovery binding registry (`DiscoveryRegistry`)
  - heartbeat/liveness monitoring with optional auto-revocation (`HeartbeatMgr`)
- `internal/deleg` (M07)
  - scope attenuation (`Attenuate`)
  - delegation token issuance with depth/TTL constraints (`DelegSvc`)
  - delegation-chain integrity checks (`VerifyChain`, `VerifyChainHash`)
- `internal/obs` + `internal/handler` (M08)
  - centralized RFC 7807 problem factory (`WriteProblem`)
  - Prometheus collectors and recorder helpers
  - health and metrics HTTP handlers (`HealthHdl`, `MetricsHdl`)
- `demo/resource_server` (M11)
  - FastAPI resource server with 4 endpoints and dual-mode auth middleware
- `demo/agents` (M12)
  - Python demo agents: BrokerClient, AgentBase, DataRetriever, Analyzer, ActionTaker
  - Orchestrator driving full A->B->C workflow with delegation

## Repository layout (current)

```text
cmd/
  broker/
internal/
  audit/
  authz/
  cfg/
  deleg/
  handler/
  identity/
  mutauth/
  obs/
  revoke/
  store/
  token/
docs/
  USER_GUIDE.md
  DEVELOPER_GUIDE.md
  API_REFERENCE.md
  GIT_WORKFLOW.md
  api/openapi.yaml
  developer/
    scaffold.md
    identity.md
    token.md
    authz.md
    audit.md
    revoke.md
    mutauth.md
    deleg.md
    obs.md
    resource_server.md
    demo_agents.md
demo/
  resource_server/     -- M11 FastAPI resource server
  agents/              -- M12 Python demo agents
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
   - `docs/developer/<module>.md`
   - `docs/api/openapi.yaml`
   - `CHANGELOG.md`
4. Run `./scripts/gates.sh task`.
   - includes `SECURITY` (`gosec` + `govulncheck`) and fails if either tool is missing.
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

- M00 scaffold: `docs/developer/scaffold.md`
- M01 identity: `docs/developer/identity.md`
- M02 token: `docs/developer/token.md`
- M03 authorization: `docs/developer/authz.md`
- M04 revocation: `docs/developer/revoke.md`
- M05 audit: `docs/developer/audit.md`
- M06 mutual auth: `docs/developer/mutauth.md`
- M07 delegation: `docs/developer/deleg.md`
- M08 observability: `docs/developer/obs.md`
- M11 resource server: `docs/developer/resource_server.md`
- M12 demo agents: `docs/developer/demo_agents.md`

## Seed tokens (dev/test bootstrap)

Set `AA_SEED_TOKENS=true` to have the broker print a launch token and admin token to stdout on startup:

```bash
AA_SEED_TOKENS=true go run ./cmd/broker
# Output includes:
# SEED_LAUNCH_TOKEN=<hex>
# SEED_ADMIN_TOKEN=<jwt>
```

The smoke test (`cmd/smoketest/main.go`) uses these tokens to exercise the full broker workflow against the real binary, including admin-scoped revoke authorization. This is the live test (Tier 3) per ADR-001.

## Documentation policy (done criteria)

Documentation is required for task/module completion:

- User guidance:
  - runtime and troubleshooting in `docs/USER_GUIDE.md`
- Developer guidance:
  - architecture + extension notes in `docs/DEVELOPER_GUIDE.md` and `docs/developer/*.md`
- API guidance:
  - human reference in `docs/API_REFERENCE.md`
  - machine contract in `docs/api/openapi.yaml`
- Change tracking:
  - `CHANGELOG.md` update under `[Unreleased]`
