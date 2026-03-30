# Documentation Overhaul — Full Audit and Instructions

**Branch:** `fix/docs-overhaul` (already created from develop)
**Date:** 2026-03-29
**Author:** Cowork (post B0-B4 review)

README.md has already been rewritten by Cowork. The remaining files need updating by Claude Code using this audit as the guide.

---

## Ground Truth: What the Code Actually Does

### HTTP Routes (from cmd/broker/main.go)

| Method | Path | Auth | Handler | Response key field |
|--------|------|------|---------|--------------------|
| GET | /v1/health | None | HealthHdl | status, version, uptime, db_connected, audit_events_count |
| GET | /v1/metrics | None | MetricsHdl | Prometheus text |
| GET | /v1/challenge | None | ChallengeHdl | nonce, expires_in |
| POST | /v1/token/validate | None | ValHdl | valid, claims OR valid, error |
| POST | /v1/register | Launch token | RegHdl | agent_id, access_token, expires_in |
| POST | /v1/token/renew | Bearer (ValMw) | RenewHdl | access_token, expires_in |
| POST | /v1/token/release | Bearer (ValMw) | ReleaseHdl | 204 No Content |
| POST | /v1/delegate | Bearer + any scope | DelegHdl | access_token, expires_in, delegation_chain |
| POST | /v1/revoke | Bearer + admin:revoke:* | RevokeHdl | revoked, level, target, count |
| GET | /v1/audit/events | Bearer + admin:audit:* | AuditHdl | events, total, offset, limit |
| POST | /v1/admin/auth | None (rate-limited 5/s) | AdminHdl.handleAuth | access_token, expires_in, token_type |
| POST | /v1/admin/launch-tokens | Bearer + admin:launch-tokens:* | AdminHdl.handleCreateLaunchToken | launch_token, expires_at, policy |
| POST | /v1/admin/apps | Bearer + admin:launch-tokens:* | AppHdl.handleRegisterApp | app_id, name, client_id, client_secret, scopes, token_ttl |
| GET | /v1/admin/apps | Bearer + admin:launch-tokens:* | AppHdl.handleListApps | apps[], total |
| GET | /v1/admin/apps/{id} | Bearer + admin:launch-tokens:* | AppHdl.handleGetApp | app record |
| PUT | /v1/admin/apps/{id} | Bearer + admin:launch-tokens:* | AppHdl.handleUpdateApp | updated app record |
| DELETE | /v1/admin/apps/{id} | Bearer + admin:launch-tokens:* | AppHdl.handleDeregisterApp | app_id, status, deregistered_at |
| POST | /v1/app/auth | None (rate-limited 10/min/client) | AppHdl.handleAppAuth | access_token, expires_in, token_type, scopes |

**ROUTES THAT DO NOT EXIST (remove from all docs):**
- POST /v1/token (sidecar endpoint — REMOVED)
- POST /v1/token/exchange (sidecar mediated — REMOVED)
- POST /v1/sidecar/activate (sidecar activation — REMOVED)
- POST /v1/admin/sidecar-activations (sidecar admin — REMOVED)
- GET /v1/admin/sidecars (sidecar listing — REMOVED)

### Environment Variables (from internal/cfg/cfg.go)

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| AA_ADMIN_SECRET | string | (required) | Shared admin secret (plaintext or bcrypt hash via aactl init) |
| AA_PORT | string | 8080 | HTTP listen port |
| AA_BIND_ADDRESS | string | 127.0.0.1 | Bind address (0.0.0.0 for Docker) |
| AA_LOG_LEVEL | string | verbose | quiet, standard, verbose, trace |
| AA_TRUST_DOMAIN | string | agentauth.local | SPIFFE trust domain |
| AA_DEFAULT_TTL | int | 300 | Default token TTL seconds |
| AA_APP_TOKEN_TTL | int | 1800 | App JWT TTL seconds |
| AA_MAX_TTL | int | 86400 | Maximum token TTL ceiling. 0 = disabled. |
| AA_DB_PATH | string | ./agentauth.db | SQLite path |
| AA_SIGNING_KEY_PATH | string | ./signing.key | Ed25519 key path |
| AA_CONFIG_PATH | string | (none) | Config file from aactl init |
| AA_SEED_TOKENS | string | false | Print seed tokens on startup |
| AA_AUDIENCE | string | agentauth | JWT audience claim. Empty = skip. |
| AA_TLS_MODE | string | none | none, tls, mtls |
| AA_TLS_CERT | string | (none) | TLS cert PEM |
| AA_TLS_KEY | string | (none) | TLS key PEM |
| AA_TLS_CLIENT_CA | string | (none) | Client CA PEM (mtls) |
| MODE | string | development | development or production |

