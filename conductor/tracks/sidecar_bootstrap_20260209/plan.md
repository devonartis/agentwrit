# Implementation Plan: Sidecar-First Developer Bootstrap (Broker Support)

## Phase 1: Global Infrastructure & Middleware
- [x] Task: Implement Global Request-ID Middleware (5431984)
    - [ ] [Write Tests] Verify unique ID generation and context propagation
    - [ ] [Implement] Middleware to inject `X-Request-ID` into context and response headers
- [x] Task: Implement HTTP Request Logging Middleware (8a9d3b4)
    - [ ] [Write Tests] Verify logging of method, path, status, and latency
    - [ ] [Implement] Logger middleware using `internal/obs`
- [x] Task: Standardize Error Contract & Problem Writer (47a22db)
    - [ ] [Write Tests] Verify `error_code` and `hint` are present in JSON responses
    - [ ] [Implement] Update `handler.WriteProblem` and align `admin/admin_hdl.go:writeProblem`
- [ ] Task: Conductor - User Manual Verification 'Phase 1: Global Infrastructure & Middleware' (Protocol in workflow.md)
- [ ] Task: Quality Gate Check: Run `./scripts/gates.sh task`

## Phase 2: Foundation & Model Updates
- [ ] Task: Update Token Claims for Sidecar Identity
    - [ ] [Write Tests] Verify `sid` (internal JWT claim) is handled and validated
    - [ ] [Implement] Add `Sid` string field to `internal/token.TknClaims`; update `Validate()`
- [ ] Task: Implement Activation Token Persistence & Replay Protection
    - [ ] [Write Tests] Verify JTI consumption and metadata (exp, scope) tracking
    - [ ] [Implement] Extend `Store` with `ConsumeActivationToken(jti string, exp int64) error`
- [ ] Task: Conductor - User Manual Verification 'Phase 2: Foundation & Model Updates' (Protocol in workflow.md)
- [ ] Task: Quality Gate Check: Run `./scripts/gates.sh task`

## Phase 3: Sidecar Activation (Admin & SAP)
- [ ] Task: Implement Admin Activation Issuance
    - [ ] [Write Tests] Verify `POST /v1/admin/sidecar-activations` mints correct JWTs
    - [ ] [Implement] `AdminSvc` logic to mint activation tokens with stable `sid`
- [ ] Task: Implement Sidecar Activation Protocol (SAP) Handler
    - [ ] [Write Tests] Test successful activation, `activation_token_replayed`, and `invalid_activation_token`
    - [ ] [Implement] `SidecarActivateHdl` using single-use JTI enforcement
- [ ] Task: Conductor - User Manual Verification 'Phase 3: Sidecar Activation (Admin & SAP)' (Protocol in workflow.md)
- [ ] Task: Quality Gate Check: Run `./scripts/gates.sh task`

## Phase 4: Token Exchange & Scoping
- [ ] Task: Implement Token Exchange Service Logic
    - [ ] [Write Tests] Verify scope subset enforcement and `sid` -> `sidecar_id` injection
    - [ ] [Implement] `TknSvc.Exchange` logic with attenuation checks
- [ ] Task: Implement Token Exchange Handler & Revocation Semantics
    - [ ] [Write Tests] Verify `403 scope_escalation_denied` and rule: **Downstream agent tokens remain valid until expiry after sidecar revocation.**
    - [ ] [Implement] `TokenExchangeHdl`; enforce STA revocation rules
- [ ] Task: Conductor - User Manual Verification 'Phase 4: Token Exchange & Scoping' (Protocol in workflow.md)
- [ ] Task: Quality Gate Check: Run `./scripts/gates.sh task`

## Phase 5: Integration & Observability
- [ ] Task: Route Registration & Rate Limiting in `main.go`
    - [ ] [Implement] Register routes with split defaults: **Prod (5/min, 10 burst)** vs **Dev (10/min, 50 burst via `AA_DEV_PROFILE=true`)**
- [ ] Task: Implement Sidecar Audit Logging
    - [ ] [Implement] Add `sidecar_activated`, `sidecar_activation_failed`, `sidecar_token_exchanged` events
- [ ] Task: Conductor - User Manual Verification 'Phase 5: Integration & Observability' (Protocol in workflow.md)
- [ ] Task: Quality Gate Check: Run `./scripts/gates.sh task`

## Phase 6: Documentation & E2E Verification
- [ ] Task: Update OpenAPI Specification and Claims Documentation
    - [ ] [Implement] Update `openapi.yaml` and `API_REFERENCE.md` with `sid` and `sidecar_id` schemas
- [ ] Task: Execute E2E Live Test Suite
    - [ ] [Write Tests] Create script to verify success, replay fail, escalation fail, and **Proxy Trust (`X-Forwarded-For` ignored unless `AA_TRUST_PROXY=true`)**
    - [ ] [Verify] Run full `./scripts/gates.sh module` suite
- [ ] Task: Conductor - User Manual Verification 'Phase 6: Documentation & E2E Verification' (Protocol in workflow.md)
