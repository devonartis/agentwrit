# Live Test Guide

This is the step-by-step guide for how live tests are written and executed in this project. Every phase and fix must produce a live test following this process. The Docker stack must be running — self-hosted binary tests are NOT live tests.

Read this entire document before writing or running any test.

---

## What Is a Live Test?

A live test is an operator, developer, or security reviewer sitting at a terminal, running commands against the real Docker stack, and recording what happened. It is NOT a script, NOT a bash chain, NOT automation. It's a person doing the thing and saving the evidence.

---

## Directory Structure

Every phase or fix gets its own directory under `tests/`:

```
tests/<phase-or-fix>/
  user-stories.md     — all stories with personas, steps, acceptance criteria
  env.sh              — environment variables (source once before testing)
  evidence/
    README.md         — summary table with verdicts + open issues
    story-N-<name>.md — one file per story with banner + output + verdict
```

---

## Step 1: Write User Stories First

Before writing any code or running any test, write the user stories. Each story says who is doing what and why, in plain language.

```markdown
### P0-S3: Sidecar Activate Endpoint Is Gone

The security reviewer calls the old endpoint where a sidecar exchanged
its activation token for a bearer token. It should no longer exist.

**Route:** POST /v1/sidecar/activate
**Tool:** curl
**Expected:** 404
```

**Personas and their tools — never mix these:**
- **Operator** — uses `aactl` commands. Operators don't hand-craft HTTP.
- **Developer** — uses `curl` / HTTP client. Developers have no CLI.
- **Security Reviewer** — uses whichever tool proves the security property.

---

## Step 2: Set Up the Environment

Before running any test:

1. Build aactl to `./bin/aactl` — not `/tmp/`, not `go run`
2. Run `./scripts/stack_up.sh` to bring up the Docker stack
3. Verify the broker is healthy: `curl http://127.0.0.1:8080/v1/health`
4. Source the environment file once: `source ./tests/<phase>/env.sh`

The env.sh file sets the broker URL and admin secret so you don't repeat them on every command:

```bash
#!/usr/bin/env bash
export BROKER_URL=http://127.0.0.1:8080
export AACTL=./bin/aactl
export AACTL_BROKER_URL=$BROKER_URL
export AACTL_ADMIN_SECRET=change-me-in-production
```

---

## Step 3: Run Each Story and Record Evidence

This is the most important part. Each story is run ONE AT A TIME. The banner comes first, then the command runs, and the output is piped directly into the evidence file. The banner and the output are ONE thing — they go into the file together in a single call.

### How the Coding Agent Must Execute Each Story

The coding agent runs each story as a single bash call that:
1. Writes the banner (who, what, why, how, expected) into the evidence file
2. Runs the actual command and pipes the output into the same file
3. Appends the verdict
4. Displays the complete file so the user can see the full evidence

**This is how a call looks for a curl story:**

```bash
F=tests/phase-0/evidence/story-S3-sidecar-activate-gone.md
cat > "$F" << 'BANNER'
# P0-S3 — Sidecar Activate Endpoint Is Gone

Who: The security reviewer.

What: Before Phase 0, the broker had a route at POST /v1/sidecar/activate
where a sidecar exchanged its one-time activation token for a bearer token.
This was the most security-sensitive part of the sidecar flow — it's where
tokens were issued. We removed it because there are no sidecars in the stack.

Why: If this route still responds, someone with a stolen activation token could
potentially get a bearer token from the broker.

How to run: Source the environment file. Then send a POST to the old sidecar
activation URL on the broker.

Expected: HTTP 404 — the route no longer exists.

## Test Output

BANNER
source ./tests/phase-0/env.sh && curl -s -w "\nHTTP %{http_code}" \
  -X POST "$BROKER_URL/v1/sidecar/activate" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After that runs, the agent reads the output and adds the verdict:

```bash
echo "PASS — The broker returned 404. The old sidecar activate route is fully removed." >> "$F"
```

**This is how a call looks for an aactl story:**

```bash
F=tests/phase-0/evidence/story-R1-register-app.md
cat > "$F" << 'BANNER'
# P0-R1 — Operator Registers a New App

Who: The operator.

What: The operator registers a new app called cleanup-test on the broker
using aactl. This is a regression test — app registration is the core
Phase 1A feature. We need to confirm it still works after removing the
sidecar routes and changing the admin login format in Phase 0.

Why: If app registration broke during the Phase 0 cleanup, it means the
cleanup damaged something it shouldn't have.

How to run: Source the environment file. Then run aactl app register with
the app name and scopes. Save the credentials — they're needed for R2, R3,
and R4.

Expected: The broker creates the app and returns app_id, client_id, and
client_secret. The CLI warns to save the secret.

## Test Output

BANNER
source ./tests/phase-0/env.sh && ./bin/aactl app register \
  --name cleanup-test --scopes "read:data:*,write:logs:*" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

### Key Rules for the Coding Agent

- **One story at a time.** Run one, get the output, record the verdict, then move to the next. Do NOT fire multiple stories in parallel — you lose the output.
- **Banner goes IN the call.** The who/what/why/how/expected is part of the bash command that writes the evidence file. It is not a separate step.
- **Output pipes into the file.** The command output goes directly into the evidence file with `>> "$F" 2>&1`. You don't copy-paste later.
- **Display the file after.** End every call with `cat "$F"` so the user sees the complete evidence.
- **Verdict is based on what you see.** After the call completes and you see the output, append the verdict. Don't pre-write "PASS" before you see the result.

