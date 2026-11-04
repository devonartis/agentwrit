# S11 — Following the Refusal Message, Word for Word, Fixes the Push [ACCEPTANCE]

Run 3 (after the fix for #95), in a brand-new practice area.

Who: A developer, or an agent, who has just been refused at push and has no help except the
message on the screen. The security reviewer watches and checks the result.

What: When the push guard refuses a push, it prints what to do. Before this fix it said
"Remove from commit: `git reset HEAD <path>`". That command only unstages a file. It does not take
it out of the commit, so anyone who followed it was refused again. The message now gives
different steps. The developer follows only those steps, exactly as printed, in two situations:

- Case A: the private file (`MEMORY.md`) came along in the same commit as real work (a `README.md`
  change).
- Case B: the private file (`FLOW.md`) was committed on its own, on top of a branch that is already
  on the remote and otherwise clean.

Why: If the guard's own message sends people in a circle, they reach for `--no-verify` and skip
the guard entirely (see S6). The message has to get an ordinary person unstuck on its own.

How to run: Source `tests/pre-push-guard/env.sh` (in this run the practice area was placed in the
session scratch folder and is shown as `$GUARD_TEST_DIR`). Build a fresh practice remote and a
clone of `fix/pre-push-advice-line` with `core.hooksPath = .githooks`. For each case, make the
commit with `--no-verify` (as in S2), push, read the refusal message, then run only the commands
it prints for that situation and push again. Check the remote and the developer's disk.

Expected:
- The refusal message offers "amend" for a latest commit with other work in it, "reset --soft
  HEAD~1" for a latest commit holding only private files, and advice for older commits. It no
  longer offers `git reset HEAD <path>` as a way to remove a file from a commit.
- Case A: after the steps, the push succeeds. The remote commit holds `README.md` only, with no
  `MEMORY.md`.
- Case B: after the steps, the push succeeds or reports the branch is already up to date. The
  remote has no `FLOW.md`.
- Both private files are still on the developer's disk, ignored.

## Test Output

### Setup

Initialized empty Git repository in $GUARD_TEST_DIR/remote.git/
Cloning into '$GUARD_TEST_DIR/clone'...
done.

### Case A: private file came along with real work

[s11a d076bf4] s11a: real work plus private file
 2 files changed, 3 insertions(+)
 create mode 100644 MEMORY.md

$ git push origin s11a

ERROR: push rejected — private (gitignored) paths would be pushed:

  MEMORY.md

Private files must never enter the public repo.
To fix, for each path listed above:

  If it is in your LATEST commit:
    1. git rm --cached <path>         (the file stays on your disk)
    2. git commit --amend --no-edit
       or, if that commit held nothing but private files:
       git reset --soft HEAD~1
    3. Push again.

  If it is in an OLDER commit: rewrite that commit before pushing
  (for example with git rebase -i) and remove the path there.

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

--- following the printed steps (latest commit, other work in it) ---
$ git rm --cached MEMORY.md
rm 'MEMORY.md'
$ git commit --amend --no-edit
[s11a 83f6d8f] s11a: real work plus private file
 Date: Wed Nov 4 17:11:09 2026 -0500
 1 file changed, 2 insertions(+)
$ git push origin s11a
To $GUARD_TEST_DIR/remote.git
 * [new branch]      s11a -> s11a

files in the remote s11a commit:
README.md
MEMORY.md in the remote s11a tree? (a line below means yes):
MEMORY.md on the developer disk:
!! MEMORY.md

### Case B: private file committed on its own, on an already-pushed branch

$ git push origin s11b   (clean branch, put on the remote first)
To $GUARD_TEST_DIR/remote.git
 * [new branch]      s11b -> s11b
[s11b 6eea255] s11b: private file alone
 1 file changed, 1 insertion(+)
 create mode 100644 FLOW.md

$ git push origin s11b

ERROR: push rejected — private (gitignored) paths would be pushed:

  FLOW.md

Private files must never enter the public repo.
To fix, for each path listed above:

  If it is in your LATEST commit:
    1. git rm --cached <path>         (the file stays on your disk)
    2. git commit --amend --no-edit
       or, if that commit held nothing but private files:
       git reset --soft HEAD~1
    3. Push again.

  If it is in an OLDER commit: rewrite that commit before pushing
  (for example with git rebase -i) and remove the path there.

error: failed to push some refs to '$GUARD_TEST_DIR/remote.git'

--- following the printed steps (latest commit held nothing but private files) ---
$ git rm --cached FLOW.md
rm 'FLOW.md'
$ git reset --soft HEAD~1
$ git push origin s11b
Everything up-to-date

what is staged after the steps (nothing below means nothing staged):
FLOW.md in the remote s11b tree? (a line below means yes):
FLOW.md on the developer disk:
!! FLOW.md

## Verdict

PASS — Both refusals printed the new steps, and the old "git reset HEAD <path>" advice is gone. Case A: following step 1 plus amend, the push succeeded, the remote commit holds README.md only, and MEMORY.md is not on the remote. Case B: following step 1 plus "reset --soft HEAD~1", nothing was left staged, the push reported "Everything up-to-date", and FLOW.md is not on the remote. In both cases the private file is still on the developer's disk, ignored ("!!"). A person who follows only the message gets a clean push.
