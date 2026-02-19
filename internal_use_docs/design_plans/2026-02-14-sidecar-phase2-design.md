# Sidecar Phase 2 Design — Per-Agent Registration + Auto-Renewal

**Date:** 2026-02-14
**Status:** Approved
**Author:** Divine + Dai
**Depends on:** Phase 1 (merged to develop, commit `8ffb4bd`)

---

## Problem

Phase 1 built a working sidecar that abstracts broker complexity from developers. But it has two limitations:

1. **No auto-renewal.** The sidecar's own bearer token expires after 900s with no proactive refresh. In a long-running deployment, the sidecar silently becomes unable to exchange tokens.

2. **No per-agent identity.** All token requests go through the sidecar's single SPIFFE ID. Agents don't get their own identity — they share the sidecar's. This means no per-agent revocation, no per-agent audit trail, and no path to mutual auth (Phase 3).

---

## Decision

Add two capabilities to the sidecar:

1. **Background auto-renewal goroutine** — proactively renews the sidecar's bearer token at 80% of TTL.
2. **Per-agent registration** — lazy (on first `POST /v1/token`) with sidecar-managed keypairs by default, plus explicit BYOK via `POST /v1/register` for agents that bring their own Ed25519 keys.

Agent state is ephemeral — cleared on sidecar restart. Agents re-register transparently on next token request.

---

## Architecture

### File Changes

```
cmd/sidecar/
  main.go              ← MODIFY: wire renewal goroutine + agent registry
  config.go            ← MODIFY: add AA_SIDECAR_RENEWAL_BUFFER env var
  bootstrap.go         ← MODIFY: return token expiry for renewal scheduling
  handler.go           ← MODIFY: lazy registration before token exchange
  broker_client.go     ← MODIFY: add challenge + register broker calls
  renewal.go           ← NEW: background token renewal goroutine
  registry.go          ← NEW: in-memory agent registry (keypairs, SPIFFE IDs)
  register_handler.go  ← NEW: POST /v1/register + GET /v1/challenge endpoints
```

### New Components

**`renewal.go`** — `startRenewal(ctx, state, bc, interval)` goroutine.
- Uses `sync.RWMutex` on `sidecarState` to atomically swap tokens
- Renews at 80% of TTL by default (configurable via `AA_SIDECAR_RENEWAL_BUFFER`)
- Exponential backoff on failure: 1s, 2s, 4s, ..., 30s cap
- Sets `state.healthy = false` if token actually expires
- Auto-recovers when renewal succeeds after degraded state
- Shuts down via `ctx.Done()`

**`registry.go`** — `agentRegistry` struct.
- `sync.RWMutex` protecting `map[string]*agentEntry`
- Key: `agent_name:task_id`
- Entry: `{spiffeID, keypair (nil for BYOK), registeredAt}`
- Per-agent mutex for registration serialization (prevents duplicate challenge-response on concurrent first requests)
- Ephemeral — no persistence across restarts

**`register_handler.go`** — Two new endpoints:
- `GET /v1/challenge` — proxies broker challenge (for BYOK developers)
- `POST /v1/register` — explicit registration with developer-provided public key

---

## API Surface (Phase 2)

| Method | Path | Purpose | Change |
|--------|------|---------|--------|
| `POST` | `/v1/token` | Request scoped token (lazy-registers if needed) | Modified |
| `POST` | `/v1/token/renew` | Renew developer token | Unchanged |
| `GET` | `/v1/health` | Readiness + renewal status | Modified |
| `GET` | `/v1/challenge` | Proxy broker challenge (for BYOK) | **New** |
| `POST` | `/v1/register` | Explicit agent registration (BYOK) | **New** |

---

## Data Flow: Lazy Registration (Default Path)

When `POST /v1/token` arrives for an unregistered agent:

```
Developer                      Sidecar                              Broker
   │                              │                                    │
   │ POST /v1/token               │                                    │
   │ {agent_name: "reader",       │                                    │
   │  task_id: "t-1",             │                                    │
   │  scope: ["read:data:*"]}     │                                    │
   │─────────────────────────────>│                                    │
   │                              │                                    │
   │                    Check registry: "reader:t-1"                   │
   │                    NOT FOUND → lazy register                      │
   │                              │                                    │
   │                              │  1. Generate Ed25519 keypair       │
   │                              │                                    │
   │                              │  2. GET /v1/challenge              │
   │                              │────────────────────────────────────>│
   │                              │  {nonce}                           │
   │                              │<────────────────────────────────────│
   │                              │                                    │
   │                              │  3. Sign nonce with private key    │
   │                              │                                    │
   │                              │  4. POST /v1/register              │
   │                              │  {launch_token, public_key,        │
   │                              │   signed_nonce, agent_name}        │
   │                              │────────────────────────────────────>│
   │                              │  {agent_id (SPIFFE), token}        │
   │                              │<────────────────────────────────────│
   │                              │                                    │
   │                    Store in registry:                              │
   │                    key="reader:t-1"                                │
   │                    val={spiffe_id, keypair, registered_at}        │
   │                              │                                    │
   │                              │  5. POST /v1/token/exchange        │
   │                              │  {agent_id: spiffe_id,             │
   │                              │   scope, ttl}                      │
   │                              │────────────────────────────────────>│
   │                              │  {access_token}                    │
   │                              │<────────────────────────────────────│
   │                              │                                    │
   │ 200 {access_token,           │                                    │
   │      expires_in, scope,      │                                    │
   │      agent_id}               │                                    │
   │<─────────────────────────────│                                    │
```

Subsequent requests for the same `agent_name:task_id` skip steps 1-4.

---

## Data Flow: BYOK Registration (Explicit Path)

Developer handles their own Ed25519 signing:

```
Developer                      Sidecar                              Broker
   │                              │                                    │
   │ GET /v1/challenge            │                                    │
   │─────────────────────────────>│                                    │
   │                              │  GET /v1/challenge                 │
   │                              │────────────────────────────────────>│
   │                              │  {nonce}                           │
   │                              │<────────────────────────────────────│
   │ {nonce}                      │                                    │
   │<─────────────────────────────│                                    │
   │                              │                                    │
   │ Sign nonce with own key      │                                    │
   │                              │                                    │
   │ POST /v1/register            │                                    │
   │ {agent_name, task_id,        │                                    │
   │  public_key, signature}      │                                    │
   │─────────────────────────────>│                                    │
   │                              │  POST /v1/register                 │
   │                              │  {launch_token, public_key,        │
   │                              │   signed_nonce, agent_name}        │
   │                              │────────────────────────────────────>│
   │                              │  {agent_id}                        │
   │                              │<────────────────────────────────────│
   │                              │                                    │
   │ {agent_id, spiffe_id}        │  Store: no private key (BYOK)     │
   │<─────────────────────────────│                                    │
   │                              │                                    │
   │ POST /v1/token (as before)   │                                    │
   │─────────────────────────────>│  Uses cached spiffe_id             │
```

After BYOK registration, `POST /v1/token` works identically to the lazy path.

---

## Data Flow: Auto-Renewal

```
              Sidecar (background goroutine)              Broker
                         │                                   │
                  Start: token_expiry = 900s                 │
                  renewal_at = 720s (80%)                    │
                         │                                   │
                   sleep(720s)                               │
                         │                                   │
                         │  POST /v1/token/renew             │
                         │  Auth: Bearer <sidecar-jwt>       │
                         │───────────────────────────────────>│
                         │  {access_token: <new-jwt>,        │
                         │   expires_in: 900}                 │
                         │<───────────────────────────────────│
                         │                                   │
                   Atomic swap: state.token = new-jwt        │
                   Reset timer: sleep(720s)                  │
                         │                                   │
                   [repeat until ctx.Done()]                 │
```

On failure: exponential backoff. If token expires, `state.healthy = false`.

