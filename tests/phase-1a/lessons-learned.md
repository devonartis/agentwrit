# Phase 1a — Lessons Learned

**Date:** 2026-03-03
**Branch:** `feature/phase-1a-app-registration`

---

## 1. Sidecar has no defined use case

The sidecar was starting with every Docker stack despite the architecture decision (Session 20) that it's "optional." Optional for what was never defined. No story in Phase 1a uses it. No story in the PRD defines when it's needed.

**Action taken:** Removed sidecar from `docker-compose.yml` and `stack_up.sh` entirely. It can be added back when a concrete use case is documented in the PRD.

---

## 2. Acceptance tests must be the operator experience — not scripts

The agent kept trying to wrap acceptance tests in bash scripts, chained commands with `tee` and `echo`, and build automation. That's not an acceptance test.

An acceptance test is: an operator sits at a terminal, runs the commands the same way they would on a VPS, and sees the expected output. If that doesn't work, the tooling is broken — not the test.

**What went wrong:**
- Built `aactl` to `/tmp/` instead of a proper location
- Inlined `AACTL_BROKER_URL=... AACTL_ADMIN_SECRET=...` on every command instead of setting env once
- Used `go run ./cmd/aactl` instead of a compiled binary
- Used `curl` for health checks when `aactl` should be the tool
- Tried to write a test script instead of just running the commands

**Rule:** If you wouldn't do it on a VPS, don't do it in the test.

---

## 3. Where does the admin get the admin secret?

This is undocumented. The operator needs to know:
1. Generate a secret: `openssl rand -hex 32`
2. Deploy the broker with `AA_ADMIN_SECRET=<that value>`
3. Set their own shell: `export AACTL_ADMIN_SECRET=<same value>` and `export AACTL_BROKER_URL=http://<broker-host>:8080`

There is no `aactl init` or `aactl configure` command. The operator has to know the env var names and set them manually. This is a gap in operator tooling.

**Missing tools identified:**
- No documented install/build step for `aactl` (e.g. `make install` or `go install`)
- No `aactl configure` or onboarding command
- No documentation for "how an operator gets started from zero"

---

## 4. Credential handoff was not documented

The flow where the operator registers an app and hands `client_id` + `client_secret` to the 3rd party developer was implicit. No story or diagram showed this handoff clearly.

**Action taken:** Added a credential flow diagram and explicit handoff language to `tests/phase-1a-user-stories.md` (Story 1 and Story 6).

---

## 5. User stories were missing basic progressive testing

- No story verified the broker starts with zero apps (empty state)
- No story verified the app list grows incrementally (1 → 2 → 3 apps)
- Each app having distinct credentials was not explicitly tested

**Action taken:** Added Story 0 (empty state) and rewrote Story 2 (multi-app list progression).

---

## 6. `client_secret` format — `sk_live_` prefix

The user story acceptance criteria says `client_secret` should have an `sk_live_` prefix. The implementation generates a plain 64-char hex string without it. This needs to be resolved before the test can pass Story 1.

**Status:** Open — decision pending on whether to fix the implementation or update the criteria.