**ENV VARS THAT DO NOT EXIST (remove from all docs):**
- AA_SIDECAR_PORT, AA_SIDECAR_SCOPE_CEILING, AA_BROKER_URL
- AA_SOCKET_PATH, AA_SIDECAR_CA_CERT, AA_SIDECAR_TLS_CERT, AA_SIDECAR_TLS_KEY
- AA_SIDECAR_CB_* (circuit breaker)

### aactl CLI Commands (from cmd/aactl/)

```
aactl [--json]
  init [--mode {dev|prod}] [--force] [--config-path PATH]
  app register --name NAME --scopes SCOPE_CSV [--token-ttl N]
  app list [--json]
  app get APP_ID
  app update --id APP_ID [--scopes SCOPE_CSV] [--token-ttl N]
  app remove --id APP_ID
  token release --token TOKEN_JWT
  revoke --level {token|agent|task|chain} --target TARGET
  audit events [--agent-id ID] [--task-id ID] [--event-type TYPE]
               [--since RFC3339] [--until RFC3339] [--outcome {success|denied}]
               [--limit N] [--offset N] [--json]
```

**COMMANDS THAT DO NOT EXIST (remove from all docs):**
- aactl sidecars list
- aactl sidecars ceiling get/set
- Any sidecar-related aactl commands

### Docker Compose (docker-compose.yml)

- ONE service: broker
- Port: ${AA_HOST_PORT:-8080}:8080
- Volume: broker-data:/data
- Network: agentauth-net (bridge)
- Health check: GET http://localhost:8080/v1/health every 2s
- NO sidecar service

---

## File-by-File Instructions

### 1. docs/architecture.md — MAJOR REWRITE

