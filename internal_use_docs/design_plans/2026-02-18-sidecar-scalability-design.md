# Sidecar Scalability Design — Developer Self-Service + Multi-Tenant Sidecar

**Date:** 2026-02-18
**Status:** Draft
**Author:** Divine + Claude
**Pattern Reference:** `docs/security-pattern-reference.md`
**Related Plans:** `2026-02-14-sidecar-developer-experience-design.md`, `2026-02-14-sidecar-implementation-plan.md`

---

## Problem

The current sidecar model requires **one sidecar per application**, each manually configured by an operator with:

1. `AA_ADMIN_SECRET` — the broker's admin secret (leaked to every deployment)
2. `AA_SIDECAR_SCOPE_CEILING` — comma-separated scope ceiling

### Why This Doesn't Scale

| Concern | Impact |
|---------|--------|
| **1:1 sidecar-to-app ratio** | 10 apps = 10 sidecars, each a separate process with its own port, health checks, and resource overhead |
| **Operator bottleneck** | Every new app requires operator intervention to configure and deploy a sidecar |
| **Secret sprawl** | `AA_ADMIN_SECRET` (the master admin credential) is distributed to every sidecar instance |
| **No staggered onboarding** | Adding app #10 at month 6 requires the same manual ceremony as app #1 at day 0 |
| **No self-service** | Developers cannot register their own apps — they must file a ticket and wait for an operator |

### Current Bootstrap Flow (What We're Fixing)

```
Operator                        Sidecar                      Broker
   │                               │                            │
   ├── deploy sidecar with ────────▶                            │
   │   AA_ADMIN_SECRET +           │                            │
   │   AA_SIDECAR_SCOPE_CEILING    │                            │
   │                               ├── POST /v1/admin/auth ────▶│
   │                               │   (uses AA_ADMIN_SECRET)   │
   │                               │◄── admin JWT ──────────────┤
   │                               │                            │
   │                               ├── POST /v1/admin/          │
   │                               │   sidecar-activations ────▶│
   │                               │   (uses admin JWT)         │
   │                               │◄── activation token ───────┤
   │                               │                            │
   │                               ├── POST /v1/sidecar/        │
   │                               │   activate ───────────────▶│
   │                               │◄── sidecar bearer token ──┤
   │                               │                            │
```

Every step above repeats for **every app**. The admin secret is the same everywhere.

---

## Decision

Implement a **three-tier approach** that progressively reduces operator involvement:

1. **Developer Self-Service Registration** (broker API) — developers register apps and receive per-app credentials
2. **Multi-Tenant Sidecar** — a single sidecar serves multiple apps, each with their own scope ceiling
3. **Embedded Go SDK** (optional) — for advanced teams that want to skip the sidecar entirely

### Design Principles

- **Narrow-only**: no change can expand scopes beyond what the operator initially approved
- **Admin secret stays with operators**: developers never see `AA_ADMIN_SECRET`
- **Backward compatible**: existing single-tenant sidecar continues to work unchanged
- **Audit everything**: every app registration, scope change, and activation is audit-logged

---

## Approach 1: Developer Self-Service Registration API

### New Broker Endpoints

#### `POST /v1/apps` — Register an Application
**Auth:** Admin JWT (operator-only)
**Purpose:** Operator pre-registers an app with a scope ceiling and receives per-app credentials.

```json
// Request
{
  "app_name": "data-pipeline",
  "description": "ETL pipeline for customer data",
  "scope_ceiling": ["read:data:*", "write:pipeline:*"],
  "max_ttl": 600,
  "contact": "team-data@company.com"
}

// Response
{
  "app_id": "app_01HZX...",
  "app_secret": "as_live_7f3a...",      // ← per-app secret, NOT the admin secret
  "scope_ceiling": ["read:data:*", "write:pipeline:*"],
  "created_at": "2026-02-18T10:00:00Z"
}
```

The operator gives `app_id` + `app_secret` to the developer. These credentials:
- Can only bootstrap a sidecar scoped to the registered ceiling
- Cannot create launch tokens, revoke other apps, or access admin endpoints
- Are revocable per-app without affecting other apps

