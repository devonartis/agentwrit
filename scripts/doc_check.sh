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

# ── Godoc check ──────────────────────────────────────────────────────
# Verify all exported symbols in internal/ packages have doc comments.
echo "[DOC] checking godoc comments on exported symbols..."
GODOC_MISSING=0
for gofile in "$ROOT"/internal/*/*.go; do
  # Skip test files.
  case "$gofile" in *_test.go) continue ;; esac

  # Find exported declarations without a preceding // comment.
  # Match lines like: func FooBar(, type FooBar , var FooBar, const FooBar
  prev=""
  while IFS= read -r line; do
    if echo "$line" | grep -Eq '^(func|type|var|const) [A-Z]' || \
       echo "$line" | grep -Eq '^func \([^)]+\) [A-Z]'; then
      if ! echo "$prev" | grep -Eq '^\s*//'; then
        relpath="${gofile#"$ROOT"/}"
        symbol=$(echo "$line" | sed -E 's/^(func (\([^)]+\) )?|type |var |const )([A-Za-z0-9_]+).*/\3/')
        echo "[DOC:WARN] missing godoc: $relpath -> $symbol"
        GODOC_MISSING=$((GODOC_MISSING + 1))
      fi
    fi
    prev="$line"
  done < "$gofile"
done

if [ "$GODOC_MISSING" -gt 0 ]; then
  fail "found $GODOC_MISSING exported symbols without godoc comments"
fi
echo "[DOC:PASS] all exported symbols have godoc comments"

# ── Endpoint-OpenAPI parity check ────────────────────────────────────
# Verify every mux.Handle path in main.go appears in openapi.yaml.
echo "[DOC] checking endpoint-OpenAPI parity..."
MAIN_FILE="$ROOT/cmd/broker/main.go"
OPENAPI_FILE="$ROOT/docs/api/openapi.yaml"
PARITY_MISSING=0

if [ -f "$MAIN_FILE" ] && [ -f "$OPENAPI_FILE" ]; then
  # Extract endpoint paths from mux.Handle/mux.HandleFunc calls.
  grep -oE 'mux\.Handle(Func)?\("([^"]+)"' "$MAIN_FILE" | \
    sed -E 's/mux\.Handle(Func)?\("([^"]+)"/\2/' | \
    while IFS= read -r endpoint; do
      # Convert /v1/foo to the YAML key format: /v1/foo:
      if ! grep -qF "${endpoint}:" "$OPENAPI_FILE"; then
        echo "[DOC:WARN] endpoint $endpoint in main.go but not in openapi.yaml"
        PARITY_MISSING=$((PARITY_MISSING + 1))
      fi
    done
  # Re-check the exit status by running it again (subshell issue).
  PARITY_COUNT=$(grep -oE 'mux\.Handle(Func)?\("([^"]+)"' "$MAIN_FILE" | \
    sed -E 's/mux\.Handle(Func)?\("([^"]+)"/\2/' | \
    while IFS= read -r endpoint; do
      if ! grep -qF "${endpoint}:" "$OPENAPI_FILE"; then
        echo "MISS"
      fi
    done | wc -l | tr -d ' ')
  if [ "$PARITY_COUNT" -gt 0 ]; then
    fail "found $PARITY_COUNT endpoints in main.go missing from openapi.yaml"
  fi
fi
echo "[DOC:PASS] all endpoints documented in openapi.yaml"

# ── Module doc check ─────────────────────────────────────────────────
# Verify docs/developer/<module>.md exists for known completed modules.
echo "[DOC] checking module documentation..."
MODULE_DOCS=("scaffold" "identity" "token" "authz" "revoke" "mutauth")
for mod in "${MODULE_DOCS[@]}"; do
  check_file "docs/developer/${mod}.md"
done
echo "[DOC:PASS] all module docs present"

# ── Module doc section-depth check ────────────────────────────────────
# Verify each module doc contains required sections:
#   - Purpose/scope statement ("Purpose" or "What exists")
#   - Design/architecture rationale ("Design" or "Architecture" or "Decision")
echo "[DOC] checking module doc section depth..."
MOD_DEPTH_MISSING=0
for mod in "${MODULE_DOCS[@]}"; do
  modfile="docs/developer/${mod}.md"
  # Check purpose/scope section.
  if ! grep -Eq '^## (Purpose|What exists)' "$ROOT/$modfile"; then
    echo "[DOC:WARN] $modfile missing required Purpose/scope section"
    MOD_DEPTH_MISSING=$((MOD_DEPTH_MISSING + 1))
  fi
  # Check design/architecture section.
  if ! grep -Eq '^## .*(Design|Architecture|Decision)' "$ROOT/$modfile"; then
    echo "[DOC:WARN] $modfile missing required Design/Architecture/Decision section"
    MOD_DEPTH_MISSING=$((MOD_DEPTH_MISSING + 1))
  fi
done

if [ "$MOD_DEPTH_MISSING" -gt 0 ]; then
  fail "found $MOD_DEPTH_MISSING missing required sections in module docs"
fi
echo "[DOC:PASS] all module docs have required section depth"
