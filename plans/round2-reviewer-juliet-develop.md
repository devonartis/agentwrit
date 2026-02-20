# Compliance Review: Round 2 -- Reviewer Juliet

**Date:** 2026-02-20
**Branch:** develop (code snapshot at /tmp/agentauth-develop/)
**Pattern Document:** Security-Pattern-That-Is-Why-We-Built-AgentAuth.md v1.2
**Reviewer:** Juliet (Independent Security Compliance Reviewer)

---

## Methodology

This review examines every component and requirement stated in the security pattern document and maps each to specific code evidence in the develop branch. Verdicts are:

- **COMPLIANT** -- The code fully satisfies the stated requirement.
- **PARTIALLY COMPLIANT** -- The code addresses the requirement but with gaps or limitations.
- **NOT COMPLIANT** -- The code does not address the requirement.

Requirements marked as optional, aspirational, or "recommended" in the pattern are noted but not penalized if absent.

---

## Component 1: Ephemeral Identity Issuance

### Requirement 1.1: Each agent receives a unique, cryptographically-verifiable identity

**Pattern quote:** "At spawn time, each agent receives a unique, cryptographically-verifiable identity that cannot be reused."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/identity/id_svc.go`, lines 178-184
- `randomInstanceID()` generates 8 random bytes (16 hex chars) per registration (line 253-258)
- `NewSpiffeId()` constructs the full SPIFFE URI

```go
instanceID := randomInstanceID()
agentID, err := NewSpiffeId(s.trustDomain, req.OrchID, req.TaskID, instanceID)
```

Each registration produces a unique instance ID from `crypto/rand`, making collision statistically negligible.

**Verdict: COMPLIANT**

### Requirement 1.2: Identity follows SPIFFE format with orchestration context

**Pattern quote:** "spiffe://trust-domain/agent/{orchestration_id}/{task_id}/{instance_id}"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/identity/spiffe.go`, lines 17-28
- Uses the official `go-spiffe/v2` library for trust domain and path validation
- Format: `spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}`

```go
id, err := spiffeid.FromSegments(td, "agent", orchID, taskID, instanceID)
```

- `ParseSpiffeId` (lines 35-48) validates and extracts components, enforcing the `/agent/{orchID}/{taskID}/{instanceID}` structure.

**Verdict: COMPLIANT**

### Requirement 1.3: Cryptographically bound to agent runtime environment

**Pattern quote:** "Cryptographically bound to agent runtime environment"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/identity/id_svc.go`, lines 142-168
- Agent presents an Ed25519 public key during registration
- Agent must sign a one-time nonce with its private key
- The broker verifies the signature before issuing identity:

```go
if !ed25519.Verify(pubKey, nonceBytes, sigBytes) {
    return nil, ErrInvalidSignature
}
```

- Public key is stored with the agent record (line 209) for later use in mutual authentication.

The identity is bound to a cryptographic key pair held by the agent. It is not bound to infrastructure attestation properties (container hash, K8s metadata, etc.), which would require a SPIRE agent deployment. The pattern lists SPIFFE/SPIRE attestation as one of several "Implementation Options" -- the code implements a challenge-response key-binding approach instead.

**Verdict: COMPLIANT** -- The identity is cryptographically bound to the agent's key material. Platform attestation is listed as optional infrastructure in the pattern.

### Requirement 1.4: Bootstrap Problem (Secret Zero)

**Pattern quote:** "A fundamental challenge in any identity system is the 'secret zero' problem..."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/admin/admin_svc.go`, lines 187-257
- File: `/tmp/agentauth-develop/internal/identity/id_svc.go`, lines 117-121
- Bootstrap is handled via pre-authorized launch tokens created by an admin
- Launch tokens are opaque, cryptographically random (32 bytes), single-use by default
- They expire after a configurable TTL and carry a scope ceiling

The pattern describes the bootstrap problem and lists several approaches (SPIRE attestation, CIMD, one-time registration tokens). The code implements the "one-time registration token" approach (pattern line 291: "agents receive a one-time registration token that can only be used once and only from expected locations"). Launch tokens are consumed on use.

**Verdict: COMPLIANT**

---

## Component 2: Short-Lived Task-Scoped Tokens

### Requirement 2.1: JWT tokens with narrow scope limited to specific resources

