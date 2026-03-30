# SEC-L2b: HTTP Security Hardening — Live Test Evidence

**Branch:** `fix/sec-l2b`
**Date:** 2026-03-30
**Stack:** Broker compiled binary (VPS mode) on macOS
**Broker version:** Built from fix/sec-l2b branch (B0-B5 cherry-picks)

## What Changed

This batch adds three security improvements to every HTTP response the broker sends:

1. **SecurityHeaders middleware** — X-Content-Type-Options, X-Frame-Options, Cache-Control on all responses
2. **Global body limit** — 1 MB maximum request body on all endpoints (prevents denial of service)
3. **Error sanitization** — validation, renewal, and auth middleware errors now return generic messages instead of revealing internal details

## Story Results

| Story | Description | Persona | Tool | VPS | Container |
|-------|------------|---------|------|-----|-----------|
| S1 | App gets generic error for bad token | App | curl | PASS | — |
| S2 | App gets generic error for revoked token | App | curl | PASS | — |
| S3 | Tampered token rejected without detail leak | Security | curl | PASS | — |
| S4 | Security headers on all responses | Security | curl | PASS | — |
| S5 | HSTS present when TLS enabled | Security | curl | SKIP | — |
| S6 | Oversized body rejected (413) | Security | curl | PASS | — |

**S5 SKIP reason:** HSTS header is only added when `AA_TLS_MODE=tls` or `mtls`. VPS test runs without TLS certificates. This is by design — HSTS on plain HTTP would cause browsers to refuse future connections.

## Container Mode

Container mode was verified separately via `./scripts/test_batch.sh B5 --all` (G4-G6 PASS) and the full `integration.sh` acceptance run against Docker. See `.plans/cherry-pick/TESTING.md` for gate evidence.

## Open Issues

None. All five planned items verified. Code review passed (see `B5-review.md`).
