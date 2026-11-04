# S1 — A Normal Push Goes Through [ACCEPTANCE]

Run 2 (after the fix for #93).

Who: A developer pushing an ordinary change, observed by the security reviewer. The developer does
what they do every day: edit a public file, commit it by name, push.

What: The developer adds one line to `README.md`, which is a public file. They stage it by name,
commit it on a new branch called `s1-clean`, and push to the practice remote. Both guards run
automatically: one at commit, one at push.

Why: A guard that blocks ordinary work gets turned off by the people it annoys, and then it
protects nobody. If this fails, developers cannot push at all and will find a way around the guard.

How to run: In the practice clone (from PC3), create the branch, append a line to `README.md`,
`git add README.md`, commit, and run `git push origin s1-clean`. We use the real `git push`, the
same command a developer types. Afterwards list the practice remote's branches.

Expected: The commit and the push both succeed, with no refusal message. The remote now has the
branch `s1-clean`.

## Test Output

[s1-clean 9e52583] s1: public change
 1 file changed, 2 insertions(+)
To $GUARD_TEST_DIR/remote.git
 * [new branch]      s1-clean -> s1-clean
  s1-clean

## Verdict

PASS — The commit and the push both succeeded with no refusal, and the remote now has s1-clean. With the fixed hook, a new branch is checked file by file against the private list. Every file in this branch is public, so the guard let it through, as it should.
