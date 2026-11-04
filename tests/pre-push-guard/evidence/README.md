# Pre-Push Guard — Live Test Evidence

**Date:** 2026-11-04
**Branch:** `fix/pre-push-guard-stories` (pre-push fix for #93 plus these stories)
**Mode:** CLI-only. No broker is involved. Pushes go to a throwaway local bare remote, never to GitHub.

## Run history

| Run | What was tested | Result |
|-----|-----------------|--------|
| Run 1 | Pre-push hook as merged in PR #92 | **Stopped at S2: FAIL.** The hook split git's input on tabs, but git sends spaces, so it let every push through. Evidence is in git history at commit `08e3e31`. |
| Run 2 | Pre-push hook after the fix (#93), plus the pre-commit hook (S8–S10, added on the owner's rule that guards live in git hooks) | All 13 stories run. See below. |
| Run 3 | Pre-push refusal message after the fix for #95 (S11), in a fresh practice area | S11 PASS. |

## Story Results (Run 2)

| Story | Description | Persona | Tool | Verdict |
|-------|------------|---------|------|---------|
| PC1 | Both hooks in the repo and runnable | Security Reviewer | ls, git | PASS |
| PC2 | `core.hooksPath` set to `.githooks` | Security Reviewer | git | PASS |
| PC3 | Fresh practice clone and empty remote | Security Reviewer | git | PASS |
| S1 | A normal push goes through | Developer / Security Reviewer | git | PASS |
| S2 | A private file (`MEMORY.md`) is refused at push | Developer / Security Reviewer | git | PASS (failed in run 1) |
| S3 | A public change plus a `.plans/` file is refused, and only the private file is named | Security Reviewer | git | PASS |
| S4 | A private file on an existing branch is refused, and the remote doesn't move | Security Reviewer | git | PASS |
| S5 | After removing the private file, work pushes cleanly | Developer / Security Reviewer | git | PASS, with a correction to the story steps |
| S6 | Known gap: `--no-verify` skips both hooks | Security Reviewer | git | PASS (gap shown as documented) |
| S7 | Known gap: a clone without `core.hooksPath` has no guard | Security Reviewer | git | PASS (gap shown as documented) |
| S8 | The commit-time guard refuses a force-added `TECH-DEBT.md` | Developer / Security Reviewer | git | PASS |
| S9 | `git add .` leaves private files out | Developer / Security Reviewer | git | PASS |
| S10 | `git commit -a` leaves private files out | Developer / Security Reviewer | git | PASS |
| S11 | Following the refusal message word for word fixes the push (run 3) | Developer / Security Reviewer | git | PASS |

## What this means in plain terms

In any copy of the repository that has been set up (`core.hooksPath = .githooks`), a private file
cannot be committed by accident, and cannot be pushed by accident even when the commit check was
skipped. Blanket habits (`git add .`, `git commit -a`) do not pick up private files in the first
place. This holds for every tool that runs git, not just one agent.

## Open Issues

1. **`--no-verify` skips both hooks (S6).** This is a limit of git. Any person or agent who adds the
   flag gets past both checks. Closing it would need a check git can't skip, such as a CI job on the
   PR that fails when a gitignored path is in the diff. Owner decision.
2. **A clone without `core.hooksPath` is unprotected (S7).** The setting has to be added on every
   computer and in every agent's clone. Nothing warns when it is missing. Owner decision on whether
   to enforce it (for example, a session-start check in `sync.sh`).
3. ~~**The pre-push hook's advice line is misleading (found in S5).**~~ **Fixed (#95).** The
   refusal now gives steps that remove the file from the commit. S11 proves that following them
   word for word gives a clean push, both when the private file came with other work and when it
   was committed alone.
4. **Story S5's steps assumed the private commit also held public work.** The test-author should
   update S5 to cover a commit that holds only the private file. The implementer did not edit the
   story; the deviation is recorded in the S5 evidence.
5. **The pre-push hook still fails open when it cannot compare commits** (for example, when the
   remote has a commit this clone never fetched). Raised in the stories' open questions. Not tested
   in this run.
