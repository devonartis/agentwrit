# PC1 — The Hook Is on `develop` and Can Run [PRECONDITION]

Who: The security reviewer. Their job is to prove the guard works, so they first check the guard
is actually installed.

What: The guard is a small program file called `pre-push`, kept in the `.githooks` folder. Git runs
it automatically just before a push leaves the machine. The reviewer checks that the file is part
of the repository and is marked as runnable.

Why: Git quietly ignores a hook that is not marked runnable. It does not warn anyone. If the file
were not runnable, every push would go through unchecked and everyone would believe they were
protected.

How to run: From the repository folder, list the file with `ls -l`, then ask git how it records
the file with `git ls-files -s`. No special setup is needed.

Expected: The file exists, its permissions include "x" (runnable), and git records it with mode
`100755` (the code for "runnable file").

## Test Output

-rwxr-xr-x@ 1 devonartis  staff  3337 Nov  4 16:38 .githooks/pre-push
100755 1e6d4ee110757479f5d381e4229ba061bb400297 0	.githooks/pre-push

## Verdict

PASS — The hook file exists, is runnable (-rwxr-xr-x), and git records it as mode 100755. Git will run it on push.