**Pattern quote:** "Agents receive JWT tokens with narrow scope limited to specific resources and actions required for their task."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/token/tkn_claims.go`, lines 30-45
- Token claims include: `iss`, `sub`, `aud`, `exp`, `nbf`, `iat`, `jti`, `scope`, `task_id`, `orch_id`, `delegation_chain`, `chain_hash`
- Scope uses the `action:resource:identifier` format (e.g., `read:data:project-42`)
- File: `/tmp/agentauth-develop/internal/authz/scope.go`, lines 39-47 -- `ParseScope` enforces three-part colon-separated format

The token structure matches the pattern's specified structure closely:
- `sub` = SPIFFE agent ID
- `aud` = audience (optional)
- `exp`, `iat` = timestamps
- `jti` = unique token identifier
- `scope` = task-scoped permissions
- `task_id`, `orch_id` = orchestration context
- `delegation_chain` = delegation provenance

**Verdict: COMPLIANT**

### Requirement 2.2: Ed25519 / EdDSA signatures

**Pattern quote:** "Cryptographic signatures using Ed25519 or RSA keys"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/token/tkn_svc.go`, lines 194-213
- JWT header: `{"alg": "EdDSA", "typ": "JWT"}`
- Signing: `ed25519.Sign(s.signingKey, []byte(signingInput))`
- Verification: `ed25519.Verify(s.pubKey, []byte(signingInput), sigBytes)` (lines 143)
- Key generation: `ed25519.GenerateKey(rand.Reader)` in broker main (line 61)

**Verdict: COMPLIANT**

### Requirement 2.3: Token TTL matching task duration (1-15 min typical, up to 60 min)

**Pattern quote:** "For most agent tasks, a default TTL of 5 minutes provides a reasonable balance."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/cfg/cfg.go`, line 44 -- DefaultTTL = 300 seconds (5 minutes)
- File: `/tmp/agentauth-develop/internal/identity/id_svc.go`, lines 186-189 -- TTL defaults to launch token's MaxTTL, falling back to 300s
- File: `/tmp/agentauth-develop/internal/handler/token_exchange_hdl.go`, line 21 -- maxExchangeTTL = 900 (15 min cap for sidecar exchange)

The default 5-minute TTL matches the pattern's recommended default exactly. The ceiling (900s for exchange, configurable MaxTTL per launch token) stays within the pattern's acceptable range of 1-60 minutes.

**Verdict: COMPLIANT**

### Requirement 2.4: Token validation (signature, expiration, scope, JTI, issuer, subject)

**Pattern quote:** "Validate: signature, expiration, scope, revocation status, delegation chain"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/token/tkn_svc.go`, lines 130-163 -- `Verify()` checks:
  1. Token format (3-part JWT)
  2. Ed25519 signature
  3. Claims decode
  4. `claims.Validate()` checks issuer == "agentauth", subject non-empty, JTI non-empty, expiration, nbf
- File: `/tmp/agentauth-develop/internal/token/tkn_claims.go`, lines 61-78 -- `Validate()`
- File: `/tmp/agentauth-develop/internal/authz/val_mw.go`, lines 64-103 -- middleware adds revocation check
- File: `/tmp/agentauth-develop/internal/authz/val_mw.go`, lines 111-130 -- `RequireScope` checks scope coverage

**Verdict: COMPLIANT**

### Requirement 2.5: Scope enforcement at registration (attenuation only)

**Pattern quote:** "Credential service validates agent identity and authorizes scope"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/identity/id_svc.go`, lines 123-134
- Comment: `// CRITICAL: Check scope BEFORE consuming the launch token`

```go
if !authz.ScopeIsSubset(req.RequestedScope, ltRec.AllowedScope) {
    // ... audit, metric, return ErrScopeViolation
}
```

- File: `/tmp/agentauth-develop/internal/authz/scope.go`, lines 74-87 -- `ScopeIsSubset` enforces that every requested scope is covered by an allowed scope, supporting wildcard attenuation.

**Verdict: COMPLIANT**

---

## Component 3: Zero-Trust Enforcement

### Requirement 3.1: Every request is authenticated and authorized independently

