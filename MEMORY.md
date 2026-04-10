# MEMORY.md — agentauth-core

## Recent Lessons (last 3 sessions — older archived to MEMORY_ARCHIVE.md)

### M-sec CI/build/gates v1 shipped — all-nighter (2026-04-10, Phase A–D execution)

**What happened:** Ran the CI/gates strategy from [[Obsidian KB Decision 015]] straight through to shipped. 03:40 decision → 09:00 running on main. 31/31 tasks done. Three PRs merged (PR #3 main implementation, PR #4 strip-script mid-merge fix, PR #5 README badges + `.vscode/` gitignore). Two `develop → main` strip merges landed clean: `a72a959` and `4213cf8`. Both branches now protected behind `gates-passed`. Decision 016 written to capture the reasoning shift behind Decision 014 (policy unchanged, justification updated). Rewrote the `obsidian:log` skill mid-session to support dual-write mode with log voice + journal voice as separate shapes.

**What we discovered — golden information:**

- **"Don't improvise around automated safety nets."** The strip script refused to run mid-merge because of its dirty-tree guard — its own documented merge flow was impossible to execute. My first instinct was to manually `git rm` the conflicted files and proceed. User caught it immediately: "we spent three hours automating CI so we never depend on manual runs, and now we were about to depend on a manual process that isn't even documented. Fix the script." The strip script is the ONLY automated barrier between private `develop` and public `main`. Improvising past it ONCE establishes the precedent that improvising is OK and eventually something sensitive escapes. This is the most important lesson of the session. Write it on the wall.

- **Prebuilt action binaries are a hidden dependency on THEIR build toolchain, not yours.** `golangci-lint-action@v6.5.2` with `version: v1.64.8` took 3 CI iterations to diagnose. The pre-built `golangci-lint` binary was compiled against Go 1.22 — my `go.mod` has `toolchain go1.25.9` (required for the stdlib CVE fixes). 1.22-era linter crashes on the SSA analysis pass when parsing 1.25 code. Local developers don't see it because brew's `golangci-lint` is built against whatever Go the Homebrew bottle tracks (currently 1.25). Fix: dropped the action entirely, `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8` on the CI runner so it compiles the linter against the runner's Go, matching `go.mod`. Rule of thumb: when you're on the leading edge of a language version, install language tooling from source so the runner's compiler does the work.

- **GHAS gates three workflows on private repos: dep-review, CodeQL SARIF upload, Scorecard SARIF upload.** $49/committer/month to enable, free on public repos. User's call was defer — repo flips public in ~1 week, cost isn't worth the coverage gap, and we only have 5 direct deps anyway. Remaining coverage while parked: `govulncheck` (stdlib + Go module CVEs live, blocking), `gosec` (app-layer static analysis), contamination grep, L2.5 smoke. All three deferred workflows parked on `workflow_dispatch:` only with job bodies preserved. Re-enable is a one-line revert after public flip. Tracked as TD-VUL-005/006 with a consolidated fix sequence.

- **jq's `//` operator treats `false` as empty.** `jq -r '.valid // empty'` on `{"valid": false}` returns `""`, not `"false"`. Caught because I ran the L2.5 smoke against a live broker before pushing — if I'd trusted the unit tests alone, the smoke would have failed in CI and I'd have been chasing a phantom "revocation not enforced" bug. Use plain `.valid` when you need to distinguish `false` from missing.

- **`git merge -X ours` does not resolve modify/delete conflicts.** It only handles content conflicts where both sides modified the same region. Modify/delete is structural and needs explicit `git rm` (keep deletion) or `git add` (keep modification). I tried `-X ours` as a shortcut and it still left the files as `DU` (deleted-unmerged).

- **Plan drafts are wrong about wire format more often than they are about architecture.** The plan had `/v1/revoke` accepting `{"kind": "agent", "id": "..."}`. The real handler wants `{"level": "agent", "target": "..."}`. Verified by reading `internal/handler/revoke_hdl.go`. **Rule: read the handler, not the plan, for DTO shapes.** Same class of bug that made acceptance tests in B6 reach for the wrong role model.

- **Bidirectional defense layers must enforce the SAME list.** Audit during the strip fix found `.vscode/` missing from both `strip_for_main.sh` and `.githooks/pre-commit`, AND `adr/` in the strip script but not in the pre-commit hook. Two layers disagreeing is worse than one layer — you think you have defense in depth but the overlap isn't where you think it is. Added both missing entries and a sync-note comment in the hook pointing at the strip script. **Rule: when two files enforce the same invariant, they need a note telling future editors to keep both in sync.**

- **VSCode's watch loop recreated `.vscode/settings.json` between `rm` and `git commit`.** The first strip merge leaked it — the script ran `git rm .vscode/settings.json`, staged the deletion, half a second later VSCode wrote the file back for Snyk IDE prefs, `git add -A` restaged it, merge commit landed with `.vscode/settings.json` on main. I caught it with `git ls-tree -r HEAD | grep vscode` right after the commit. The pre-commit hook should have caught it but didn't because `git config core.hooksPath .githooks` is a per-clone setup step I'd never run in this checkout. **Three layers of defense now active:** (1) `.vscode/` in `.gitignore` (PR #5), (2) strip script removes it on merge, (3) pre-commit hook blocks commits to main. I want at least two of the three to hold when I make a mistake, and tonight I needed exactly two because the hook wasn't installed. Also: after cloning any repo with `.githooks/`, immediately run `git config core.hooksPath .githooks`.

- **Unbounded background tasks must be killed explicitly.** A `docker compose logs -f broker` shell sat in `ps aux` for ~90 minutes because I forgot to kill it after its "monitor" purpose ended. User caught it and said "that shell should not be running it was 30 minutes ago." `run_in_background: true` + `<task-notification>` only works for bounded commands (like `go test -race`, `gh run watch --exit-status`) that eventually exit on their own. For unbounded tails, the pattern breaks. **Rule: check `ps aux | grep -E "gh run watch|go test|docker compose logs"` between phases to catch ones that slip through.**

- **GitHub Actions job IDs cannot contain `.`** — `smoke-l2.5` was invalid and `actionlint` caught it before first push. Renamed to `smoke-l25` in 4 places (ci.yml job, ci.yml `gates-passed` needs list, ci.yml `GATE_LIST` comment, gates.sh `GATES_FULL` array). Kept "L2.5" as the test-taxonomy documentation label — only the machine-readable identifier dropped the period.

- **Gosec uses JSON for config even though the file is named `.gosec.yml`.** The extension is historical and misleading. Wrote YAML the first time and gosec refused to load it: `"Failed to load config: invalid character '#' looking for beginning of value"`. Rewrote as JSON, problem solved. Keep the `.yml` extension for consistency with other linter configs but the content MUST be JSON.

- **Contribution policy reasoning shifted tonight without the policy changing.** Decision 014's original reason for "no PRs" was "can't verify PRs safely without manual review work." CI now catches bad PRs mechanically — that reason no longer holds. But the policy still holds because the NEW reason is capacity: I haven't decided to accept the review cost, which is about bandwidth and coaching, not safety. Wrote Decision 016 to capture the shift so future-me doesn't see the CI badges and misread Decision 014 as obsolete. **Rule when reasoning changes but a policy doesn't: write a new decision that updates the reasoning. Don't edit the old decision in place.**

**User corrections (golden — blog material):**

1. **"Fix the script, don't improvise around the safety net."** The session-defining correction. When the strip script blocked me mid-merge and I was about to manually `git rm` the conflicted files, user pulled me back. "We spent three hours automating CI so we never depend on manual runs." This reframed the entire remaining work. The right response to an automated safety net blocking you is to fix the safety net, not bypass it.

2. **"Why is this in my voice — this is where we should be using a log, not a journal note."** I wrote the project work log in first-person journal voice with "I kept thinking X, then realized Y" narrative beats. User corrected: work logs are for reconstruction by future-you, they should be terse/past-tense/factual ("Cut branch. Wrote file. Ran test. 3 passed, 2 failed. Fixed by X."). Daily notes are for first-person journaling. Two different voices, not one. I conflated them repeatedly and had to rewrite the work log AND rewrite the `obsidian:log` skill to explicitly separate the two voices.

3. **"It is definitely a log, this is not how people write in their journal in my voice."** After I rewrote the daily note entry once, it was STILL changelog-shaped with bold headers per lesson. User had to correct me again. The pattern I keep slipping into is bold-header changelog format (`**Don't X**`). Fix: re-read Divine's actual recent entries (01:30 AM rebrand, 03:40 AM CI strategy) before drafting, and explicitly block bold headers in the skill's validation step (`awk ... | grep -c '^\*\*[A-Z]'` must return 0).

4. **"We should be creating different docs instead of one continuous long thing that goes on for years."** User's preferred pattern for work logs: each session gets its own dated file (`YYYY-MM-DD-slug.md`) in the project KB folder, not appending to a single rolling log. Matches the existing `showcase-authagent/2026-03-08-sessions.md` pattern. One file per session, never one file per project.

5. **"Why are we searching when you should know? Let's stop NOW and write the obsidian log like the decision flow."** When Divine asked where AgentAuth work logs go in the KB, I started doing a directory search. User pulled me back — I should have just known from the `obsidian:decision` skill's existing config (`~/.claude/obsidian-projects.json`). The right move was to look at the config I'd already written, not to re-explore the vault. Rule: when there's a config file, check it before searching.

6. **"WTF — you did not add an AgentAuth entry, we always have a strict rule to bi-link, so what did you do."** I wrote a 2000-word dump into the daily note without creating the matching AgentAuth work-log file, AND the wikilinks I added were one-way (daily note → project files, no backlinks from project files to the daily note). Rule: bidirectional always. Forward link on daily, backlink on project file, matching pair. If you write only one side, it's a silent graph failure.

7. **"The content should be a new document in the folder, not the same document. I think the old log was just appending to one doc."** Clarifying what (4) meant: the old `obsidian:log` skill was appending everything to the daily note. The new pattern is write the detail to a NEW file in the project KB folder and a short summary to the daily note. One session = one new work log file + one daily summary entry + bidirectional links.

8. **"This is dangerous because we had this automated — if you forget something, then we will make it public."** When I was contemplating manually stripping files from the main-bound merge, user reminded me of the blast radius. The strip-for-main flow isn't just about convenience — it's the safety net that prevents accidentally publishing dev artifacts when the repo flips public. Manual stripping once means the muscle memory to do it again next time, and eventually something sensitive slips.

9. **"That shell should not be running, it was 30 minutes ago."** The stale `docker compose logs -f` shell. User was watching `ps aux` in a separate terminal while I worked and caught it. The background task pattern (`run_in_background: true` + wait for notification) works fine for bounded commands but fails open for unbounded tails that never exit.

10. **"Not the skill — run the skill to add the logs. Did we complete the log correctly?"** When I kept editing the skill instead of testing it by using it, user redirected me: stop updating the skill, use the skill to produce the outputs, verify the outputs are correct, THEN move on. Rule: tool-building has a natural stopping point where you have to use the tool and validate the output. Don't get stuck in the build loop.

**Session thoughts:**

- **The feedback loop between "decision recorded" and "decision shipped" was the thing to optimize for.** Decision 015 was written at 03:40 AM as a strategy doc. The pipeline was running on real code at 06:28 AM. Three hours from "this is what we're going to build" to "it's running." Most of my code changes over the past year sit in a spec for weeks and by the time I come back to them half the context is gone. Tonight the loop stayed tight enough that every correction Divine made landed while the constraints were still alive in memory. When you can keep the loop that tight, you should — the correction cost is much lower than the re-derivation cost of stale context.

- **The `obsidian:log` skill refactor mid-session was the right call.** The old skill was a 93-line stub that conflated daily notes and work logs. The new skill is 385 lines with: project detection via `~/.claude/obsidian-projects.json`, dual-write mode, mandatory voice calibration (read real entries before drafting), explicit separation of log voice (past-tense, factual, subject=work) from journal voice (first-person, discursive, subject=me), length discipline on daily note summaries (≤30 lines, enforced by awk validation), bold-header anti-pattern detection (grep in validation step), bidirectional link enforcement (grep in validation step). The rewrite was a detour but without it the log would have kept failing the same way.

- **The strip script bug was always there — we just hadn't exercised it.** The dirty-tree guard was added for safety ("don't strip on top of uncommitted work") but it conflicted with the documented mid-merge flow ("merge develop --no-commit, then strip"). Nobody noticed because the first develop→main merge on 2026-04-04 was a fast-forward with no conflicts — the script ran on a clean tree and the guard never fired. Tonight was the first merge with actual conflicts and the guard blew up on its own documented usage. **Rule: test automation against the messy path, not just the happy path.** A safety guard that blocks the documented use case is the same bug category as a test that doesn't test anything.

- **The two strip merges (`a72a959` and `4213cf8`) were the only direct pushes to `main` in the session.** Both went through `git push` instead of PRs because branch protection has `enforce_admins: false` (I'm admin, I bypass). That's intentional for the maintainer-only workflow, but it's worth noticing: I used the admin bypass for strip merges because those aren't reviewable content (they're mechanical strip+commit+push, fully automated) but I did NOT use the admin bypass for any other work. PR #3, PR #4, PR #5 all went through real PR review (self-review) and CI gates. **Maintainer admin bypass is for the strip merges only. Everything else goes through PR + gates.**

- **Rewriting the `obsidian:log` skill showed me that the `obsidian:decision` skill's structure was doing a lot of work.** Dual-write, config-driven project detection, voice calibration, bidirectional links, index updates, validation — none of that was invented for the log skill, it was adapted from the decision skill. The `obsidian:decision` skill I built earlier today was the template. **General rule: once you've built a good skill, the next skill in the same family is a translation, not a ground-up build.** Recognize the shared structure and reuse it.

- **Decision 016 is the kind of decision I want more of: "the reasoning changed but the policy didn't."** Most decisions change either the policy or the reasoning. A decision that only updates the reasoning is rare and valuable because it prevents the "we shipped CI, so Decision 014 must be obsolete" misreading that future-me would absolutely make. The decisions folder now has an audit trail of BOTH what we decided and WHY we decided it, and the reasoning can evolve without losing the policy history.

**What's NOT done (handoff to next session):**

- **PR #5 `docs/readme-badges-gitignore` has been merged and strip-merged to main.** No follow-up needed on that branch.
- **TD-019 domain registration** — blocks SECURITY.md contact, CLA legal text, CODE_OF_CONDUCT.md contact. Domain is `agentwrit.com` per Decision 013. Needs DNS setup + email-alias configuration + update of all placeholder `@agentauth.dev` / `@agentauthdev._` references.
- **`docs/` directory refactor** — user identified tonight that it has no table of contents, files are scattered, some files leak meta-tags that shouldn't be public-facing. Needs a new tech debt entry in `TECH-DEBT.md` and a separate cycle.
- **ADR for the M-sec technical architecture** — Decision 015 captures the strategic "why," but the specific "how" (exact workflow file structure, exact gate list, exact pinned-SHA approach) should live in `adr/` so it's coupled to the code.
- **AgentWrit rebrand execution** — now unblocked by the CI safety net. This is the work Decision 015 was specifically a prerequisite for.
- **Contribution policy exit-criteria list (Decision 016)** — 9 items, none done yet. The first one (repo flips public = Phase 4) is the gating event.
- **`.plans/specs/2026-04-10-ci-build-gates-msec-plan.md`** — the plan file is still in the repo even though all 31 tasks are complete. Can be moved to `.plans/archive/` or left as a reference. User's call.

### ADR vs Decision split, skill build, and branch cleanup (2026-04-10)

**What happened:** Long session that restructured how decisions get captured and cleaned up months of branch debt.

**The ADR vs Decision distinction (golden):** Earlier in the day the user noticed the `decisions/` directory in the repo was a grab bag — technical choices like "fork point" mixed with business choices like "open-core model" and "AGPL license." Restructured into two tracks:
- **ADRs** (`adr/` in the repo) — technical decisions about the code. Stay with the code on `develop`, stripped from `main`. 6 files.
- **Decisions** (Obsidian KB only, never in any repo) — strategy, licensing, business, marketing, cross-project thinking. 8 files at parent-project level in KB.

**The classification principle:** "If the deployed code changes because of this decision, it's an ADR. If not, it's a Decision." One sentence. Everything else follows. This replaced a 13-row lookup table as the primary classification rule — the table became examples of the principle, not the rule itself. Rebrand = marketing (code unchanged) = Decision. License = legal file change (wire format unchanged) = Decision. Fork point = defines what code exists = ADR. Code comment standard = changes how code looks = ADR.

**Built `/obsidian:decision` skill in three passes:**
1. First draft: rigid MUSTs, 9 hard rules, surface-all-decisions at start of every run
2. Audit by skill-creator agent exposed the failures — heavy-handed MUSTs conflict with the skill-creator guidance to "explain the why," classification was mechanical rather than principle-first, no validation step
3. Rewrite: "Why these rules exist" section replaces rigid block (each rule explains the failure it prevents), classification leads with principle, added `validate` step after writes that checks frontmatter fields + array types + wikilink resolution

**Then user pushed back on surface-all-decisions.** "Showing all decisions upfront burns credits and I don't need that — I can ask when I need it." Rewrote `surface_context` → `check_duplicate`: only runs a cheap title grep when topic overlap is suspected. Full listing only on explicit ask. Reading a specific decision (e.g. "what did we decide about licensing?") = grep for it, read the one file. **Lesson: read on demand, not on spec.**

**Obsidian-agent is the first tool for vault queries, not grep.** Installed obsidian-agent globally (44 tools). For vault lookup, `smart-search` (BM25 ranking) is the default, not grep. Added a short pointer in global `~/.claude/CLAUDE.md` (2 lines) with full reference at `~/.claude/skills/obsidian:decision/references/obsidian-agent-commands.md`. First attempt at the CLAUDE.md section was ~40 lines — user called it out: "way too much content for global, god forbid every entry was like this you write a book claude.md would not be optimized." Trimmed to 2 lines. **Lesson: global instructions stay lean, details live in reference files.**

**Decision 014 captured using the new skill end-to-end:** "No external contributions, bug reports only." The distinction: public visibility and accepting contributions are separate decisions. Open-source AGPL license ≠ accepting PRs. Bug *reports* welcome, bug *fix* PRs not accepted until the contribution workflow is documented and tested. The file at `KB/10-Projects/AgentAuth/decisions/014-no-external-contributions.md` has explicit exit criteria (test plan + merge plan + contribution guide + tested with one non-maintainer) so future-you knows when to supersede it.

**User corrections (golden — blog material):**
1. **"Contributor" scope was wrong initially.** Agent wrote Decision 014 framing as "bug fixes allowed, feature PRs not." User corrected: no bug fix PRs either — bug *reports* only. Every PR needs review/test/merge work. There's no such thing as a low-effort PR review. "Bug fix PRs" sounds safe but still needs the workflow.
2. **Reading places without permission.** Earlier I read the agentauth-python README when user asked about docs in agentauth-core. User called it out: "why are you reading places i did not give you access to read this session." Valid. Should have asked before reaching into another repo.
3. **Heavy-handed rules vs explained reasoning.** When writing the first skill draft I had 9 MUSTs and multiple "Never skip this step" phrases. Skill-creator audit + user pushback showed: rules that explain *why* they exist are more durable than rules enforced with threats. The "Why these rules exist" framing actually includes the historical incidents that motivated each rule.

**Memory is not append-only (session lesson):** Earlier memory tracked two "unlogged branches" (`fix/app-launch-tokens-endpoint` and `fix/docs-overhaul`) as pending FLOW.md entries. Both branches had been merged weeks ago. The memory entry stayed. Every session that loaded memory saw the stale reference and wasted attention confirming the branches were actually merged. **Rule: when merging a branch referenced in memory, update/delete the memory entry in the same session.** New feedback memory captures this: `~/.claude/projects/.../memory/feedback_clean_memory_before_merge.md`.

**The python agent didn't follow the skill.** A separate Claude session working in agentauth-python wrote a per-repo "Decision 001: rebrand" file AND created Decision 013 at parent level AND created an empty `agentauth-python-sdk/decisions/` KB folder — none of which matched what the skill would have done. The skill existed but that session didn't invoke it. Root causes: (1) rebrand was misclassified as an ADR when it's clearly a marketing decision, (2) skill wasn't invoked at all — possibly because the work predated the restructure we did tonight, but also because the session was creating decision files without consulting any capture skill. **The fix is the classification principle + the skill's default-to-Decision behavior + better trigger phrasing in the skill description.**

**Branch cleanup — 15 branches deleted:** Session ended with a full repo audit. Found 7 B0-B6 migration cherry-pick branches still existing months after merge, plus a `develop-harness-backup` (autonomous coding harness work already cherry-picked), a `devin/1775212397-add-wiki-pages` branch (unsolicited Devin PR that duplicated docs already in the repo and did a bad job), the merged `docs/readme-sdk-demo` branch, `whitesource/configure` auto-scanner branch, and two already-merged `fix/app-launch-tokens-endpoint` / `fix/docs-overhaul`. All gone. Repo now has exactly `develop` + `main` locally and remotely.

**Root cleanup:** Deleted 7 stray scratch files from root and `docs/` that had accumulated from mid-April "scratch pad" sessions — `DEVELOPMENT_STANDARDS.md`, `MiniMaxPythonSDK_REVIEW.md`, `SDK_BLUEPRINT.md`, `GeminiReview/` folder, `docs/python-sdk-design{,-v2,-final}.md` (three versions of the same SDK design doc that ended up in the broker repo by mistake).

**Merged `docs/readme-sdk-demo` to develop** as `511dde6`. The branch carried the CONTRIBUTING rewrite (which is now inconsistent with Decision 014 — needs follow-up update on develop), the README SDK section (questionable value, user was skeptical earlier), the ADR structure, SECURITY fixes, and the root cleanup.

**What's NOT done (handoff to next session):**
- CONTRIBUTING.md update per Decision 014 (no external contributions) — current version still encourages PRs, inconsistent with new policy
- README SDK section — user was questioning its value; may need to remove or rework
- Domain placeholder emails in CLA.md, ENTERPRISE_LICENSE.md, SECURITY.md, CODE_OF_CONDUCT.md — per Decision 013 domain is `agentwrit.com`
- `docs/api/openapi.yaml` still says Apache 2.0
- `docs/getting-started-developer.md` needs SDK link

---

### Public release readiness session (2026-04-08)

**What happened:** Implemented the “public release readiness” plan: created `.plans/release-readiness.md` (merge checklist, license tradeoffs Apache vs source-available), `.plans/reviews/public-release-review-2026-04-08.md` (structured review snapshot), updated `AGENTS.md` / `FLOW.md`, fixed `CONTRIBUTING.md` (wrong clone URL, wrong import path, obsolete `smoketest` in tree), fixed `SECURITY.md` (stale limitations + broken KNOWN-ISSUES link), added `CODE_OF_CONDUCT.md`.

**Standing rule:** License is now AGPL-3.0 + CLA + enterprise summary. All four files (`LICENSE`, `CLA.md`, `ENTERPRISE_LICENSE.md`, `CONTRIBUTING.md`) must stay in sync on license references. Domain name decision (TD-019) blocks going public — all contact emails are placeholder.

**What's NOT done:** GitHub public flip; external security audit; domain name decision (TD-019); `docs/getting-started-developer.md` needs SDK link; OpenAPI spec still says Apache 2.0.

**Pending review branches (2026-04-08):**
- **Core:** `docs/readme-sdk-demo` — SDK section + MedAssist demo showcase in README
- **Python SDK:** `docs/readme-license-cleanup` — MIT LICENSE file, pyproject fix, README rewrite with demo section
- These depend on each other — core README links to SDK repo content. Review together.

---

## Mission

**Build the open-source core of AgentAuth** — a production-grade, pluggable credential broker for AI agents implementing the **[Ephemeral Agent Credentialing v1.3](https://github.com/devonartis/AI-Security-Blueprints/blob/main/patterns/ephemeral-agent-credentialing/versions/v1.3.md)** security pattern.

**Core principles:**
- **Pattern-driven:** Every feature, fix, and design decision traces to the v1.3 pattern document. The code implements all 8 core components.
- **Pluggable architecture:** The core is designed so enterprise modules (HITL, OIDC provider, Resource Server, MCP integration, cloud credential exchange, federation bridge) can plug in without modifying core code. Interfaces and extension points over hard-coded integrations.
- **Zero add-on contamination:** No HITL, OIDC, cloud, federation, or sidecar code in this repo. Those are enterprise modules that plug into the core.
- **Minimal dependencies:** Ed25519/JWT/hash-chain/scope/revocation all use Go stdlib. Only 5 direct Go dependencies. Strong supply chain story.

## Origin

This repo was cloned from `agentauth-internal` at commit `2c5194e` (TD-006: Per-App JWT TTL). It contains all 8 v1.3 blueprint core components plus the complete app credential lifecycle. Zero HITL code — verified.

**Fork point:** `2c5194e` — all 8 core components + app registration + app launch tokens + per-app configurable TTL.

## Open-Core Model

AgentAuth uses an open-core model:

- **Core (this repo):** 8 blueprint components + App credential lifecycle. Pluggable extension points. Will become open-source.
- **Enterprise modules (separate repos, future):** HITL approval flow, OIDC provider, Resource Server, MCP integration, cloud credential exchange, federation bridge. Plug into core via interfaces. Stays private/enterprise.

Both the legacy repos are kept as private archives:

- `agentauth-internal` (git@github.com:devonartis/agentauth-internal.git) — 412 incremental commits, real feature-by-feature history
- `agentauth` (git@github.com:divineartis/agentauth.git) — production hardening commits, enterprise add-ons, migration planning docs

## Branching Model

GitFlow: `main` → `develop` → `fix/*` or `feature/*` branches. Cherry-pick batches use `fix/` branches merged to `develop`, then `develop` merged to `main` after verification.

## Current State

**Migration: B6 acceptance tests PASS — pending code review and merge (last batch).** B0-B5 merged. B6 on `fix/sec-a1` with all gates green and 4/4 acceptance stories PASS.

**Current branch:** `fix/sec-a1` — ready for merge after code review. Then post-migration cleanup (Go module path update, final verification, remote swap), then switch to `devflow` for new feature development.

## Key Documents (in legacy agentauth repo)

| Document | Path (in agentauth repo) | What |
|----------|-------------------------|------|
| Feature Inventory | `.plans/modularization/Cowork-Feature-Inventory.md` | Master inventory: milestones, cherry-pick list, delete list, execution steps |
| Cherry-Pick Guide | `.plans/modularization/Cherry-Pick-Guide.md` | Batch-by-batch cherry-pick instructions with conflict resolution guidance |
| Repo Directory Map | `.plans/modularization/Repo-Directory-Map.md` | What's in each repo, directory trees, quick reference |
| Feature Inventory (docx) | `.plans/modularization/Cowork-Feature-Inventory.docx` | Word doc version of the inventory |

## Cherry-Pick Batches

| Batch | What | Commits | Status |
|-------|------|---------|--------|
| B0: Sidecar Removal | Remove sidecar subsystem | `34bb887` `909a777` | **done** — merged |
| B1: P0 | Persistent signing key, graceful shutdown | `9c1d51d` `f96549f` `6d0d77d` `cec8b34` `0fef76b` `e823bea` | **done** — merged |
| B2: P1 | Config file parser, bcrypt admin auth, aactl init | `313aa41` `869a8f7` `58cbce2` `4978ecd` `866cc78` `3dfada7` `ebc4884` `1c5f293` | **done** — merged |
| B3: SEC-L1 | Bind address, TLS enforcement, timeouts, weak secret denylist | `632b224` `6fa0198` `574d3b9` `cd09a34` `5489679` | **done** — merged |
| B4: SEC-L2a | Token alg/kid validation, MaxTTL, revocation hardening | `8e63989` `0526c46` `c24e442` `67aeda7` `b78edb8` `ecb4c86` `078a674` `8366fa9` | **done** — 13/13 PASS, merged |
| B5: SEC-L2b | Security headers, MaxBytesBody, error sanitization | `daf2995` `e592acc` `2857b3a` `247727c` `c5da6c4` | **done** — G1-G6 PASS, 5/5 acceptance PASS, 1 SKIP, merged |
| B6: SEC-A1 + Gates | TTL bypass fix, gates regression | `9422e7c` `e395a15` | **done** — G1-G6 PASS, 4/4 acceptance PASS, pending merge |

## Tech Debt

See `TECH-DEBT.md` at repo root for the full tech debt tracker.

## Cowork ↔ Claude Code Coordination

When both Cowork and Claude Code are active, read `COWORK_SESSION.md` for shared state. It tracks who changed what and what's uncommitted.

## Docker Lifecycle & Admin Secret

**Standard test secret:** `live-test-secret-32bytes-long-ok` — used by `live_test.sh`, `live_test_docker.sh`, `test_batch.sh`, and the `broker-up` skill. Do NOT use any other secret for testing.

**Secret flow:** `AA_ADMIN_SECRET` env var → `docker-compose.yml` passes via `${AA_ADMIN_SECRET:-change-me-in-production}` → container env → `cfg.Load()` reads `os.Getenv("AA_ADMIN_SECRET")` → `main.go` fatals if empty. See `internal/cfg/cfg.go` and `cmd/broker/main.go`.

**Docker lifecycle scripts:** Use `scripts/stack_up.sh` (build + start) and `scripts/stack_down.sh` (teardown with `-v --remove-orphans`) for Docker operations. Raw `docker compose build` is OK for build-only (G4 gate). Do NOT use raw `docker compose down` — always use `stack_down.sh`.

## Acceptance Tests

Each cherry-pick batch has acceptance tests in `tests/<batch-name>/`:
- `user-stories.md` — stories with Who/What/Why/How/Expected
- `integration.sh` — automated script that runs all stories + regression tests against a live broker
- `evidence/` — terminal output from test runs

**Pattern:** Legacy tests in `agentauth/tests/` must be adapted for core before use. Remove all OIDC/HITL/cloud/sidecar/federation references. Update ports (8443), registration flow (launch tokens), and endpoint paths.

| Batch | Tests | Stories |
|-------|-------|---------|
| B0 | `tests/p0-production-foundations/` | 7 (K1-K5, S1-S2) |
| B1 | `tests/p0-production-foundations/` | Same as B0 |
| B2 | `tests/p1-admin-secret/` | 9 stories + 3 security reviews |
| B3 | `tests/sec-l1/` | 12 stories |
| B4 | `tests/sec-l2a/` | 13 stories (S1-S7, N1-N5, SEC1) |
| B5 | `tests/sec-l2b/` | 6 stories (S1-S4,S6 + S5 skip) + 4 regression (R1-R4) |
| B6 | `tests/sec-a1/` | 4 stories (S1-S2, S3, R1) |

## Standing Rules

- **Live tests require Docker** — `./scripts/stack_up.sh` first. No Docker = not a live test.
- **No add-on code in core** — zero tolerance. `grep -ri "hitl\|approval\|oidc\|federation\|cloud\|sidecar" internal/ cmd/` must return nothing.
- **Cherry-pick one batch at a time** — build + test after each batch before proceeding.
- **Acceptance tests adapted for core** — legacy tests have OIDC/HITL/sidecar code. Always audit and adapt before copying to core.
- **Docs update WITH every code change** — if code changes behavior, the docs update goes in the same commit or the same branch. No "fix docs later." B0-B4 proved that deferred doc updates cause massive drift. The doc files to check: `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.
- **Use `cherrypick-devflow` skill** for migration. Use `devflow` for new features after migration.
- **Pluggable architecture** — core code must expose interfaces and extension points. Enterprise modules plug in; they never get baked into core.
- **MEMORY.md lessons learned EVERY session** — before clearing context or ending a session, update MEMORY.md with lessons learned. This is not optional. If you learned something, write it down. If the user corrected you, write down what they said and why.
- **Strong code comments on ALL code** — every function, handler, and type must have comments explaining what it does, who can call it (role/scope), why it exists, and its boundaries. See `.claude/rules/golang.md` for the full standard. Code must be self-documenting — if you have to read three other files to understand who can call a function, the comments are insufficient.
- **Role model document required** — `docs/roles.md` defines who does what: Admin (operator), App (software managing agents), Agent (does work). All code and tests must align with these roles. See `TECH-DEBT.md` TD-012 for the gap. No acceptance test should be written without understanding the role model first.

## Backburner Designs (review after migration is complete)

- **Acceptance test automation + verification** — `.plans/designs/acceptance-test-automation.md`. Born during B5: how to automate story evidence creation while maintaining template compliance, and how to verify the agent followed the template with a deterministic hook. The `integration.sh` script is a CI smoke test — it does NOT produce proper evidence files. Three options captured: review hook, verify-evidence skill, or a runner script that produces template-compliant evidence. Review once B6 is merged.

