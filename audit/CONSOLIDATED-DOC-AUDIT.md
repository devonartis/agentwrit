# Consolidated Documentation Audit — AgentAuth Core

**Date:** 2026-03-29
**Branch:** `fix/docs-overhaul`

---

## Why Three Reports Exist and Why They Disagree

Three separate audits were performed at different times by different approaches:

| Report | Location | Method | When | Agent Count |
|--------|----------|--------|------|-------------|
| **Docx Audit** | `AgentAuth_Doc_Audit.docx` | Manual end-to-end read of all 13 docs, cross-referenced against specific code files | Earlier session | 1 (Cowork) |
| **Docs Audit** | `docs/DOCS-AUDIT-REPORT.md` | Similar manual approach, doc-first | Mid-session | 1 (Claude Code) |
| **Evidence Audit** | `tests/sec-l2a/evidence/DOC-AUDIT-REPORT.md` | Code-first: 9 parallel agents extracted ALL code entities, then checked doc coverage | Late session | 9 agents |

### Why they disagree:

1. **Direction of analysis.** The docx and docs reports read the docs first, then spot-checked code. The evidence report started from code and checked docs. Code-first finds gaps the doc-first approach misses (undocumented features, missing packages). Doc-first finds presentation issues the code-first misses.

2. **Severity calibration.** The docx report found 17 findings and called 97% accuracy. The evidence report found 41+ findings. The difference: the docx report counted verified-correct items and treated many issues as LOW that the evidence report called HIGH (e.g., wrong env var names, fabricated store types).

3. **Scope.** The evidence report's 9 agents covered every package, every error, every audit event — finding gaps the single-pass approach missed (25 vs 17 audit events, 20+ undocumented sentinel errors, `RequireAnyScope` undocumented). Note: `docs/examples/*.md` files are templates to be written — they were excluded from this consolidated audit.

4. **Timing.** The docx was written before B4 fixes were applied. The evidence report was written after. Some findings in the docx about the revoker being nil are now fixed.

---

## Consolidated Findings — Deduplicated and Prioritized

All three reports agree on the major issues. Below is the single source of truth, with each finding tagged by which reports caught it.

---

## CRITICAL (4 findings)

### C-1: Signing key persistence misrepresented as ephemeral

**Found by:** All 3 reports (Docx F-1, Docs CRIT-8, Evidence CRIT-1)
**Locations:** `architecture.md:399`, `getting-started-operator.md:139,263`
**Wrong:** "Fresh Ed25519 keys every startup. All previously issued tokens become unverifiable."
**Right:** `keystore.LoadOrGenerate()` loads existing key from `AA_SIGNING_KEY_PATH`. New key only generated if file missing. Tokens survive restarts.
**Fix:** Rewrite all 3 locations. Describe persistent key with manual rotation. Add key management best practices.

### C-2: `internal/keystore` package absent from architecture.md

**Found by:** Evidence report (CRIT-2), Docx (F-15, rated LOW)
**Wrong:** Not in directory layout, dependency graph, or component table.
**Right:** Package handles Ed25519 key persistence — a core security function. Called directly from `main.go`.
**Fix:** Add to Foundation Layer in component diagram, add to dependency graph, add package description.

### C-3: Admin auth mechanism wrong in architecture.md

**Found by:** Evidence report (CRIT-3). Docx missed this. Docs report missed this.
**Wrong:** Design Decision #5 says `subtle.ConstantTimeCompare`.
**Right:** Uses `bcrypt.CompareHashAndPassword` since B2. Secret is bcrypt-hashed at startup.
**Fix:** Rewrite Design Decision #5.

### C-4: Store types in architecture.md are fabricated

**Found by:** Evidence report (CRIT-4). Docx missed this. Docs report missed this.
**Wrong:** Pattern Components table lists `NonceTbl`, `AgentTbl`, `AppTbl`, `RevokeTbl`, `AuditTbl`.
**Right:** None exist. Actual types: `SqlStore`, `LaunchTokenRecord`, `AgentRecord`, `AppRecord`, `RevocationEntry`.
**Fix:** Replace fabricated type names with actual types.

---

## HIGH (17 findings)

> **Note:** `docs/examples/*.md` files are templates to be created — they were not audited.
> Findings from agents that incorrectly audited examples have been removed.

### H-1: CHANGELOG.md HITL contamination

**Found by:** Evidence contamination scan only
**Issue:** Lines 40-98 document HITL features as if added to core. HITL is enterprise-only.
**Fix:** Remove HITL entries from CHANGELOG.

### H-2: Wrong aactl env var names in operator guide