**Pattern quote:** "Every request is authenticated and authorized independently, with no implicit trust."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/authz/val_mw.go`, lines 64-103 -- `ValMw.Wrap()` extracts Bearer token, verifies signature, checks revocation on every request
- File: `/tmp/agentauth-develop/cmd/broker/main.go`, lines 151-161 -- all authenticated endpoints are wrapped:

```go
mux.Handle("POST /v1/token/renew", problemdetails.MaxBytesBody(valMw.Wrap(renewHdl)))
mux.Handle("POST /v1/token/exchange", problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("sidecar:manage:*", tokenExchangeHdl))))
mux.Handle("POST /v1/delegate", problemdetails.MaxBytesBody(valMw.Wrap(delegHdl)))
mux.Handle("POST /v1/revoke", problemdetails.MaxBytesBody(valMw.Wrap(valMw.RequireScope("admin:revoke:*", revokeHdl))))
mux.Handle("GET /v1/audit/events", valMw.Wrap(valMw.RequireScope("admin:audit:*", auditHdl)))
```

Every request to an authenticated endpoint goes through full token verification (signature + expiry + revocation) independently.

**Verdict: COMPLIANT**

### Requirement 3.2: Mutual TLS (mTLS) between agent and server

**Pattern quote:** "Transport Layer: Mutual TLS (mTLS) between agent and server"

**Code evidence:**
- File: `/tmp/agentauth-develop/cmd/broker/main.go`, line 174 -- `http.ListenAndServe(addr, rootHandler)` -- plain HTTP, no TLS configuration.

The broker listens on plain HTTP. There is no TLS configuration in the broker code. The pattern lists mTLS as a core enforcement point. However, production deployments commonly terminate TLS at a reverse proxy (noted in `rate_mw.go` line 92: "Production deployments MUST place the broker behind a TLS-terminating reverse proxy"). The broker code itself does not implement mTLS directly.

**Verdict: PARTIALLY COMPLIANT** -- The broker does not natively implement mTLS. It relies on infrastructure-level TLS termination. The pattern requires mTLS as a transport-layer enforcement point. While proxy-terminated TLS is a valid deployment model for server-side TLS, true mutual TLS (where both client and server present certificates) is not implemented.

### Requirement 3.3: No trust based on network location

**Pattern quote:** "No trust based on network location"

**Code evidence:**
- All authenticated endpoints require Bearer token validation regardless of network origin
- Rate limiting uses IP (File: `/tmp/agentauth-develop/internal/authz/rate_mw.go`) for throttling only, not for trust decisions
- No IP allowlists or network-based trust in any handler

**Verdict: COMPLIANT**

---

## Component 4: Automatic Expiration and Revocation

### Requirement 4.1: Time-based expiration

**Pattern quote:** "Time-based: Maximum lifetime reached (1-15 minutes typical)"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/token/tkn_svc.go`, lines 86-93 -- `exp` set to `now + TTL`
- File: `/tmp/agentauth-develop/internal/token/tkn_claims.go`, lines 72-73 -- validation rejects expired tokens:

```go
if c.Exp != 0 && now > c.Exp {
    return ErrTokenExpired
}
```

**Verdict: COMPLIANT**

### Requirement 4.2: Active Revocation List (ARL) checked on each validation

**Pattern quote:** "Active Revocation List (ARL) checked on each validation"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/revoke/rev_svc.go` -- full four-level revocation service
- File: `/tmp/agentauth-develop/internal/authz/val_mw.go`, lines 93-98:

```go
if m.revSvc != nil && m.revSvc.IsRevoked(claims) {
    // ... respond 403, record audit event
}
```

Revocation is checked on every request through the validation middleware, matching the ARL pattern.

**Verdict: COMPLIANT**

### Requirement 4.3: Four revocation levels (token, agent, task, chain)

**Pattern quote:** "Token-level, Agent-level, Task-level, Delegation-chain-level"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/revoke/rev_svc.go`, lines 30-36:

```go
tokens map[string]bool // JTI -> revoked
agents map[string]bool // agent SPIFFE ID -> revoked
tasks  map[string]bool // task_id -> revoked
chains map[string]bool // root delegator agent ID -> revoked
```

- `IsRevoked` (lines 52-82) checks all four levels in order
- `Revoke` (lines 89-112) supports all four levels: "token", "agent", "task", "chain"
- Chain-level revocation targets the root delegator (line 76): `claims.DelegChain[0].Agent`

