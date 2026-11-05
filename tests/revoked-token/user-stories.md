# FIX-1 Revoked Token Audit — Acceptance Stories

**Issue:** devonartis/agentwrit#97
**Branch:** `fix/revoked-token-audit`
**Written by:** test-author, from the approved design and spec only (not from the code).
**Folder:** `tests/revoked-token/` (stories, `env.sh`, `evidence/`)

---

## The Plain-Language Version

AgentWrit gives AI agents short-lived passes (tokens). An operator can cancel a
pass early. That is called **revoking** it.

When someone uses a cancelled pass, two things must happen:

1. **The person holding the pass is told only "no."** The answer is the same
   plain refusal they would get for any bad pass: `401` and the words
   `token verification failed`. The word "revoked" must never appear. If it did,
   a thief would learn they had been caught.
2. **The operator is told everything.** The audit log gets one entry named
   `token_revoked_access` saying whose pass it was (agent, task, orchestrator),
   which door it was tried on, and that it was denied.

**The bug being fixed:** today, step 1 works but step 2 does not. The audit log
records the attempt as an ordinary `token_auth_failed` with no owner, so the
operator cannot tell a stolen cancelled pass from a typo. Two smaller fixes
ride along: running `awrit token release` a second time on the same token must
exit quietly with success, and the docs must say `401`, not `403`.

---

## Infrastructure Prerequisites

| Prerequisite | Purpose | Smoke Test Story | Status |
|-------------|---------|-----------------|--------|
| Compiled `./bin/broker` and `./bin/awrit` (Go 1.24+) | The system under test; never `go run` | RT-PC1 | NOT VERIFIED |
| Compiled `./bin/edsign` (built from `tests/sec-l2b/edsign`, Go stdlib only) | Lets a test agent sign the registration challenge | RT-PC1 | NOT VERIFIED |
| `curl` and `jq` | App/Developer HTTP calls; reading JSON | RT-PC1 | NOT VERIFIED |
| Docker + Docker Compose | Container mode (run first) | RT-PC2 | NOT VERIFIED |
| `AA_ADMIN_SECRET` exported in the shell (strong, 32+ chars) | Broker start-up and operator login | RT-PC2 | NOT VERIFIED |
| Running broker (container, then VPS) | Everything below | RT-PC2 | NOT VERIFIED |
| Test apps (normal and short-lived) and a working agent registration | Fixtures for every story | RT-PC3 | NOT VERIFIED |

If any prerequisite cannot be verified, live testing STOPS.

---

## How To Read These Stories

**Modes.** Every story runs in **Container mode first**, then **VPS mode**
(project rule in `CLAUDE.md`). Set `RT_MODE=container` or `RT_MODE=vps` before
sourcing `env.sh`. Each evidence file has one section per mode.

**Personas never mix.**
- **Operator** uses only `./bin/awrit`.
- **App / Developer** uses only `curl` (the helper functions in `env.sh` wrap
  `curl`; they add no behaviour). The app is a real outside program: it
  registers agents, holds their tokens and uses them.
- **Security Reviewer** uses whichever of the two proves the property.
- When a story needs an action by another persona (for example the operator
  cancelling a pass the app is holding), the step is labelled with who does it.

**Counting audit events.** Several stories count audit events before and after
one action. The broker must have no other traffic during that moment, and the
counts are read by the Operator with `awrit audit events`. `rt_audit_count TYPE`
(from `env.sh`) prints the count of one event type.

**Reading one event in full.** The `awrit audit events` table shortens the
detail column and does not show the path, task or orchestrator. Use
`./bin/awrit audit events --json` to see every field (`agent_id`, `task_id`,
`orch_id`, `outcome`, `resource`, `detail`).

**Evidence hygiene.** Never write a full token, `client_secret` or admin secret
into an evidence file. Show at most the first 20 characters, then `...`.
Evidence must not contain absolute local paths.

**The generic refusal.** In these stories "the generic refusal" means:
HTTP status `401`, JSON field `detail` equal to `token verification failed`,
and the body contains no `revoked` in any capitalisation.

**One test agent per story.** Every story that needs an agent registers its own
with its own unique task id (the stories name them, for example `rt-task-s4`),
so cancelling "everything for task X" in one story cannot affect another.

---

