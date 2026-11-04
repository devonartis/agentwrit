# S10 — `git commit -a` Leaves Private Files Out [ACCEPTANCE]

Run 2. New story, added after the owner raised `commit -a` as a risk.

Who: A developer, or an agent, who commits with `git commit -a` (commit every changed file) instead
of staging files by name, observed by the security reviewer.

What: Same situation as S9: a changed public file (`README.md`) and private files on disk
(`FLOW.md`, `TECH-DEBT.md`). This time the developer stages nothing and runs `git commit -a`.

Why: `commit -a` is the other "grab everything" habit. If it picked up private files, it would be a
second blanket route into history. This proves it only picks up files git already tracks.

How to run: In the practice clone, branch from `fix/pre-push-guard-stories` as `s10`. Add a line to
`README.md`. Run `git commit -a -m "s10"` without staging anything first. List the files in that
commit, and show the status of the private files.

Expected: The commit contains `README.md` only. The private files are not in it and still show as
ignored ("!!").

## Test Output

[s10 5cca7ed] s10
 1 file changed, 2 insertions(+)

files in the s10 commit:
README.md
private files on disk after the commit:
!! FLOW.md
!! TECH-DEBT.md

## Verdict

PASS — "git commit -a" produced a commit containing README.md only. FLOW.md and TECH-DEBT.md stayed out and are still ignored on disk. commit -a only picks up files git already tracks.
