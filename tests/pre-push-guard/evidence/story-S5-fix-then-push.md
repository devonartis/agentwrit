# S5 — Removing the File Lets the Push Through [ACCEPTANCE]

Run 2 (after the fix for #93). Not run in run 1.

Who: The developer who was stopped in S4, observed by the security reviewer.

What: The guard refused the developer's push in S4 and told them how to fix it. The developer
follows that advice: take `FLOW.md` out of the commit (keeping it on disk), rewrite the commit
without it, and push again.

Why: A guard that refuses with no clear way out is a guard people disable. This proves the
refusal message leads to a fix that works, and that the private file stays out of the remote
afterwards.

How to run: In the practice clone on `s1-clean` (from S4), run `git rm --cached FLOW.md`, then
`git commit --amend --no-verify -m "s5: private file removed"`, then `git push origin s1-clean`.
Afterwards list the files in the remote's `s1-clean` and look for `FLOW.md`.

Expected: The push succeeds. `FLOW.md` is not in the remote's file list. It is still on the
developer's disk, now untracked again.

## Test Output

rm 'FLOW.md'
You asked to amend the most recent commit, but doing so would make
it empty. You can repeat your command with --allow-empty, or you can
remove the commit entirely with "git reset HEAD^".
On branch s1-clean
No changes

ERROR: push rejected — private (gitignored) paths would be pushed:

  FLOW.md

Private files must never enter the public repo.
Remove from index:  git rm --cached <path>
Remove from commit: git reset HEAD <path>

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

--- Note from the reviewer ---
Attempt 1 above used the steps written in the story. They assumed the S4 commit also held a public change. It held only FLOW.md, so git refused to amend it into an empty commit. The commit kept FLOW.md, and the guard correctly refused the push again. That is a mistake in the story steps, not in the hook.

Also found: the hook's advice line "Remove from commit: git reset HEAD <path>" only unstages a file. It does not take a file out of a commit that already exists. Logged as an open issue.

Attempt 2: follow git's own suggestion and drop the commit with "git reset HEAD^". That keeps FLOW.md on disk. Then push again.

Everything up-to-date

Attempt 2b: continue working. Add a public README line, stage it by name, commit normally (commit-time check active), push.
[s1-clean 6cee6b4] s5: public change after removing private file
 1 file changed, 2 insertions(+)
To $GUARD_TEST_DIR/remote.git
   9e52583..6cee6b4  s1-clean -> s1-clean

Is FLOW.md in the remote s1-clean? (lines matching FLOW.md below; none means absent):
FLOW.md still on the developer disk, untracked:
!! FLOW.md

## Verdict

PASS, with a correction to the story steps — The story's own steps (amend) did not apply, because the S4 commit held only the private file. The guard kept refusing, which is correct. After the developer dropped that commit with "git reset HEAD^", the next real change committed and pushed cleanly (9e52583..6cee6b4). FLOW.md is not on the remote and is still on the developer's disk, ignored ("!!"). Open issue: the hook's advice line "git reset HEAD <path>" does not remove a file from an existing commit and should be corrected.