**Issues found:**
- Multiple mermaid diagrams include sidecar as a component
- References to cmd/sidecar/ (dead code, deleted)
- References to docker-compose.uds.yml (doesn't exist)
- Sidecar component section with file paths
- Missing: App Service component, B2-B4 features (bcrypt admin auth, MaxTTL, token hardening)

**Instructions:**
- Remove ALL sidecar references from diagrams and text
- Update architecture flowchart to match README.md (broker only, with Identity/Token/Authz/Revoke/Audit/Deleg/App/Admin/Obs/Store)
- Add App Service component description
- Update data flow diagrams (no sidecar in any flow)
- Mention config file support (aactl init) in config section
- Mention MaxTTL ceiling in token service description
- Mention bcrypt admin auth in admin service description

### 2. docs/concepts.md — MODERATE REWRITE

**Issues found:**
- Component 6 (Agent-to-Agent Mutual Auth) references sidecar bootstrapping
- 7-component flowchart includes sidecar
- Some conceptual flows mention sidecar as intermediary

**Instructions:**
- Update 7-component flowchart to remove sidecar
- Update mutual auth section to describe direct broker registration
- Replace sidecar-mediated flows with direct broker flows
- Keep conceptual explanations intact (they're good) — just fix the implementation references

### 3. docs/api.md — MAJOR REWRITE

**Issues found (CRITICAL):**
- Lines 52, 248, 255: shows client_id/client_secret auth format — code returns migration error for this
- References sidecar on port 8081
- Shows POST /v1/token endpoint (doesn't exist)
- Token exchange endpoint /v1/token/exchange (doesn't exist)
- Sidecar activation endpoint (doesn't exist)
- Missing: App management endpoints (POST/GET/PUT/DELETE /v1/admin/apps, POST /v1/app/auth)
- Missing: MaxTTL behavior, token hardening (alg/kid validation)
- Response field: must be `access_token` everywhere, NOT `token`

**Instructions:**
- Rewrite endpoint list to match the ground truth table above EXACTLY
- Remove ALL sidecar endpoints
- Add ALL app management endpoints with request/response examples
- Fix auth response to show `{"access_token":"...","expires_in":300,"token_type":"Bearer"}`
- Update sequence diagram to show broker-only flows
- Add MaxTTL clamping behavior notes
- Add token validation notes (alg=EdDSA, kid matching)

### 4. docs/getting-started-user.md — MAJOR REWRITE

**Issues found (CRITICAL):**
- Two-path architecture (Sidecar Path vs Direct Path) — sidecar path doesn't exist
- Quick start references port 8081 and sidecar health check
- POST /v1/token on sidecar (doesn't exist)
- Health check response format is incomplete

**Instructions:**
- Remove the two-path architecture entirely — there is only one path (direct broker)
- Rewrite quick start to match README.md quick start
- Show the full workflow: admin auth → create app → create launch token → agent register → use token
- Make it novice-friendly: explain every step, what it does, and why
- Use correct response formats (access_token, not token)
- Show correct health check response (include db_connected, audit_events_count)

### 5. docs/getting-started-developer.md — MAJOR REWRITE

**Issues found (CRITICAL):**
- Documents sidecar as the recommended path
- Shows sidecar SDK integration (port 8081)
- BYOK registration through sidecar endpoint
- Token exchange flow via sidecar

**Instructions:**
- Remove ALL sidecar content
- Show direct broker integration patterns:
  1. App auth (client_id/client_secret → access_token)
  2. Agent registration (launch token + challenge-response)
  3. Token renewal (POST /v1/token/renew with Bearer)
  4. Token release (POST /v1/token/release — task complete signal)
  5. Delegation (POST /v1/delegate — scope attenuation)
- Python examples using requests library
- TypeScript examples using fetch
- Show the renewal-at-80%-TTL pattern (this was good, keep it)
- Make it novice-friendly: explain WHY each step matters

### 6. docs/getting-started-operator.md — MAJOR REWRITE

**Issues found:**
- Sidecar bootstrap and health check sections
- Sidecar config env vars
- Sidecar deployment topology
- Missing: aactl init (config generation)
- Missing: MaxTTL configuration
- Missing: App management via aactl

**Instructions:**
- Remove ALL sidecar config and deployment content
- Add aactl init section (dev mode vs prod mode)
- Add config file explanation (env vars override config file)
- Add app management workflow (aactl app register/list/get/update/remove)
- Add MaxTTL configuration guidance
- Add monitoring section (health check, metrics endpoint, audit trail)
- Docker deployment section (docker-compose.yml, stack_up.sh)
- TLS/mTLS setup

### 7. docs/sidecar-deployment.md — DELETE THIS FILE

The entire file is about deploying sidecar. Sidecar doesn't exist. Delete it.

### 8. docs/troubleshooting.md — MODERATE UPDATE

**Issues found:**
- Sidecar troubleshooting section
- UDS references
- Diagnostic flowchart includes sidecar checks

**Instructions:**
- Remove sidecar troubleshooting
- Update diagnostic flowchart to broker-only
- Add troubleshooting for new features: MaxTTL clamping, admin secret rejection (weak secret denylist), config file issues
- Keep the structure (error message → possible cause → fix) — it's good

### 9. docs/common-tasks.md — MODERATE UPDATE

**Issues found:**
- Sidecar operations tasks
- Some API calls use wrong field names

**Instructions:**
- Remove sidecar tasks
- Add app management tasks (register app, list apps, app auth)
- Fix all API field names to use access_token
- Add MaxTTL configuration task
- Add config file setup task (aactl init)

### 10. docs/integration-patterns.md — MODERATE UPDATE

**Issues found:**
- Sidecar integration pattern
- Some flows show sidecar as intermediary

**Instructions:**
- Remove sidecar integration pattern
- Update all flows to show direct broker interaction
- Keep the 6-pattern structure (it's good) — just fix implementation details
- Update Python examples to use correct endpoints and field names

### 11. docs/aactl-reference.md — MODERATE UPDATE

**Issues found:**
- References sidecar commands (sidecars list, sidecars ceiling get/set)
- Notes about production auth being incomplete

**Instructions:**
- Remove all sidecar commands
- Add app management commands (register, list, get, update, remove)
- Add aactl init command documentation
- Update auth documentation (bcrypt is now implemented)

### 12. docs/examples/*.md (4 files) — MODERATE UPDATE

**Issues found:**
- Sidecar flows in sequence diagrams
- Some Python examples use sidecar endpoints

**Instructions:**
- Update all sequence diagrams to show direct broker interaction
- Fix Python examples to use correct endpoints (POST /v1/admin/auth, POST /v1/app/auth, POST /v1/register)
- Fix response field names (access_token, not token)
- Keep the scenarios — they're excellent real-world examples

### 13. docs/api/openapi.yaml — MAJOR UPDATE

**Issues found:**
- 51 sidecar endpoint references
- Sidecar-specific schemas
- Missing app management endpoints

**Instructions:**
- Remove ALL sidecar endpoints and schemas
- Add app management endpoints (POST/GET/PUT/DELETE /v1/admin/apps, POST /v1/app/auth)
- Fix response schemas to use access_token
- Add MaxTTL-related fields where applicable

### 14. scripts/verify_compose.sh — FIX

**Issue:** Checks for sidecar service in docker-compose.yml which no longer exists.

**Fix:** Remove the sidecar service check. Only check for broker service.

### 15. .plans/TECH-DEBT.md — UPDATE

After all docs are updated, mark the documentation drift items (TD-D01 through TD-D18) as RESOLVED. Add any new items discovered during this overhaul.

---

## Quality Standards

1. **Novice-friendly:** A developer who has never seen AgentAuth should be able to follow any getting-started guide from step 1 to working system without asking questions. Explain WHY, not just HOW.
2. **Accurate:** Every command, endpoint, field name, and env var must match the actual code. Use the ground truth tables above.
3. **Professional:** Clean formatting, consistent terminology, no TODO/FIXME/HACK markers in user-facing docs.
4. **No sidecar anywhere:** Zero references to sidecar in any user-facing document. The sidecar was removed in B0.
5. **Mermaid diagrams:** Every diagram must be valid mermaid syntax and accurately represent the current architecture (broker only, no sidecar).
