# PC3 — A Throwaway Clone and Remote Exist [PRECONDITION]

Who: The security reviewer.

What: The reviewer does not push to GitHub. They build a safe practice area: an empty "remote"
folder that stands in for the public repository, and a fresh copy of the project that pushes to it.
The copy is told to use the hook, the same way a developer's copy would be.

Why: If a test pushed to the real public repository and the guard failed, a private file would be
public forever. The practice remote also lets us prove, by looking inside it afterwards, that a
refused push put nothing there.

How to run: Source `tests/pre-push-guard/env.sh` for the folder names (in this run the practice
area was placed in the session scratch folder and the path is shown as `$GUARD_TEST_DIR` below).
Create an empty bare remote with `git init --bare`. Clone the project on branch `develop` into the
practice area. Point the clone's `origin` at the practice remote. Set `core.hooksPath` to
`.githooks` in the clone. Then list the remote's branches.

Expected: The remote has no branches. The clone is on `develop` and reports `core.hooksPath` as
`.githooks`.

## Test Output

hint: Using 'master' as the name for the initial branch. This default branch name
hint: will change to "main" in Git 3.0. To configure the initial branch name
hint: to use in all of your new repositories, which will suppress this warning,
hint: call:
hint:
hint: 	git config --global init.defaultBranch <name>
hint:
hint: Names commonly chosen instead of 'master' are 'main', 'trunk' and
hint: 'development'. The just-created branch can be renamed via this command:
hint:
hint: 	git branch -m <name>
hint:
hint: Disable this message with "git config set advice.defaultBranchName false"
Initialized empty Git repository in $GUARD_TEST_DIR/remote.git/
Cloning into '$GUARD_TEST_DIR/clone'...
done.

--- checks ---
clone branch:
develop
clone core.hooksPath:
.githooks
branches on the practice remote (nothing below this line means empty):

## Verdict

PASS — The practice remote is empty, the clone is on develop, and the clone uses .githooks. Every later push goes to this remote and nothing can reach GitHub.
