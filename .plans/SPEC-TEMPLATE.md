# [Title]: [Short Description]

**Status:** Spec | In Progress | Complete
**Priority:** P0/P1/P2 — [one-line justification]
**Effort estimate:** [time estimate]
**Depends on:** [what must be done first]
**Architecture doc:** [path to relevant architecture doc]
**Tech debt:** [TD-xxx reference if applicable]

---

## Overview: What We're Building and Why

[Narrative explanation. Not bullets — tell the story. Why does this matter? What's the context? What came before this and what comes after? Write it so someone who missed the last three sessions understands what's happening and why.]

**What changes:** [One paragraph listing all modifications — data model, endpoints, CLI, behavior.]

**What stays the same:** [One paragraph confirming what is NOT touched. This prevents scope creep and reassures reviewers.]

---

## Problem Statement

[What's broken, missing, or insufficient today. Be specific — reference the code, the config, the user experience. If this came from tech debt, a NIST gap, or a user complaint, say so.]

---

## Goals

1. [Numbered list of outcomes this spec delivers]
2. [Each goal is testable — if you can't write a test for it, it's not a goal]

---

## Non-Goals

1. [What this spec explicitly does NOT do, with a reference to where/when it will be addressed]
2. [Prevents scope creep. If someone asks "what about X?" the answer is here.]

---

## User Stories

### Operator Stories

1. **As an operator**, I want [action] so that [benefit].

### Developer Stories

2. **As a developer**, I want [action] so that [benefit].

### Security Stories

3. **As a security reviewer**, I want [property] so that [security justification].

---

## Schema Changes

[Exact SQL for any database modifications. Include column types, defaults, constraints, and indexes. If no schema changes, write "None." and explain why.]

```sql
-- Example:
ALTER TABLE apps ADD COLUMN token_ttl INTEGER NOT NULL DEFAULT 1800;
```

**Migration notes:** [Is this additive (safe) or destructive? Does it require data backfill? What happens to existing rows?]

---

## API Contract

[Request/response examples for every new or changed endpoint. Show the exact JSON shape a developer or operator will see. Include error responses.]

### `POST /v1/example`

**Request:**
```json
{
  "field": "value"
}
```

**Response (201):**
```json
{
  "result": "value"
}
```

**Error (400):**
```json
{
  "type": "invalid_request",
  "title": "Validation failed",
  "detail": "Explanation of what went wrong"
}
```

---

## What Needs to Be Done

### 1. [Component or Capability]

[What changes, why, and how. Reference specific files and functions. Enough detail for a coding agent to implement without guessing.]

### 2. [Next Component]

[Same pattern.]

---

## Edge Cases & Risks

| Case | What Happens | Mitigation |
|------|-------------|------------|
| [Scenario] | [Consequence] | [How we handle it] |

[Include: race conditions, migration timing, failure modes, concurrency, config mistakes, backward compat edge cases.]

---

## Backward Compatibility

### Breaking Changes

[List every breaking change. If none, write "None." Breaking = existing clients/workflows stop working.]

### Non-Breaking Changes

[List behavior changes that are compatible. Existing clients continue to work but may see new fields, different defaults, etc.]

### Migration Path

[If an operator upgrading from the previous version needs to do anything, describe it step by step. If nothing required, say "Automatic — no operator action needed."]

---

## Rollback Plan

[If this goes wrong after deployment, how does the operator undo it?]

1. [Step-by-step rollback procedure]
2. [Include: schema rollback (if applicable), config revert, binary rollback]
3. [Data safety: is any data lost on rollback?]

---

## Success Criteria

- [Bulleted list of observable outcomes that prove the spec is complete]
- [Each criterion maps to at least one user story]
- [Include both positive (it works) and negative (it rejects bad input) criteria]

---

## Testing Workflow

> **Before writing any test code**, extract the user stories from the `## User Stories` section above into a standalone file:
> `tests/<phase-or-fix>/user-stories.md`
>
> This is required by the project workflow (CLAUDE.md). The coding agent writes user stories first, saves them to `tests/`, then writes test code against them. Do not skip this step.