#### `POST /v1/apps/{id}/activate` — Activate a Sidecar for an App
**Auth:** App secret (via `X-App-Secret` header)
**Purpose:** Sidecar uses per-app credentials instead of admin secret.

```json
// Request
{
  "app_id": "app_01HZX...",
  "app_secret": "as_live_7f3a..."
}

// Response (same as current /v1/sidecar/activate)
{
  "access_token": "eyJ...",
  "expires_in": 900,
  "token_type": "Bearer",
  "sidecar_id": "sc_01HZX..."
}
```

#### `PUT /v1/apps/{id}/scope` — Update Scope Ceiling
**Auth:** Admin JWT (operator-only)
**Purpose:** Narrow or widen an app's ceiling (narrow triggers automatic token revocation).

#### `DELETE /v1/apps/{id}` — Decommission an App
**Auth:** Admin JWT (operator-only)
**Purpose:** Revoke app credentials and all associated tokens.

#### `GET /v1/apps` — List Registered Apps
**Auth:** Admin JWT (operator-only)
**Purpose:** Operator dashboard for visibility.

### New Sidecar Config (replaces AA_ADMIN_SECRET for developer use)

```bash
# Developer-facing config (new)
AA_APP_ID=app_01HZX...
AA_APP_SECRET=as_live_7f3a...

# Operator-facing config (existing, still works)
AA_ADMIN_SECRET=supersecret
AA_SIDECAR_SCOPE_CEILING=read:data:*,write:pipeline:*
```

The sidecar detects which credentials are present and chooses the appropriate bootstrap flow:
- If `AA_APP_ID` + `AA_APP_SECRET` → uses new `POST /v1/apps/{id}/activate`
- If `AA_ADMIN_SECRET` + `AA_SIDECAR_SCOPE_CEILING` → uses existing admin bootstrap (backward compatible)

### New Bootstrap Flow (What Developers See)

```
Operator                Developer               Sidecar              Broker
   │                       │                       │                    │
   ├── POST /v1/apps ─────────────────────────────────────────────────▶│
   │◄── app_id + app_secret ──────────────────────────────────────────┤
   │                       │                       │                    │
   ├── give app_id + ─────▶│                       │                    │
   │   app_secret          │                       │                    │
   │                       ├── deploy sidecar ────▶│                    │
   │                       │   with AA_APP_ID +    │                    │
   │                       │   AA_APP_SECRET       │                    │
   │                       │                       ├── POST /v1/apps/  │
   │                       │                       │   {id}/activate ─▶│
   │                       │                       │◄── sidecar token ─┤
   │                       │                       │                    │
```

The developer never touches the admin secret. The sidecar scope ceiling is embedded in the app registration, not in env vars.

---

## Approach 2: Multi-Tenant Sidecar

### Problem with Single-Tenant

Each sidecar process has **one** `AA_SIDECAR_SCOPE_CEILING`. Serving multiple apps with different scope needs requires multiple sidecar processes.

### Multi-Tenant Solution

A single sidecar instance can serve multiple registered apps. Instead of a single scope ceiling, the sidecar holds a **routing table** mapping `app_id → scope_ceiling`.

#### Config

```bash
# Multi-tenant mode
AA_SIDECAR_MODE=multi-tenant
AA_ADMIN_SECRET=supersecret        # Still operator-only for management
AA_SIDECAR_PORT=8081
```

Or, the sidecar can pull the app registry from the broker at startup:

```bash
AA_SIDECAR_MODE=multi-tenant
AA_SIDECAR_SYNC_INTERVAL=30       # Pull registered apps every 30s
```

#### Token Requests Include app_id

```python
# Developer code (multi-tenant sidecar)
resp = requests.post("http://localhost:8081/v1/token", json={
    "app_id": "app_01HZX...",        # ← identifies which app's ceiling to use
    "app_secret": "as_live_7f3a...", # ← authenticates the app
    "agent_name": "data-reader",
    "task_id": "task-789",
    "scope": ["read:data:*"]
})
```

