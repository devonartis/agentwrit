# Track Specification: Sidecar-First Developer Bootstrap (Broker Support)

## Overview
Implementation of the broker-side support for the "Sidecar-First" developer bootstrap flow (ADR-005). This track establishes the core security contract, token exchange mechanisms, and traceability features required for sidecar-mediated agent authentication.

## Objectives
- Implement the Sidecar Activation Protocol (SAP) with single-use enforcement.
- Establish the Sidecar Token Authority (STA) model using stable sidecar identities.
- Implement broker-derived `sidecar_id` mapping for agent tokens.
- Provide high-fidelity diagnostics and a normative error-code framework.

## Functional Requirements

### 1. Sidecar Activation Issuance (Admin Flow)
- **Endpoint:** `POST /v1/admin/sidecar-activations`
- **Auth:** Required `admin:sidecar:issue` scope.
- **Function:** Mints a "Sidecar-Activation" token.
- **Contract:** `iss: agentauth`, `aud: sidecar_activation`, `sub: admin`, `sid: [generated_stable_id]`.

### 2. Sidecar Activation Protocol (SAP)
- **Endpoint:** `POST /v1/sidecar/activate`
- **Request:** `{ "activation_token": "JWT" }`
- **Enforcement:** 
    - Verifies activation token signature and claims.
    - **Single-Use:** Broker checks JTI against `SqlStore`. Replay attempts fail.
    - **Stable Identity:** The `sid` claim from the activation token is carried forward into the issued sidecar token.

### 3. Sidecar Token Authority (STA) & Token Exchange
- **Exchange Endpoint:** `POST /v1/token/exchange`
- **Auth:** Required Bearer token with `sidecar:manage` scope.
- **Request:** `{ "agent_id": "spiffe://...", "scope": ["..."], "ttl": 300 }`
- **Enforcement:**
    - **Scope Subset:** Requested scope MUST be a subset of the sidecar's authorized scope.
    - **Broker Injection:** Broker derives `sidecar_id` from the sidecar's `sid` claim and injects it into the issued agent token.
- **Revocation Semantics:** If a sidecar token is revoked, the sidecar loses the authority to use `/v1/token/exchange`. Downstream agent tokens remain valid until their (short-lived) expiry to maintain performance and simplicity.

### 4. Normative Error & Diagnostics
All sidecar-related endpoints MUST return an RFC 7807 problem object containing `error_code` and `request_id`.

| Scenario | HTTP Status | error_code |
|----------|-------------|------------|
| Replayed activation token | 401 | `activation_token_replayed` |
| Invalid/Expired activation token | 401 | `invalid_activation_token` |
| Scope escalation attempt | 403 | `scope_escalation_denied` |
| Malformed JSON body | 400 | `invalid_request` |
| Internal processing failure | 500 | `internal_error` |

### 5. Rate Limiting & Proxy Trust
- **Policy:** `POST /v1/sidecar/activate` limited to 10 requests/minute with a **burst of 50** to support multi-agent local startup.
- **Override:** Limits are env-tunable via `AA_DEV_PROFILE=true` to allow higher thresholds during development.
- **Trust:** Honor `X-Forwarded-For` when `AA_TRUST_PROXY=true` is configured.

## Technical Requirements
- **OpenAPI:** Update `docs/api/openapi.yaml` with `/v1/sidecar/activate`, `/v1/token/exchange`, and `/v1/admin/sidecar-activations`.
- **Audit Events:**
    - `sidecar_activated` (Success)
    - `sidecar_activation_failed` (Failure + Reason)
    - `sidecar_token_exchanged` (Lineage: Sidecar SID -> Agent ID)

## Acceptance Criteria
- [ ] **Positive:** Admin can mint an activation token; sidecar can successfully swap it for a functional token.
- [ ] **Negative (Replay):** Reusing an activation token results in `401 activation_token_replayed`.
- [ ] **Negative (Escalation):** Sidecar attempting to exchange a token with scope outside its ceiling results in `403 scope_escalation_denied`.
- [ ] **Negative (Spoofing):** Client-provided `sidecar_id` in requests is ignored; the broker-derived `sid` is used.
- [ ] **Diagnostics:** Every error response contains a valid `request_id` that matches the broker's audit log.
