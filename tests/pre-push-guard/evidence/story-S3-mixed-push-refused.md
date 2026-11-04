# S3 — One Private File Spoils the Whole Push [ACCEPTANCE]

Run 2 (after the fix for #93). Not run in run 1.

Who: The security reviewer.

What: In practice, a private file rarely travels alone. It rides along with real work. The
reviewer makes one commit that contains a legitimate public change (a line in `README.md`) and a
private planning note inside `.plans/`. The private note was force-added and the commit-time check
was skipped, the same slip as S2. They then push the branch `s3-mixed`.

Why: If a real change can carry a private file out with it, the guard only catches careless pushes
of private files on their own, which is the rare case. The guard also has to name the file to
remove, or the person stuck behind it will look for a way around it.

How to run: In the practice clone, branch from `fix/pre-push-guard-stories` as `s3-mixed`. Add a
line to `README.md` and stage it by name. Create `.plans/s3-note.txt` and `git add -f` it. Commit
with `--no-verify`. Run `git push origin s3-mixed`. Afterwards list the practice remote's branches.

Expected: The push is refused. The output names `.plans/s3-note.txt` and does **not** list
`README.md`. The remote still has only `s1-clean`.

## Test Output


ERROR: push rejected — private (gitignored) paths would be pushed:

  .plans/s3-note.txt

Private files must never enter the public repo.
Remove from index:  git rm --cached <path>
Remove from commit: git reset HEAD <path>

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

branches on the practice remote after the push:
  s1-clean

## Verdict

PASS — The whole push was refused. Only .plans/s3-note.txt was named (README.md was not listed), and the remote still has only s1-clean. A real change cannot carry a private file out with it.
