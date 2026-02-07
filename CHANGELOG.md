# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and this project follows Semantic Versioning.

## [Unreleased]

### Added
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
