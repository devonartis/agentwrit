# Phase 1B — App-Scoped Launch Tokens

**Status:** Not started
**Depends on:** Phase 1A complete (merge to develop)

## Folder structure

```
tests/phase-1b/
  user-stories.md    — acceptance criteria (extract from spec before coding)
  env.sh             — test environment variables
  evidence/          — per-story evidence files after live test
  lessons-learned.md — issues found during testing
```

## Regression stories from Phase 1A

The following Phase 1A stories must be re-run as regression tests during Phase 1B acceptance testing:

- Story 1 (register app) — verify registration still works after launch token changes
- Story 6 (developer auth) — verify app auth still issues correct JWT
- Story 5 (deregister) — verify deregistration still blocks auth
- Story 11 (regression) — always run

See `tests/phase-1a/user-stories.md` for the full acceptance criteria.

## Rules

- Two personas: operator (`aactl`), developer (`curl`/REST API)
- Build `./bin/aactl` before testing — not `go run`, not `/tmp/`
- Source `env.sh` once — don't inline env vars
- Evidence saved per-story in `evidence/`
- No cutting corners. See `tests/phase-1a/lessons-learned.md`.