## Preconditions

### RT-PC1: Tools and Compiled Binaries Are Present [PRECONDITION]

**Persona:** Operator / Security Reviewer
**Mode:** CLI-only (no broker needed)

Checks that the compiled programs and helper tools the tests rely on exist.

**Steps:**
1. `go version` (1.24 or newer), `jq --version`, `curl --version`.
2. `go build -o bin/broker ./cmd/broker`
3. `go build -o bin/awrit ./cmd/awrit`
4. `go build -o bin/edsign ./tests/sec-l2b/edsign`
5. `ls -l bin/broker bin/awrit bin/edsign`

**Expected:** every command succeeds; the three binaries exist and are
executable. Nothing in the later stories is run with `go run`.

---

### RT-PC2: Broker Is Up and the Operator Can Sign In [PRECONDITION]

**Persona:** Operator
**Mode:** Container, then VPS (separate runs)

**Steps (Container):**
1. `export AA_ADMIN_SECRET='<strong secret>'` (do not paste it into evidence).
2. `source ./tests/revoked-token/env.sh` (default `RT_MODE=container`).
3. `./scripts/stack_up.sh`
4. `curl -s "$BROKER_URL/v1/health"` (the status must say the broker is healthy).
5. `./bin/awrit app list`

**Steps (VPS):** stop the container (`docker compose down`), then
`RT_MODE=vps` and source `env.sh` again, start `./bin/broker` in a second
terminal (it reads the `AA_*` variables `env.sh` exports; admin secret via the
environment or `--admin-secret` as in the project docs), then repeat 4 and 5.

**Expected:** health check answers with a healthy status; `awrit app list`
exits 0 (an empty list is fine). If sign-in fails the operator login is broken
and no later story can run.

---

### RT-PC3: Test Apps and Agent Registration Work [PRECONDITION]

**Persona:** Operator (registers apps), then App (signs in, registers one agent)
**Mode:** Container, then VPS

Creates the fixtures used by every story.

**Steps:**
1. Operator, normal app:
   `./bin/awrit app register --name rt-app --scopes "$RT_SCOPE" --json > "$RT_WORKDIR/app.json"`
2. Operator, short-lived app (its sign-in tokens last only a few seconds, used
   to obtain an expired token):
   `./bin/awrit app register --name rt-app-short --scopes "$RT_SCOPE" --token-ttl "$RT_SHORT_TTL" --json > "$RT_WORKDIR/app-short.json"`
3. App: `rt_app_login app.json` then `echo "${APP_TOKEN:0:20}..."`
4. App: `rt_new_agent rt-agent-pc3 rt-orch rt-task-pc3` then
   `echo "$AGENT_ID"` and `echo "${AGENT_TOKEN:0:20}..."`

**Expected:** both registrations succeed (the files contain `client_id` and
`client_secret`; do not print the secret); the app receives a token; the agent
receives an `agent_id` beginning `spiffe://` and a token. If the short-lived
app's TTL of `RT_SHORT_TTL` seconds is rejected by the broker, raise
`RT_SHORT_TTL` and record the change in the evidence.

---

## Acceptance Stories

### RT-S1: A Developer's App Holding a Revoked Agent Token Is Refused [ACCEPTANCE]

**Persona:** App (curl). The operator's cancel step is labelled.
**Mode:** Container, then VPS
**Real outside consumer:** yes. A separate program (the app) that registered the
agent, holds its token, and uses it against the broker's protected endpoint.
**Covers:** C1, C6 (before-state), design Developer story.