---

## Error Handling

### Renewal Failures
- Broker unreachable → exponential backoff (1s → 2s → 4s → ... → 30s cap)
- Token expired (all retries exhausted) → `state.healthy = false`
- Health endpoint returns `{status: "degraded", broker_connected: false}`
- Token requests return `503 Service Unavailable`
- Auto-recovers when renewal succeeds

### Lazy Registration Failures
- Challenge fetch fails → `502` with `"detail": "broker challenge unavailable"`
- Registration fails → `502` with broker error detail
- Developer sees one HTTP error, not the multi-step breakdown

### BYOK Registration Failures
- Missing fields → `400`
- Invalid public key → `400` with `"detail": "invalid public key: must be 32-byte Ed25519"`
- Challenge expired → `410 Gone` with `"detail": "challenge expired, fetch a new one"`
- Broker rejection → `502`

### Race Conditions
- Two concurrent `POST /v1/token` for same unregistered agent → per-agent mutex serializes registration. First acquires lock, second waits. Only one challenge-response runs.
- Renewal goroutine vs handler reading token → `sync.RWMutex` on `sidecarState`. Handlers take read lock, renewal takes write lock.

---

## Configuration

### New Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_SIDECAR_RENEWAL_BUFFER` | `0.8` | Fraction of TTL at which to renew (0.5-0.95) |

### Existing Variables (Unchanged)

| Variable | Default | Description |
|----------|---------|-------------|
| `AA_BROKER_URL` | `http://localhost:8080` | Broker base URL |
| `AA_ADMIN_SECRET` | (required) | Shared secret for bootstrap |
| `AA_SIDECAR_SCOPE_CEILING` | (required) | Max scopes sidecar can issue |
| `AA_SIDECAR_PORT` | `8081` | Sidecar HTTP port |

---

## Testing Strategy

### New Unit Tests

| Component | Tests | What they verify |
|-----------|-------|-----------------|
| `renewal.go` | 4 | Happy renewal, backoff on failure, recovery after degraded, context cancellation |
| `registry.go` | 5 | Store/lookup, duplicate registration, concurrent access, BYOK vs managed, ephemeral |
| `register_handler.go` | 5 | Happy BYOK, missing fields (400), invalid key (400), challenge proxy, broker rejection (502) |
| `handler.go` (modified) | 3 new | Lazy registration on first request, cached agent on second, registration failure → 502 |

### Extended Integration Test

Extend `TestIntegration_DeveloperFlow` to cover:
1. First token request triggers lazy registration (verify agent exists at broker)
2. Second token request uses cached registration (no extra broker calls)
3. BYOK flow: developer-generated keypair, explicit registration, then token request
4. Sidecar token renewal: set short TTL, wait, verify new token still works

---

## Pattern Compliance Progress

After Phase 2:
- [x] 1. Ephemeral Identity — **now per-agent SPIFFE IDs** (upgraded from shared sidecar ID)
- [x] 2. Short-Lived Scoped Tokens
- [x] 3. Zero-Trust Enforcement
- [x] 4. Expiration & Revocation — **now per-agent revocable**
- [x] 5. Audit Logging — **now per-agent audit trail** (broker logs individual agent_ids)
- [ ] 6. Mutual Auth (Phase 3)
- [ ] 7. Delegation Chains (Phase 3)

---

## Scope Boundaries

**Build:**
- Background renewal goroutine with exponential backoff
- In-memory agent registry (ephemeral)
- Lazy registration on first `POST /v1/token`
- BYOK registration via `GET /v1/challenge` + `POST /v1/register`
- Updated health endpoint with renewal status
- Unit tests for all new components
- Extended integration test

**Do NOT build:**
- Persistent agent state (file/DB)
- Multi-scope ceiling fix (separate concern)
- Structured logging migration (Phase 3+)
- Rate limiting (Phase 3+)
- Mutual auth routes (Phase 3)
- Delegation chains (Phase 3)

---

**END OF DESIGN**