**Verdict: COMPLIANT**

### Requirement 4.4: Task-based expiration (agent signals task completion)

**Pattern quote:** "Task-based: Agent signals task completion"

**Code evidence:**
- There is no explicit "task complete" endpoint or signal mechanism in the broker API
- However, tokens expire automatically and agents can stop renewing
- The heartbeat manager (`/tmp/agentauth-develop/internal/mutauth/heartbeat.go`) auto-revokes agents that stop heartbeating

The pattern describes task-based expiration as one of three trigger types. The code implements time-based expiration and revocation capability, plus heartbeat-based auto-revocation (which serves a similar purpose -- detecting when agents are no longer active). There is no explicit "I'm done" signal endpoint.

**Verdict: PARTIALLY COMPLIANT** -- Time-based expiry and heartbeat-based auto-revocation cover most scenarios, but explicit task-completion signaling is not implemented.

### Requirement 4.5: Anomaly-based revocation triggers

**Pattern quote:** "Anomaly detection triggers immediate credential revocation" and "Anomaly Detection (Optional but Recommended)"

**Code evidence:**
- The heartbeat manager auto-revokes agents that miss heartbeats (file: `/tmp/agentauth-develop/internal/mutauth/heartbeat.go`, lines 126-134)
- No behavioral anomaly detection (unusual access patterns, unexpected scopes, etc.)

The pattern marks anomaly detection as "Optional but Recommended."

**Verdict: PARTIALLY COMPLIANT** -- Heartbeat-based liveness monitoring exists, but broader behavioral anomaly detection is absent. Since the pattern marks this as optional, this is an acceptable gap for the current release.

---

## Component 5: Immutable Audit Logging

### Requirement 5.1: Tamper-proof, append-only storage

**Pattern quote:** "All agent actions logged to tamper-proof, append-only storage"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/audit/audit_log.go`
- Hash chain: Each event's `Hash` is `SHA-256(prevHash | id | timestamp | fields)` (lines 232-237)
- Genesis hash: 64 zero characters (line 105)
- No delete or update methods exist on `AuditLog` -- only `Record` and `Query`/`Events`
- SQLite persistence via `SaveAuditEvent` (file: `/tmp/agentauth-develop/internal/store/sql_store.go`, lines 340-371)
- No DELETE or UPDATE SQL statements exist for the audit_events table

The `AuditLog` struct is append-only by design. The hash chain provides tamper evidence. SQLite persistence ensures durability across restarts.

**Verdict: COMPLIANT**

### Requirement 5.2: PII sanitization

**Pattern quote:** (Not explicitly in core requirements, but the code implements it)

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/audit/audit_log.go`, lines 241-266
- `sanitizePII` masks values associated with keywords: "secret", "password", "token_value", "private_key"

**Verdict: COMPLIANT** (exceeds pattern requirements)

### Requirement 5.3: Structured log schema with agent_id, task_id, orch_id, action, outcome

**Pattern quote:** Log schema with timestamp, agent_id, task_id, orchestration_id, action, resource, outcome

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/audit/audit_log.go`, lines 59-69:

```go
type AuditEvent struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    EventType string    `json:"event_type"`
    AgentID   string    `json:"agent_id,omitempty"`
    TaskID    string    `json:"task_id,omitempty"`
    OrchID    string    `json:"orch_id,omitempty"`
    Detail    string    `json:"detail"`
    Hash      string    `json:"hash"`
    PrevHash  string    `json:"prev_hash"`
}
```

The schema covers: timestamp, agent_id, task_id, orch_id, event_type (action), and detail (which captures resource and outcome in free-form text). The `Hash` and `PrevHash` fields provide the tamper-evident chain. The pattern's `bytes_transferred` and separate `resource`/`outcome` fields are embedded in the `Detail` string rather than as separate structured fields.

**Verdict: COMPLIANT** -- All required data is captured. The schema structure differs slightly from the pattern's example (some fields in Detail rather than as separate columns) but the information is present and queryable.

### Requirement 5.4: Query API for forensics and compliance

**Pattern quote:** "Query API for forensics and compliance"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/handler/audit_hdl.go` -- `GET /v1/audit/events`
- Supports filtering by: agent_id, task_id, event_type, since, until
- Supports pagination: limit (default 100, max 1000) and offset
- Protected by admin scope: `admin:audit:*`
- SQLite-backed query: `/tmp/agentauth-develop/internal/store/sql_store.go`, lines 421-512

