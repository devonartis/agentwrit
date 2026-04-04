# MEMORY Archive — agentauth-core

Archived lessons and session history. See MEMORY.md for current context.

---

### B6 Session (2026-03-30) — CRITICAL lessons learned

**What went wrong — user corrections:**

1. **Agent kept skipping banners on acceptance tests.** User had to stop me THREE times because I jumped straight to running curl commands without writing the Who/What/Why/How/Expected banner first. The template is non-negotiable. Banner goes IN the bash call, not as a separate step. Verdict comes AFTER seeing output, never pre-written.

2. **Agent built the first acceptance test against the admin flow instead of the app flow.** User caught it: "why are we using launch-token from admin to check agents?" In production, APPS create launch tokens for agents, not admin. Admin registers apps, apps manage agents. The agent didn't know this because nothing in the code or docs explained the role model.

3. **Agent called the handler ownership issue a "code smell" when it was actually a missing foundational document.** User walked me through why `admin:launch-tokens:*` exists (admin needs authority over launch tokens for revocation/oversight) and why admin creating agents is the wrong use of that scope. The agent kept downgrading the severity because it didn't understand the system's intent. User escalated: "you are writing code that you are not properly documenting the code nor giving app documentation."

4. **Agent tried to fix test failures inline instead of running all tests first.** User corrected: "why are you not running acceptance tests all of them then we search on we fix afterwards it is a loop." Run everything, see what fails, then fix. Don't stop to debug after every failure.

5. **Agent put tech debt in MEMORY.md.** User: "that is stupid we should have a TECH-DEBT.md." Then agent put it in `.plans/TECH-DEBT.md`. User: "that should be on the root not in the .plans folder." TECH-DEBT.md already existed at `.plans/` — agent didn't check first before trying to create a new file.

6. **Agent wrote code comments that restated what the code does.** User corrected: "a person or agent can read the code by itself to know what it does." Comments must tell you what reading the code alone would NOT tell you: who calls it, why it exists, security boundaries, design history. If you have to read three other files to understand who can call a function, the comments are insufficient.

**What we discovered — golden information:**

- **Code comments are the interface between human intent and agent execution.** Multiple agent sessions wrote and reviewed code without flagging that the role model was undocumented. Each agent looked at the code, made assumptions, and moved on. Comments that explain roles and boundaries would have prevented every mistake in this session. Without them, agents compound wrong assumptions across sessions.
- **If comments are strong, you can generate missing docs FROM the comments.** If comments are weak, you can't build docs, you can't build correct tests, and agents keep making the same mistakes. Strong comments → correct tests → correct docs. Weak comments → compounding errors.
- **The three roles are: Admin (operator — manages apps, revokes, audits), App (software — manages its own agents within scope ceiling), Agent (does work with short-lived scoped tokens).** This was nowhere in the code or docs. Now in TECH-DEBT.md as TD-012 (CRITICAL) and partially in code comments on `tkn_svc.go`.
- **`admin:launch-tokens:*` scope makes sense for oversight (list, inspect, revoke launch tokens) but the code lets admin CREATE launch tokens with no scope ceiling.** That's a design issue (TD-013), not a code smell. Admin-created agents have no AppID, no scope ceiling, no traceability.
- **Regression unit tests belong BEFORE the gate suite**, not after. New Step 3 in cherrypick-devflow. The tests get included in G2 (unit tests gate), catching regressions before spending time on Docker builds and acceptance tests.
- **Think through the test plan BEFORE writing code.** The agent kept jumping to curl commands, hitting wrong field names, wrong endpoints, wrong flows — all because it didn't verify the API contract first. Banner-first forces you to think about WHO does WHAT before typing a single command.

---

### B5 Acceptance Testing (2026-03-30) — CRITICAL lessons

- **Acceptance tests are NOT integration scripts.** `integration.sh` runs PASS/FAIL checks but cuts corners: no individual story files, no executive-readable banners, no proper personas. It's a CI smoke test. Real acceptance tests produce individual `story-*.md` files per the `LIVE-TEST-TEMPLATE.md`.
- **Executives and QA testers read acceptance evidence.** Every banner (Who/What/Why/How/Expected) must make sense to a non-technical reader. Write for the executive, not the engineer.
- **Personas must reflect production reality.** "Developer (curl)" is wrong when the real actor is an automated App. Ask: "Who does this in production?" App = automated software. Developer = human exploring. Operator = human managing. Security Reviewer = verifying controls.
- **Ground every story in reality.** If using curl to emulate an app, say so: "We emulate what the app does in production." Don't describe testing mechanics — describe the real-world scenario.
- **Legacy acceptance tests need deep adaptation.** The legacy `integration.sh` had: wrong response field names (`token` vs `access_token`), wrong request field names (`allowed_scopes` vs `allowed_scope`, missing `agent_name`), wrong registration flow (simple name+scopes vs challenge-response with Ed25519), wrong nonce encoding (base64 vs hex), OIDC endpoints that don't exist in core. Every field must be verified against actual handler structs.
- **One story at a time, verdict earned.** Don't pre-write PASS. Run the story, see the output, then write the verdict based on what you actually observed.
- **LIVE-TEST-TEMPLATE updated** with: "Who Reads These Tests?" section, App persona, "Ground Every Story in Reality" guidance, Bad/Good banner examples.

