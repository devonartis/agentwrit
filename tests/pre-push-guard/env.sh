#!/usr/bin/env bash
# Source once before running the pre-push-guard stories:  source ./tests/pre-push-guard/env.sh
#
# The stories never push to GitHub. They push a throwaway clone to a throwaway local
# "remote" so a rejected push proves nothing left the machine, and an allowed push
# cannot pollute the public repo.

REPO_ROOT="$(git rev-parse --show-toplevel)"
GUARD_TEST_DIR="${TMPDIR:-/tmp}/pre-push-guard-test"
GUARD_REMOTE="$GUARD_TEST_DIR/remote.git"
GUARD_CLONE="$GUARD_TEST_DIR/clone"

export REPO_ROOT GUARD_TEST_DIR GUARD_REMOTE GUARD_CLONE
