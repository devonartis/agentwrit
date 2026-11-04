# PC2 — Git Is Told to Use the Hooks [PRECONDITION]

Run 2 (after the fix for #93).

Who: The security reviewer.

What: Having the hook files in the repository is not enough. Git only runs them in a copy of the
repository where a setting called `core.hooksPath` points at the `.githooks` folder. The reviewer
asks git what that setting is in the repository under test.

Why: A fresh copy of the repository does not have this setting, so commits and pushes from it are
not checked at all. If this prints nothing, every "the guard let it through" result below would
mean the guard never ran. Story S7 shows exactly that case.

How to run: From the repository folder, run `git config --get core.hooksPath`.

Expected: It prints `.githooks`.

## Test Output

.githooks

## Verdict

PASS — Git prints .githooks, so it will run both hooks in this repository.
