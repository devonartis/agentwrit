# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [Unreleased]

### Fixed
- P0 security: peer substitution vulnerability in `MutAuthHdl.RespondToHandshake` — any registered agent could respond to a handshake meant for a different peer. Added mandatory peer identity check (`ErrPeerMismatch`) and optional `DiscoveryRegistry` binding verification.
- P1 security: initiator identity spoofing in `MutAuthHdl.RespondToHandshake` — initiator token subject was not cross-checked against declared `InitiatorID`, allowing tampered handshake requests to impersonate a different agent. Added `ErrInitiatorMismatch` check.

### Added
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
