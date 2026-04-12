# Karpathy Guidelines — Coding Discipline

Derived from Andrej Karpathy's observations on common LLM coding pitfalls. These run alongside all other project rules — not optional, not situational.

## 1. Think Before Coding

State assumptions explicitly before implementing. If multiple interpretations exist, present them — don't pick silently. If something is unclear, stop and ask. Push back when a simpler approach exists.

## 2. Simplicity First

Minimum code that solves the problem. Nothing speculative.

- No features beyond what was asked
- No abstractions for single-use code
- No "flexibility" that wasn't requested
- No error handling for impossible scenarios

AgentAuth has 5 direct dependencies by design. Every abstraction is a candidate to reject. If a change balloons a function, stop and reconsider.

## 3. Surgical Changes

Touch only what you must. Clean up only your own mess.

- Don't improve adjacent code, comments, or formatting
- Don't refactor things that aren't broken
- Match existing style, even if you'd do it differently
- If you notice unrelated issues — note them in TECH-DEBT.md, don't fix them inline

**This is especially critical in `authz/`, `token/`, and `revoke/`.** Small "improvements" to adjacent security code introduce subtle regressions that tests don't catch because the behavior change wasn't the stated goal.

Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

Define success criteria before starting. Loop until verified.

- "Fix the bug" → write a story that reproduces it, run against the binary, record evidence, then mark PASS
- "Add validation" → write tests for invalid inputs, then make them pass
- For multi-step tasks, state a brief plan with a verify step per item

Weak criteria ("make it work") require constant clarification. Strong criteria let you loop independently.