**Verdict: COMPLIANT**

### Requirement 5.5: Complete logging (all actions, success and failure)

**Pattern quote:** "Complete (all actions, success and failure)"

**Code evidence:**
- Comprehensive event type constants in `/tmp/agentauth-develop/internal/audit/audit_log.go`, lines 29-53:
  - Admin auth success and failure
  - Launch token issued and denied
  - Sidecar activation issued, success, failure
  - Agent registered
  - Registration policy violation
  - Token issued, revoked, renewed, renewal failed
  - Delegation created, delegation attenuation violation
  - Resource accessed
  - Token auth failed, token revoked access
  - Scope violation, scope ceiling exceeded
  - Scope ceiling updated

Both success and failure paths are logged across all operations.

**Verdict: COMPLIANT**

---

## Component 6: Agent-to-Agent Mutual Authentication

### Requirement 6.1: Both agents present and validate credentials

**Pattern quote:** "When agents communicate, both verify each other's identity and authorization."

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/mutauth/mut_auth_hdl.go`
- Three-step handshake protocol:
  1. `InitiateHandshake` (lines 65-96): Initiator's token is verified, both agents confirmed registered
  2. `RespondToHandshake` (lines 100-162): Responder verifies initiator's token, identity match check, signs initiator's nonce, provides counter-nonce
  3. `CompleteHandshake` (lines 166-193): Initiator verifies responder's token, looks up responder's public key, verifies nonce signature

**Security properties verified in code:**
- Initiator identity match: `initClaims.Sub != req.InitiatorID` -> `ErrInitiatorMismatch` (line 108)
- Peer mismatch prevention: `respClaims.Sub != req.TargetAgentID` -> `ErrPeerMismatch` (line 131)
- Responder identity match: `claims.Sub != resp.ResponderID` -> `ErrResponderMismatch` (line 172)
- Both agents must be registered (store lookups at lines 72, 77, 114, 125, 178)
- Nonce signature verification with stored Ed25519 public key (line 184)

**Verdict: COMPLIANT**

### Requirement 6.2: Discovery registry for agent endpoint resolution

**Pattern quote:** (Supporting infrastructure for mutual auth)

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/mutauth/discovery.go` -- `DiscoveryRegistry`
- Bind/Resolve/Unbind/VerifyBinding operations
- Integrated into handshake as optional verification (lines 138-144 of `mut_auth_hdl.go`)

**Verdict: COMPLIANT**

### Requirement 6.3: Heartbeat-based liveness monitoring

**Pattern quote:** (Supporting infrastructure for agent lifecycle)

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/mutauth/heartbeat.go` -- `HeartbeatMgr`
- Background sweep goroutine monitors missed heartbeats
- Auto-revocation after configurable miss threshold (default 3 misses at 30s interval)
- Integration with `revoke.RevSvc` for automatic credential invalidation

**Verdict: COMPLIANT**

---

## Component 7: Delegation Chain Verification

### Requirement 7.1: Cryptographic lineage -- each delegation step creates a signed record

**Pattern quote:** "Each delegation step creates a signed, append-only record linking to the previous step"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/deleg/deleg_svc.go`, lines 124-143
- New delegation record is created with agent, scope, timestamp
- Record is signed with broker's Ed25519 key: `signRecord` (lines 183-188)
- Signature covers canonical content: `agent|scope_csv|timestamp_rfc3339`

```go
newRecord := token.DelegRecord{
    Agent:       delegatorClaims.Sub,
    Scope:       delegatorClaims.Scope,
    DelegatedAt: time.Now().UTC(),
}
newRecord.Signature = s.signRecord(newRecord)
chain = append(chain, newRecord)
```

**Verdict: COMPLIANT**

### Requirement 7.2: Scope attenuation -- permissions can ONLY narrow

