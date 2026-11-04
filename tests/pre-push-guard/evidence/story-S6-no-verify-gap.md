# S6 — Known Gap: `--no-verify` Skips the Guards [ACCEPTANCE]

Run 2 (after the fix for #93). Not run in run 1.

Who: The security reviewer. Their job here is to show plainly what the guards do **not** stop.

What: Git lets anyone skip hooks by adding `--no-verify`. The reviewer commits a private file
(`CLAUDE.md`, the project's private agent instructions), skipping the commit-time check, and
pushes with `git push --no-verify`, skipping the push check too.

Why: Leaders reading "guard installed" may take it to mean "private files cannot leak". That is
not true, and this story shows why. Anyone, or any agent, who adds `--no-verify` gets past both
hooks. This is a limit of git, not a defect in the hooks. It is listed as an open issue so no one
mistakes the hooks for a complete control.

How to run: In the practice clone, branch from `fix/pre-push-guard-stories` as `s6-noverify`.
Create `CLAUDE.md`, `git add -f CLAUDE.md`, `git commit --no-verify`, then
`git push --no-verify origin s6-noverify`. Afterwards list the files in the remote's
`s6-noverify` and look for `CLAUDE.md`.

Expected: The push **goes through**, and `CLAUDE.md` is on the remote. The verdict is PASS when
the evidence shows the gap exactly as documented.

## Test Output

[s6-noverify 7ed5b6d] s6: private file, hooks skipped
 1 file changed, 1 insertion(+)
 create mode 100644 CLAUDE.md
To $GUARD_TEST_DIR/remote.git
 * [new branch]      s6-noverify -> s6-noverify

Is CLAUDE.md in the remote s6-noverify? (a line below means yes):
CLAUDE.md

## Verdict

PASS (gap shown as documented) — With --no-verify on both commit and push, neither hook ran, and CLAUDE.md reached the remote. This is a limit of git and is listed under Open Issues. It is not a pass for "private files cannot leak".
