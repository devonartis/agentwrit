# Decision 009: Acceptance Tests Required Before Merge

**Date:** 2026-03-29
**Status:** Final

## Decision

Every batch/feature must run its acceptance tests against a live broker before merging. The process:
1. Build the code (compiled binary, never `go run`)
2. Run gate checks (build, lint, unit tests, contamination)
3. Run each acceptance story against Docker per `tests/LIVE-TEST-TEMPLATE.md`
4. All stories must PASS with recorded evidence before merge to develop

VPS mode (bare binary) first, Container mode (Docker) second. Always both — they catch different bugs.

## Why

Unit tests prove functions work. Acceptance tests prove a real user can accomplish a real task end-to-end. The distinction matters because integration bugs hide in the wiring between components — config loading, dependency injection, handler registration — none of which unit tests cover.

B4 proved this: unit tests all passed, but `TknSvc.revoker` was nil at runtime because the wiring in `main.go` was missing. Only the live test caught it.

## Audiences

- **Executives** read test evidence to decide if a release is safe
- **Manual QA testers** follow evidence to verify features
- **Security reviewers** audit evidence for vulnerabilities

If any of these audiences can't understand the evidence, the test is incomplete.