The sidecar validates:
1. `app_id` exists in the routing table
2. `app_secret` matches
3. Requested `scope` is a subset of the app's registered `scope_ceiling`
4. Proceeds with token exchange using the app's specific sidecar token

#### Architecture

```
┌──────────────┐
│  App A       │──┐
│  (app_id: a) │  │
└──────────────┘  │   ┌─────────────────────┐    ┌──────────┐
                  ├──▶│  Multi-Tenant        │───▶│  Broker  │
┌──────────────┐  │   │  Sidecar             │    │  :8080   │
│  App B       │──┤   │  :8081               │    └──────────┘
│  (app_id: b) │  │   │                      │
└──────────────┘  │   │  Routing table:      │
                  │   │  app_a → ceiling_a   │
┌──────────────┐  │   │  app_b → ceiling_b   │
│  App C       │──┘   │  app_c → ceiling_c   │
│  (app_id: c) │      └─────────────────────┘
└──────────────┘
```

### Benefits

| Before | After |
|--------|-------|
| 10 apps = 10 sidecars | 10 apps = 1 sidecar |
| 10 × `AA_ADMIN_SECRET` | 0 × `AA_ADMIN_SECRET` for developers |
| Operator deploys each | Operator registers apps via API; developer deploys once |
| No visibility | `GET /v1/apps` shows all registered apps |

---

## Approach 3: Embedded Go SDK (Optional, Phase 2)

For teams that don't want a sidecar at all, provide a Go library that handles the full bootstrap in-process:

```go
import "github.com/divineartis/agentauth/sdk"

client, err := sdk.NewClient(sdk.Config{
    BrokerURL: "http://broker:8080",
    AppID:     os.Getenv("AA_APP_ID"),
    AppSecret: os.Getenv("AA_APP_SECRET"),
})
// client auto-bootstraps, caches tokens, handles renewal + circuit breaking

token, err := client.GetToken(ctx, sdk.TokenRequest{
    AgentName: "data-reader",
    TaskID:    "task-789",
    Scope:     []string{"read:data:*"},
})
```

This is purely additive — the sidecar remains the recommended default for non-Go apps.

---

## Implementation Plan

### Phase 1: App Registration API (Broker-Side)

**Files to create/modify:**
- `internal/app/` (new package) — `AppSvc`, `AppHdl`, app store operations
- `internal/store/sql_store.go` — add app registration maps
- `internal/admin/admin_hdl.go` — wire new routes
- `cmd/broker/main.go` — initialize `AppSvc`

**New types:**
```go
// internal/app/app_svc.go
type AppRecord struct {
    AppID        string
    AppName      string
    Description  string
    AppSecretHash []byte    // bcrypt or argon2
    ScopeCeiling []string
    MaxTTL       int
    Contact      string
    CreatedAt    time.Time
    CreatedBy    string
    Active       bool
}

type AppSvc struct {
    store    *store.SqlStore
    tknSvc   *token.TknSvc
    auditLog *audit.AuditLog
}
```

**Endpoints:**
| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| `POST` | `/v1/apps` | Admin JWT | Register app |
| `GET` | `/v1/apps` | Admin JWT | List apps |
| `GET` | `/v1/apps/{id}` | Admin JWT | Get app details |
| `PUT` | `/v1/apps/{id}/scope` | Admin JWT | Update ceiling |
| `DELETE` | `/v1/apps/{id}` | Admin JWT | Decommission |
| `POST` | `/v1/apps/{id}/activate` | App secret | Bootstrap sidecar |

### Phase 2: Sidecar Bootstrap Refactor

**Files to modify:**
- `cmd/sidecar/config.go` — add `AA_APP_ID`, `AA_APP_SECRET` detection
- `cmd/sidecar/bootstrap.go` — add `bootstrapWithAppCredentials()` path
- `cmd/sidecar/client.go` — add `appActivate()` broker call

**Logic:**
```go
func bootstrap(bc *brokerClient, cfg sidecarConfig) (*sidecarState, error) {
    if cfg.AppID != "" && cfg.AppSecret != "" {
        return bootstrapWithAppCredentials(bc, cfg)  // new path
    }
    return bootstrapWithAdminSecret(bc, cfg)         // existing path (renamed)
}
```