The developer's app has an agent working for it. The operator decides the
agent must stop and cancels its pass. The app, unaware, keeps using the pass.
The app must get the same plain refusal it would get from an expired pass, so
its code has one path: get a new token. It must not be told the pass was
cancelled.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s1 rt-orch rt-task-s1`.
2. App, before cancelling (shows the pass is good): `rt_call POST /v1/token/validate "" "{\"token\":\"$AGENT_TOKEN\"}"` then `cat "$RT_BODY"`.
3. Operator: `./bin/awrit revoke --level token --target "$(jwt_claim "$AGENT_TOKEN" jti)"`
4. App, the pass is now cancelled:
   `rt_call POST /v1/token/renew "$AGENT_TOKEN"` then `cat "$RT_BODY"`
5. App, the public check apps use to decide whether to trust an agent:
   `rt_call POST /v1/token/validate "" "{\"token\":\"$AGENT_TOKEN\"}"` then `cat "$RT_BODY"`

**Expected:**
- Step 2: `200`, `"valid": true`.
- Step 4: status `401`; the generic refusal; body does NOT contain `revoked`
  (case-insensitive: `grep -i revoked "$RT_BODY"` finds nothing).
- Step 5: `200` with `"valid": false` and the generic text `token is invalid or
  expired`; body does NOT contain `revoked`. (Audit behaviour of this public
  endpoint is out of scope for FIX-1 and is NOT asserted here.)
- The app received no new token in step 4 (no `access_token` in the body).

---

### RT-S2: A Revoked Pass Teaches the Holder Nothing [ACCEPTANCE]

**Persona:** Security Reviewer (curl for the holder's view)
**Mode:** Container, then VPS
**Covers:** C1, design Security Reviewer story 1.

A thief steals a pass, the operator cancels it, and the thief tries it. The
reviewer proves the broker's answer is indistinguishable from the answer for a
pass that never existed, so the thief cannot tell they have been caught.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s2 rt-orch rt-task-s2`.
2. Operator: `./bin/awrit revoke --level agent --target "$AGENT_ID"`
3. Reviewer, the revoked pass:
   `rt_call POST /v1/token/renew "$AGENT_TOKEN"` ; save the status as `S_REV`;
   `cp "$RT_BODY" "$RT_WORKDIR/body-revoked.json"`
4. Reviewer, a garbage pass:
   `rt_call POST /v1/token/renew "not-a-real-token"` ; save status as `S_BAD`;
   `cp "$RT_BODY" "$RT_WORKDIR/body-garbage.json"`
5. Compare: `jq -S 'del(.request_id)' "$RT_WORKDIR/body-revoked.json"` and the
   same for `body-garbage.json`. Also `diff` the two outputs.
6. Reviewer: `grep -ic revoked "$RT_WORKDIR/body-revoked.json"`, and look at
   response headers with `curl -si -X POST "$BROKER_URL/v1/token/renew" -H "Authorization: Bearer $AGENT_TOKEN" | grep -i revoked`.

**Expected:**
- `S_REV` and `S_BAD` are both `401`.
- The two bodies are identical apart from `request_id` and `instance`-style
  per-request values (the `diff` shows nothing else); `detail` is
  `token verification failed` in both.
- The `grep` counts in step 6 are `0`, and no response header contains "revoked".

---

### RT-S3: The Operator Finds the Revoked-Pass Use and Can Tell Whose It Was [ACCEPTANCE]

**Persona:** Operator (awrit) after an App action
**Mode:** Container, then VPS
**Covers:** C2, R1, design Operator story 1.

The operator's reason for revoking a pass is to know if it is used again. After
the cancelled pass is tried, the operator opens the audit log and sees a single
clear entry saying which agent's pass it was, which task, and which door it was
tried on.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s3 rt-orch-s3 rt-task-s3`.
2. Operator: `./bin/awrit revoke --level token --target "$(jwt_claim "$AGENT_TOKEN" jti)"`
3. Operator, counts before: `rt_audit_count token_revoked_access`; `rt_audit_count token_auth_failed` (write down both: `R0`, `F0`).
4. App uses the cancelled pass once: `rt_call POST /v1/token/renew "$AGENT_TOKEN"`
5. Operator, counts after: `rt_audit_count token_revoked_access` (`R1`); `rt_audit_count token_auth_failed` (`F1`).
6. Operator, the table view:
   `./bin/awrit audit events --event-type token_revoked_access --agent-id "$AGENT_ID"`
7. Operator, the full view:
   `./bin/awrit audit events --event-type token_revoked_access --agent-id "$AGENT_ID" --json`

**Expected:**
- Step 4: the generic refusal (`401`).
- `R1 - R0 = 1` (exactly one new revoked-use event) and `F1 - F0 = 0` (no
  `token_auth_failed` for this request).
- Step 6 lists the event under the agent's SPIFFE id (= `$AGENT_ID`).
- Step 7 shows exactly one event for this agent with:
  `event_type` `token_revoked_access`; `agent_id` = `$AGENT_ID`;
  `task_id` = `rt-task-s3`; `orch_id` = `rt-orch-s3`; `outcome` `denied`;
  `resource` `/v1/token/renew`; `detail` `revoked token used | path=/v1/token/renew`.
- The event text contains no part of the token itself.

---

### RT-S4: Token-Level Revocation Is Refused and Audited [ACCEPTANCE]

**Persona:** App (uses the pass), Operator (revokes and checks)
**Mode:** Container, then VPS
**Covers:** C3 (token level), C1, C2.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s4 rt-orch rt-task-s4`.
2. Operator: `./bin/awrit revoke --level token --target "$(jwt_claim "$AGENT_TOKEN" jti)"` (expect count 1).
3. Operator: record `R0=$(rt_audit_count token_revoked_access --agent-id "$AGENT_ID")` and `F0=$(rt_audit_count token_auth_failed)`.
4. App: `rt_call POST /v1/token/renew "$AGENT_TOKEN"`; `cat "$RT_BODY"`.
5. Operator: `R1=...`, `F1=...` as in step 3; then
   `./bin/awrit audit events --event-type token_revoked_access --agent-id "$AGENT_ID" --json`.

