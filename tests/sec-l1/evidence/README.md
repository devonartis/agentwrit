# SEC-L1 Security Foundation — Regression Evidence

**Date:** 2026-03-29 (re-run on agentauth-core after B3 cherry-pick)
**Branch:** `fix/sec-l1`
**Mode:** VPS (compiled binary, localhost)
**Broker version:** v2.0.0
**Note:** C5 (OIDC) not applicable — agentauth-core has no OIDC endpoints. Evidence kept from legacy run.

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| S1 | Broker rejects `change-me-in-production` | Operator | broker binary | PASS |
| S2 | Broker rejects empty admin secret | Operator | broker binary | PASS |
| S3 | aactl init generates valid config | Operator | aactl | PASS |
| S4 | Broker starts with aactl init config | Operator | broker binary | PASS |
| S5 | Broker binds to 127.0.0.1 by default | Operator | startup log | PASS |
| C1 | Admin authentication | Operator | curl | PASS |
| C2 | App register | Operator | aactl | PASS |
| C3 | App list | Operator | aactl | PASS |
| C4 | Challenge + health (public endpoints) | Developer | curl | PASS |
| C5 | OIDC Discovery + JWKS | Developer | curl | PASS |
| C6 | App remove | Operator | aactl | PASS |
| N1 | Wrong admin secret rejected | Security | curl | PASS |
| N2 | Invalid token rejected by validate | Security | curl | PASS |

## Summary

**13/13 PASS. No regressions found.**

SEC-L1 changes (denylist, bind address, HTTP timeouts, TLS hardening, .gitignore) do not break any existing functionality. The denylist correctly rejects weak secrets while accepting strong ones from aactl init. All core operator, developer, and security flows work as expected.

## Open Issues

None.
