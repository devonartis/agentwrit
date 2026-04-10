# Decision 007: Code Comments Explain Roles, Not Restate Code

**Date:** 2026-03-30
**Status:** Final

## Decision

Comments must explain what reading the code alone would NOT tell you:
- **Who** calls this and why (role, endpoint, scope)
- **Why** this exists (business reason, security property, design decision)
- **Boundaries** (what this code is NOT responsible for)
- **History** (why something looks wrong but is intentional)

Never restate what the code does. Full standard in `.claude/rules/golang.md`.

## Why

During B6 acceptance test authoring, the agent built tests against the admin flow instead of the app flow because no code comments explained which role calls which endpoint. Multiple prior sessions wrote and reviewed this code without flagging the gap.

The problem wasn't that the code was unclear — the logic was fine. The problem was that you couldn't tell who was supposed to call it without reading three other files. That's what comments are for.

## What it replaced

The default "restate the function name" style: `// handleCreateLaunchToken handles launch token creation.` — which tells you nothing you can't see from the function signature.
