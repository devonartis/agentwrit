# PROCESS-LESSON.md

How a wrong architecture decision survived 4 sessions and what we learned about the development process.

## The Three-Step Process (Named Here)

| Step | Name | What It Does | Who Did It This Time |
|------|------|-------------|---------------------|
| 1 | **Architecture Challenge** | Questions fundamentals against user needs and the pattern. "Should this thing exist at all?" | CoWork (Session 18) |
| 2 | **Gap Analysis** | Inventories what's broken against an agreed architecture. "Given the design, what's missing?" | Gap analysis (Session 18) |
| 3 | **Process Maps** | Documents how it works for every persona. "How does each user experience this?" | Process maps (Session 18) |

**The order matters.** You can't do Gap Analysis until the architecture is settled. You can't do Process Maps until the gaps are known. Running steps 2-3 against the wrong architecture produces correct-looking documents that are fundamentally wrong.

---

## What Happened: The Full History

### Session 14 — User Asks the Right Question

The user raised fundamental architecture questions during Fix 5 (sidecar UDS):

> - Why do sidecars exist? What's the alternative?
> - Can apps talk directly to the broker with client_id/client_secret instead?
> - How would scope siloing work without sidecars?
> - How do operators deploy sidecars for new apps?
> - How do 3rd-party SDK consumers onboard?

These are **Architecture Challenge** questions. The user was asking: "Does this component need to exist in the first place?"

### Session 15 — Agents Defend the Code Instead of Answering

A 4-agent team debated the questions. They produced ADR-002: "Keep sidecars as the primary and only current model."

**What went wrong:** The agents answered from the code. They read how sidecars work, confirmed they work correctly, and concluded they should stay. The analysis was technically accurate — sidecars DO enforce scope ceilings, DO provide DX benefits, DO offer UDS security.

But the user didn't ask "do sidecars work?" The user asked "should sidecars be required?" Those are different questions with different answer sources:

| Question | Answer Source |
|----------|--------------|
| "Do sidecars work?" | Read the code |
| "Should sidecars be required?" | Read the pattern, the user needs, the production deployment model |

The agents never checked:
- **The pattern** — Ephemeral Agent Credentialing v1.2 does NOT require sidecars. The sidecar is an implementation choice, not a pattern requirement.
- **The user's production need** — "Docker is infrastructure. This should run on someone's virtual server. If it can't, it's wrong."
- **3rd-party developer experience** — A developer connecting their app should NOT need to understand sidecars, infrastructure, or admin secrets.

### Session 18 — The Gap Surfaces Again

During demo app work, the same problem resurfaced: every sidecar self-provisions with the admin secret. Apps don't exist as entities. Onboarding an app means editing docker-compose. `BIG_BAD_GAP.md` was created documenting the blocker.

### Session 18 (Later) — Architecture Challenge Done Right

The CoWork Architecture document (`CoWork-Architecture-Direct-Broker.md`) did what Session 15 should have done:

1. **Checked the pattern** — confirmed sidecars are not required
2. **Checked user needs** — apps need client_id + client_secret, not activation tokens
3. **Checked production reality** — sidecar-mandatory doesn't work for 3rd-party developers
4. **Proposed the fix** — apps as first-class entities, 3 paths (SDK/Proxy/HTTP), sidecar optional

### Session 18 (Later) — Gap Analysis Against Wrong Architecture

Meanwhile, the gap analysis produced 24 gaps — all technically correct. But many gaps (especially the 6 blockers around app registration) were diagnosed as "missing features" when the real problem was "wrong architecture."

Example: Gap G-4 "Helper self-provisions (bypasses activation)" was flagged as a missing workflow. The CoWork doc correctly identified this as: "The activation endpoint exists but nothing uses it because the architecture doesn't have apps as entities."

The gap analysis was Step 2 running before Step 1 was complete.

### Session 18 (Later) — The Wrong Pushback

When comparing the gap analysis with the CoWork doc, I pushed back on the CoWork time estimate (2-3 days → 4-5 days). The user's response:

> "so to push back on size you should have pushed back on the sidecar now to agree with it"

The correct pushback was on the architecture decision (ADR-002), not on the schedule. Pushing back on size is trivial. Pushing back on "you're building on the wrong foundation" is the high-value feedback.

---

## The Standing Rule

**When someone asks "why does this exist?" — the answer CANNOT come from reading the current code.**

The answer must come from:
1. **The user's actual question** — what are they really asking?
2. **The pattern** — does the security pattern require this component?
3. **Production needs** — how will real users (operators, developers, apps) interact with this in production?

Code tells you what IS. The user is asking what SHOULD BE. Those are different questions.

---

## What ADR-002 Got Wrong

ADR-002 (Session 15) concluded: "Keep sidecars as the primary and only current model."

Rejected alternatives included:
- Direct broker access — "requires broker changes, no use case today"
- Hybrid model — "doubles maintenance"
- Remove sidecars entirely — "loses DX, resilience, UDS, scope siloing"

**Every rejection was based on the current code**, not on what production needs:

| ADR-002 Said | Reality |
|-------------|---------|
| "No use case for direct access" | 3rd-party developers ARE the use case |
| "Doubles maintenance" | 3 paths (SDK/Proxy/HTTP) is correct; the pattern supports it |
| "Loses DX" | Forcing sidecar deployment IS the DX problem |

**ADR-002 is now invalidated** by the CoWork Architecture document, which correctly answers the Session 14 questions from the pattern and production needs, not from the current code.

---

## Process Fix: How to Prevent This

### Before answering "should X exist?"

```
1. Read the pattern (Ephemeral Agent Credentialing v1.2)
   → Does the pattern require X?

2. Read the user's question (MEMORY.md, conversation)
   → What is the user actually asking? What need triggered this?

3. Check production reality
   → How do real users (operator, developer, app) experience X?

4. THEN read the code
   → Does the implementation match what the pattern and users need?
```

### The red flag

If your answer to "should X exist?" relies primarily on "X works correctly and provides these benefits" — you're answering the wrong question. Working correctly is necessary but not sufficient. The question is whether X should be required, optional, or removed entirely.

---

## Artifacts

| Artifact | Location | What It Contains |
|----------|----------|-----------------|
| BIG_BAD_GAP.md | repo root | Session 18 blocker discovery (app registration broken) |
| CoWork-Architecture-Direct-Broker.md | `.plans/` | Correct architecture: apps as entities, sidecar optional, 3 paths |
| 01-gap-analysis-process-map.md | `.plans/` | 24 gaps inventoried (valid analysis, ran against incomplete architecture) |
| 02-production-process-maps.md | `.plans/` | Plain-language process maps for 4 personas |
| SVG diagrams (01-06) | `.plans/diagrams/` | Visual process maps viewable in VS Code |
| ADR-002 | `plans/2026-02-25-sidecar-architecture-decision.md` | **INVALIDATED** — kept sidecars mandatory based on code analysis only |

---

## Timeline

```
Session 14  User asks "why do sidecars exist?"         ← Architecture Challenge question
Session 15  Agents answer from code → ADR-002          ← WRONG: answered "do they work?" not "should they be required?"
Session 16  Fix 6 implemented, demo-ready declared
Session 17  Demo app stories designed
Session 18  Demo app hits the wall → BIG_BAD_GAP.md    ← Same problem resurfaces
Session 18  CoWork doc does Architecture Challenge      ← RIGHT: answers from pattern + user needs
Session 18  Gap Analysis + Process Maps created         ← Steps 2-3 (valid, but Step 1 wasn't done yet)
Session 18  Process lesson documented (this file)       ← Standing rule established
```