### B5 Cherry-Pick (2026-03-30) — technical lessons

- B5: Commit `247727c` was empty after conflict resolution — content already present from `e592acc`. Skipped safely.
- B5: `e592acc` conflict in `main.go` contained OIDC routes and cloud handler. All dropped — add-on code.
- B5: Missing `context` and `errors` imports in `handler_test.go` after cherry-pick. LSP diagnostics caught it.
- B5: `curl -sI -X POST` returns empty headers for POST endpoints — use `curl -s -D - -o /dev/null` instead to dump headers on POST requests.
- jcodemunch indexes code symbols only — not markdown docs. Use context-mode for doc analysis.
- `settings.json` (project, committed) vs `settings.local.json` (personal, gitignored). Broad tool permissions go in project-level.
- Post-merge doc verification caught 2 critical inaccuracies: middleware ordering was backwards in architecture.md (19 route rows + prose), MaxBytesBody attributed to wrong source file in implementation-map.md. Fixed. Always verify docs against actual code after sub-agent updates.
- `cherrypick-devflow` skill updated: added Step 4 (Application Docs) and Step 5 (Acceptance Tests). Skill now has `references/acceptance-examples.md` with real bash examples showing how to create story evidence files.
- Skills use `references/` directory for companion docs that get loaded on demand. Keeps SKILL.md lean (<500 lines) while providing examples and detailed guidance.

## Archived Lessons (B0-B2 era)

- The original agentauth repo was a file COPY (not clone) of agentauth-internal — that's why it had no history. This time we cloned properly so all 412 commits are preserved.
- Phase 1C-alpha (`3f9639f`) looks clean but has `hitl_scopes` baked into the app data model in 4 source files. Fork point must be `2c5194e` (TD-006) to get truly zero HITL.
- SEC-L1/L2a/L2b commits are on the P2 branch which also has OIDC code. Cherry-picks from these commits may have OIDC context in conflict markers — always check for IssuerURL, federation, thumbprint, jwk references and drop them.
- `cfg.go` is the most conflict-prone file — it gets modified by P1, SEC-L1, and SEC-L2a. Each batch adds fields to the same struct.
- B0 sidecar removal cherry-pick (`34bb887`) conflicted in 5 files (MEMORY.md, tkn_svc.go, renew_hdl.go, sql_store.go x3, admin_hdl_test.go). Key resolution: remove SidecarID from IssueReq/claims, remove ScopeCeiling from renewResp, remove sidecar CRUD/tables from store, remove stale sidecar comment from routes. Keep app-level code intact.
- Cherry-pick brings stale files from agentauth (flow.md, .vscode/, .plans/production-gap-analysis.md) — always unstage and discard these before committing.
- G6 smoke test failed with 401 because `test_batch.sh` used a different secret than `docker-compose.yml`'s default. Root cause: `docker-compose.yml` has `AA_ADMIN_SECRET=${AA_ADMIN_SECRET:-change-me-in-production}` — if the export doesn't reach the container, it gets the wrong secret. Fix: export `AA_ADMIN_SECRET` at script level BEFORE any Docker commands.
- `live_test_docker.sh` still references sidecar (`broker sidecar` in compose commands) — tracked as TD-S03. Needs decision: delete or rewrite.
- B2 (P1): cfg.Load() now returns (Cfg, error) — breaking API change, all callers updated. Admin auth uses bcrypt.CompareHashAndPassword, not subtle.ConstantTimeCompare.
- B2 conflicts: cfg.go had HITL fields (HITLApprovalTTL) — dropped. admin_hdl_test.go had HITL gate tests (~300 lines) — dropped entirely. CHANGELOG.md and docs/api.md had sidecar sections — dropped.
- Config file security: symlink rejection (os.Lstat + ModeSymlink), permission checks (rejects looser than 0600), O_EXCL atomic creation. All from security review fix commits.
- `~/.agentauth/config` on the host machine causes cfg tests to fail — they pick it up as a fallback. Fix: set `HOME` to `t.TempDir()` in tests, or delete the file. Tracked by test isolation fixes.
- Security review fix commits reference finding IDs (C-1, I-3, etc.) in commit messages — keep this pattern for traceability.
- Tech debt added: TD-S06 (rate limiting on admin auth), TD-S07 (post-migration doc refresh).
- Docker image name is `agentauth-core-broker` (not `agentauth-broker`). Container mode tests must use the correct image name.
