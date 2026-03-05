# TD-006: Per-App Configurable JWT TTL

**Status:** Spec
**Priority:** P1 — blocks Phase 1C (app revocation, audit, secret rotation)
**Effort estimate:** 0.5 day
**Depends on:** Phase 1b (app model, app auth, app JWT issuance)
**Architecture doc:** `../.plans/CoWork-Architecture-Direct-Broker.md`
**Tech debt:** TD-006

---

## Overview: What We're Building and Why

When an app authenticates with the broker (`POST /v1/app/auth`), the broker issues a short-lived JWT that the app uses to create launch tokens, register agents, and query audit events. Right now, that JWT always lives for exactly 5 minutes — it's a hardcoded constant (`const appTokenTTL = 300`) buried in `internal/app/app_svc.go`. The operator can't change it. Every app gets the same TTL regardless of its use case.

Five minutes is too short for machine-to-machine workflows. An ETL pipeline that authenticates, creates a launch token, registers an agent, and starts processing data can easily exceed 5 minutes before the agent is even running. The developer has to re-authenticate mid-workflow, adding complexity and failure modes that shouldn't exist.

This fix makes app JWT TTL configurable at two levels. First, the operator sets a global default via `AA_APP_TOKEN_TTL` (defaulting to 1800 seconds / 30 minutes). Second, the operator can override the TTL per-app at registration time (`aactl app register --token-ttl`) or update it later (`aactl app update --token-ttl`). The per-app value is stored in the database and used at authentication time. Safety bounds (60s–86400s) prevent misconfiguration.

**What changes:** New `token_ttl` column on the `apps` table. New `AA_APP_TOKEN_TTL` env var for the global default. `AppRecord` struct gains a `TokenTTL` field. `AuthenticateApp()` reads the per-app TTL instead of using a constant. `aactl app register` gains `--token-ttl` flag. New `aactl app update` subcommand for modifying existing apps. Existing apps migrate to 1800s default.

**What stays the same:** App authentication endpoint (`POST /v1/app/auth`) request and response shape unchanged. App registration endpoint request shape gains an optional field but existing requests without it continue to work. Admin JWT TTL unchanged. Sidecar TTL unchanged. All other token issuance paths untouched.

---

## Problem Statement

App JWT TTL is hardcoded to 300 seconds (5 minutes) via `const appTokenTTL = 300` in `internal/app/app_svc.go:24`. This value is too short for production machine-to-machine workflows and cannot be changed by the operator — not globally, not per-app. The global `AA_DEFAULT_TTL` config exists but is not used for app tokens (the constant takes precedence). There is no per-app TTL field in the `AppRecord` struct or the `apps` SQLite table.

This blocks Phase 1C, which adds app-level revocation and secret rotation — features that depend on operators having control over app token lifetimes.

---

## Goals

1. Operators can set a global default app JWT TTL via `AA_APP_TOKEN_TTL` env var (default 1800s / 30 min)
2. Operators can override TTL per-app at registration time (`aactl app register --token-ttl 3600`)
3. Operators can update an existing app's TTL (`aactl app update --token-ttl 3600 --id APP_ID`)
4. App authentication uses the per-app TTL stored in the database
5. Safety bounds (60s–86400s) prevent misconfigured TTLs
6. Existing apps automatically get the 1800s default via schema migration

---

## Non-Goals

1. **Admin JWT TTL configurability** — tracked as TD-010, future work
2. **Sidecar activation TTL configurability** — tracked as TD-010, future work
3. **Per-request TTL from the developer** — developers don't choose their own token lifetime; the operator controls it
4. **Automatic TTL adjustment** — no adaptive TTL based on usage patterns
5. **TTL in the app auth response body** — the JWT `exp` claim already communicates expiry to the developer

---

## User Stories

### Operator Stories

1. **As an operator**, I want to set a global default TTL for app JWTs so that I don't have to specify a TTL for every app I register.

2. **As an operator**, I want to set a custom TTL when registering an app so that long-running ETL apps can have 60-minute tokens while short-lived bots keep 30-minute tokens.

3. **As an operator**, I want to update an existing app's TTL without re-registering it so that I can adjust token lifetimes in production without credential rotation.

4. **As an operator**, I want the broker to reject TTL values outside safe bounds so that a typo doesn't create tokens that live for a year or expire instantly.

### Developer Stories

5. **As a developer**, I want my app's JWT to live long enough for my workflow to complete so that I don't have to re-authenticate mid-process.

### Security Stories

6. **As a security reviewer**, I want app JWT TTL bounded between 60 seconds and 24 hours so that tokens can't be configured to live indefinitely or expire uselessly fast.

