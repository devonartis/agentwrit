# Testing — Rules of Engagement

## Audience

Tests are written for two audiences who may not be deeply technical:

1. **Executives** read test evidence to decide if a release is safe. They need to understand the business risk without asking an engineer to translate.
2. **Manual QA testers** follow test evidence to verify features. They need clear steps they can follow and clear pass/fail criteria.

If either audience can't read your test evidence and understand what happened — the test is incomplete.

## Live / Acceptance Tests

- A live test is a person (or agent acting as a person) running commands against the REAL system and recording what happened. Not a script. Not automation.
- Compiled binaries only. Never `go run` or equivalent interpreted shortcuts.
- VPS mode first (bare binary), Container mode second. Always both. They catch different bugs.
- One story at a time. Run, record output, write verdict, then next.
- Verdict is earned — never write PASS before seeing output.

## Story Classifications

Every story MUST be tagged:
- `[PRECONDITION]` — proves infrastructure/setup works, not the feature itself
- `[ACCEPTANCE]` — proves a real user can accomplish a real task end-to-end

At least one `[ACCEPTANCE]` story must involve a real third-party consumer — something outside the system that trusts and uses the system's output.

## The Banner — Mandatory on Every Evidence File

Every evidence file starts with a plain-language banner:

| Part | Purpose |
|------|---------|
| **Who** | Which persona is doing this |
| **What** | What they're doing and what changed — plain English |
| **Why** | What breaks if this fails — make a non-technical person uncomfortable about the failure scenario |
| **How to run** | Step-by-step a QA person can follow without deep technical knowledge |
| **Expected** | What a passing result looks like, in plain language |

### Banner Language Standard

Write like you're explaining to a manager, not an engineer.

GOOD: "If this route still responds, someone with a stolen token could gain access to the system."
BAD: "If the endpoint is still registered in the mux, the bootstrap flow remains exploitable via token replay."

GOOD: "The operator tries to log in using the command line tool."
BAD: "The operator authenticates using the new admin auth shape with constant-time comparison."

## Personas — Never Mix

| Persona | Tool | Never uses |
|---------|------|-----------|
| Operator | CLI tool | raw HTTP / curl |
| Developer | curl / HTTP client | CLI tool |
| Security Reviewer | whatever proves the property | N/A |

## Unit Tests

- Table-driven tests with subtests.
- Test files live next to the code they test.
- `go test ./...` for full suite, `-short` to skip long-running tests.
