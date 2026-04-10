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
# Safe merge flow (develop → main):
#   1. Commit/push all work on develop
#   2. git checkout main
#   3. git merge develop --no-commit
#   4. ./scripts/strip_for_main.sh
#   5. git add -A
#   6. git commit -m "merge: develop → main (stripped dev files)"
#
# Safety checks:
#   - Refuses to run on the develop branch (must be on main or detached)
#   - Refuses to run with uncommitted changes unless --dry-run
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
echo ""

# Safety: refuse real runs on develop (dry-run is fine — it doesn't touch files)
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
if [[ $DRY_RUN -eq 0 && "$BRANCH" == "develop" ]]; then
  echo "ERROR: refusing to run on 'develop' branch."
  echo "This script strips dev files — run it only on 'main' (or a detached HEAD for a"
  echo "temporary test). On develop you'd lose your tracked dev files."
  echo ""
  echo "Safe flow: checkout main, merge develop --no-commit, then run this script."
  echo "To preview from develop: ./scripts/strip_for_main.sh --dry-run"
  exit 1
fi

# Safety: refuse real runs with uncommitted changes
if [[ $DRY_RUN -eq 0 ]] && ! git diff --quiet HEAD -- 2>/dev/null; then
  echo "ERROR: uncommitted changes detected. Commit or stash first."
  echo "(Running the strip with dirty state would mix strips with other edits.)"
  echo ""
  echo "Override: pass --dry-run to preview without stripping."
  exit 1
fi

STRIPPED=0

strip_path() {
  local path="$1"
  if [[ -e "$path" ]]; then
    if [[ $DRY_RUN -eq 1 ]]; then
      echo "  would remove: $path"
    else
      rm -rf "$path"
      echo "  removed: $path"
    fi
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
