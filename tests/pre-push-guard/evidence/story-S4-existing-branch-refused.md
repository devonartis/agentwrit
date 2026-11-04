# S4 — A Private File on an Existing Branch Is Refused [ACCEPTANCE]

Run 2 (after the fix for #93). Not run in run 1.

Who: The security reviewer.

What: Most pushes go to a branch that already exists on the remote, not a new one. The reviewer
goes back to `s1-clean`, which S1 already pushed. They add `FLOW.md` (private: the project's
decision log) in a new commit on top, again force-added with the commit-time check skipped, and
push the same branch again.

Why: For an existing branch, the guard compares the new commit against what the remote already has.
If that comparison were broken, as the input-reading line was in run 1, private files could ride
out on everyday pushes even if brand-new branches were caught.

How to run: In the practice clone, `git switch s1-clean`. Note the remote's tip for `s1-clean`.
Create `FLOW.md`, `git add -f FLOW.md`, commit with `--no-verify`, and run
`git push origin s1-clean`. Then read the remote's tip for `s1-clean` again.

Expected: The push is refused and names `FLOW.md`. The remote's `s1-clean` still points at the same
commit as before the push.

## Test Output

remote s1-clean tip BEFORE the push:
9e5258341d0a8a750bf7b5f70af4cce5d00e13fd

ERROR: push rejected — private (gitignored) paths would be pushed:

  FLOW.md

Private files must never enter the public repo.
Remove from index:  git rm --cached <path>
Remove from commit: git reset HEAD <path>

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

remote s1-clean tip AFTER the push:
9e5258341d0a8a750bf7b5f70af4cce5d00e13fd

## Verdict

PASS — The push to the existing branch was refused and named FLOW.md. The remote s1-clean tip is 9e52583 both before and after, so nothing moved. The guard compares correctly against what the remote already has.
