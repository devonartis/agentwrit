# P1: Admin Secret (Bcrypt + aactl init) — Live Test Evidence

**Date:** 2026-03-29 (re-run on agentauth-core after B2 cherry-pick)
**Branch:** `fix/p1-admin-secret`
**Broker version:** v2.0.0
**Binaries:** `./bin/broker`, `./bin/aactl`
**Admin secret:** `live-test-secret-32bytes-long-ok` (gate tests), per-story generated (acceptance)

## Testing Modes

Every broker story was tested in **VPS mode first** (compiled binary on host),
then **Container mode second** (Docker). CLI-only stories test `aactl init`
without involving the broker. See `docs/internal/dev-qa-guide.md` for details.

## Story Results

| Story | Description | Persona | Mode | Verdict |
|-------|------------|---------|------|---------|
| S1 | Generate admin secret (dev mode) | Operator | CLI only | PASS |
| S2 | Generate admin secret (prod mode) | Operator | CLI only | PASS |
| S3 | Refuse to overwrite existing config | Operator | CLI only | PASS |
| S4 | Force-overwrite config | Operator | CLI only | PASS |
| S5a | Config file boot (VPS) | Operator | VPS | PASS |
| S5b | Config file boot (Container) | Operator | Container | PASS |
| S6 | Backward compat — env var works | Developer | VPS + Container | PASS |
| S7 | Env var overrides config file | Developer | VPS + Container | PASS |
| S8 | Dev mode startup warning | Operator | VPS + Container | PASS |
| S9 | Bcrypt timing resistance | Security | VPS | PASS |

## Security Reviews (Step 8.5)

| Domain | Agent | Findings | Evidence |
|--------|-------|----------|----------|
| Bcrypt + Secrets | Agent 1 | 2 CRITICAL, 2 IMPORTANT | [security-review-bcrypt-secrets.md](security-review-bcrypt-secrets.md) |
| aactl init CLI | Agent 2 | 1 CRITICAL, 3 IMPORTANT | [security-review-aactl-init.md](security-review-aactl-init.md) |
| Config Precedence | Agent 3 | 1 CRITICAL, 1 IMPORTANT | [security-review-config-precedence.md](security-review-config-precedence.md) |

**Consolidated:** 4 CRITICAL, 6 IMPORTANT. Fix plan at `.plans/p1-security-fix-plan.md`.

## Open Issues

Security findings pending fix — see fix plan.
