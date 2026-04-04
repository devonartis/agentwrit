# [Title]: [Short Description]

**Status:** Spec | In Progress | Complete
**Priority:** P0/P1/P2 — [one-line justification]
**Effort estimate:** [time estimate]
**Depends on:** [what must be done first]
**Architecture doc:** [path to relevant design doc]
**Tech debt:** [TD-xxx reference if applicable]

---

## Overview

[Narrative explanation — what, why, and context. Tell the story so someone
who missed the last three sessions understands. Include the problem statement:
what's broken, missing, or insufficient today. Reference specific code, config,
or user experience.]

**What changes:** [One paragraph listing all modifications.]

**What stays the same:** [One paragraph confirming what is NOT touched.]

---

## Goals & Success Criteria

1. [Goal — stated as a testable outcome]
2. [Each goal IS its own success criterion — if you can't test it, rewrite it]
3. [Include both positive (it works) and negative (it rejects bad input)]

---

## Non-Goals

1. [What this spec explicitly does NOT do, with where/when it will be addressed]

---

## User Stories

### Operator Stories

1. **As an operator**, I want [action] so that [benefit].

### Developer Stories

2. **As a developer**, I want [action] so that [benefit].

### Security Stories

3. **As a security reviewer**, I want [property] so that [justification].

---

## Contract Changes

**Schema:** [Exact SQL for any DB changes, or "None — no schema changes."]

**API:** [Request/response examples for new/changed endpoints, or "None — no
API contract changes." Include error responses if applicable.]

---

## Codebase Context & Changes

> **The spec author already read these files.** Capture the exact code
> sections here so the planning agent (`writing-plans`) does NOT need to
> re-read them. Each subsection is one file region: what it does today,
> what needs to change, and why.

### 1. `path/to/file.go:NN-MM` — [What this section does]

```go
// Paste the exact code that will be modified.
```

**Change:** [What to do — enough detail for a coding agent to implement
without guessing.]

### 2. `path/to/another-file.go:NN-MM` — [Description]

```go
// Same pattern. One subsection per file or code region.
```

**Change:** [What to do.]

---

## Edge Cases & Risks

| Case | What Happens | Mitigation |
|------|-------------|------------|
| [Scenario] | [Consequence] | [How we handle it] |
| [Backward compat issue] | [Impact] | [Migration path or "automatic"] |
| [Rollback scenario] | [Data safety] | [Step-by-step rollback] |

[Include: race conditions, failure modes, concurrency, config mistakes,
backward compat, and rollback — all in one table.]

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the
> `## User Stories` section above into a standalone file:
> `tests/<phase-or-fix>/user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent
> writes user stories first, saves them to `tests/`, then writes test code
> against them. Do not skip this step.

---

## Implementation Plan

> **After acceptance tests are written**, create the implementation plan
> using the `superpowers:writing-plans` skill.
>
> **Required skill:** `superpowers:writing-plans`
> **Save to:** `.plans/YYYY-MM-DD-<topic>-plan.md` (NOT `docs/plans/`)
>
> The plan must follow the superpowers format:
> - **Plan header:** Goal, Architecture, Tech Stack
> - **Task structure:** Exact file paths, TDD steps (failing test → run →
>   implement → run → commit), exact commands with expected output
> - **Task-to-story mapping:** Each task maps to one or more acceptance
>   test stories from `tests/<feature>/user-stories.md`
> - **Plan header must reference this spec:**
>   `**Spec:** .plans/specs/YYYY-MM-DD-<topic>-spec.md`
>
> **Execution:** Use `superpowers:executing-plans` (separate session or
> subagent-driven). The coding agent follows the plan task-by-task.
>
> Do not skip this step. The plan is the bridge between "what to build"
> (this spec) and "how to build it" (TDD tasks).
