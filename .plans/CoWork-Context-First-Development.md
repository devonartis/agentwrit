# Context-First Development (CFD)

**Build the map before you build the code. The human is the architect. The agent is the builder.**

This process was derived from real experience building AgentAuth with AI coding agents. Every missing step was discovered the hard way — by hitting a gap mid-session that could have been prevented with upfront context.

---

## The Core Idea

AI coding agents are skilled but literal. A human developer can discover context as they go — noticing existing patterns, asking questions mid-code, recognizing when a change might break something upstream. Agents don't do that. They build what's described, exactly as described.

**Context-First Development** means: every step before coding exists to build the context document that an agent needs to build correctly. The output of the pre-coding phases isn't just "a plan" — it's a machine-readable specification that any coding agent can follow.

---

## 4 Phases, 13 Steps

### Phase 1: DISCOVER — Map the Territory
*Owner: Human (with agent assistance for exploration)*
*Goal: Know what exists before changing anything*

**Step 1.1: Current State Inventory**
- Map every capability, endpoint, flow, and security property the system currently has
- This is the "before" picture — without it, you can't prove "after" doesn't break things
- Agent role: agent explores the codebase and generates the inventory; human reviews and corrects
- Deliverable: `capability-inventory.md`

**Step 1.2: Brainstorm / Pattern Input**
- Human provides the idea, pattern, or requirement
- If there's a source pattern, extract its MUST/SHOULD requirements as a checklist
- Deliverable: `pattern-requirements.md`

**Step 1.3: Constraint & Compliance Check**
- Compare current state against pattern requirements
- Identify: compliant, partially compliant, missing, non-compliant
- Document opt-outs explicitly ("we use X instead of Y because Z")
- Deliverable: `compliance-matrix.md`

### Phase 2: DESIGN — Architect the Change
*Owner: Human (agent drafts, human decides)*
*Goal: Design the change AND prove it doesn't break anything*

**Step 2.1: PRD + Architecture Decisions**
- Write PRD and spec with a "What MUST NOT Change" section
- Create Architecture Decision Records (ADRs) for every "do we need X?" question
- ADR format: Context → Decision → Consequences
- Deliverables: `prd.md`, `spec.md`, `adrs/` directory

**Step 2.2: Impact Analysis & Capability Preservation Proof**
- For every capability in the inventory, state: PRESERVED / ENHANCED / NEW / REMOVED
- Verify all security invariants are maintained
- Rule: if you can't fill this table completely, you don't understand the change well enough to build it
- Deliverable: `impact-analysis.md`

**Step 2.3: Implementation Details**
- Design the implementation with explicit "code paths that change" and "code paths that MUST NOT change"
- The "do not touch" list prevents agents from refactoring functions they shouldn't
- Deliverable: `implementation.md`

**Step 2.4: Peer Review Gate**
- Get the design reviewed before breaking into tasks — by a human, a different AI agent, or both
- Reviewer identifies gaps → each gap gets a decision with rationale → documented in design doc
- Deliverable: `review-decisions.md`

### Phase 3: BUILD — Agent Executes
*Owner: Agent (human reviews)*
*Goal: Build exactly what was designed, nothing more, nothing less*

**Step 3.0: Agent Context Briefing**
- Compile one document that the agent reads before writing any code
- Not the PRD, not the spec — a briefing: what exists, what we're building, what not to touch, how to verify
- This is "CLAUDE.md for this specific task"
- Deliverable: `TASK-BRIEF.md`

**Step 3.1: Task Breakdown**
- Break implementation into ordered tasks with dependencies
- Each task includes: description, files to change, files NOT to change, verification command
- Deliverable: `tasks.md`

**Step 3.2: User Stories + Negative Cases**
- Write testable user stories with acceptance criteria
- Add negative test cases: "given X, this should NOT happen"
- Deliverable: `user-stories.md`

**Step 3.3: Implementation**
- Agent reads TASK-BRIEF.md → picks next task → implements → runs verification → commits
- Human reviews PRs

### Phase 4: VERIFY — Prove It Works, Prove Nothing Broke
*Owner: Both*
*Goal: Verify the build matches the design AND existing capabilities survive*

