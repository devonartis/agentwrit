# S2 — A Private File Is Refused [ACCEPTANCE]

Run 2 (after the fix for #93). In run 1 this story FAILED: the private file reached the remote.
That evidence is in git history at commit 08e3e31.

Who: A developer who makes the exact slip the push guard exists for, observed by the security
reviewer.

What: `MEMORY.md` is one of the project's private files. It holds internal notes and must never
appear in the public repository. The developer force-adds it (`git add -f` overrides the ignore
list) and commits it with `--no-verify`, which skips the check that normally stops this at commit
time (S8 tests that check). We emulate this slip on purpose. They then try to push the branch
`s2-private` to the practice remote.

Why: If this push goes through, the private file is public. A public repository is copied and
archived by others within minutes, so removing it afterwards does not take it back. The push guard
is the last thing standing between that slip and a permanent leak.

How to run: In the practice clone, branch from `fix/pre-push-guard-stories` as `s2-private`.
Create `MEMORY.md` with a line of text, then `git add -f MEMORY.md`, then
`git commit --no-verify`. Run `git push origin s2-private`. Afterwards list the practice remote's
branches.

Expected: The push is refused with the message "push rejected — private (gitignored) paths would
be pushed" and `MEMORY.md` is named. The remote still has only `s1-clean`. There is no
`s2-private` branch, so nothing arrived.

## Test Output


ERROR: push rejected — private (gitignored) paths would be pushed:

  MEMORY.md

Private files must never enter the public repo.
Remove from index:  git rm --cached <path>
Remove from commit: git reset HEAD <path>

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

branches on the practice remote after the push:
  s1-clean

## Verdict

PASS — The push was refused with "push rejected", MEMORY.md was named, and the remote still has only s1-clean. Nothing arrived. This is the case that failed in run 1, and the fix for #93 closes it.
