# PC1 — The Hooks Are in the Repository and Can Run [PRECONDITION]

Run 2 (after the fix for #93). Run 1's evidence, which found the pre-push hook broken, is in git
history at commit 08e3e31.

Who: The security reviewer. Their job is to prove the guards work, so they first check the guards
are actually installed.

What: The guards are two small program files, `pre-commit` and `pre-push`, kept in the `.githooks`
folder. Git runs them automatically when someone commits and when someone pushes. The reviewer
checks that both files are part of the repository and are marked as runnable. This run uses the
branch `fix/pre-push-guard-stories`, which carries the fixed pre-push hook. The fix is not on
`develop` yet.

Why: Git quietly ignores a hook that is not marked runnable. It does not warn anyone. If either
file were not runnable, commits or pushes would go through unchecked while everyone believed they
were protected.

How to run: From the repository folder, list both files with `ls -l`, then ask git how it records
them with `git ls-files -s`.

Expected: Both files exist, their permissions include "x" (runnable), and git records both with mode
`100755` (the code for "runnable file").

## Test Output

-rwxr-xr-x@ 1 devonartis  staff  1751 Oct 27 23:25 .githooks/pre-commit
-rwxr-xr-x  1 devonartis  staff  3551 Nov  4 16:47 .githooks/pre-push
100755 4dab55ba656584281584796c99ee66199e9cfc37 0	.githooks/pre-commit
100755 3123eef6fe3ee9d6d0c765031bc9bfd3b464ebb5 0	.githooks/pre-push

## Verdict

PASS — Both hooks exist, are runnable (-rwxr-xr-x), and git records both as mode 100755. Git will run them on commit and on push.