**Expected:** step 4 is the generic refusal; `R1-R0 = 1`; `F1-F0 = 0`; the event
names `$AGENT_ID` and task `rt-task-s4`, outcome `denied`, resource
`/v1/token/renew`.

---

### RT-S5: Agent-Level Revocation Is Refused and Audited [ACCEPTANCE]

**Persona:** App (uses the pass), Operator (revokes and checks)
**Mode:** Container, then VPS
**Covers:** C3 (agent level).

Cancelling an agent cancels every pass it holds.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s5 rt-orch rt-task-s5`.
2. Operator: `./bin/awrit revoke --level agent --target "$AGENT_ID"`
3. Operator: record `R0`/`F0` as in RT-S4.
4. App: `rt_call POST /v1/token/renew "$AGENT_TOKEN"`; `cat "$RT_BODY"`.
5. Operator: record `R1`/`F1`; show the event with `--json`.

**Expected:** same as RT-S4, with task `rt-task-s5`.

---

### RT-S6: Task-Level Revocation Is Refused and Audited for Every Agent in the Task [ACCEPTANCE]

**Persona:** App, Operator
**Mode:** Container, then VPS
**Covers:** C3 (task level).

Cancelling a task cancels the passes of all agents working on it. Two agents
share the task so the story shows the operator can tell them apart.

**Steps:**
1. App: `rt_app_login app.json`.
2. App: `rt_new_agent rt-agent-s6a rt-orch rt-task-s6`; keep `A_TOKEN=$AGENT_TOKEN`, `A_ID=$AGENT_ID`.
3. App: `rt_new_agent rt-agent-s6b rt-orch rt-task-s6`; keep `B_TOKEN`, `B_ID` likewise.
4. Operator: `./bin/awrit revoke --level task --target rt-task-s6` (expect count of at least 2).
5. Operator: record `R0=$(rt_audit_count token_revoked_access)` and `F0`.
6. App: `rt_call POST /v1/token/renew "$A_TOKEN"`, then the same with `"$B_TOKEN"`.
7. Operator: `R1`, `F1`; then
   `./bin/awrit audit events --event-type token_revoked_access --task-id rt-task-s6 --json`.

**Expected:** both calls in step 6 get the generic refusal; `R1-R0 = 2`
(exactly one event per use); `F1-F0 = 0`; the listing shows one event naming
`$A_ID` and one naming `$B_ID`, both with task `rt-task-s6`, outcome `denied`.

---

### RT-S7: Chain-Level Revocation Is Refused and Audited [ACCEPTANCE]

**Persona:** App (delegates and uses the pass), Operator
**Mode:** Container, then VPS
**Covers:** C3 (chain level).

Agent A hands part of its authority to agent B (a delegation). The operator
cancels the whole chain starting at A. B's delegated pass must be refused.

**Steps:**
1. App: `rt_app_login app.json`.
2. App: `rt_new_agent rt-agent-s7a rt-orch rt-task-s7a`; keep `A_TOKEN`, `A_ID`.
3. App: `rt_new_agent rt-agent-s7b rt-orch rt-task-s7b`; keep `B_ID`.
4. Agent A delegates to B:
   `rt_call POST /v1/delegate "$A_TOKEN" "{\"delegate_to\":\"$B_ID\",\"scope\":[\"$RT_SCOPE\"],\"ttl\":300}"` (expect `200`);
   `D_TOKEN=$(jq -r .access_token "$RT_BODY")`.
5. App: confirm the delegated token works before cancelling (do not use renew, it retires the token). Use
   `rt_call POST /v1/token/validate "" "{\"token\":\"$D_TOKEN\"}"` and expect `"valid": true`.
6. Operator: `./bin/awrit revoke --level chain --target "$A_ID"`
7. Operator: record `R0=$(rt_audit_count token_revoked_access)`, `F0`.
8. App: `rt_call POST /v1/token/renew "$D_TOKEN"`; `cat "$RT_BODY"`.
9. Operator: `R1`, `F1`; then
   `./bin/awrit audit events --event-type token_revoked_access --json`.

**Expected:** step 8 is the generic refusal; `R1-R0 = 1`; `F1-F0 = 0`; the new
event names the delegated token's agent (`$B_ID`) as `agent_id`, outcome
`denied`, resource `/v1/token/renew`. (See open question 3 in the report: the
event's `task_id` for a delegated token is not stated in the spec; record what
is shown but do not fail on it.)

---

### RT-S8: Every Authenticated Door Records the Door That Was Tried [ACCEPTANCE]

**Persona:** Security Reviewer (curl), Operator for audit
**Mode:** Container, then VPS
**Covers:** C2, R1 (resource = request path), "any authenticated route".

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s8 rt-orch rt-task-s8`; keep `S8_TOKEN=$AGENT_TOKEN` and `S8_ID=$AGENT_ID`.
   Then `rt_new_agent rt-agent-s8b rt-orch rt-task-s8b` and keep `B_ID=$AGENT_ID` (a real delegate target).
