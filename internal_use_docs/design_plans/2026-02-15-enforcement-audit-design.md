# Enforcement Layer & Audit Gap Fix — Design Document

**Date:** 2026-02-15
**Status:** Approved
**Author:** Divine + Dai
**Scope:** Broker audit middleware, sidecar audit logging, developer enforcement SDK, framework middleware

---

## 1. Problem Statement

AgentAuth has three enforcement gaps:

1. **Audit trail blind spots.** The `ValMw` middleware has an `AuditRecorder` interface wired in (`internal/authz/val_mw.go:46`) but never calls it. Token verification failures, revoked token usage, and scope check failures produce 401/403 HTTP responses but write nothing to the immutable audit trail. This violates Pattern Component 5 (Immutable Audit Logging) and fails QA-302 ("All Actions Logged — Success and Failure").

2. **No developer-side enforcement guidance.** The developer docs show how to validate a token (`POST /v1/token/validate`) but provide zero guidance on how to check scopes in resource server code. Developers have no copy-paste pattern for "validate token → check scope → act or deny."

3. **No enforcement SDK.** Each developer team must reimplement scope matching logic independently. This leads to inconsistent enforcement, missed checks, and resource server denials that are invisible to the audit trail.

---

## 2. Design Overview

Three deliverables, shipped in order:

| Deliverable | What It Is | Ships When |
|-------------|-----------|------------|
| **Audit Gap Fix** | Add `Record()` calls in middleware denial paths | Immediate (broker code change) |
| **Interim Developer Docs** | "Always validate, always check scope" pattern with code examples | Immediate (docs change) |
| **Enforcement SDK (Layer 1)** | Python + Go library: `validate_token()`, `check_scope()`, `report_event()` | Next module |
| **Framework Middleware (Layer 2)** | FastAPI, Flask, net/http decorators built on the SDK | After SDK ships |

---

## 3. Audit Gap Fix

### 3.1 Current State

The `AuditRecorder` interface exists on `ValMw` but is never called:

```go
// internal/authz/val_mw.go
type ValMw struct {
    tknSvc   TokenVerifier
    revSvc   RevocationChecker
    auditLog AuditRecorder       // ← wired in, never used
}
```

Denial paths at lines 67, 72, 79, 84, and 106 return HTTP errors but produce no audit events.

### 3.2 New Audit Event Types

Add these constants to `internal/audit/audit_log.go`:

| Event Type | Trigger | Key Fields |
|------------|---------|------------|
| `token_auth_failed` | Bad signature, expired, malformed JWT | detail (error), request path |
| `token_revoked_access` | Revoked token presented on any endpoint | agent_id, task_id, orch_id, request path |
| `scope_violation` | Token lacks required scope for endpoint | agent_id, task_id, required scope, actual scope |
| `scope_ceiling_exceeded` | Sidecar: requested scope > configured ceiling | agent_name, requested scopes, ceiling scopes |
| `delegation_attenuation_violation` | Delegation attempted to widen scope | delegator agent_id, target agent, requested vs allowed |

### 3.3 Changes Required

**File: `internal/authz/val_mw.go` — `Wrap()` method**

Add `Record()` calls on each denial path:

- **Line 67 (missing auth header):** `m.auditLog.Record(EventTokenAuthFailed, "", "", "", "missing authorization header | path="+r.URL.Path)`
- **Line 72 (invalid scheme):** `m.auditLog.Record(EventTokenAuthFailed, "", "", "", "invalid authorization scheme | path="+r.URL.Path)`
- **Line 79 (verification failed):** `m.auditLog.Record(EventTokenAuthFailed, "", "", "", "token verification failed: "+err.Error()+" | path="+r.URL.Path)`
- **Line 84 (revoked token):** `m.auditLog.Record(EventTokenRevokedAccess, claims.Sub, claims.TaskId, claims.OrchId, "revoked token used | path="+r.URL.Path)`

**File: `internal/authz/val_mw.go` — `WithRequiredScope()` function**

This function is currently standalone (no receiver, no access to `auditLog`). Two options:

- **Option A:** Make it a method on `ValMw` so it can access `m.auditLog`
- **Option B:** Accept `AuditRecorder` as a parameter

Option A is cleaner. Change `WithRequiredScope` to `ValMw.RequireScope()`:

```go
func (m *ValMw) RequireScope(scope string, next http.Handler) http.Handler {
    // ... existing logic ...
    // On denial:
    if m.auditLog != nil {
        m.auditLog.Record(EventScopeViolation, claims.Sub, claims.TaskId, claims.OrchId,
            "scope_violation | required="+scope+" | actual="+strings.Join(claims.Scope, ",")+" | path="+r.URL.Path)
    }
}
```

**File: `cmd/sidecar/handler.go` — scope ceiling check (line 78-82)**

The sidecar already logs via `obs.Warn` and increments `RecordScopeDenial()`. Add a call to report the event to the broker's audit trail. This requires a new broker endpoint (see Section 5.3).

**File: `internal/deleg/deleg_svc.go` — scope attenuation check**

Add audit recording when delegation attempts to widen scope (around line 107-110).

### 3.4 Nil Safety

All `Record()` calls must guard against nil `auditLog`:

```go
if m.auditLog != nil {
    m.auditLog.Record(...)
}
```

This preserves backward compatibility when `auditLog` is not provided (e.g., in tests or minimal deployments).

### 3.5 Key Property

After this fix, **every 401 and 403 response produces an audit event** with the agent's identity (when available from token claims). This enables operators to detect compromise patterns — an agent rapidly accumulating 403s is a clear signal of prompt injection or misconfiguration.

---

## 4. Interim Developer Docs

### 4.1 Location

New section in `docs/getting-started-developer.md` titled **"Enforcing Scopes in Your Resource Server"**, placed after the "Using Your Token" section.

### 4.2 The Pattern

Three steps, always in order:

1. **Validate the token** — call `POST /v1/token/validate` on the broker
2. **Check the scope** — verify the token's scopes cover the action
3. **Act or deny** — if scope doesn't cover, return 403

### 4.3 Python Example

```python
import requests
import os

BROKER = os.environ.get("AGENTAUTH_BROKER_URL", "https://agentauth.internal.company.com")

def require_scope(request, required_scope):
    """Validate token and check scope. Call this in every endpoint handler."""
    token = request.headers.get("Authorization", "").removeprefix("Bearer ")
    if not token:
        raise HTTPException(401, "missing bearer token")

    # Step 1: Validate token
    resp = requests.post(f"{BROKER}/v1/token/validate", json={"token": token})
    result = resp.json()
    if not result["valid"]:
        raise HTTPException(403, f"invalid token: {result.get('error', 'unknown')}")

    # Step 2: Check scope
    claims = result["claims"]
    if not scope_covers(claims["scope"], required_scope):
        raise HTTPException(403,
            f"scope {claims['scope']} does not cover {required_scope}")

    return claims  # Pass to handler for audit/attribution


def scope_covers(allowed_scopes, required_scope):
    """Check if any allowed scope covers the required scope.
    Uses the same action:resource:identifier matching as the broker."""
    r_parts = required_scope.split(":")
    if len(r_parts) != 3:
        return False
    for allowed in allowed_scopes:
        a_parts = allowed.split(":")
        if len(a_parts) != 3:
            continue
        if a_parts[0] == r_parts[0] and a_parts[1] == r_parts[1]:
            if a_parts[2] == "*" or a_parts[2] == r_parts[2]:
                return True
    return False
```

### 4.4 Go Example

```go
func requireScope(token string, required string) (*Claims, error) {
    // Step 1: Validate
    resp, err := http.Post(brokerURL+"/v1/token/validate",
        "application/json", tokenBody(token))
    // ... parse response ...

    // Step 2: Check scope
    if !scopeCovers(claims.Scope, required) {
        return nil, fmt.Errorf("scope %v does not cover %s", claims.Scope, required)
    }
    return claims, nil
}
```

### 4.5 Explicit Callout

The docs state:

> **This is interim guidance.** When the AgentAuth SDK ships, it replaces these manual checks with a single function call. But the principle never changes: **validate first, check scope second, act third.** Never skip the scope check — a valid token does not mean the agent is authorized for this specific action.

---

## 5. Enforcement SDK Design

### 5.1 Layer 1: Validation SDK (`agentauth-sdk`)

A lightweight library in Python and Go with three core functions.

#### Python API