**Found by:** All 3 reports (Docx F-11 rated LOW, Docs HIGH-2, Evidence HIGH-2)
**Wrong:** `AGENTAUTH_BROKER_URL`, `AGENTAUTH_ADMIN_SECRET`
**Right:** `AACTL_BROKER_URL`, `AACTL_ADMIN_SECRET`
**Fix:** Replace in `getting-started-operator.md:78-79`. The docx report rated this LOW because it viewed them as "SDK" vs "CLI" vars — but there is no SDK, so these are just wrong.

### H-3: api.md app management endpoints comprehensively wrong

**Found by:** All 3 reports (Docx F-3/F-4, Docs CRIT-2, Evidence HIGH-3)
**Issues:** Wrong scopes (`admin:apps:*` vs `admin:launch-tokens:*`), wrong request fields (`client_id` doesn't exist), wrong response field (`id` vs `app_id`), wrong status code (204 vs 200 for DELETE), fake pagination (`limit`/`offset` not implemented).
**Fix:** Rewrite entire App Management section of api.md.

### H-4: Ghost route `POST /v1/token/exchange`

**Found by:** Evidence report (HIGH-4). Docx missed. Docs report missed.
**Issue:** In architecture.md route table and middleware diagram. Not registered in code. No handler file exists. Also in `handler/doc.go`.
**Fix:** Remove from architecture.md, remove from handler/doc.go.

### H-5: openapi.yaml `RegisterRequest` schema wrong

**Found by:** All 3 reports (Docx F-2, Docs O-01/O-02, Evidence HIGH-5)
**Wrong:** Field `nonce_signature` (should be `signature`). Missing 4 of 7 required fields.
**Fix:** Rename field, add missing required fields.

### H-6: openapi.yaml `DelegateRequest` missing `delegate_to`

**Found by:** All 3 reports (Docx F-7, Docs O-09, Evidence HIGH-6)
**Fix:** Add `delegate_to` to required fields and properties.

### H-7: openapi.yaml `delegation_chain` type wrong

**Found by:** All 3 reports (Docx F-8, Docs O-10, Evidence HIGH-7)
**Wrong:** `array of strings`. **Right:** `array of DelegRecord objects`.
**Fix:** Create DelegRecord schema, reference it.

### H-8: App auth rate limit wrong in api.md

**Found by:** All 3 reports (Docx F-3 context, Docs CRIT-3, Evidence HIGH-8)
**Wrong:** "5 req/s, burst 10". **Right:** 10/min (0.167/s), burst 3 per client_id.
**Fix:** Update rate limit in api.md.

### H-9: Revocation inside Verify() not documented

**Found by:** Evidence report only (HIGH-9). Docx and Docs reports missed this.
**Issue:** B4 moved revocation check into `TknSvc.Verify()`. Architecture.md shows it as middleware-only. The `Revoker` interface, `SetRevoker()`, `RevokeByJTI()` are undocumented.
**Fix:** Update architecture.md request lifecycle diagram and component descriptions.

### H-10: `internal/app` package absent from dependency graph

**Found by:** Evidence report only (HIGH-10).
**Fix:** Add to Package Dependency Graph with its dependencies.

### H-11: `ValMw.RequireAnyScope()` undocumented

**Found by:** Evidence report only (HIGH-11).
**Issue:** Used on launch-tokens endpoint for dual admin/app scope. Not in any doc.
**Fix:** Document in architecture.md and api.md.

### H-12: 25 audit events, docs say 17

**Found by:** Evidence report only (HIGH-12). Docx said 21 verified correct — missed 4 app events.
**Fix:** Update concepts.md event list to include all 25.

### H-13: integration-patterns.md uses legacy admin auth (400 error)

**Found by:** Docs report (CRIT-1 context). Evidence report (HIGH-15).
**Wrong:** `{"client_id": "operator", "client_secret": ...}`. Broker explicitly rejects with 400.
**Fix:** Change to `{"secret": "..."}`.

### H-14: common-tasks.md uses denylisted secret

**Found by:** Docs report. Evidence report (HIGH-16). Docx missed.
**Issue:** `change-me-in-production` is on the denylist. Broker refuses to start.
**Fix:** Replace with a safe example secret like `your-secure-secret-here`.

### H-15: common-tasks.md registration uses HMAC (wrong algorithm)

**Found by:** All 3 reports (Docx CRIT-1 context, Docs CRIT-1, Evidence HIGH-17)
**Fix:** Rewrite to use Ed25519 signing of hex-decoded nonce bytes.

### H-16: troubleshooting.md suggests `AA_ADMIN_SECRET=""` (denylisted)

**Found by:** Evidence report (HIGH-18). Docs report context. Docx missed.
**Fix:** Change to "unset the variable" instead of setting to empty.

### H-17: Wrong audit event type names in common-tasks.md

**Found by:** Evidence report (HIGH-19). Docs report (F-5).
**Wrong:** `token_acquired`, `delegation_issued`, `launch_token_created`.
**Right:** `token_issued`, `delegation_created`, `launch_token_issued`.
**Fix:** Replace with correct names.

---

## MEDIUM (18 findings)

| ID | Finding | Found By | Fix |
|----|---------|----------|-----|
| M-1 | `AA_CONFIG_PATH` env var undocumented | Evidence | Add to operator config table |
| M-2 | `AA_BIND_ADDRESS` missing from config table | Evidence, Docs | Add to operator config table |
| M-3 | Config file path `.yaml` extension mismatch | All 3 (Docx F-12) | Remove `.yaml` from docs |
| M-4 | `aactl app update/remove` positional vs `--id` flag | Evidence, CLI audit | Fix in operator guide |
| M-5 | Request lifecycle diagram missing audience check | Evidence | Add step to diagram |
| M-6 | Launch-tokens scope documented as admin-only | Evidence | Document dual-scope |
| M-7 | openapi.yaml AuditEvent schema: 6 of 14 fields | Evidence, Docx | Add 8 missing fields |
| M-8 | openapi.yaml ProblemDetail missing fields | Evidence | Add `error_code`, `request_id`, `hint` |
| M-9 | openapi.yaml audit limit default 50 vs 100 | All 3 (Docx F-10) | Change to 100 |
| M-10 | openapi.yaml health status enum wrong | All 3 (Docx F-9) | Change to `["ok"]` |
| M-11 | Version 3.0.0 in docs, 2.0.0 in code | All 3 (Docx F-6) | Decide and align |
| M-12 | 20+ sentinel errors not in troubleshooting | Evidence | Add error reference |
| M-13 | GET audit uses JSON body in integration-patterns | Evidence | Change to query params |
| M-14 | Troubleshooting error messages don't match code | Evidence, Docs | Update to actual messages |
| M-15 | `common-tasks.md` says aactl is "demo/dev only" | Evidence, CLI audit | Remove stale note |
| M-16 | Outcome filter values inconsistent across docs | All 3 (Docx F-5) | Standardize to `success`/`denied` |
| M-17 | Verify() check order oversimplified in concepts.md | Evidence | Add alg/kid/format steps |
| M-18 | "Enterprise" label in common-tasks.md version | Docs report (C-1) | Remove |

---

## Recommended Fix Phases

### Phase 1 — Security-Critical (1 hour)
- C-1: Fix key persistence claims (3 locations)
- H-18: Remove `AA_ADMIN_SECRET=""` suggestion
- H-16: Replace denylisted secret in examples
- H-2: Fix aactl env var names

### Phase 2 — API Contract Fixes (2 hours)
- H-3: Rewrite api.md app management section
- H-5, H-6, H-7: Fix openapi.yaml schemas (register, delegate)
- H-8: Fix rate limit
- M-9, M-10: Fix openapi defaults/enums
- M-7, M-8: Complete openapi schemas

### Phase 3 — Architecture Rewrite (2 hours)
- C-2, C-3, C-4: Fix keystore, admin auth, store types
- H-4: Remove ghost `/v1/token/exchange`
- H-9, H-10, H-11: Update revocation model, app package, RequireAnyScope
- M-5: Add audience check to lifecycle diagram

### Phase 4 — Examples & Guides (2 hours)
- H-13, H-14: Rewrite all 4 example registration flows
- H-15: Fix legacy auth shape in integration-patterns
- H-17: Fix HMAC registration in common-tasks
- H-19: Fix audit event names

### Phase 5 — Completeness (1 hour)
- H-1: Remove HITL from CHANGELOG
- H-12: Update event type list (17 -> 25)
- M-12: Add sentinel errors to troubleshooting
- M-11: Resolve version number
- Remaining MEDIUM items

**Total estimated effort: ~8 hours of focused work.**

---

## Files to Move/Clean Up

The three separate audit reports should be archived:
- `AgentAuth_Doc_Audit.docx` — move to `audit/`
- `docs/DOCS-AUDIT-REPORT.md` — move to `audit/`
- `tests/sec-l2a/evidence/DOC-AUDIT-REPORT.md` — move to `audit/`

This consolidated report (`audit/CONSOLIDATED-DOC-AUDIT.md`) is the single source of truth.
