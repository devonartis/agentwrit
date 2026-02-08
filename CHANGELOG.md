# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [Unreleased]

### Fixed
- P0 security: delegated JWT issuance dropped `delegation_chain` data because `TknSvc.Issue` always reset the claim to empty. `IssueReq` now accepts `DelegChain`, delegated token issuance passes the chain through, and renewal preserves the chain.
- P1 security: runtime auth middleware did not enforce delegation-chain signature/scope checks. `ValMw` now validates non-empty chains via `deleg.VerifyChain` and denies malformed chains.
- P1 policy: delegation depth checks were bypassable on re-delegation because chain depth was not carried in tokens. With chain propagation fixed, second-hop depth is now enforced correctly.
- P1 gate hardening: `scripts/gates.sh task` now includes `SECURITY` (`gosec` + `govulncheck`) and fails with actionable install messages when tools are missing.
- P0 security: peer substitution vulnerability in `MutAuthHdl.RespondToHandshake` — any registered agent could respond to a handshake meant for a different peer. Added mandatory peer identity check (`ErrPeerMismatch`) and optional `DiscoveryRegistry` binding verification.
- P1 security: initiator identity spoofing in `MutAuthHdl.RespondToHandshake` — initiator token subject was not cross-checked against declared `InitiatorID`, allowing tampered handshake requests to impersonate a different agent. Added `ErrInitiatorMismatch` check.
- P2: pass `nil` `DiscoveryRegistry` in `main.go` instead of empty non-nil instance — an unpopulated registry would reject all handshakes via `ErrAgentNotBound` if the handler were ever exposed.
- P1 documentation: ADR-001 "What the smoke test does" section overstated coverage — claimed admin auth, audit trail, scope mismatch, and hash chain testing that the actual smoketest does not implement. Corrected to match actual 10-step coverage and documented deferred items.
- P2: live gate pass text in `live_test.sh` was outdated ("error-paths validated") — updated to reflect actual 10-step end-to-end lifecycle coverage.
- P2 security: responder identity spoofing in `MutAuthHdl.CompleteHandshake` — responder token subject was not cross-checked against declared `ResponderID`, same vulnerability class as the initiator check. Added `ErrResponderMismatch` and changed `GetAgent` to use verified `claims.Sub`.
- P2: reverted premature `NewDiscoveryRegistry()` wiring in `main.go` — non-nil empty registry activates binding checks that reject all agents. Discovery enforcement deferred until binding lifecycle (bind on register, unbind on revoke) is implemented.

### Added
- Module M08-T01 observability baseline:
  - Shared RFC 7807 problem factory in `internal/obs` (`WriteProblem`)
  - Handler and authz paths now emit centralized `application/problem+json` payloads
  - Factory unit test coverage in `internal/obs/rfc7807_factory_test.go`
- Module M07 delegation chain verification:
  - Scope attenuation (`Attenuate`) with actionable error detail on escalation attempts
  - `DelegSvc` for delegation token creation with TTL enforcement and depth limits (max 3)
  - Chain verification (`VerifyChain`) with Ed25519 signature validation per hop and revocation checks
  - SHA-256 chain hash for tamper detection and chain-level revocation
  - `POST /v1/delegate` endpoint with RFC 7807 errors (401, 403 scope-escalation, 403 depth-exceeded)
  - Live smoke test extended with delegation steps (delegate, scope escalation blocked, delegation token validated)
  - Integration tests: happy path, scope escalation blocked, re-delegation blocked, depth limits
- Module M06 mutual authentication:
  - 3-step agent-to-agent handshake protocol (`MutAuthHdl`)
  - Discovery binding registry for agent-to-endpoint mapping and MITM prevention
  - Heartbeat/liveness monitoring with optional auto-revocation via `RevSvc`
  - `GetAgent` store method for agent identity lookup
  - Mutauth components wired into `cmd/broker/main.go` with graceful shutdown
  - Integration tests for handshake and discovery flows
- ADR-001 live testing infrastructure:
  - `AA_SEED_TOKENS=true` bootstrap flag for dev/test launch and admin token seeding
  - `cmd/smoketest/main.go` — full workflow smoke test against real compiled binary
  - Updated `scripts/live_test.sh` to build and run smoke test (replaces error-path-only test)
  - Covers: health, challenge, register, validate, protected access, renew, revoke, revocation check
- Module M04 revocation service:
  - 4-level token revocation (token/agent/task/delegation_chain)
  - `RevChecker` interface for pluggable revocation backends
  - `POST /v1/revoke` endpoint with RFC 7807 error responses
  - Integration with `ValMw` authorization middleware
  - In-memory revocation sets with `RWMutex` for read-heavy access
- Documentation hardening:
  - Renamed `docs/dev/` to `docs/developer/` per ADD-6.7
  - Enriched scaffold.md with design rationale and extension points
  - Added DelegRecord/DelegChain documentation to token.md
  - Created `docs/developer/revoke.md` with decision record
  - Added demo operation flow to README.md
  - Added godoc comments to all exported symbols
  - Hardened `doc_check.sh` with godoc and endpoint-OpenAPI parity checks
- Module M00 scaffold with Go broker entrypoint and `/v1/health`.
- Structured logging package (`internal/obs`) with stdout/stderr routing and tests.
- Environment configuration loader (`internal/cfg`).
- Baseline storage placeholders (`internal/store`).
- Quality gate runner (`scripts/gates.sh`) and documentation checker (`scripts/doc_check.sh`).
- Module M01 identity issuance baseline:
  - SPIFFE ID generation/validation/parsing
  - Launch token creation and single-use validation
  - Ed25519 key management and JWK public-key parsing
  - `GET /v1/challenge` and `POST /v1/register` handlers
  - Integration flow test and live endpoint checks
- Module M02 token service baseline:
  - Token claims model and validation
  - Scope parsing/matching/subset logic
  - Signed token issue/verify/renew service
  - `POST /v1/token/validate` and `POST /v1/token/renew` handlers
  - Integration and live coverage for token endpoints
- Module M03 zero-trust authorization baseline:
  - `ValMw` authorization middleware for bearer token verification
  - `WithRequiredScope` route scope injection
  - `AgentIDFromContext` helper for downstream handlers
  - Protected route `GET /v1/protected/customers/12345`
  - Why-denied structured logging for auth failures
- Documentation set:
  - `docs/USER_GUIDE.md`
  - `docs/DEVELOPER_GUIDE.md`
  - `docs/API_REFERENCE.md`
  - `docs/GIT_WORKFLOW.md`
  - `docs/api/openapi.yaml`