```python
from agentauth import AgentAuthClient

client = AgentAuthClient(
    broker_url=os.environ["AGENTAUTH_BROKER_URL"],
    cache_ttl=30,  # cache valid tokens for 30s
)

# Validate token — returns claims dict or raises AgentAuthError
claims = client.validate_token(bearer_token)

# Check scope — returns True or raises ScopeViolationError (403)
client.check_scope(claims, "write:repo:feature-xyz")

# Report event to audit trail — sends to POST /v1/audit/report
client.report_event(
    event_type="scope_violation",
    agent_id=claims["sub"],
    task_id=claims["task_id"],
    detail="attempted write:repo:main, token scope: write:repo:feature-xyz",
)
```

#### Go API

```go
client := agentauth.NewClient(os.Getenv("AGENTAUTH_BROKER_URL"))

claims, err := client.ValidateToken(bearerToken)
if err != nil { /* 401 */ }

if err := client.CheckScope(claims, "write:repo:feature-xyz"); err != nil { /* 403 */ }

client.ReportEvent(agentauth.Event{
    Type:    "scope_violation",
    AgentID: claims.Sub,
    TaskID:  claims.TaskID,
    Detail:  "attempted write:repo:main",
})
```

#### Internals

- **`validate_token()`** calls `POST /v1/token/validate` with an LRU cache (keyed by token SHA-256, TTL-bounded). Cache miss → broker call. Cache hit → return cached claims. Revoked tokens are evicted when a validation returns invalid.

- **`check_scope()`** implements the same `action:resource:identifier` matching logic as `internal/authz/scope.go`. No network call — pure local computation. This ensures scope matching is **identical** between the broker and the SDK.

