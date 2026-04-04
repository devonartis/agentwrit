# AGENTS.md

**Last updated:** 2026-04-03
**Purpose:** Short handoff note for the next conversation. This is a working-session summary, not a public product document.

---

## Current situation

We did **not** start repo cleanup.

We did strategy, competitive, and pre-cleanup assessment work only.

The repo still needs a careful cleanup plan before any deletion, moving, or public-surface pruning happens.

---

## Product framing clarified in this session

### Canonical framing
- **AgentAuth** is the product name.
- This repo is the **OSS core** of AgentAuth.
- The OSS core implements the **8 components** of **Ephemeral Agent Credentialing v1.3**.
- Broader AgentAuth packaging / binaries may include:
  - **OIDC**
  - **HITL**
  - **resource server**
  - other enterprise or deployment-specific layers

### Audit framing clarified
- **Core audit** = credential lifecycle and security events
  - issuance
  - scope / allowed actions
  - renew / revoke / release / expiry
  - lineage / identity / task linkage
- **Resource server / downstream logging** = what the agent actually did end-to-end
- Docs should eventually explain how users can log:
  - `agent_id`
  - `task_id`
  - delegation lineage / chain hash
  into their own logging or resource server

---

## What was created this session

### Competitive / strategy docs
- `.plans/designs/2026-04-03-competitive-landscape-review.md`
- `.plans/designs/2026-04-03-competitive-positioning.md`
- `.plans/designs/2026-04-03-threat-model-competitive-matrix.md`

These now reflect:
- AgentAuth = product
- this repo = OSS core
- 8-component core framing
- broader platform story
- competitor analysis beyond OSS
- threat-model-first comparison framing

### Pre-cleanup review doc
- `.plans/designs/2026-04-03-pre-cleanup-docs-public-surface-assessment.md`

This is the key handoff document for cleanup planning.

It contains:
- code baseline used for review
- doc/code drift findings
- public/private surface findings
- files likely not meant to ship publicly
- planning-only cleanup workstreams

---

## Important findings from the pre-cleanup assessment

### Public docs drift found
- `README.md` still says **v1.2** and **7-component**
- some docs still say **7-component** while others imply **8 components**
- public naming still uses **agentauth-core** in places
- `docs/troubleshooting.md` still has stale `config.yaml` examples
- some public docs reference internal artifacts like `TECH-DEBT.md`

### Files likely not meant for the public repo surface
Examples identified in the assessment:
- `MEMORY.md`
- `MEMORY_ARCHIVE.md`
- `FLOW.md`
- `TECH-DEBT.md` (likely sanitize/remove before release)
- `COWORK_SESSION.md`
- `COWORK_DOCS_AUDIT.md`
- `CLAUDE.md`
- `.claude/`
- `docs/patent/`
- `audit/` review artifacts
- stray `.docx` files
- `Archive.zip`
- `tests/FUCKING QUETIONS.MD `
- `.DS_Store` junk files

**Nothing was removed.** These were only identified.

---

## What NOT to do next without a plan

Do **not**:
- start deleting files ad hoc
- merge/remove docs just because they look duplicated
- remove internal files before classification
- rewrite public docs without first deciding the canonical product story
- assume the cleanup is just cosmetic

The right order is:
1. classify public vs internal
2. decide canonical public story
3. map docs to that story
4. only then plan actual cleanup

---

## Recommended next step

When we resume, the best next task is:

### Build a cleanup plan, not execute cleanup
That plan should include:
- canonical public story
- file classification matrix
- public docs alignment list
- internal/private removal or relocation list
- sequence of safe cleanup batches

Suggested output:
- a dedicated `.plans/` cleanup plan or spec
- grouped into small batches so nothing useful gets deleted by accident

---

## Important conversation constraint from this session

A path was mentioned and then explicitly **not** to be reviewed:
- `/Users/divineartis/proj/claude_leaked/claude-code/study_code`

Do **not** review that path unless the user explicitly asks again.

---

## Session intent summary

This session was about:
- clarifying product framing
- expanding competitive analysis beyond OSS
- grounding comparisons in the 40-threat model
- producing a pre-cleanup assessment so future cleanup can be planned safely

This session was **not** about:
- executing cleanup
- deleting files
- finalizing public docs
- changing repo structure

---

## If resuming cold

Read these first:
1. `SOUL.md`
2. `AGENTS.md`
3. `.plans/designs/2026-04-03-pre-cleanup-docs-public-surface-assessment.md`
4. `.plans/designs/2026-04-03-competitive-positioning.md`
5. `MEMORY.md`
6. `FLOW.md`
7. `.plans/tracker.jsonl`

Then propose a **cleanup plan only** unless the user says to execute changes.