**Pattern quote:** "Permissions can ONLY be narrowed at each delegation hop, never expanded"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/deleg/deleg_svc.go`, lines 107-116:

```go
if !authz.ScopeIsSubset(req.Scope, delegatorClaims.Scope) {
    // ... audit event, return ErrScopeViolation
}
```

- `ScopeIsSubset` in `/tmp/agentauth-develop/internal/authz/scope.go` enforces that every requested scope is covered by an allowed scope, with wildcard support. This is a strict attenuation check.
- Violations are audited as `EventDelegationAttenuationViolation`

**Verdict: COMPLIANT**

### Requirement 7.3: Verifiable chain with hash linking

**Pattern quote:** "Any verifier can trace the complete authorization path back to the original principal"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/deleg/deleg_svc.go`, lines 139-143
- `computeChainHash` (lines 204-210): SHA-256 of JSON-serialized delegation chain

```go
chainHash, err := computeChainHash(chain)
```

- Hash stored in token claims as `chain_hash`
- File: `/tmp/agentauth-develop/internal/token/tkn_claims.go`, line 44: `ChainHash string`
- Delegation chain stored in token claims: `DelegChain []DelegRecord`

**Verdict: COMPLIANT**

### Requirement 7.4: Maximum delegation depth limit (recommended: 5 hops)

**Pattern quote:** "Set maximum delegation depth limits (recommended: 5 hops)"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/deleg/deleg_svc.go`, line 32:

```go
const maxDelegDepth = 5
```

- Enforced at lines 102-104:

```go
currentDepth := len(delegatorClaims.DelegChain)
if currentDepth >= maxDelegDepth {
    return nil, ErrDepthExceeded
}
```

Exactly matches the pattern's recommendation.

**Verdict: COMPLIANT**

### Requirement 7.5: Chain-level revocation

**Pattern quote:** "Revoke all downstream delegated credentials"

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/revoke/rev_svc.go`, lines 74-79:

```go
if len(claims.DelegChain) > 0 {
    if s.chains[claims.DelegChain[0].Agent] {
        return true
    }
}
```

Revoking the root delegator's agent ID invalidates all tokens in that delegation lineage.

**Verdict: COMPLIANT**

---

## Additional Security Properties

### Admin Authentication

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/admin/admin_svc.go`, lines 153-165
- Constant-time comparison: `subtle.ConstantTimeCompare(secretBytes, s.adminSecret)` prevents timing attacks
- Failed auth attempts are logged to audit trail
- Admin JWT is short-lived: 300 seconds (5 minutes)
- File: `/tmp/agentauth-develop/cmd/broker/main.go`, lines 55-58 -- broker refuses to start without AA_ADMIN_SECRET

**Verdict: COMPLIANT** (defense in depth)

### Rate Limiting

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/authz/rate_mw.go` -- per-IP token bucket rate limiter
- Returns 429 with Retry-After header when exceeded
- Applied to admin auth endpoint (noted in admin handler registration)

**Verdict: COMPLIANT** (defense in depth)

### Nonce Management

**Code evidence:**
- File: `/tmp/agentauth-develop/internal/store/sql_store.go`, lines 113-150
- 32-byte cryptographic nonces (64 hex chars)
- 30-second TTL
- One-time use enforcement (consumed flag)
- Prevents replay attacks on the challenge-response registration flow

**Verdict: COMPLIANT**

### Request Body Size Limits

**Code evidence:**
- File: `/tmp/agentauth-develop/cmd/broker/main.go` -- `problemdetails.MaxBytesBody()` wraps POST endpoints
- Prevents resource exhaustion attacks

**Verdict: COMPLIANT** (defense in depth)

---

## Compliance Summary Table

