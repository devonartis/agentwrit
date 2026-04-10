#!/usr/bin/env bash
set -euo pipefail

# strip_for_main.sh — remove development-only files before merging develop → main.
#
# Purpose:
#   The `develop` branch carries internal tracking files (MEMORY.md, FLOW.md,
#   TECH-DEBT.md, planning docs, agent configuration, etc.) that are useful
#   during development but should never appear on `main`. This script
#   removes them so `main` stays clean and presentable.
#
# Usage:
#   ./scripts/strip_for_main.sh [--dry-run]
#
#   --dry-run   Print what would be removed without actually removing anything
#
# What this script protects:
#   develop is private and messy; main is the public branch that ships
#   outside the door. This script is the automated safety net that keeps
#   private material off main. It never modifies develop — it only runs
#   on main (or a detached HEAD for testing) to ensure the main tree is
#   stripped before a merge commit lands.
#
# Safe merge flow (develop → main):
#   1. Commit/push all work on develop
#   2. git checkout main
#   3. git merge develop --no-commit       (may conflict on modify/delete)
#   4. ./scripts/strip_for_main.sh         (resolves strip-target conflicts
#                                           by deleting + staging together;
#                                           see "Mid-merge mode" below)
#   5. git add -A                          (stage any remaining clean merges)
#   6. git commit -m "merge: develop → main (stripped dev files)"
#
# Safety checks:
#   - ABSOLUTE: refuses to run on the develop branch. No exceptions.
#     develop is the source of truth for dev artifacts; stripping there
#     would destroy work. See guard below.
#   - Outside a merge: refuses to run with uncommitted changes. Mixing
#     strips with unrelated edits in one commit would destroy the audit
#     trail of what-got-stripped-when.
#   - Mid-merge (.git/MERGE_HEAD exists): the dirty-tree check is skipped
#     because a --no-commit merge always leaves a dirty tree. The
#     "refuses on develop" check is still enforced — we can never be
#     mid-merge ON develop (develop is the source, not the target).
#
# Mid-merge mode:
#   When .git/MERGE_HEAD exists, strip_path uses `git rm -rf` so modify/
#   delete conflicts (a file deleted on main but modified on develop) are
#   resolved by keeping the deletion AND staging the resolution in one
#   step. Without this, the script would delete the file but git would
#   still see it as conflicted, blocking the commit.
#
# What it removes:
#   - Internal tracking: MEMORY.md, MEMORY_ARCHIVE.md, FLOW.md, TECH-DEBT.md,
#     AGENTS.md, CLAUDE.md, COWORK_SESSION.md, COWORK_DOCS_AUDIT.md
#   - Planning directory: .plans/
#   - Agent configuration: .claude/, .agents/
#   - Session state: .active_module
#   - Internal docs: docs/QA_REPORT_*.md, tests/*/evidence/DOC-AUDIT-REPORT.md
#   - Internal reports: audit/, AgentAuth_*.docx
#   - Utility scripts: generate_pdf.py
#
# What it keeps:
#   - All code (cmd/, internal/, pkg/)
#   - Public docs (docs/*.md — anything meant for external readers)
#   - CHANGELOG.md, README.md, CONTRIBUTING.md, SECURITY.md, LICENSE
#   - Tests and test evidence (except DOC-AUDIT-REPORT.md)
#   - Build/deploy files (Dockerfile, docker-compose*.yml, go.mod, scripts/)
#
# main is public-facing product only. All internal thinking — decisions,
# architecture rationale, plans, memory — lives on develop. If a decision
# needs to be shared publicly, write a version in docs/ intentionally.

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
  echo "=== strip_for_main.sh (DRY RUN) ==="
  echo "No files will be removed."
else
  echo "=== strip_for_main.sh ==="
fi

# Detect mid-merge state. MERGE_HEAD exists only between `git merge --no-commit`
# (or a conflict pause) and the eventual merge commit. When this is set we
# know the dirty working tree is expected from the merge, not from random edits.
GIT_DIR=$(git rev-parse --git-dir 2>/dev/null || echo ".git")
IN_MERGE=0
if [[ -f "$GIT_DIR/MERGE_HEAD" ]]; then
  IN_MERGE=1
  echo "(mid-merge mode: MERGE_HEAD present)"
fi
echo ""

