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
# Usage (from the develop branch, after merging into a main integration branch):
#   git checkout main
#   git merge develop --no-commit
#   ./scripts/strip_for_main.sh
#   git add -A
#   git commit -m "merge: develop → main (stripped dev files)"
#
# Or as a merge hook if you want it automatic.
#
# What it removes:
#   - Internal tracking: MEMORY.md, MEMORY_ARCHIVE.md, FLOW.md, TECH-DEBT.md,
#     AGENTS.md, CLAUDE.md, CLEANUP_STATUS.md
#   - Planning directory: .plans/
#   - Agent configuration: .claude/, .agents/
#   - Session state: .active_module
#   - Coordination files: COWORK_SESSION.md, COWORK_DOCS_AUDIT.md
#   - Internal docs: docs/QA_REPORT_*.md, /tests/*/evidence/DOC-AUDIT-REPORT.md
#   - Internal reports: /audit/, /AgentAuth_*.docx
#   - Utility scripts: /generate_pdf.py
#
# What it keeps:
#   - All code (cmd/, internal/, pkg/)
#   - Public docs (docs/*.md that aren't internal)
#   - CHANGELOG.md, README.md, CONTRIBUTING.md, SECURITY.md, LICENSE
#   - Tests and test evidence (except DOC-AUDIT-REPORT.md)
#   - Build/deploy files (Dockerfile, docker-compose*.yml, go.mod, scripts/)

STRIPPED=0

strip_path() {
  local path="$1"
  if [[ -e "$path" ]]; then
    rm -rf "$path"
    echo "  removed: $path"
    STRIPPED=$((STRIPPED + 1))
  fi
}

echo "=== strip_for_main.sh ==="
echo ""

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
echo "=== stripped: $STRIPPED paths ==="

# Guard: verify the build still passes after stripping
if command -v go &>/dev/null; then
  echo ""
  echo "=== verifying build ==="
  if go build ./... 2>&1; then
    echo "build: PASS"
  else
    echo "build: FAIL — strip removed something the code depends on"
    exit 1
  fi
fi