### Phase 3: Multi-Tenant Sidecar

**Files to create/modify:**
- `cmd/sidecar/routing.go` (new) — app routing table, sync from broker
- `cmd/sidecar/handler.go` — add `app_id` field to `tokenReq`, route through table
- `cmd/sidecar/config.go` — add `AA_SIDECAR_MODE` env var

**Data structures:**
```go
type appRoute struct {
    AppID        string
    ScopeCeiling []string
    SidecarToken string    // per-app sidecar token from broker
    ExpiresAt    time.Time
}

type routingTable struct {
    mu     sync.RWMutex
    routes map[string]*appRoute
}
```

### Phase 4: Go SDK (Optional)

**Files to create:**
- `sdk/client.go` — main client with auto-bootstrap
- `sdk/config.go` — configuration
- `sdk/token.go` — token cache and renewal

### Phase 5: Testing + Documentation

- Unit tests for `internal/app/` (100% coverage target)
- Integration tests: multi-tenant sidecar serving 3 apps simultaneously
- Live test: end-to-end developer self-service flow
- Update `docs/api.md`, `docs/getting-started-developer.md`, `docs/getting-started-operator.md`
- Update `docs/architecture.md` with multi-tenant diagrams
- ADR for the self-service decision

---

## Security Analysis

### Threat: App Secret Compromise

**Before:** Leaked `AA_ADMIN_SECRET` = full broker admin access (all apps compromised).
**After:** Leaked `app_secret` = one app compromised. Operator revokes via `DELETE /v1/apps/{id}`.

### Threat: Scope Escalation via Multi-Tenant Routing

**Mitigation:** The broker enforces scope attenuation. Even if a developer passes `scope: ["admin:*"]`, the broker rejects it because `admin:*` is not in the app's registered `scope_ceiling`.

### Threat: App Registration Spam

**Mitigation:** `POST /v1/apps` requires admin JWT. Developers cannot self-register — operators pre-register apps and hand out credentials.

### Compliance Impact

| Framework | Mapping |
|-----------|---------|
| NIST 800-207 ZTA | Per-app identity improves Policy Decision Point (PDP) granularity |
| NIST AI RMF MAP-1.5 | Provenance tracking per application, not per sidecar |
| OWASP Agentic Top 10 A01 | Eliminates admin secret sprawl |
| SOC 2 CC6.1 | Logical access controls per application boundary |

---

## Migration Path

### Existing Single-Tenant Deployments

**No changes required.** The existing `AA_ADMIN_SECRET` + `AA_SIDECAR_SCOPE_CEILING` flow continues to work. Multi-tenant is opt-in.

### Gradual Migration

1. Operator registers existing apps via `POST /v1/apps`
2. Operator distributes `app_id` + `app_secret` to teams
3. Teams update sidecar config from `AA_ADMIN_SECRET` → `AA_APP_ID` + `AA_APP_SECRET`
4. Once all apps migrate, operator can optionally consolidate to multi-tenant sidecar

---

## Open Questions

1. **App secret rotation**: Should we support rotating `app_secret` without downtime? (Likely yes — dual-secret window during rotation.)
2. **Rate limits per app**: Should the multi-tenant sidecar enforce per-app rate limits, or rely on the broker's existing rate limiting?
3. **SDK language support**: Start with Go only, or also provide Python/TypeScript SDKs?
4. **App grouping**: Should apps be groupable (e.g., by team or environment) for bulk scope management?

---

## Timeline Estimate

| Phase | Effort | Depends On |
|-------|--------|------------|
| Phase 1: App Registration API | 2-3 weeks | — |
| Phase 2: Sidecar Bootstrap Refactor | 1 week | Phase 1 |
| Phase 3: Multi-Tenant Sidecar | 2-3 weeks | Phase 2 |
| Phase 4: Go SDK | 2 weeks | Phase 1 |
| Phase 5: Testing + Docs | 1-2 weeks | All phases |

**Total: 8-12 weeks**
