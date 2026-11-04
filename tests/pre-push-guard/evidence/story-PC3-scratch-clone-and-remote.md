# PC3 — A Throwaway Clone and Remote Exist [PRECONDITION]

Run 2 (after the fix for #93). A brand-new practice area was built for this run. Nothing from
run 1 is reused.

Who: The security reviewer.

What: The reviewer does not push to GitHub. They build a safe practice area: an empty "remote"
folder that stands in for the public repository, and a fresh copy of the project that pushes to it.
The copy is told to use the hooks, the same way a developer's copy would be. It is cloned from the
branch `fix/pre-push-guard-stories`, because that is where the fixed hook lives until it is merged.

Why: If a test pushed to the real public repository and the guard failed, a private file would be
public forever. The practice remote also lets us prove, by looking inside it afterwards, that a
refused push put nothing there.

How to run: Source `tests/pre-push-guard/env.sh` for the folder names (in this run the practice
area was placed in the session scratch folder and the path is shown as `$GUARD_TEST_DIR` below).
Create an empty bare remote with `git init --bare`. Clone the project on branch
`fix/pre-push-guard-stories` into the practice area. Point the clone's `origin` at the practice
remote. Set `core.hooksPath` to `.githooks` in the clone. Then check the clone's branch and setting,
and list the remote's branches.

Expected: The remote has no branches. The clone is on `fix/pre-push-guard-stories` and reports
`core.hooksPath` as `.githooks`.

## Test Output

Initialized empty Git repository in $GUARD_TEST_DIR/remote.git/
Cloning into '$GUARD_TEST_DIR/clone'...
done.

--- checks ---
clone branch:
fix/pre-push-guard-stories
clone core.hooksPath:
.githooks
branches on the practice remote (nothing below this line means empty):

## Verdict

PASS — The practice remote is empty, the clone is on fix/pre-push-guard-stories (the branch with the fixed hook), and the clone uses .githooks. Every later push goes to this remote and nothing can reach GitHub.
