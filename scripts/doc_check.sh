#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

fail() {
  echo "[DOC:FAIL] $1"
  exit 1
}

check_file() {
  local file="$1"
  [ -f "$ROOT/$file" ] || fail "missing file: $file"
}

check_contains() {
  local file="$1"
  local pattern="$2"
  grep -Eq "$pattern" "$ROOT/$file" || fail "$file missing required content: $pattern"
}

check_file "README.md"
check_file "CHANGELOG.md"
check_file ".active_module"
check_file "docs/USER_GUIDE.md"
check_file "docs/DEVELOPER_GUIDE.md"
check_file "docs/API_REFERENCE.md"
check_file "docs/GIT_WORKFLOW.md"
check_file "docs/api/openapi.yaml"

check_contains "README.md" "Documentation"
check_contains "README.md" "docs/USER_GUIDE.md"
check_contains "README.md" "docs/DEVELOPER_GUIDE.md"
check_contains "README.md" "docs/API_REFERENCE.md"
check_contains "README.md" "docs/GIT_WORKFLOW.md"
check_contains "README.md" "CHANGELOG.md"

check_contains "CHANGELOG.md" "^## \\[Unreleased\\]"
check_contains "CHANGELOG.md" "^### Added"
check_contains ".active_module" "^M[0-9]{2}$"

check_contains "docs/USER_GUIDE.md" "## Start the broker"
check_contains "docs/USER_GUIDE.md" "## Run quality gates"
check_contains "docs/USER_GUIDE.md" "AA_LOG_LEVEL"

check_contains "docs/DEVELOPER_GUIDE.md" "## Development workflow"
check_contains "docs/DEVELOPER_GUIDE.md" "## Documentation policy"

check_contains "docs/API_REFERENCE.md" "## Endpoints currently implemented"
check_contains "docs/API_REFERENCE.md" "GET /v1/health"

check_contains "docs/GIT_WORKFLOW.md" "## Branching model"
check_contains "docs/GIT_WORKFLOW.md" "main"
check_contains "docs/GIT_WORKFLOW.md" "develop"
check_contains "docs/GIT_WORKFLOW.md" "feature/"
check_contains "docs/GIT_WORKFLOW.md" "## Commit standards"
check_contains "docs/GIT_WORKFLOW.md" "\.active_module"

check_contains "docs/api/openapi.yaml" "^openapi:"
check_contains "docs/api/openapi.yaml" "/v1/health:"

echo "[DOC:PASS] documentation baseline checks passed"
