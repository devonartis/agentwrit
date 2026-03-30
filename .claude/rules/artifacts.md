# Project Artifacts — What Goes Where

**Do NOT mix these up. Each file has ONE purpose.**

## MEMORY.md — Lessons Learned, Session Thoughts, Golden Knowledge
- **Lessons learned** — what went wrong, what the user corrected, what was discovered. Written honestly and in detail: "agent did X wrong, user corrected with Y, because Z." These are golden — they feed blog posts, newsletter content, and future session context.
- **Session thoughts** — things the user fought over, rants, realizations about the project, about agentic coding, about Claude Code, about harness engineering. The raw thinking that led to decisions.
- **Why we did something** — the reasoning and context behind choices, not just what was chosen
- **Only keep 3 sessions** — newest session always on TOP. Older sessions archived to `MEMORY_ARCHIVE.md`. Both files: newest entry first.
- **Standing rules** — things that must always be true across sessions
- **Tech debt pointer** — points to `TECH-DEBT.md`, does NOT duplicate entries
- **This is blogging material.** Write it like it matters — because it does. The lessons about code comments, role models, agent mistakes — that's content for articles about agentic coding and harness engineering.
- **Never put here:** decisions/actions (FLOW.md), tech debt details (TECH-DEBT.md), changelog entries (CHANGELOG.md)

## FLOW.md — Decision Log
- **Decisions and actions only** — "we decided X because Y", "cherry-picked commits A and B", "deferred Z until after W"
- **Append-only** — each entry records what happened and why, in chronological order
- **Status of current work** — but only as "B6 Status: MERGED", not lessons or standing rules
- **Never put here:** lessons learned, standing rules, session thoughts, tech debt

## TECH-DEBT.md (repo root) — Tech Debt Tracker
- **All tech debt entries** with ID, description, severity, affected files
- **Detail sections** for items that need explanation
- **Never put tech debt in MEMORY.md** — MEMORY.md only points to this file

## CHANGELOG.md — What Changed (user-facing)
- **What was added/changed/fixed** per batch or release
- **Cherry-pick details** — conflicts, contamination status
- **Not a decision log** — just what shipped