---

## Step 4: What the Evidence File Looks Like When It's Done

This is a real completed evidence file from Phase 0. An executive, a QA reviewer, or another coding agent should be able to read this and understand exactly what happened without knowing anything about curl or HTTP:

```markdown
# P0-R4 — Audit Trail Records All the Activity

Who: The operator.

What: The operator pulls the full audit trail from the broker to check that
everything that happened during these tests was recorded. The audit trail is
how the operator knows what's going on — every app registration, every login,
every failed request gets logged. The operator checks for two specific events:
the app registration from R1 (app_registered) and the developer login from R2
(app_authenticated). The operator also scans the entire trail to make sure no
client_secret values leaked into the logs.

Why: If audit events are missing, the operator loses visibility into the system.
If secrets appear in audit records, that's a security breach. Both would be
serious regressions.

How to run: Source the environment file. Then run aactl audit events. Look for
app_registered and app_authenticated events. Check that no client_secret values
appear anywhere.

Expected: app_registered and app_authenticated events present. No client_secret
values in any event.

## Test Output

ID          TIMESTAMP                       EVENT TYPE         AGENT ID                     OUTCOME  DETAIL
evt-000001  2026-03-04T14:34:11.469587841Z  admin_auth                                      success  admin authenticated as admin
evt-000002  2026-03-04T14:35:15.451494926Z  admin_auth                                      success  admin authenticated as admin
evt-000003  2026-03-04T14:35:15.721592801Z  app_registered                                  success  app=cleanup-test client_id=ct-09ccbf99777a scopes=[read:d...
evt-000004  2026-03-04T14:35:45.641544759Z  app_authenticated                               success  client_id=ct-09ccbf99777a app_id=app-cleanup-test-c0e7b8
evt-000005  2026-03-04T14:36:08.137592047Z  scope_violation    app:app-cleanup-test-c0e7b8  denied   scope_violation | required=admin:audit:* | actual=app:lau...
evt-000006  2026-03-04T14:36:26.78621875Z   admin_auth                                      success  admin authenticated as admin
Showing 6 of 6 events (offset=0, limit=100)

## Verdict

PASS — All events recorded: app_registered (evt-000003), app_authenticated
(evt-000004), scope_violation from R3 (evt-000005). No client_secret values
in any event. Audit trail is complete.
```

---

## The Banner — What It Must Contain

Every evidence file starts with a plain language banner. This is NOT optional. This is what makes the evidence readable by anyone.

The banner has five parts:

| Part | What it says | Example |
|------|-------------|---------|
| **Who** | Which persona is doing this | "The security reviewer." |
| **What** | What they're doing and what changed, in plain English | "Before Phase 0, the broker had a route where a sidecar exchanged its activation token for a bearer token. We removed it because there are no sidecars in the stack." |
| **Why** | Why this test matters — what breaks if it fails | "If this route still responds, someone with a stolen activation token could get a bearer token from the broker." |
| **How to run** | Step-by-step instructions a QA person can follow | "Source the environment file. Then send a POST to the old sidecar activation URL on the broker." |
| **Expected** | What the output should be, in plain language | "HTTP 404 — the route no longer exists." |

### Banner Language Rules

**Write it like you're explaining to a manager, not an engineer.**

GOOD: "The operator tries to log in to the broker using the command line tool. Before this fix, the login required two fields — a username and a password. Now it only requires the password."

BAD: "The operator authenticates with the broker using the new admin auth shape. The broker validates the shared secret using constant-time comparison and returns a short-lived admin JWT."

GOOD: "If this route still responds, someone with a stolen activation token could get a bearer token from the broker."

BAD: "If the endpoint is still registered in the mux, the sidecar bootstrap flow remains exploitable via token replay."

---

## Evidence README

The `evidence/README.md` summarizes all stories in one table:

```markdown
# Phase 0 — Legacy Cleanup: Live Test Evidence

**Date:** 2026-03-04
**Branch:** `fix/phase-0-legacy-cleanup`
**Stack:** Broker only (no sidecar in docker-compose)
**Broker version:** v2.0.0

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| P0-S1 | Sidecar list endpoint is gone | Security | curl | PASS |
| P0-R1 | Regression: register app | Operator | aactl | PASS |

## Open Issues

None.
```

---

## Rules

1. **Docker required.** `./scripts/stack_up.sh` must run first. Self-hosted binary tests are NOT live tests.
2. **Stories first.** Write user stories before writing any test code or running any command.
3. **Personas matter.** Operator uses `aactl`. Developer uses `curl`. Never mix.
4. **Banner is mandatory.** Every evidence file starts with who/what/why/how/expected in plain language.
5. **Plain language.** An executive should be able to read the evidence and understand what happened. No jargon, no unexplained flags, no abbreviations.
6. **One story at a time.** Run one, record the output, write the verdict, then move to the next. Don't fire multiple stories in parallel.
7. **Output goes in the file.** The command output pipes directly into the evidence file. Don't copy-paste later.
8. **One file per story.** Named `story-N-<slug>.md`.
9. **Source env.sh once.** Don't inline env vars on every command.
10. **Build aactl to `./bin/aactl`.** Not `/tmp/`, not `go run`.
11. **Verdict is earned.** Don't write PASS before you see the output. Read the result, then write the verdict.