2. Operator: `./bin/awrit revoke --level agent --target "$S8_ID"`
3. Reviewer, with the revoked pass, call three doors, noting each status:
   - `rt_call POST /v1/token/renew "$S8_TOKEN"`
   - `rt_call POST /v1/delegate "$S8_TOKEN" "{\"delegate_to\":\"$B_ID\",\"scope\":[\"$RT_SCOPE\"]}"`
   - `rt_call POST /v1/token/release "$S8_TOKEN"`
4. Operator: `./bin/awrit audit events --event-type token_revoked_access --agent-id "$S8_ID" --json`

**Expected:** all three calls return the generic refusal (`401`, no
"revoked"). Step 4 shows exactly three events for `$S8_ID`, one each with
`resource` `/v1/token/renew`, `/v1/delegate`, `/v1/token/release`, each with
`outcome` `denied` and `detail` `revoked token used | path=<that path>`. No
`token_released` event is written for the refused release.

---

### RT-S9: Bad Passes (Expired, Tampered, Garbage, Never-Issued) Get the Same Refusal and No Revoked Event [ACCEPTANCE]

**Persona:** Security Reviewer (curl), Operator for counts
**Mode:** Container, then VPS
**Covers:** C4.

Each kind of bad pass is tried once. The refusal must be identical to the
revoked case, but the audit log must call it an ordinary failure, because it
is not a cancelled pass.

**Steps:** for each case, Operator records `R0=$(rt_audit_count token_revoked_access)`
and `F0=$(rt_audit_count token_auth_failed)`, the Reviewer makes the call, and
the Operator records `R1`, `F1`.
1. Expired: `rt_app_login app-short.json`; keep `E_TOKEN=$APP_TOKEN`;
   `sleep "$RT_EXPIRY_WAIT"`; `rt_call POST /v1/token/renew "$E_TOKEN"`.
2. Tampered: `rt_app_login app.json`; `rt_new_agent rt-agent-s9 rt-orch rt-task-s9`;
   `rt_call POST /v1/token/renew "$(rt_tamper_token "$AGENT_TOKEN")"`.
3. Garbage: `rt_call POST /v1/token/renew "not-a-real-token"`.
4. Never issued: `rt_call POST /v1/token/renew "$(rt_forge_token spiffe://agentwrit.local/agent/x/y/z)"`.
5. Missing header entirely: `rt_call POST /v1/token/renew ""`.
6. Operator: `./bin/awrit audit events --event-type token_auth_failed --limit 10`.

