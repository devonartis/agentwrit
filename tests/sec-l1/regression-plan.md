# SEC-L1 Security Foundation — Regression Plan

> **Date:** 2026-03-27
> **Branch:** `fix/sec-l1`
> **Author:** Claude Code (regression runner)
> **Status:** Plan written, not yet executed

---

## What Changed

SEC-L1 made five changes to the **server startup path** only. No endpoint, handler, or token logic was modified.

### Change 1: Admin Secret Denylist (H5)

**File:** `internal/cfg/cfg.go` — `Load()`

**Before:** Any value accepted for `AA_ADMIN_SECRET`, including empty string and `change-me-in-production`.

**After:** `Load()` rejects `change-me-in-production` and empty string with an error directing the operator to `awrit init`.

**Why this matters for regression:** The P0/P1/P2 test env.sh files all use `AACTL_ADMIN_SECRET=change-me-in-production`. The broker now refuses to start with this value. This is the **only breaking change** in SEC-L1. All regression tests must use a proper secret from `awrit init`.

### Change 2: Bind Address Default

**Files:** `internal/cfg/cfg.go` (new `BindAddress` field), `cmd/broker/main.go` (addr construction)

**Before:** Broker bound to `:8080` (all interfaces — `0.0.0.0`).

**After:** Broker binds to `127.0.0.1:8080` (localhost only). Override with `AA_BIND_ADDRESS=0.0.0.0`.

**Why this matters for regression:** Tests hitting `http://127.0.0.1:8080` continue to work. Tests hitting `http://localhost:8080` also work (localhost resolves to 127.0.0.1). No breakage expected, but the startup log now shows `127.0.0.1:8080` instead of `:8080`.

### Change 3: HTTP Server Timeouts (C1)

**File:** `cmd/broker/serve.go` — new `buildServer()` function

**Before:** `http.Server` created with zero-value timeouts (no limits).

**After:** ReadTimeout 15s, ReadHeaderTimeout 5s, WriteTimeout 30s, IdleTimeout 120s, MaxHeaderBytes 1MB.

**Why this matters for regression:** Normal requests complete well within these timeouts. No breakage expected for any standard API call. Would only affect extremely slow clients or very large request bodies (>1MB).

### Change 4: TLS Hardening (C2)

**File:** `cmd/broker/serve.go` — `buildServer()` sets `tls.Config`

**Before:** TLS mode set `&tls.Config{}` with Go defaults.

**After:** `MinVersion: TLS 1.2`, AEAD-only cipher suites (GCM + ChaCha20-Poly1305).

**Why this matters for regression:** Regression tests run in `TLSMode=none` (dev mode), so TLS config is not applied. No breakage expected. TLS hardening only affects production deployments with `AA_TLS_MODE=tls` or `AA_TLS_MODE=mtls`.

### Change 5: .gitignore Hygiene (M12)

**File:** `.gitignore`

Added `.env` and `.env.*` patterns. No runtime impact.

---

## Risk Assessment

| Change | Risk Level | Why |
|--------|-----------|-----|
| Admin denylist | **HIGH** | Intentional breaking change — broker rejects previously-accepted secrets |
| Bind address | **LOW** | Default changed but 127.0.0.1 is where tests already connect |
| HTTP timeouts | **LOW** | Normal requests are well under 15s read / 30s write |
| TLS hardening | **NONE** | Only applies when TLSMode != "none", tests run in dev mode |
| .gitignore | **NONE** | No runtime impact |

---

## Regression Test Matrix

### Startup & Config (Operator Persona — awrit)

| ID | Test | Tool | Why This Specific Test |
|----|------|------|----------------------|
| S1 | Broker rejects `change-me-in-production` | broker binary | Denylist enforcement — the core security fix. Must prove the broker refuses to start. |
| S2 | Broker rejects empty admin secret | broker binary | Denylist enforcement — empty string case. |
| S3 | `awrit init` generates valid config | `awrit init` | Operators must use `awrit init` now. Must still work. |
| S4 | Broker starts with `awrit init` config | broker binary | Proves the happy path works after denylist rejects weak secrets. |
| S5 | Startup log shows `127.0.0.1:8080` | broker startup output | Bind address default changed — verify it's visible in logs. |

### Core Flows (Proves timeouts/bind address don't break anything)

| ID | Test | Tool | Why This Specific Test |
|----|------|------|----------------------|
| C1 | Admin authentication | curl `POST /v1/admin/auth` | Root of bootstrap chain — if admin auth breaks, nothing works. |
| C2 | App register | `awrit app register` | awrit auto-authenticates via admin token through ValMw. |
| C3 | App list | `awrit app list` | Same auth path, different endpoint. |
| C4 | Challenge + health | curl `GET /v1/challenge`, `GET /v1/health` | Public endpoints — confirms routing unaffected. |
| C5 | OIDC Discovery + JWKS | curl `GET /.well-known/openid-configuration`, `GET /v1/jwks` | Public endpoints — confirms OIDC still works. |
| C6 | App remove | `awrit app remove` | Cleanup + proves admin scope auth still works. |

### Negative Tests

| ID | Test | Tool | Why This Specific Test |
|----|------|------|----------------------|
| N1 | Wrong admin secret rejected | curl `POST /v1/admin/auth` | Auth path unchanged but must confirm no regression. |
| N2 | Invalid token rejected | curl `POST /v1/token/validate` | Token validation unchanged but proves timeouts don't interfere. |

---

## Execution Order

```
S1 (reject weak secret) → S2 (reject empty) → S3 (awrit init) → S4 (broker starts) → S5 (bind address log)
                                                                       |
                                                                       +→ C1 (admin auth) → C2 (app register) → C3 (app list) → C6 (app remove)
                                                                       |
                                                                       +→ C4 (challenge + health)
                                                                       |
                                                                       +→ C5 (discovery + JWKS)
                                                                       |
                                                                       +→ N1 (wrong secret) → N2 (invalid token)
```

S1 and S2 run **before** the broker starts (they prove the broker refuses to start).
S3 generates the config. S4 starts the broker. All remaining tests run against the live broker.

---

## What Should NOT Need Testing (and Why)

| Area | Why It's Safe |
|------|--------------|
| Token issuance/verification | `TknSvc` not modified — no kid, iss, or claims changes |
| Scope checking | String comparison on already-verified claims — unchanged |
| Revocation | RevSvc stores JTI/agent_id as strings — no JWT parsing |
| Audit trail | AuditLog.Record() takes structured fields — no JWT parsing |
| Delegation | DelegSvc unchanged — same Verify + Issue path |
| HITL approval | ApprovalSvc unchanged |
| Cloud credentials | CloudCredSvc unchanged |
| Agent registration | IdSvc.Register unchanged |

---

## Infrastructure Prerequisites

| Prerequisite | Purpose | Status |
|-------------|---------|--------|
| Go 1.22+ compiler | Build broker and awrit binaries | NOT VERIFIED |
| No external services needed | SEC-L1 tests run against localhost broker only | — |

No Docker, no AWS, no ngrok, no Python required. All tests run in VPS mode against the local broker binary.

---

## Evidence Format

Each test produces a banner with who/what/why/how/expected, followed by piped command output, followed by a verdict. Evidence saved to `tests/fix-sec-l1/evidence/`.

## Pass Criteria

All tests must PASS. The denylist tests (S1, S2) PASS by proving the broker **fails** to start. All other tests PASS by producing the expected output.
