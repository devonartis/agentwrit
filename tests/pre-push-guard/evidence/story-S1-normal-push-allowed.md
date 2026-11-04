# S1 — A Normal Push Goes Through [ACCEPTANCE]

Who: A developer pushing an ordinary change, observed by the security reviewer. The developer does
what they do every day: edit a public file, commit, push.

What: The developer adds one line to `README.md`, which is a public file. They commit it on a new
branch called `s1-clean` and push it to the practice remote. The guard runs automatically as part
of the push.

Why: A guard that blocks ordinary work gets turned off by the people it annoys, and then it
protects nobody. If this fails, developers cannot push at all and will find a way around the guard.

How to run: In the practice clone (from PC3), create the branch, append a line to `README.md`,
commit, and run `git push origin s1-clean`. We use the real `git push`, the same command a
developer types. Afterwards list the practice remote's branches.

Expected: The push succeeds, with no rejection message. The remote now has the branch `s1-clean`.

## Test Output

To $GUARD_TEST_DIR/remote.git
 * [new branch]      s1-clean -> s1-clean
  s1-clean

## Verdict

PASS — The push succeeded with no rejection and the remote now has s1-clean. The guard stays out of the way of ordinary work.

Note added after S2: S2 showed the guard never inspects a real push (it fails open). So this PASS proves only that ordinary pushes are not blocked. It does not prove the guard looked at the push and approved it. Re-run S1 once the hook is fixed.