- **`report_event()`** calls `POST /v1/audit/report` (new endpoint, see below). Events are appended to the same hash-chained audit trail as broker events. The call is fire-and-forget with a bounded retry queue (don't block the request on audit reporting).

#### Token Validation Cache

```
Token arrives → SHA-256(token) → LRU lookup
  ├─ Cache hit + not expired → return cached claims (0ms)
  └─ Cache miss or expired → POST /v1/token/validate → cache result → return claims (~5ms)
```

Cache TTL defaults to 30 seconds. This means a revoked token could be accepted for up to 30 seconds after revocation. This is an acceptable trade-off: the broker's middleware catches revoked tokens on broker-side calls, and the resource server cache is bounded.

### 5.2 Layer 2: Framework Middleware

Built on top of the SDK. Ships after Layer 1.

#### Python — FastAPI

```python
from agentauth.fastapi import require_scope

@app.put("/repos/{repo}/contents/{path}")
@require_scope("write:repo:{repo}")  # {repo} extracted from path params
async def put_contents(repo: str, path: str):
    claims = request.state.agentauth_claims  # injected by middleware
    ...
```

#### Python — Flask

```python
from agentauth.flask import require_scope

@app.route("/repos/<repo>/contents/<path>", methods=["PUT"])
@require_scope("write:repo:{repo}")
def put_contents(repo, path):
    claims = g.agentauth_claims
    ...
```

#### Go — net/http

```go
mux.Handle("PUT /repos/{repo}/contents/",
    agentauth.RequireScope("write:repo:{repo}", handler))
```

#### Middleware Behavior

1. Extract Bearer token from `Authorization` header
2. Call `client.ValidateToken()` (with caching)
3. Substitute path parameters into scope template (e.g., `{repo}` → `feature-xyz`)
4. Call `client.CheckScope()` with the resolved scope
5. **On denial:** return RFC 7807 403 response AND call `client.ReportEvent()` to audit trail
6. **On success:** store claims in request context for the handler

### 5.3 New Broker Endpoint: `POST /v1/audit/report`

Closes the loop between resource servers and the broker's audit trail.

```
POST /v1/audit/report
Authorization: Bearer <token-with-audit:report:*-scope>
Content-Type: application/json

{
    "event_type": "scope_violation",
    "agent_id": "spiffe://agentauth.local/agent/orch-001/task-xyz/a1b2c3",
    "task_id": "task-xyz",
    "orch_id": "orch-001",
    "detail": "attempted write:repo:main, token scope: write:repo:feature-xyz"
}
```

**Required scope:** `audit:report:*`

This scope is automatically included in sidecar tokens. Resource servers authenticate with the sidecar to get a reporting token, or piggyback on the agent's own token if it has `audit:report:*` scope.

**Validation:**
- `event_type` must be a recognized event type string
- `agent_id` must be a valid SPIFFE ID format (if provided)
- `detail` is PII-sanitized before storage (same `sanitizePII()` function)
- Events are appended to the same hash-chained `AuditLog` as broker events

**Rate limiting:** Per-source rate limit (100 events/minute per token) to prevent audit log flooding from a compromised resource server.

### 5.4 Capability Summary

| Capability | Manual (Interim) | SDK (Layer 1) | Middleware (Layer 2) |
|------------|-----------------|---------------|---------------------|
| Token validation | Developer writes HTTP call | `client.validate_token()` with caching | Automatic |
| Scope checking | Developer implements matching | `client.check_scope()` — identical to broker | Declared on route |
| Path param substitution | Manual string formatting | Manual | Automatic (`{repo}` → value) |
| Audit reporting | Not done | `client.report_event()` | Automatic on deny |
| Framework integration | Manual per handler | Manual per handler | Decorator/wrapper |
| Can developer forget? | Yes | Yes (must call check) | No (declared on route) |

---

## 6. Implementation Sequence

| Step | Deliverable | Depends On |
|------|------------|------------|
| 1 | Add audit event type constants to `audit_log.go` | Nothing |
| 2 | Wire `Record()` calls into `ValMw.Wrap()` denial paths | Step 1 |
| 3 | Convert `WithRequiredScope()` to `ValMw.RequireScope()` method | Step 2 |
| 4 | Update `cmd/broker/main.go` route wiring for new method signature | Step 3 |
| 5 | Add audit recording to sidecar scope ceiling denial | Step 1 |
| 6 | Add audit recording to delegation attenuation denial | Step 1 |
| 7 | Write interim developer docs in `getting-started-developer.md` | Nothing |
| 8 | Implement `POST /v1/audit/report` broker endpoint | Step 1 |
| 9 | Implement Python SDK (`agentauth` package) | Step 8 |
| 10 | Implement Go SDK (`agentauth` package) | Step 8 |
| 11 | Implement FastAPI middleware | Step 9 |
| 12 | Implement Flask middleware | Step 9 |
| 13 | Implement Go net/http middleware | Step 10 |

Steps 1-7 are immediate. Steps 8-13 are next module.

---

## 7. Testing Strategy

### Audit Gap Fix

- Unit test: `ValMw.Wrap()` with a mock `AuditRecorder` — verify `Record()` called on each denial path
- Unit test: `ValMw.RequireScope()` — verify `Record()` called with correct event type and scope details
- Integration test: send bad token, revoked token, and insufficient-scope token to broker — verify audit events appear in `GET /v1/audit/events`
- QA-302 (Pattern Component 5): re-run to verify "All Actions Logged (Success and Failure)"

### SDK

- Unit test: `check_scope()` with comprehensive scope matching cases (exact, wildcard, denial)
- Unit test: `validate_token()` cache behavior (hit, miss, expiry, revocation eviction)
- Integration test: SDK against live broker — validate, check, report cycle
- Integration test: framework middleware against live broker — full request flow

### Manual Pattern Tests

Update `scripts/pattern_manual_tests.py` to verify:
- Denied requests produce audit events
- Sidecar scope ceiling violations produce audit events
- `POST /v1/audit/report` accepts and stores resource server events

---

## 8. Open Questions

1. **Should `POST /v1/audit/report` accept batch events?** A resource server handling hundreds of requests per second would benefit from batching. Trade-off: complexity vs throughput.

2. **SDK package naming.** Python: `agentauth` or `agentauth-sdk`? Go: `github.com/divineartis/agentauth/sdk` or separate repo?

3. **Should the SDK validate tokens locally (verify JWT signature) or always call the broker?** Local validation requires distributing the broker's public key to SDK instances. Faster but more complex key management.

4. **Cache invalidation on revocation.** The 30-second cache TTL means a revoked token could be accepted for up to 30 seconds by a resource server. Is this acceptable, or do we need a push-based invalidation channel (WebSocket/SSE from broker to SDKs)?
