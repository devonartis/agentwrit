# Pre-Push Guard — User Stories

**Feature:** `.githooks/pre-push` (merged in PR #92). Before a `git push` leaves the machine, the
hook lists every file the push adds or changes. If any of them is on the private list
(`.gitignore`: `.plans/`, `MEMORY.md`, `FLOW.md`, `TECH-DEBT.md`, `CLAUDE.md`, …) the push is
refused and the files are named.

**Why it exists:** the public repo is permanent. A private file pushed there cannot be taken back.
The pre-commit hook only runs on `git commit` and can be skipped, so this is the last check before
the files leave the machine.

**Persona:** Security Reviewer. Their job is to prove the guard stops private files and to state
plainly what it does not stop. They use `git`, the real tool that runs the hook. No broker,
`awrit` or `curl` is involved.

**Mode:** CLI-only. The broker is not involved, so VPS and Container modes do not apply.

**Real consumer:** `git push` itself. The hook is run by git against a real remote (a local bare
repository), not called as a script. We emulate "someone pushes to the public repo" with a local
bare remote so nothing reaches GitHub.

**Authored by:** the test-author, from the PR #92 description and the hook's own header comment.
The implementer does not edit these stories.

---

## Infrastructure Prerequisites

| Prerequisite | Purpose | Smoke Test Story | Status |
|-------------|---------|-----------------|--------|
| `git` and `bash` on the machine | The hook is a bash script run by git | PC1 | NOT VERIFIED |
| `.githooks/pre-push` present and executable on `develop` | The thing under test | PC1 | NOT VERIFIED |
| `core.hooksPath = .githooks` in the clone under test | Git only runs the hook where this is set | PC2 | NOT VERIFIED |
| Throwaway clone + local bare remote | Safe place to push to; proves what arrived | PC3 | NOT VERIFIED |

If any prerequisite cannot be verified, the live tests stop.

---

## PC1 — [PRECONDITION] The hook is on `develop` and can run

The reviewer checks that `.githooks/pre-push` exists in the repository and is marked executable.

**Mode:** CLI-only. **Tool:** `ls -l`, `git ls-files -s`.
**Expected:** the file is tracked and its mode is `100755`.

**Why it is a precondition:** a hook that is not executable is silently ignored by git. Every later
"the push went through" would then look like the guard failing to catch something, when it never ran.

---

## PC2 — [PRECONDITION] Git is told to use the hook

The reviewer checks that the clone under test is configured to run hooks from `.githooks`.

**Mode:** CLI-only. **Tool:** `git config --get core.hooksPath`.
**Expected:** prints `.githooks`.

**Why it is a precondition:** the hook ships in the repository, but git only runs it where this
setting exists. A fresh clone does not have it. This is the gap the PR description calls out, and
S7 shows what happens without it.

---

## PC3 — [PRECONDITION] A throwaway clone and remote exist

The reviewer builds a scratch clone of the repository and points it at an empty local bare
repository standing in for "the public remote". The clone gets `core.hooksPath = .githooks`.

**Mode:** CLI-only. **Tool:** `git clone`, `git init --bare`, `git config`.
**Expected:** the bare remote is empty (no branches), the clone is on `develop`, and the hook runs
in the clone.

**Why it is a precondition:** every push in the stories below goes to this remote. An empty remote
lets later stories prove that a rejected push put nothing there.

---

## S1 — [ACCEPTANCE] A normal push goes through

A developer changes a file that is meant to be public (a line in `README.md`), commits, and pushes
a new branch to the remote.

**Mode:** CLI-only. **Tool:** `git commit`, `git push`.
**Expected:** the push succeeds. The branch exists on the remote.

**Why:** a guard that blocks ordinary work gets switched off. This proves it stays out of the way.

---

## S2 — [ACCEPTANCE] A private file is refused

Someone force-adds `MEMORY.md` (a private file) and commits it, skipping the pre-commit hook with
`--no-verify`. This is the exact slip the pre-push hook exists for, and we emulate it on purpose.
They then try to push.

**Mode:** CLI-only. **Tool:** `git add -f`, `git commit --no-verify`, `git push`.
**Expected:**
- The push fails with `push rejected — private (gitignored) paths would be pushed`.
- The output names `MEMORY.md`.
- The remote still has no branches. Nothing arrived.

**Why:** this is the leak. If the push goes through, a private file is permanently public.

---

## S3 — [ACCEPTANCE] One private file spoils the whole push

The reviewer pushes a commit containing a legitimate public change and a file inside `.plans/`.

**Mode:** CLI-only. **Tool:** `git add -f`, `git commit --no-verify`, `git push`.
**Expected:**
- The push is refused.
- The output names the `.plans/` file and does **not** list the public file.
- The remote still has no branches.

**Why:** a mixed commit is how this happens in practice. The guard must not let a real change carry
a private file out with it, and must tell the person which file to remove.

---

## S4 — [ACCEPTANCE] A private file on an existing branch is refused

The reviewer first pushes a clean branch (S1 state). They then add `FLOW.md` in a new commit on the
same branch and push again.

**Mode:** CLI-only. **Tool:** `git push`.
**Expected:**
- The second push is refused and names `FLOW.md`.
- The remote branch still points at the clean commit from the first push.

**Why:** the guard compares the new commit against what the remote already has. This proves it
catches a file added later to a branch that already exists, not only on a brand-new branch.

---

## S5 — [ACCEPTANCE] Removing the file lets the push through

After S3 or S4, the reviewer removes the private file from the commit (the fix the hook's message
suggests) and pushes again.

**Mode:** CLI-only. **Tool:** `git rm --cached`, `git commit --amend`, `git push`.
**Expected:** the push succeeds, and the remote does not contain the private file.

**Why:** a guard that refuses with no way out is a guard people bypass. This proves the message
leads to a working fix.

---

## S6 — [ACCEPTANCE] Known gap: `--no-verify` skips the guard

The reviewer commits a private file and pushes with `git push --no-verify`.

**Mode:** CLI-only. **Tool:** `git push --no-verify`.
**Expected:** the push **goes through** and the private file reaches the remote. This is not a
defect in the hook. Git does not run hooks when told to skip them.

**Why this story exists:** the PR description lists this as a limit. The evidence has to show it
plainly so no one reads "guard installed" as "private files cannot leak". The verdict is **PASS
when the gap is shown as documented**, and it goes into the README's Open Issues section as a
limit, not a pass for the feature.

---

## S7 — [ACCEPTANCE] Known gap: no setting, no guard

The reviewer makes a second clone and does **not** set `core.hooksPath`. They commit a private file
and push.

**Mode:** CLI-only. **Tool:** `git clone`, `git push`.
**Expected:** the push **goes through**. Git does not know about the hook, so nothing checks the
push.

**Why this story exists:** the hook lives in the repository, but the switch that turns it on lives
in each person's clone. Anyone who clones fresh is unprotected until they set it. The evidence has
to make that visible. The verdict rules are the same as S6: the gap is shown as documented, and it
is listed under Open Issues.

---

## Out of scope

- Server-side protection (GitHub branch rules, push rules). The hook is local only.
- Private files that `.gitignore` does not list. The hook only knows the ignore list.
- Pushing to the real GitHub remote. The stories use a local bare repository so nothing can reach
  the public repo.

## Open questions for the owner

1. S6 and S7 prove two ways around the guard. Should the project add a second layer so these gaps
   are closed (for example, a CI check on the PR that fails if a gitignored path is in the diff)?
   The stories can be extended once that is decided.
2. The hook skips the check silently if it cannot compare the pushed commit with the remote's
   (for example, the remote has a commit this clone has never fetched). Should that case refuse
   instead? Worth a story if yes.
