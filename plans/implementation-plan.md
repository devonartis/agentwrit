# Implementation Plan: Full Compliance and Sidecar Sprawl Fix

**Date:** 2026-02-20
**Authors:** integration-lead, security-architect, system-designer, code-planner, devils-advocate
**Branch:** develop (all code references are to develop branch)
**Companion Document:** [Design Solution](./design-solution.md)

---

## Phase Ordering

```
Phase 1 (Security)               Phase 2 (Compliance)             Phase 3 (Operations)
+---------------------------+    +---------------------------+    +------------------+
| Fix 1: mTLS in broker     |    | Fix 3: Audience validation|    | Fix 5: UDS       |
| Fix 2: Revocation persist |    | Fix 4: Token release      |    | Fix 6: Audit     |
+---------------------------+    +---------------------------+    +------------------+
```

---

## Per-Fix Details

| Fix | Feature Branch | Est. Lines |
|-----|---------------|-----------|
| 1 | `feature/broker-tls` | ~60 |
| 2 | `feature/revocation-persistence` | ~120 |
| 3 | `feature/audience-validation` | ~50 |
| 4 | `feature/token-release` | ~60 |
| 5 | `feature/sidecar-uds` | ~40 |
| 6 | `feature/structured-audit` | ~200 |

Each fix must pass `./scripts/gates.sh task` before PR.

---

## Post-Implementation Compliance

| Requirement | Before | After | Fix |
|------------|--------|-------|-----|
| 3.3 mTLS transport | PARTIAL (4 reviewers) | COMPLIANT | Fix 1 |
| 4.4 Task-completion signal | PARTIAL (Juliet) | COMPLIANT | Fix 4 |
| 4.4 Revocation persistence | PARTIAL (Kilo) | COMPLIANT | Fix 2 |
| 5.2 Structured audit schema | PARTIAL (Kilo) | COMPLIANT | Fix 6 |
| Token audience validation | NOT CHECKED | COMPLIANT | Fix 3 |
| Sidecar port sprawl | N/A (operational) | RESOLVED | Fix 5 |

---

## Implementation Notes (Code-Planner Verification)

These notes are verified against the develop branch source code and are intended for implementers.

### Fix 1 (mTLS) — Exact change location
`cmd/broker/main.go:174`: Replace `http.ListenAndServe(addr, rootHandler)` with a conditional:
- Mode `none`: keep current call
- Mode `tls`: `http.ListenAndServeTLS(addr, certFile, keyFile, rootHandler)`
- Mode `mtls`: construct `tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool}`, wrap in `http.Server{TLSConfig: tlsCfg}`, call `srv.ListenAndServeTLS(certFile, keyFile)`
New cfg fields: `AA_TLS_MODE`, `AA_TLS_CERT`, `AA_TLS_KEY`, `AA_TLS_CLIENT_CA`.

### Fix 2 (Revocation persistence) — Signing key interaction
Revocations survive broker restart, but signing keys are ephemeral (regenerated at startup). After restart, all pre-restart tokens fail signature verification before the revocation check runs. Pre-restart revocation entries in SQLite are therefore dead weight — they will never match a live token because all pre-restart tokens already fail signature verification. This is not a security problem. The `revocations` table has no `expires_at` column by design (see Fix 2 schema rationale in design doc). Safe cleanup is deferred; document in operator runbook that the table grows indefinitely until a safe cleanup PR lands.

### Fix 3 (Audience validation) — Optional when AA_AUDIENCE unset
When `AA_AUDIENCE` env var is not set (empty), audience validation should be skipped entirely — not fail-closed. This preserves backward compatibility for deployments that have never set an audience. When set, `ValidateWithAudience()` should verify the token's `Aud` slice contains the expected audience string. The `ValMw.Wrap()` in `internal/authz/val_mw.go` is the injection point — pass the configured audience from `cfg.Audience` into the middleware.

### Fix 5 (UDS) — Socket path timing
The sidecar ID (`state.sidecarID`) is only available after bootstrap completes (set in `bootstrap()` return value in `cmd/sidecar/main.go`). The socket path must therefore come from `AA_SOCKET_PATH` env var set at deploy time, not derived from the sidecar ID at runtime. If operators want per-sidecar paths, they configure `AA_SOCKET_PATH=/var/run/agentauth/myapp.sock` per deployment. Do not attempt to dynamically generate the socket path post-bootstrap.

### Fix 6 (Structured audit) — Hash coverage requirement
The `computeHash()` function in `internal/audit/audit_log.go:232-238` currently hashes: `prevHash | id | timestamp | eventType | agentID | taskID | orchID | detail`. When new fields (`Resource`, `Outcome`, `DelegDepth`, `DelegChainHash`, `BytesTransferred`) are added, they MUST be included in the hash input string. Omitting them from the hash means a tampered value in those fields would not break the chain — defeating tamper evidence. The Detail field should be kept for backward compatibility but populated alongside the new structured fields for existing callers during transition.

---

**END OF IMPLEMENTATION PLAN**