**Step 4.1: Verification Tasks**
- Run the verification tasks from user stories
- Run in Docker (live tests), not just locally
- Deliverable: `test-results.md`

**Step 4.2: Regression Verification**
- For every PRESERVED capability, run a test proving it still works
- For every ENHANCED capability, test both old and new behavior
- Deliverable: `regression-results.md`

**Step 4.3: Post-Build Review**
- Compare what was built against the design
- Check: did implementation drift? Were "do not touch" files modified? Do actual code paths match the plan?
- Agent can self-audit by reviewing its own diff against the design doc
- Deliverable: `post-build-review.md`

---

## Directory Structure

```
feature-name/
  TASK-BRIEF.md           # Step 3.0 — the ONE doc the agent reads first
  capability-inventory.md  # Step 1.1 — what exists today
  pattern-requirements.md  # Step 1.2 — what the pattern demands
  compliance-matrix.md     # Step 1.3 — current vs. required
  prd.md                   # Step 2.1 — what we're building + what must NOT change
  impact-analysis.md       # Step 2.2 — preserved / enhanced / new / removed
  implementation.md        # Step 2.3 — code paths: change vs. do-not-touch
  review-decisions.md      # Step 2.4 — peer review gaps + decisions
  tasks.md                 # Step 3.1 — ordered tasks with verification
  user-stories.md          # Step 3.2 — stories + negative cases
  adrs/                    # Architecture Decision Records
    001-sidecar-optional.md
    002-ed25519-over-spire.md
```

---

## TASK-BRIEF.md Template

```markdown
# Task Brief: [Feature Name]

## Context
- What this feature does (1-2 sentences)
- Why we're building it (the problem it solves)
- Link to PRD: ./prd.md
- Link to pattern: [URL or file]

## Current State
- Link to capability inventory: ./capability-inventory.md
- Key capabilities that MUST be preserved: [list the critical ones]

## Design Decisions
- Link to impact analysis: ./impact-analysis.md
- Link to ADRs: ./adrs/
- Key constraints: [list non-obvious constraints]
- OPTED OUT OF: [things we explicitly don't use, and why]

## Implementation Rules
- Link to implementation details: ./implementation.md
- DO NOT TOUCH: [files/functions that must not be modified]
- CHANGE: [files/functions that should be modified]
- NEW: [files/functions to create]

## Tasks
- Link to tasks: ./tasks.md
- Link to user stories: ./user-stories.md

## Verification
- Run after each task: [command]
- Run after all tasks: [command]
- Regression test: [command or checklist]
- Success criteria: [what "done" looks like]
```

---

## Cross-Agent Compatibility

TASK-BRIEF.md is the universal interface. Any agent that reads files can follow it.

| Agent | How It Discovers the Process |
|-------|------------------------------|
| Claude Code / Cowork | CLAUDE.md says "Read TASK-BRIEF.md first" |
| Cursor / Windsurf | .cursorrules references TASK-BRIEF.md |
| Copilot Workspace | TASK-BRIEF.md provided as spec input |
| Any agent with file access | Standard markdown — just read it |

---

## Skill Decomposition (for Multi-Step Skills)

Each phase maps to a potential skill:

| Skill | Phase | What It Does |
|-------|-------|-------------|
| `cfd:discover` | Phase 1 | Explores codebase, generates capability inventory, extracts pattern requirements, creates compliance matrix |
| `cfd:design` | Phase 2 | Drafts PRD with "must not change" section, generates impact analysis table, creates implementation details with change/no-change lists |
| `cfd:brief` | Phase 3 (prep) | Compiles TASK-BRIEF.md from all Phase 1-2 deliverables, generates task breakdown, writes user stories with negative cases |
| `cfd:verify` | Phase 4 | Runs verification tasks, regression tests against preservation matrix, generates post-build review by diffing implementation against design |

Each skill reads the outputs of the previous skill. The chain is: `discover` → `design` → `brief` → (human hands off to agent for build) → `verify`.

---

*Context-First Development — derived from real session experience, March 2026*
