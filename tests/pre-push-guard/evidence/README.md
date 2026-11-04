# Pre-Push Guard — Live Test Evidence

**Date:** 2026-11-04
**Branch:** `fix/pre-push-guard-stories` (hook under test: `.githooks/pre-push` as merged to `develop` in PR #92)
**Mode:** CLI-only. No broker is involved. Pushes go to a throwaway local bare remote, never to GitHub.
**Run status:** STOPPED after S2. The guard does not work, so the remaining stories would only repeat the failure.

## Story Results

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| PC1 | Hook is on `develop` and executable | Security Reviewer | ls, git | PASS |
| PC2 | `core.hooksPath` set to `.githooks` | Security Reviewer | git | PASS |
| PC3 | Throwaway clone and empty remote exist | Security Reviewer | git | PASS |
| S1 | A normal push goes through | Developer / Security Reviewer | git | PASS (with a caveat: see the S1 note) |
| S2 | A private file is refused | Developer / Security Reviewer | git | **FAIL** |
| S3 | One private file spoils the whole push | Security Reviewer | git | NOT RUN |
| S4 | Private file on an existing branch is refused | Security Reviewer | git | NOT RUN |
| S5 | Removing the file lets the push through | Security Reviewer | git | NOT RUN |
| S6 | Known gap: `--no-verify` skips the guard | Security Reviewer | git | NOT RUN |
| S7 | Known gap: no setting, no guard | Security Reviewer | git | NOT RUN |

## Open Issues

1. **The pre-push hook on `develop` blocks nothing (S2 FAIL).** Git sends the hook its push details
   separated by spaces. The hook splits them on tabs, never sees the commit, and exits "allowed".
   A force-added `MEMORY.md` reached the practice remote. A hand-run of the hook with tab-separated
   input refuses correctly, so the fault is confined to the input-reading line. A fix needs its own
   branch and PR, and then S1–S7 are re-run from the start.
2. Until that is fixed, the only protection against a private file reaching the public repo is
   the pre-commit hook and `.gitignore`. Both are skipped by `git add -f` plus `--no-verify`.
