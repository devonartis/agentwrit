# S8 — The Commit-Time Guard Refuses a Force-Added Private File [ACCEPTANCE]

Run 2. New story, added after the owner decided guards belong in git hooks so that every agent is
covered.

Who: A developer, or any agent, making the ordinary slip, observed by the security reviewer.

What: Git has no hook that runs on `git add`, so the earliest place a guard can act is the commit.
The developer force-adds `TECH-DEBT.md` (private: the project's internal debt list) and runs a
plain `git commit`. There's no skip flag this time, because this is the everyday case.

Why: This is the first line that stops a private file, before it ever enters history. If it
fails, the push check is the only thing left, and S6 and S7 show how that one can be missed.

How to run: In the practice clone (hooks on), branch from `fix/pre-push-guard-stories` as `s8`.
Record the branch tip. Create `TECH-DEBT.md`, `git add -f TECH-DEBT.md`, run
`git commit -m "s8"`. Record the branch tip again.

Expected: The commit is refused with "refusing commit — gitignored paths are staged in the index"
and `TECH-DEBT.md` is named. The branch tip is the same before and after, so no commit was made.

## Test Output

branch tip BEFORE:
4663250cd0783db6e8e4c1a815fc8e58d545ca25

ERROR: refusing commit — gitignored paths are staged in the index:

  TECH-DEBT.md

These paths are gitignored and must not be committed on any branch.
If you truly need to track one of them, the correct fix is to remove
the pattern from .gitignore, not to force-add past it.

To unstage: git rm --cached <path>  (keeps file on disk, removes from index)

branch tip AFTER:
4663250cd0783db6e8e4c1a815fc8e58d545ca25

## Verdict

PASS — A plain commit with TECH-DEBT.md force-added was refused, and the file was named. The branch tip is 4663250 before and after, so no commit was made. The commit-time guard stops the slip for any tool that runs git in a set-up clone.
