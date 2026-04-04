---
name: devflow
description: >
  Use when starting any development work on AgentAuth — loads the Development
  Flow, checks tracker state, and tells you which step to execute next.
  Trigger on: "start dev", "what's next", "resume work", "continue",
  "where are we", "pick up where we left off", any development request,
  or even small fixes. If someone is doing dev work on AgentAuth and hasn't
  invoked this skill yet, they should be.
---

# AgentAuth Development Flow

Start here for any development work. This skill loads context and tells you
what to do next.

## Instructions

1. Read these files in order:
   - `MEMORY.md` (at the auto-memory path)
   - `FLOW.md` (repo root) — if it doesn't exist or has no current step, start at Step 1 (Brainstorm)
   - `.plans/tracker.jsonl` (current state of all stories and tasks)

2. From FLOW.md + tracker, identify the current step (1-9):

| Step | What | Skill | Model | Done when |
|------|------|-------|-------|-----------|
| 1 | Brainstorm | `superpowers:brainstorming` | **opus** | Design doc in `.plans/designs/` |
| 2 | Write Spec | Follow `.plans/SPEC-TEMPLATE.md` | **opus** | Spec in `.plans/specs/` |
| 2.5 | **Council: Spec** | `TeamCreate` council (3 agents) | **sonnet** | Verdict is APPROVED |
| 3 | Impl Plan | `superpowers:writing-plans` | **opus** | Plan in `.plans/` with tasks |
| 4 | Acceptance Tests | Follow `tests/LIVE-TEST-TEMPLATE.md` | **opus** | Stories in `tests/<feature>/` |
| 4.5 | **Council: Tests** | `TeamCreate` council (3 agents) | **sonnet** | Verdict is APPROVED |
| 5 | Register Tracker | Update `.plans/tracker.jsonl` | any | All stories + tasks registered |
| 6 | Code | `superpowers:executing-plans` | **sonnet** | All tasks PASS, gates green |
| 7 | Review | `superpowers:requesting-code-review` + `writing-plans` | **sonnet** (review) / **opus** (plan) | Findings documented + fix plan written |
| 7.5 | Fix Findings | `superpowers:executing-plans` | **sonnet** | Fix plan complete, gates green |

| 8 | Live Test | `superpowers:verification-before-completion` | **sonnet** | Evidence files with banners. The acceptance test runner owns setup (ngrok, AWS, broker start) and teardown — `[PRECONDITION]` stories are test setup, not a separate step. |
| 8.5 | **Regression** | `regression` | **sonnet** | All previous phases PASS |
| 9 | Security Review | `superpowers:dispatching-parallel-agents` | **sonnet** | Findings reviewed, criticals fixed |
| 10 | Merge | `superpowers:finishing-a-development-branch` | any | Human approved, merged to `develop` |

**Step 2.5 + 4.5 (Council Review):** 3 different agents review specs and tests — intentionally separate from the coding agent. The council catches blind spots the author can't see. See Development-Flow.md for full protocol.

**Step 7:** Reviewer produces findings AND a fix plan (via `writing-plans`). The fix plan goes to `.plans/` with exact paths, code, and test commands. Ad-hoc review fixes skip traceability and have caused regressions.

**Step 6 + 7.5:** Use `executing-plans` for all coding — even small fixes. Ad-hoc changes outside plans broke traceability in Sessions 37/38/52/61 and caused bugs that took entire sessions to debug.

3. Announce: "Development Flow: Step N — [step name]. [X/Y tasks done]. Next: [action]."

4. Invoke the relevant superpowers skill if one is listed.

## Tool Usage — MANDATORY

### jCodeMunch FIRST (preferred for all code retrieval)

**jCodeMunch is the preferred way to explore code.** It saves context and returns only what you need.

1. **Session start:** `check_freshness` → `index_folder(incremental=true)` if stale or unindexed.
2. **Find code:** `search_symbols` → `get_symbol_source` preferred over Read/Grep.
3. **Understand structure:** `get_file_outline` preferred over reading entire files.
4. **Get function + deps:** `get_context_bundle` preferred over reading multiple files.
5. **Impact analysis:** `find_importers` / `get_blast_radius` preferred over manual grep.
6. **After editing:** `index_file(path)` if you'll reference that file again.
7. **At step boundaries:** `check_freshness` → `index_folder(incremental=true)` if stale.

**Fall back to `Read` when:** you need the file for an `Edit`, or jCodeMunch doesn't have what you need (non-code files, configs, docs, specs).

### Context Fork (for skill isolation)

Skills that produce heavy intermediate output should use `context: fork` in their
frontmatter — this runs the skill in an isolated context window, keeping the main
conversation clean. Only the result flows back.

**When to use context fork:**
- Execution steps (6, 7.5) — lots of code/test output
- Code review (7) — detailed findings analysis
- Security review (9) — multi-agent parallel output
- Debugging — intermediate investigation steps

**How it works:** Add `context: fork` to the skill's YAML frontmatter. The forked
skill inherits the parent model. To switch models, set the model on the Agent call
(e.g., `model: "sonnet"`) when dispatching the skill.

### context-mode (for large outputs)

- Use `ctx_execute` or `ctx_batch_execute` for test runs, build output, git diffs,
  log analysis, and any command producing >20 lines.
- Prefer these over piping large output through Bash directly into context.
- Use `ctx_fetch_and_index` instead of `WebFetch` for external docs.

## Rules

- Branch from `develop`, never `main`. GitFlow: `feature/*` or `fix/*`.
- Plans save to `.plans/`, NOT `docs/plans/`.
- Specs save to `.plans/specs/`.
- Designs save to `.plans/designs/`.
- Read `docs/api.md` before writing any HTTP call in tests.
- Read `cmd/aactl/` before writing any CLI command in tests.
- Update tracker when story/task status changes.
- **Run `./scripts/gates.sh task` after each commit** during Step 6. Fix lint/test failures before moving on.
- **Update `CHANGELOG.md` with every user-facing change** — in the same commit as the code. A feature or fix without a CHANGELOG entry is not done. This is a gate, not a reminder.
- A story is NOT done until Docker live test passes with recorded evidence.