7. **As a security reviewer**, I want TTL changes to be recorded in the audit trail so that I can see when and why an app's token lifetime was modified.

---

## Schema Changes

```sql
ALTER TABLE apps ADD COLUMN token_ttl INTEGER NOT NULL DEFAULT 1800;
```

**Migration notes:** Additive — SQLite `ALTER TABLE ADD COLUMN` with a `DEFAULT` value is safe. Existing rows automatically get `token_ttl = 1800`. No data backfill required. No downtime. The broker's `InitDB()` already uses `CREATE TABLE IF NOT EXISTS` with the full schema, so new deployments get the column automatically. For existing databases, the `ALTER TABLE` runs at startup if the column doesn't exist (same pattern used for other schema evolutions in the codebase).

---

## API Contract

### `POST /v1/app/auth` (unchanged request, unchanged response)

No changes to the authentication endpoint itself. The only difference is the `exp` claim inside the returned JWT will reflect the per-app TTL instead of always being `now + 300`.

### `POST /v1/admin/apps` (app registration — new optional field)

**Request (with TTL):**
```json
{
  "name": "weather-bot",
  "scopes": ["read:weather:*"],
  "token_ttl": 3600
}
```

**Request (without TTL — uses global default):**
```json
{
  "name": "weather-bot",
  "scopes": ["read:weather:*"]
}
```

**Response (201) — unchanged shape, TTL visible in app record:**
```json
{
  "app_id": "app-weather-bot-a1b2c3",
  "client_id": "wb-09ccbf99777a",
  "client_secret": "generated-secret-shown-once",
  "name": "weather-bot",
  "scopes": ["read:weather:*"],
  "token_ttl": 3600
}
```

**Error (400) — TTL out of bounds:**
```json
{
  "type": "invalid_ttl",
  "title": "Invalid token TTL",
  "detail": "token_ttl must be between 60 and 86400 seconds, got 5"
}
```

### `PUT /v1/admin/apps/{id}` (new endpoint — update app settings)

**Request:**
```json
{
  "token_ttl": 7200
}
```

**Response (200):**
```json
{
  "app_id": "app-weather-bot-a1b2c3",
  "name": "weather-bot",
  "scopes": ["read:weather:*"],
  "token_ttl": 7200,
  "status": "active",
  "updated_at": "2026-03-05T12:00:00Z"
}
```

**Error (400) — TTL out of bounds:**
```json
{
  "type": "invalid_ttl",
  "title": "Invalid token TTL",
  "detail": "token_ttl must be between 60 and 86400 seconds, got 0"
}
```

**Error (404) — app not found:**
```json
{
  "type": "app_not_found",
  "title": "App not found",
  "detail": "no app with id app-nonexistent-000000"
}
```

### `aactl app register --token-ttl`

```
$ aactl app register --name weather-bot --scopes "read:weather:*" --token-ttl 3600

App registered successfully.

  App ID:         app-weather-bot-a1b2c3
  Client ID:      wb-09ccbf99777a
  Client Secret:  sk-a1b2c3d4e5f6...
  Scopes:         [read:weather:*]
  Token TTL:      3600s (1h0m0s)

  ⚠ Save the client secret now — it cannot be retrieved later.
```

### `aactl app update --token-ttl`

```
$ aactl app update --id app-weather-bot-a1b2c3 --token-ttl 7200

App updated successfully.

  App ID:     app-weather-bot-a1b2c3
  Token TTL:  7200s (2h0m0s)
```

---

## What Needs to Be Done

### 1. Add `AA_APP_TOKEN_TTL` to Config

In `internal/cfg/cfg.go`, add an `AppTokenTTL` field to the `Cfg` struct. Load from `AA_APP_TOKEN_TTL` env var with default 1800. This is the fallback value used when an app is registered without an explicit `--token-ttl`.

### 2. Add `token_ttl` Column to Apps Table

In `internal/store/sql_store.go`, add the `token_ttl INTEGER NOT NULL DEFAULT 1800` column to the `apps` table schema. Update `AppRecord` struct with `TokenTTL int`. Update `SaveApp()`, `GetAppByClientID()`, `GetAppByID()`, and `ListApps()` to read/write the new field. Add schema migration logic for existing databases (detect missing column, run `ALTER TABLE`).

### 3. Accept TTL in App Registration

In `internal/app/app_svc.go`, update `RegisterApp()` to accept an optional TTL parameter. If provided and within bounds (60–86400), store it. If zero/omitted, use `cfg.AppTokenTTL` (the global default). Validate bounds and return `invalid_ttl` error if out of range.

### 4. Use Per-App TTL in Authentication