# Safety: ABSOLUTE refusal to run on develop. This guard exists no matter
# what mode or flag is passed, and no matter whether a merge is in progress.
# develop is the source of dev artifacts; stripping there would destroy
# work. A mid-merge on develop is also impossible in the documented flow
# (merges go FROM develop INTO main, not the other way), so reaching this
# condition means something has gone badly wrong and we must abort.
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
if [[ $DRY_RUN -eq 0 && "$BRANCH" == "develop" ]]; then
  echo "ERROR: refusing to run on 'develop' branch."
  echo "This script strips dev files — run it only on 'main' (or a detached HEAD"
  echo "for a temporary test). On develop you'd lose your tracked dev files."
  echo ""
  echo "Safe flow: checkout main, merge develop --no-commit, then run this script."
  echo "To preview from develop: ./scripts/strip_for_main.sh --dry-run"
  exit 1
fi

# Safety: refuse dirty tree UNLESS we're in the middle of a merge. A merge
# always leaves a dirty tree (staged adds, conflicted modifies), and the
# documented flow runs the strip right after `git merge --no-commit`. Outside
# a merge, a dirty tree means unrelated edits — stripping would mix them in.
if [[ $DRY_RUN -eq 0 && $IN_MERGE -eq 0 ]] && ! git diff --quiet HEAD -- 2>/dev/null; then
  echo "ERROR: uncommitted changes detected (and we are NOT mid-merge)."
  echo "Commit or stash first. Running the strip with a dirty state would"
  echo "mix strips with unrelated edits in one commit, destroying the audit"
  echo "trail of what-got-stripped-when."
  echo ""
  echo "Override: pass --dry-run to preview without stripping."
  exit 1
fi

STRIPPED=0

# strip_path removes $1 from the working tree. Behavior:
#   - dry run         : only prints what would be removed
#   - mid-merge       : uses `git rm -rf` so modify/delete conflicts are
#                       resolved (deleted AND staged as resolved) in one
#                       step. This is what makes the merge flow work.
#   - normal run      : uses `rm -rf` (classic behavior — works on tracked
#                       and untracked paths alike; any resulting deletions
#                       get staged by the caller's `git add -A`).
strip_path() {
  local path="$1"
  if [[ ! -e "$path" ]] && [[ $IN_MERGE -eq 0 ]]; then
    return 0
  fi
  if [[ $DRY_RUN -eq 1 ]]; then
    if [[ -e "$path" ]]; then
      echo "  would remove: $path"
      STRIPPED=$((STRIPPED + 1))
    fi
    return 0
  fi
  if [[ $IN_MERGE -eq 1 ]]; then
    # --ignore-unmatch: quiet no-op if path doesn't exist (covers the case
    # where the path was already deleted on main and develop also deleted
    # it, leaving nothing to strip).
    if git rm -rf --ignore-unmatch "$path" >/dev/null 2>&1; then
      if [[ -e "$path" ]]; then
        # git rm failed to touch it (e.g. untracked in merge state) — fall
        # back to plain rm and stage later via git add -A.
        rm -rf "$path"
        echo "  removed (untracked): $path"
      else
        echo "  removed (git rm): $path"
      fi
      STRIPPED=$((STRIPPED + 1))
    fi
  else
    rm -rf "$path"
    echo "  removed: $path"
    STRIPPED=$((STRIPPED + 1))
  fi
}

# Top-level internal tracking files
strip_path "MEMORY.md"
strip_path "MEMORY_ARCHIVE.md"
strip_path "FLOW.md"
strip_path "TECH-DEBT.md"
strip_path "AGENTS.md"
strip_path "CLAUDE.md"
strip_path "COWORK_SESSION.md"
strip_path "COWORK_DOCS_AUDIT.md"
strip_path ".active_module"

# Internal directories
strip_path ".plans"
strip_path ".claude"
strip_path ".agents"
strip_path "audit"
strip_path "adr"
strip_path ".vscode"  # editor-specific settings (Snyk IDE prefs, etc.)

# Internal reports and utilities
strip_path "generate_pdf.py"

# Glob patterns — expand and strip each match
for f in AgentAuth_*.docx docs/QA_REPORT_*.md; do
  [[ -e "$f" ]] && strip_path "$f"
done

# Test audit reports
for f in tests/*/evidence/DOC-AUDIT-REPORT.md; do
  [[ -e "$f" ]] && strip_path "$f"
done

echo ""
if [[ $DRY_RUN -eq 1 ]]; then
  echo "=== would strip: $STRIPPED paths ==="
else
  echo "=== stripped: $STRIPPED paths ==="
fi

# Guard: verify the build still passes after stripping
if [[ $DRY_RUN -eq 0 ]] && command -v go &>/dev/null; then
  echo ""
  echo "=== verifying build ==="
  if go build ./... 2>&1; then
    echo "build: PASS"
  else
    echo "build: FAIL — strip removed something the code depends on"
    exit 1
  fi
fi
