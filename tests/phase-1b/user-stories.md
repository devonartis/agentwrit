# Phase 1b: App-Scoped Launch Tokens — User Stories

## Personas and Tools

| Persona | Tool | Stories | What they have |
|---------|------|---------|----------------|
| Operator | `aactl` (CLI binary) | 4–6, R1–R3 | Admin secret, full control |
| Developer | `curl` / HTTP client (no SDK yet) | 1–3 | `client_id` + `client_secret` from operator |
| Security reviewer | Both | 7–8 | Verifying scope attenuation and traceability |

## Phase 1b Credential Flow

```
Operator (aactl)          Broker                    Developer (curl)
     |                      |                               |
     |-- app register ----→ |                               |
     |   scopes=["read:weather:*"]                          |
     |←- client_id,         |                               |
     |   client_secret ---- |                               |
     |                      |                               |
     |-- hands creds to developer -----------------------→  |
     |                      |                               |
     |                      |←- POST /v1/app/auth --------- |
     |                      |-- app JWT (5 min) ----------→ |
     |                      |                               |
     |                      |←- POST /v1/admin/launch-tokens|
     |                      |   Bearer: <app JWT>           |
     |                      |   allowed_scope: ["read:weather:current"]
     |                      |   ✓ subset of ceiling         |
     |                      |-- launch_token -------------→ |
     |                      |                               |
     |                      |←- GET /v1/challenge ----------|
     |                      |-- nonce -------------------→  |
     |                      |                               |
     |                      |←- POST /v1/register ---------|
     |                      |   launch_token + nonce + sig  |
     |                      |   agent inherits app_id       |
     |                      |-- agent JWT ---------------→  |
```

---

## Developer Stories

### Story 1 — Developer creates a launch token using app credentials

**As a developer with app credentials**, I want to create a launch token for my agent so that I can register the agent with the broker without asking the operator for a token.

**Precondition:** Operator has registered app `weather-bot` with scopes `["read:weather:*"]`.

**Acceptance criteria:**
- `POST /v1/app/auth` with valid `client_id` + `client_secret` returns 200 with app JWT
- `POST /v1/admin/launch-tokens` with `Bearer: <app JWT>` and `{"agent_name": "fetcher", "allowed_scope": ["read:weather:current"], "max_ttl": 300}` returns 201
- Response includes `launch_token`, `expires_at`, and `policy.allowed_scope`
- The launch token is usable (can be used to register an agent in Story 3)

### Story 2 — Developer is rejected when requesting scopes outside ceiling

**As a developer**, I want the broker to reject my launch token request if I ask for scopes outside my app's ceiling so that I know immediately what my app is allowed to do.

**Precondition:** App `weather-bot` has ceiling `["read:weather:*"]`.

**Acceptance criteria:**
- `POST /v1/admin/launch-tokens` with `Bearer: <app JWT>` and `{"agent_name": "rogue", "allowed_scope": ["write:data:all"]}` returns 403
- Response body explains what the app's ceiling is (e.g., `"allowed: [read:weather:*]"`)
- Audit event `scope_ceiling_exceeded` is recorded with the app_id, requested scopes, and ceiling
- A second request with valid scopes `["read:weather:current"]` still succeeds (the rejection doesn't lock out the app)

### Story 3 — Developer registers an agent linked to the app

**As a developer**, I want agents I register to be linked to my app so that audit trails and management operations know which agents belong to which app.

**Precondition:** Developer has a launch token from Story 1.

**Acceptance criteria:**
- Agent registration via `POST /v1/register` with the app-created launch token succeeds (200)
- Response includes `agent_id` and `access_token`
- Audit event `agent_registered` includes `app_id=<app_id>` in the detail field
- Audit event `token_issued` includes `app_id=<app_id>` in the detail field

---

## Operator Stories

### Story 4 — Operator traces launch token back to app

**As an operator**, I want to see which app created a launch token so that I can trace agent registrations back to the responsible app.

**Acceptance criteria:**
- After a developer creates a launch token (Story 1), `GET /v1/audit/events?event_type=launch_token_issued` shows the event
- The event detail includes `app_id=<the app's id>` and `created_by=app:<app_id>`
- Admin-created launch tokens do NOT have `app_id` in their audit detail

### Story 5 — Operator confirms ceiling enforcement works

**As an operator**, I want the broker to enforce each app's scope ceiling on launch token creation so that developers can't escalate their own permissions.

**Acceptance criteria:**
- Register app with ceiling `["read:weather:*"]`
- Developer authenticates and requests launch token with `["read:weather:current"]` → 201 (within ceiling)
- Developer requests launch token with `["write:data:all"]` → 403 (outside ceiling)
- Developer requests launch token with `["read:weather:*", "write:data:all"]` → 403 (partially outside ceiling)
- `GET /v1/audit/events?event_type=scope_ceiling_exceeded` shows the 403 attempts

### Story 6 — Admin launch tokens still work (backward compatible)

**As an operator**, I want admin-created launch tokens to continue working exactly as before so that existing workflows aren't broken.

**Acceptance criteria:**
- `aactl` authenticates as admin, creates a launch token with any scopes (no ceiling restriction)
- The launch token works to register an agent
- No `app_id` appears in the audit detail for admin-created tokens
- The agent registered with an admin token has no app affiliation

---

## Security Stories

### Story 7 — Scope attenuation at the launch token level

**As a security reviewer**, I want scope attenuation enforced at the launch token level so that an app with `read:weather:*` cannot create a launch token granting `write:weather:*`.

**Acceptance criteria:**
- App ceiling `["read:weather:*"]` → request `["write:weather:*"]` → 403 (action mismatch)
- App ceiling `["read:weather:*"]` → request `["read:weather:current"]` → 201 (valid attenuation)
- App ceiling `["read:weather:*"]` → request `["read:weather:*"]` → 201 (exact match is fine)
- App ceiling `["read:weather:current"]` → request `["read:weather:*"]` → 403 (can't widen with wildcard)
- Each rejection produces `scope_ceiling_exceeded` audit event

### Story 8 — Agent traceability to originating app

**As a security reviewer**, I want every agent record to carry the `app_id` of the app that created it so that compromise investigations can identify all agents belonging to a compromised app.

**Acceptance criteria:**
- Agent registered via app-created launch token: audit shows `app_id=<app_id>` on both `agent_registered` and `token_issued` events
- Agent registered via admin-created launch token: audit does NOT show `app_id=` on those events
- The traceability chain is complete: App auth → launch token (with app_id) → agent registration (with app_id) — all visible in audit trail

---

## Phase 1a Regression Stories

These stories verify that Phase 1a functionality still works after Phase 1b changes.

### Story R1 — Developer authenticates with app credentials (Phase 1a Story 6)

**Acceptance criteria:**
- `POST /v1/app/auth` with valid `client_id` + `client_secret` returns 200
- JWT carries `["app:launch-tokens:*", "app:agents:*", "app:audit:read"]`
- JWT `sub` is `app:<app_id>`

### Story R2 — App JWT cannot access admin-only endpoints (Phase 1a Story 8)

**Acceptance criteria:**
- `GET /v1/admin/apps` with app Bearer token → 403
- `GET /v1/admin/sidecars` with app Bearer token → 403

### Story R3 — Admin auth and audit flows unchanged (Phase 1a Story 11)

**Acceptance criteria:**
- Admin auth (`POST /v1/admin/auth`) still works
- `aactl audit events` returns events
- Audit hash chain is intact