**Expected:** every call returns the generic refusal (`401`,
`token verification failed`, no "revoked"); cases 1-4 each give `F1-F0 = 1` and
`R1-R0 = 0`. Case 5 may or may not write an audit event (record what happens;
it must not write `token_revoked_access`). The `token_auth_failed` listing
shows these attempts, with an empty agent id.

---

### RT-S10: A Forgery Naming a Revoked Agent Cannot Plant That Name in the Audit Log [ACCEPTANCE]

**Persona:** Security Reviewer
**Mode:** Container, then VPS
**Covers:** C4, spec edge case "forged token naming a revoked subject".

An attacker copies the identity out of a cancelled pass and signs a fake. The
broker must treat it as a plain bad pass. Otherwise an attacker could frame an
innocent agent in the audit log.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s10 rt-orch rt-task-s10`.
2. Operator: `./bin/awrit revoke --level agent --target "$AGENT_ID"`
3. Operator: record `R0=$(rt_audit_count token_revoked_access --agent-id "$AGENT_ID")`, `F0=$(rt_audit_count token_auth_failed)`.
4. Reviewer: `rt_call POST /v1/token/renew "$(rt_forge_for "$AGENT_TOKEN")"`; `cat "$RT_BODY"`.
5. Operator: `R1`, `F1`.

**Expected:** generic refusal (`401`); `R1-R0 = 0`; `F1-F0 = 1`.

---

### RT-S11: A Pass That Is Both Expired and Revoked Is Logged as an Ordinary Failure [ACCEPTANCE]

**Persona:** Security Reviewer, Operator
**Mode:** Container, then VPS
**Covers:** R3.

An expired pass is harmless whether or not it was also cancelled. The audit
entry `token_revoked_access` means "a still-live pass that was cancelled was
used"; this story proves the deliberate edge case.

**Steps:**
1. App: `rt_app_login app-short.json` and keep `E_TOKEN=$APP_TOKEN` (valid for `RT_SHORT_TTL` seconds). Do not use it yet.
2. Operator, immediately: `./bin/awrit revoke --level token --target "$(jwt_claim "$E_TOKEN" jti)"` (expect count 1).
3. Wait: `sleep "$RT_EXPIRY_WAIT"`.
4. Operator: record `R0=$(rt_audit_count token_revoked_access)`, `F0=$(rt_audit_count token_auth_failed)`.
5. App: `rt_call POST /v1/token/renew "$E_TOKEN"`; `cat "$RT_BODY"`.
6. Operator: `R1`, `F1`.

**Expected:** generic refusal (`401`, no "revoked"); `F1-F0 = 1`; `R1-R0 = 0`.
If step 2 reports the token had already expired, repeat with a larger
`RT_SHORT_TTL`.

---

### RT-S12: A Good Pass Still Works and Raises No Alarm [ACCEPTANCE]

**Persona:** App (curl)
**Mode:** Container, then VPS
**Real outside consumer:** yes (the app uses its own agent's token).
**Covers:** C6 (regression guard).

A pass that has not been cancelled must keep working exactly as before, and
must not trigger either alarm entry in the audit log.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s12a rt-orch rt-task-s12a` (keep `A_TOKEN`); `rt_new_agent rt-agent-s12b rt-orch rt-task-s12b` (keep `B_ID`).
2. Operator: record `R0`, `F0` (all `token_revoked_access`; all `token_auth_failed`).
3. App: `rt_call POST /v1/delegate "$A_TOKEN" "{\"delegate_to\":\"$B_ID\",\"scope\":[\"$RT_SCOPE\"],\"ttl\":60}"` ; `cat "$RT_BODY"`.
4. App: `rt_call POST /v1/token/renew "$A_TOKEN"` ; keep `NEW_TOKEN=$(jq -r .access_token "$RT_BODY")`. (Renewing retires the old token by design; so this is done last.)
5. App: `rt_call POST /v1/token/validate "" "{\"token\":\"$NEW_TOKEN\"}"` ; `cat "$RT_BODY"`.
6. Operator: `R1`, `F1`.

