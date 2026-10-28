# Evidence — fix/security-bumps (security dependency and toolchain bumps)

| | |
|---|---|
| **Who** | The Operator (runs the broker and manages it with the `awrit` command-line tool) and a Developer (calls the broker over HTTP with curl). |
| **What** | This branch updates the building blocks AgentWrit is made from: the Go language toolchain (1.25.11 → 1.26.10), the OpenSSL library inside the Docker image, and the `x/crypto` library. Nothing about how AgentWrit behaves was meant to change. We started the real product — first as the Docker image users run, then as a plain program on the computer — and used it the way customers do. |
| **Why** | These updates close 10 known security holes in the Go language (including one where a stolen or compromised certificate authority could still be trusted) and 3 high-severity holes found by the Docker image scan (two in OpenSSL, one in `x/crypto`). If the update had broken the product, agents could not get credentials, revoked agents might keep working, or the operator could not manage apps — and nobody would know until a customer hit it. |
| **How to run** | 1. Check out `fix/security-bumps`. 2. Start the product in Docker: `export AA_ADMIN_SECRET="$(openssl rand -base64 32)"` then `./scripts/stack_up.sh`. 3. Run all quality checks against it: `./scripts/gates.sh full`. 4. Use the operator tool: `AACTL_BROKER_URL=http://127.0.0.1:8080 AACTL_ADMIN_SECRET="$AA_ADMIN_SECRET" ./bin/awrit app register --name check --scopes 'read:data:*'`, then `awrit app list`, `awrit audit events`, `awrit app remove --id <app_id>`. 5. Stop Docker, build `./bin/broker`, start it, and run `./scripts/smoke/core-contract.sh` against it. |
| **Expected** | Every quality check passes, the Docker image contains Go 1.26.10, the 10-step customer flow (log in → issue a credential → agent signs in → credential checked → agent revoked → revoked credential refused → over-reaching request refused) passes in both Docker and plain-program mode, and the operator can register, list, audit and remove an app. |

**Verdict: PASS** — every check below passed. Raw output: [`full-testing.log`](full-testing.log).

## What was tested

- **Branch:** `fix/security-bumps` at `60c5378` (7 commits on top of GitHub `develop` `4920f10`).
- **Date:** 2026-10-28 00:27–00:29 UTC, on the owner's Mac (Apple Silicon).
- **Order:** Container mode first, VPS mode second.

| # | Mode | Check | Result |
|---|---|---|---|
| 1 | Container | `stack_up.sh` clean build (`--no-cache`) and start | PASS — health `ok`, database connected |
| 2 | Container | Go version of the binary inside the image | PASS — `go1.26.10` |
| 3 | Container | `gates.sh full` — build, vet, lint, format, contamination, unit tests, gosec, govulncheck, go-mod-verify, race tests, docker-build, smoke-l25, sbom, no-tracked-ignored | PASS — 14 of 14, 0 failed, 0 skipped |
| 4 | Container | Core-contract smoke (inside `gates.sh`, against the container) | PASS — 10 of 10 |
| 5 | Container | Operator: `awrit app register`, `app list`, `audit events`, `app remove` | PASS — all exit 0; app registered, listed as active, audit shows `app_registered`, removed (credentials revoked) |
| 6 | VPS | `./bin/broker` rebuilt, reports `go1.26.10` | PASS |
| 7 | VPS | Core-contract smoke against the bare binary | PASS — 10 of 10 |

Tool versions match CI's pins: golangci-lint v1.64.8, gosec v2.25.0, govulncheck v1.1.4 (all built with go1.26.10); syft 1.50.0.

## What was not tested

- **`gates.sh regression`** — not run. The only regression suites (`tests/f00`, `tests/fix1`) test code that is not on this branch yet.
- **TLS / mTLS compose overlays** — not run; this branch does not touch TLS code.
- **GitHub CI on the push** — not yet; the branch has not been pushed.

## Notes for reviewers

- **The Docker database was not empty at start.** Health reported `audit_events_count: 7` because the Docker volume `awrit-secbump_broker-data` was kept from an earlier smoke run on this branch the same evening. All checks passed on top of that existing data; a run on a fresh volume was not repeated.
- **`awrit app register --json` returns empty `name` and `status` fields**, while `awrit app list` shows the same app with name `secbump-check` and status `active`. Not caused by this branch (no app or CLI code changed); recorded for follow-up.
- **Secrets:** each mode used a fresh random admin secret that was never printed. Secret fields were removed from the operator output before logging. The log was scanned for secrets, passwords, bearer headers and tokens: none found.
- **Local sandbox:** the run used temporary build caches and installed the pinned tools into a temporary folder; no product files were changed by this.
