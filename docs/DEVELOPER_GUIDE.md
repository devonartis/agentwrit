# AgentAuth Developer Guide

## Architecture snapshot

Implemented packages and responsibilities:

- `cmd/broker`
  - broker process assembly and route wiring
- `internal/cfg`
  - environment-driven runtime configuration
- `internal/obs`
  - structured logging and stdout/stderr routing
- `internal/store`
  - in-memory data backing for launch tokens, nonces, and agent records
  - exported error sentinels:
    - `ErrLaunchTokenNotFound`
    - `ErrLaunchTokenExpired`
    - `ErrLaunchTokenConsumed`
    - `ErrNonceNotFound`
    - `ErrNonceExpired`
    - `ErrAgentExists`
- `internal/identity`
  - SPIFFE ID generation/parsing/validation
  - launch token create/consume logic
  - Ed25519/JWK key handling
  - `IdSvc.Register` proof-of-possession flow
- `internal/token`
  - claims model + validation
  - scope parser/matcher/subset logic
  - signed token issue/verify/renew
- `internal/audit`
  - immutable hash-chain audit event storage (`AuditLog`)
  - PII sanitization (email, phone, customer ID hashing)
  - read aggregation for repeated access events
  - chain integrity verification (`VerifyChain`)
- `internal/handler`
  - challenge/register/token validate/token renew/revoke/audit HTTP handlers
- `internal/authz`
  - zero-trust authorization middleware (`ValMw`)
  - required scope context injection and authenticated agent context helper
- `internal/revoke`
  - 4-level revocation service (token/agent/task/delegation_chain)
  - `RevChecker` interface for pluggable backends
  - integrated into `ValMw` for real-time revocation enforcement
- `internal/mutauth`
  - 3-step mutual authentication handshake (`MutAuthHdl`)
  - discovery binding registry (`DiscoveryRegistry`)
  - heartbeat/liveness monitoring with optional auto-revocation (`HeartbeatMgr`)
- `internal/deleg`
  - scope attenuation (`Attenuate`)
  - delegation token issuance with depth/TTL constraints (`DelegSvc`)
  - delegation-chain integrity checks (`VerifyChain`, `VerifyChainHash`)
- `internal/obs` + `internal/handler`
  - centralized RFC 7807 problem factory (`WriteProblem`)
  - Prometheus collectors and recorder helpers
  - health and metrics HTTP handlers (`HealthHdl`, `MetricsHdl`)
- `demo/resource_server`
  - FastAPI resource server with 4 endpoints and dual-mode auth middleware
- `demo/agents`
  - Python demo agents: BrokerClient, AgentBase, DataRetriever, Analyzer, ActionTaker
  - Orchestrator driving full A->B->C workflow with delegation
- `demo/attacks`
  - Attack simulator with 5 scenarios: credential theft, lateral movement, impersonation, privilege escalation, accountability bypass
  - Demonstrates how the broker detects and blocks each attack class
- `demo/dashboard`
  - FastAPI + HTMX real-time dashboard with SSE event streaming
  - Dark-themed UI showing broker state, agent activity, and security events

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
    attack_simulator.md
    dashboard.md
demo/
  resource_server/     -- FastAPI resource server
  agents/              -- Python demo agents
  attacks/             -- Attack simulator (5 scenarios)
  dashboard/           -- HTMX real-time dashboard
scripts/
  gates.sh
  doc_check.sh
  gitflow_check.sh
  integration_test.sh
  live_test.sh
tests/
  integration/
  live/
  e2e/
```

## Setting up the Python demo

The `demo/` directory contains Python components (resource server, demo agents, attack simulator, dashboard). To set up:

```bash
cd demo
pip install -r requirements.txt
python -m pytest -v
```

Each sub-package can also be tested individually:

```bash
python -m pytest demo/resource_server/tests/ -v
python -m pytest demo/agents/tests/ -v
python -m pytest demo/attacks/tests/ -v
python -m pytest demo/dashboard/tests/ -v
```

## Development workflow

1. Create a feature branch from `develop`.
2. Implement your change with tests.
3. Update docs in the same change:
   - `docs/USER_GUIDE.md`
   - `docs/API_REFERENCE.md`
   - `docs/developer/<module>.md`
   - `docs/api/openapi.yaml`
   - `CHANGELOG.md`
4. Run `./scripts/gates.sh task`.
   - Includes build, lint, unit tests, security scanning (`gosec` + `govulncheck`), and doc checks.
5. For larger changes, run `./scripts/gates.sh module` (adds integration + live tests).
6. Fix all gate failures before opening a PR.

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

- Scaffold: `docs/developer/scaffold.md`
- Identity: `docs/developer/identity.md`
- Token: `docs/developer/token.md`
- Authorization: `docs/developer/authz.md`
- Revocation: `docs/developer/revoke.md`
- Audit: `docs/developer/audit.md`
- Mutual auth: `docs/developer/mutauth.md`
- Delegation: `docs/developer/deleg.md`
- Observability: `docs/developer/obs.md`
- Resource server: `docs/developer/resource_server.md`
- Demo agents: `docs/developer/demo_agents.md`
- Attack simulator: `docs/developer/attack_simulator.md`
- Dashboard: `docs/developer/dashboard.md`

## Seed tokens (dev/test bootstrap)

Set `AA_SEED_TOKENS=true` to have the broker print a launch token and admin token to stdout on startup:

```bash
AA_SEED_TOKENS=true go run ./cmd/broker
# Output includes:
# SEED_LAUNCH_TOKEN=<hex>
# SEED_ADMIN_TOKEN=<jwt>
```

The smoke test (`cmd/smoketest/main.go`) uses these tokens to exercise the full broker workflow against the real binary, including admin-scoped revoke authorization.

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
