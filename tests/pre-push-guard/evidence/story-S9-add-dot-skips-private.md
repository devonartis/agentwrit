# S9 — `git add .` Leaves Private Files Out [ACCEPTANCE]

Run 2. New story, added after the owner raised the concern about people and agents that "add
everything".

Who: A developer, or an agent, who stages everything with `git add .` instead of naming files,
observed by the security reviewer.

What: The developer's folder has a public change (a line in `README.md`) and private files sitting
on disk (`FLOW.md`, `TECH-DEBT.md`, left over from earlier stories, as they would be on a real
working copy). The developer runs `git add .` and commits.

Why: Blanket adds are a common habit, for people and for agents. If `git add .` picked up private
files, every such commit would be a near-miss caught only by the hooks. This proves the ignore list
keeps them out, so the hooks only have to catch the deliberate forced case (S8).

How to run: In the practice clone, branch from `fix/pre-push-guard-stories` as `s9`. Confirm the
private files are on disk. Add a line to `README.md`. Run `git add .`, then `git commit -m "s9"`.
List the files in that commit, and show the status of the private files.

Expected: The commit succeeds and contains `README.md` only. `FLOW.md` and `TECH-DEBT.md` are not
in it and still show as ignored ("!!") on disk.

## Test Output

private files on disk before the add:
!! FLOW.md
!! TECH-DEBT.md
[s9 b1e2865] s9
 1 file changed, 2 insertions(+)

files in the s9 commit:
README.md
private files on disk after the commit:
!! FLOW.md
!! TECH-DEBT.md

## Verdict

PASS — With FLOW.md and TECH-DEBT.md on disk, "git add ." plus commit produced a commit containing README.md only. Both private files are still on disk and still ignored ("!!"). A blanket add does not pick up private files.