| # | Component / Requirement | Verdict | Evidence Location |
|---|------------------------|---------|-------------------|
| 1.1 | Unique cryptographic identity per agent | COMPLIANT | `identity/id_svc.go:178-184` |
| 1.2 | SPIFFE ID format with orchestration context | COMPLIANT | `identity/spiffe.go:17-28` |
| 1.3 | Cryptographically bound to agent | COMPLIANT | `identity/id_svc.go:142-168` |
| 1.4 | Bootstrap problem addressed | COMPLIANT | `admin/admin_svc.go:187-257` |
| 2.1 | JWT tokens with narrow task-scoped scope | COMPLIANT | `token/tkn_claims.go:30-45` |
| 2.2 | Ed25519/EdDSA signatures | COMPLIANT | `token/tkn_svc.go:194-213` |
| 2.3 | Token TTL (5 min default, 1-60 range) | COMPLIANT | `cfg/cfg.go:44`, `token_exchange_hdl.go:21` |
| 2.4 | Full token validation (sig, exp, scope, JTI) | COMPLIANT | `token/tkn_svc.go:130-163`, `authz/val_mw.go:64-103` |
| 2.5 | Scope attenuation at registration | COMPLIANT | `identity/id_svc.go:123-134` |
| 3.1 | Independent auth on every request | COMPLIANT | `authz/val_mw.go:64-103`, `cmd/broker/main.go:151-161` |
| 3.2 | Mutual TLS (mTLS) | PARTIALLY COMPLIANT | `cmd/broker/main.go:174` -- plain HTTP only |
| 3.3 | No network-location trust | COMPLIANT | No IP-based trust in any handler |
| 4.1 | Time-based token expiration | COMPLIANT | `token/tkn_claims.go:72-73` |
| 4.2 | Active revocation list on every validation | COMPLIANT | `authz/val_mw.go:93-98` |
| 4.3 | Four-level revocation (token/agent/task/chain) | COMPLIANT | `revoke/rev_svc.go:30-112` |
| 4.4 | Task-based expiration signal | PARTIALLY COMPLIANT | No explicit task-complete endpoint |
| 4.5 | Anomaly-based revocation (optional) | PARTIALLY COMPLIANT | Heartbeat auto-revoke only; no behavioral anomaly detection |
| 5.1 | Tamper-proof append-only audit log | COMPLIANT | `audit/audit_log.go:89-167` |
| 5.2 | PII sanitization | COMPLIANT | `audit/audit_log.go:241-266` |
| 5.3 | Structured log schema | COMPLIANT | `audit/audit_log.go:59-69` |
| 5.4 | Query API for forensics | COMPLIANT | `handler/audit_hdl.go`, `store/sql_store.go:421-512` |
| 5.5 | Complete logging (success + failure) | COMPLIANT | `audit/audit_log.go:29-53` |
| 6.1 | Mutual authentication handshake | COMPLIANT | `mutauth/mut_auth_hdl.go:65-193` |
| 6.2 | Discovery registry | COMPLIANT | `mutauth/discovery.go` |
| 6.3 | Heartbeat liveness monitoring | COMPLIANT | `mutauth/heartbeat.go` |
| 7.1 | Signed delegation records | COMPLIANT | `deleg/deleg_svc.go:124-143` |
| 7.2 | Scope attenuation (narrow only) | COMPLIANT | `deleg/deleg_svc.go:107-116` |
| 7.3 | Chain hash verification | COMPLIANT | `deleg/deleg_svc.go:139-143, 204-210` |
| 7.4 | Max delegation depth (5 hops) | COMPLIANT | `deleg/deleg_svc.go:32` |
| 7.5 | Chain-level revocation | COMPLIANT | `revoke/rev_svc.go:74-79` |

---

## Overall Assessment

| Category | Count |
|----------|-------|
| COMPLIANT | 26 |
| PARTIALLY COMPLIANT | 3 |
| NOT COMPLIANT | 0 |

**Overall Verdict: COMPLIANT with minor gaps**

The AgentAuth codebase on the develop branch demonstrates strong compliance with the security pattern. All seven core components are implemented with working code. The three "partially compliant" items are:

1. **mTLS (3.2):** The broker serves plain HTTP, relying on infrastructure-level TLS termination. This is a common deployment model but means mTLS (mutual client certificate verification) is not implemented in application code. This is the most significant gap.

2. **Task-based expiration signal (4.4):** No explicit "task complete" endpoint exists. Agents stop renewing tokens when done, and heartbeat monitoring catches unresponsive agents. This is a reasonable operational model but does not provide the immediate credential termination the pattern describes.

3. **Anomaly detection (4.5):** The pattern marks this as "Optional but Recommended." Heartbeat-based auto-revocation provides basic liveness monitoring, but broader behavioral anomaly detection is not present.

None of these gaps represent security vulnerabilities in the current implementation. The code demonstrates defense-in-depth through multiple layers: cryptographic challenge-response registration, Ed25519 JWT signing, four-level revocation, scope attenuation at every boundary, hash-chained audit logs, mutual authentication handshakes, signed delegation chains, rate limiting, constant-time secret comparison, and request body size limits.

---

**Signed:** Reviewer Juliet
**Date:** 2026-02-20
**Review Round:** 2