**Expected:** step 3 `200` with an `access_token` and a `delegation_chain`;
step 4 `200` with a new `access_token`; step 5 `200` with `"valid": true`;
`R1-R0 = 0` and `F1-F0 = 0`.

---

### RT-S13: Releasing a Token Twice Succeeds Quietly [ACCEPTANCE]

**Persona:** Operator (awrit only) after App setup
**Mode:** Container, then VPS
**Covers:** C7, design Operator story 2.

Operators and scripts retry. Releasing a token that is already released must
not look like a failure.

**Steps:**
1. App: `rt_app_login app.json`; `rt_new_agent rt-agent-s13 rt-orch rt-task-s13`.
2. Operator: `./bin/awrit token release --token "$AGENT_TOKEN"` ; `echo "exit=$?"`
3. Operator: `R0=$(rt_audit_count token_revoked_access --agent-id "$AGENT_ID")`
4. Operator, again, same token: `./bin/awrit token release --token "$AGENT_TOKEN"` ; `echo "exit=$?"`
5. Operator, a made-up token: `./bin/awrit token release --token "$(rt_forge_token spiffe://agentwrit.local/agent/x/y/z)"` ; `echo "exit=$?"`
6. Operator: `R1=$(rt_audit_count token_revoked_access --agent-id "$AGENT_ID")`; `./bin/awrit audit events --agent-id "$AGENT_ID"`.

**Expected:**
- Step 2: prints `Token released successfully.`; `exit=0`.
- Step 4: prints exactly `Token is not active (already released, revoked or expired) — nothing to release.`; `exit=0`; no `HTTP 401` text and no error.
- Step 5: the same "not active" message; `exit=0` (accepted trade-off: a mistyped token looks the same).
- `R1-R0 = 1`: the second release is itself recorded as a revoked-pass use for
  this agent. The audit listing also shows one `token_released` for the first release.

---

### RT-S14: The Audit Trail Stays Intact and Leaks Nothing After All of This [ACCEPTANCE]

**Persona:** Security Reviewer (awrit and curl)
**Mode:** Container, then VPS
**Covers:** design security principle (no secrets in the log), regression.
Run last, after the other stories in the same mode.

**Steps:**
1. Operator: `./bin/awrit audit events --limit 1000 --json > "$RT_WORKDIR/audit-all.json"`.
2. Reviewer: for each token and secret used in the run, search the file, for example
   `grep -c "$AGENT_TOKEN" "$RT_WORKDIR/audit-all.json"` and
   `grep -c "$(jq -r .client_secret "$RT_WORKDIR/app.json")" "$RT_WORKDIR/audit-all.json"` and
   `grep -c "$AA_ADMIN_SECRET" "$RT_WORKDIR/audit-all.json"`.
3. Reviewer: `jq -r '.events[] | select(.event_type=="token_revoked_access") | .detail' "$RT_WORKDIR/audit-all.json" | sort | uniq -c`.
4. Reviewer: `curl -s "$BROKER_URL/v1/health"`.

**Expected:** every `grep -c` prints `0`; every revoked-use detail has the form
`revoked token used | path=/v1/...`; the hash chain fields (`hash`, `prev_hash`) are present on every event. The broker is still healthy.

---

## Coverage Matrix

| Requirement | Stories |
|---|---|
| C1 revoked -> 401, generic text, no "revoked" | RT-S1, RT-S2, RT-S3, RT-S4-S8 |
| C2 + R1 exactly one `token_revoked_access`, owner, denied, path, no `token_auth_failed` | RT-S3, RT-S4-S8 |
| C3 four levels | RT-S4 (token), RT-S5 (agent), RT-S6 (task), RT-S7 (chain) |
| C4 expired / forged / garbage / never-valid | RT-S9, RT-S10 |
| R3 expired AND revoked | RT-S11 |
| C6 valid token still works | RT-S12 (RT-S1 step 2 for before-state) |
| C7 release twice | RT-S13 |
| Operator finds the use in the audit log | RT-S3 |
| Developer: same 401 as expired | RT-S1, RT-S2 |
| Security: thief learns nothing | RT-S2 |
| Security: failed check hands no claims | RT-S1 (no `access_token` returned), RT-S8 (all three doors refuse); the "no claims" rule itself is a unit-test property |
