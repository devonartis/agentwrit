# S7 — Known Gap: No Setting, No Guard [ACCEPTANCE]

Run 2 (after the fix for #93). Not run in run 1.

Who: The security reviewer.

What: The hooks live in the repository, but the switch that turns them on (`core.hooksPath`) lives
in each person's or agent's copy. A fresh clone does not have it. The reviewer makes a second clone
and does **not** set the switch. They force-add a private file (`MEMORY.md`) and use ordinary
commands, with no `--no-verify`: plain `git commit` and plain `git push`.

Why: A new computer, a new contributor, or a new agent working in a fresh clone is unprotected
until someone sets this. Nothing warns them. The evidence has to make that visible so the team
knows the setting is part of setup, not optional.

How to run: Clone the project on `fix/pre-push-guard-stories` into a second folder in the practice
area. Point its `origin` at the same practice remote. Do not set `core.hooksPath`, and confirm it
prints nothing. Branch `s7-nohook`, create `MEMORY.md`, `git add -f MEMORY.md`, `git commit`
(no skip flag), `git push origin s7-nohook`. Afterwards look for `MEMORY.md` in the remote's
`s7-nohook`.

Expected: Both the commit and the push **go through**, and `MEMORY.md` is on the remote. The
verdict is PASS when the evidence shows the gap exactly as documented.

## Test Output

Cloning into '$GUARD_TEST_DIR/clone-nohook'...
done.
core.hooksPath in the second clone (nothing below means not set):
[s7-nohook b04e3c0] s7: private file, no hook setting
 1 file changed, 1 insertion(+)
 create mode 100644 MEMORY.md
To $GUARD_TEST_DIR/remote.git
 * [new branch]      s7-nohook -> s7-nohook

Is MEMORY.md in the remote s7-nohook? (a line below means yes):
MEMORY.md

## Verdict

PASS (gap shown as documented) — In a fresh clone without core.hooksPath, a plain commit and a plain push (no skip flags) both went through, and MEMORY.md reached the remote. Any clone that has not been set up is unprotected. Listed under Open Issues.