In `internal/app/app_svc.go`, replace `const appTokenTTL = 300` with `rec.TokenTTL` in `AuthenticateApp()`. The token service already accepts TTL via `IssueReq.TTL` — just pass the app's stored value instead of the constant.

### 5. Add `UpdateApp()` Service Method

In `internal/app/app_svc.go`, add an `UpdateApp(appID string, opts UpdateOpts)` method that updates mutable fields (starting with `token_ttl`). Validate bounds. Record an `app_updated` audit event with the old and new TTL values.

### 6. Add Update App Store Method

In `internal/store/sql_store.go`, add `UpdateAppTTL(appID string, ttl int)` that updates the `token_ttl` and `updated_at` columns.

### 7. Add HTTP Handler for App Update

New handler for `PUT /v1/admin/apps/{id}`. Requires admin auth (`admin:manage` scope). Parses the request body, calls `AppSvc.UpdateApp()`, returns the updated app record.

### 8. Wire the Endpoint

In `cmd/broker/main.go`, register `PUT /v1/admin/apps/{id}` through the admin auth middleware.

### 9. Add aactl Commands

In `cmd/aactl/app.go`:
- Add `--token-ttl` flag to the existing `register` subcommand
- Add new `update` subcommand with `--id` and `--token-ttl` flags
- Display TTL in the registration and list output

### 10. Audit Event

Add `app_updated` event type to `internal/audit/audit_log.go`. Record it when TTL is changed, with detail showing old and new values.

---

## Edge Cases & Risks

| Case | What Happens | Mitigation |
|------|-------------|------------|
| Existing apps in DB have no `token_ttl` column | `ALTER TABLE` adds column with `DEFAULT 1800` | Handled at startup in `InitDB()` migration logic |
| Operator sets TTL to 0 at registration | Treated as "use global default" | `0` is not an error — it means "no override, use `AA_APP_TOKEN_TTL`" |
| Operator sets TTL to 59 or 86401 | Rejected with `invalid_ttl` error (400) | Bounds checked in `RegisterApp()` and `UpdateApp()` |
| App authenticates while TTL is being updated | Uses the TTL from the `GetAppByClientID()` read | No race — reads are atomic in SQLite, update takes effect on next auth |
| `AA_APP_TOKEN_TTL` set to invalid value (negative, huge) | Broker logs a warning and falls back to 1800 | Bounds checked in `cfg.Load()` with fallback |
| Developer has a cached token when operator reduces TTL | Existing token keeps its original `exp` — only new tokens use the new TTL | This is expected behavior; revocation handles emergency shortening |

---

## Backward Compatibility

### Breaking Changes

None. All changes are additive.

### Non-Breaking Changes

- **App JWT lifetime changes from 5 min to 30 min** for all existing apps. This is the intended fix — 5 min was too short. Developers will notice their tokens last longer. No workflows break from a longer-lived token.
- **App registration response** gains a `token_ttl` field. Clients parsing the response with strict schemas may see an unexpected field, but JSON parsers typically ignore unknown fields.
- **`aactl app list`** output gains a TTL column. Cosmetic change to CLI output.

### Migration Path

Automatic — no operator action needed. Existing databases get `token_ttl = 1800` on all app rows via the `ALTER TABLE` default. The `AA_APP_TOKEN_TTL` env var is optional; if unset, the default is 1800. Operators who want a different global default can set `AA_APP_TOKEN_TTL` in their environment or `docker-compose.yml`.

---

## Rollback Plan

1. **Binary rollback:** Deploy the previous broker binary. The old code ignores the `token_ttl` column (it reads `const appTokenTTL = 300`). The column stays in the DB but is harmless — SQLite doesn't complain about unused columns.
2. **Schema rollback:** Not required. The `token_ttl` column with its default value causes no issues for older code. If a clean rollback is desired, `ALTER TABLE apps DROP COLUMN token_ttl` works in SQLite 3.35+.
3. **Config rollback:** Remove `AA_APP_TOKEN_TTL` from the environment. The old binary doesn't read it.
4. **Data safety:** No data is lost on rollback. The only change is app tokens revert to 5-minute lifetime.

---

## Success Criteria

- App registered without `--token-ttl` gets TTL = `AA_APP_TOKEN_TTL` (default 1800s)
- App registered with `--token-ttl 3600` gets TTL = 3600s
- `aactl app update --token-ttl 7200` changes an existing app's TTL
- App authentication issues JWT with `exp = now + app.token_ttl` (not hardcoded 300)
- TTL < 60 or > 86400 is rejected at registration and update
- Existing apps in the database get TTL = 1800 after migration
- `app_updated` audit event recorded on TTL change
- Admin-created launch tokens and admin JWTs are unaffected
- `aactl app list` shows TTL column

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/td-006/user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
