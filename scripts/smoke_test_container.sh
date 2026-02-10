#!/usr/bin/env bash
set -euo pipefail

# smoke_test_container.sh — runs live smoke tests against the containerized broker.
# Target is http://localhost:8080 by default.

BASE="${1:-http://localhost:8080}"
PASS=0
FAIL=0

echo "=== Smoke Test: targeting $BASE ==="

check() {
  local name="$1"
  local method="$2"
  local url="$3"
  local expected_status="$4"
  shift 4

  local status
  status=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" "$url" "$@" 2>/dev/null) || true

  if [[ "$status" == "$expected_status" ]]; then
    echo "  PASS: $name (HTTP $status)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected HTTP $expected_status, got $status)"
    FAIL=$((FAIL + 1))
  fi
}

# 1. Health check
check "GET /v1/health" GET "$BASE/v1/health" 200

# 2. Metrics
check "GET /v1/metrics" GET "$BASE/v1/metrics" 200

# 3. Challenge
check "GET /v1/challenge" GET "$BASE/v1/challenge" 200

echo ""
echo "=== Smoke Test Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
exit 0
