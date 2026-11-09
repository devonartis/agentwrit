#!/usr/bin/env bash
# FIX-1 (issue #97) regression runner: revoked token audit.
#
# WHAT THIS IS
#   A short, automatic re-check of the core FIX-1 promises. The regression
#   gate runs it (./scripts/gates.sh regression calls `bash` on this file and
#   counts exit 0 as a pass). A QA tester can also run it by hand:
#       export AA_ADMIN_SECRET='<the running broker's admin secret>'
#       bash tests/revoked-token/regression.sh
#
#   It is NOT the acceptance test. The acceptance evidence comes from a person
#   running the stories in tests/revoked-token/user-stories.md. This runner
#   only catches a later change that quietly breaks FIX-1.
#
# WHAT IT NEEDS
#   - A broker that is ALREADY running (container or VPS mode) at
#     BROKER_URL (default http://localhost:8080).
#   - The compiled programs ./bin/awrit and ./bin/edsign (story RT-PC1 builds
#     them). Never `go run`.
#   - curl and jq.
#   - AA_ADMIN_SECRET (or AACTL_ADMIN_SECRET) in the environment.
#
# SECRETS
#   Read from the environment only. Nothing secret is printed, and nothing is
#   written to disk: tokens and app credentials live only in shell variables
#   for the few seconds the runner takes.
#
# PERSONAS (same split as the stories)
#   Operator steps use ./bin/awrit only. App steps use curl only.
#
# EXIT CODE
#   0 = every check passed. Non-zero = a check failed, a tool is missing,
#   the secret is missing, or no broker answered.
#
# SIDE EFFECT
#   Each run registers one small test app (name rt-regression-<time>-<pid>)
#   and two test agents on the broker. They are left in place, like any other
#   app, so the audit trail stays complete.

set -euo pipefail

# Run from the repository root no matter where we were called from, so the
# ./bin/ paths below work.
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

BROKER_URL="${BROKER_URL:-http://localhost:8080}"
AWRIT=./bin/awrit
EDSIGN=./bin/edsign
SCOPE="read:data:*"
RUN_ID="$(date +%s)-$$"
ORCH="rt-reg-orch-$RUN_ID"
TASK_A="rt-reg-task-a-$RUN_ID"   # agent A: its token gets revoked
TASK_B="rt-reg-task-b-$RUN_ID"   # agent B: its token stays good

FAILED=0

# ------------------------------------------------------------------ output
# pass/fail print one plain line per check. fail does not stop the run, so a
# tester sees every broken promise at once; the exit code reports it at the end.
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; FAILED=$((FAILED + 1)); }

# die stops the run when a set-up step breaks: without it, no check can mean
# anything.
die() { echo "  FAIL  (set-up) $1" >&2; exit 1; }

# ------------------------------------------------------------------ helpers
# http METHOD PATH TOKEN [JSON_BODY]
#   App persona: one curl request. Sets STATUS (HTTP code, 000 if nothing
#   answered) and BODY. TOKEN may be "" for no Authorization header.
http() {
  local args=(-s --max-time 10 -w '\n%{http_code}' -X "$1" "$BROKER_URL$2")
  if [ -n "$3" ]; then args+=(-H "Authorization: Bearer $3"); fi
  if [ -n "${4:-}" ]; then args+=(-H "Content-Type: application/json" -d "$4"); fi
  local out
  out=$(curl "${args[@]}") || out=$'\n000'
  STATUS="${out##*$'\n'}"
  BODY="${out%$'\n'*}"
}

# jwt_claim TOKEN FIELD: read one field from the token's payload (used only
# to find the token ID the operator revokes). Nothing is verified here.
jwt_claim() {
  local s
  s=$(printf '%s' "$1" | cut -d. -f2)
  s="${s//-/+}"; s="${s//_/\/}"
  case $(( ${#s} % 4 )) in 2) s="$s==";; 3) s="$s=";; esac
  printf '%s' "$s" | base64 -d 2>/dev/null | jq -r ".$2"
}

# new_agent NAME TASK: the app gets a launch token and the agent registers
# itself, exactly as in production. Sets AGENT_TOKEN and AGENT_ID.
new_agent() {
  local lt nonce signed
  http POST /v1/app/launch-tokens "$APP_TOKEN" \
    "$(jq -nc --arg n "$1" --arg s "$SCOPE" '{agent_name:$n,allowed_scope:[$s]}')"
  lt=$(jq -r '.launch_token // empty' <<<"$BODY" 2>/dev/null || true)
  [ -n "$lt" ] || die "app could not get a launch token (HTTP $STATUS)"
  http GET /v1/challenge ""
  nonce=$(jq -r '.nonce // empty' <<<"$BODY" 2>/dev/null || true)
  [ -n "$nonce" ] || die "agent could not get a registration challenge (HTTP $STATUS)"
  signed=$("$EDSIGN" "$nonce") || die "edsign could not sign the challenge"
  http POST /v1/register "" \
    "$(jq -nc --arg lt "$lt" --arg n "$nonce" --arg o "$ORCH" --arg t "$2" --arg s "$SCOPE" \
        --argjson k "$signed" \
        '{launch_token:$lt,nonce:$n,public_key:$k.public_key,signature:$k.signature,orch_id:$o,task_id:$t,requested_scope:[$s]}')"
  AGENT_TOKEN=$(jq -r '.access_token // empty' <<<"$BODY" 2>/dev/null || true)
  AGENT_ID=$(jq -r '.agent_id // empty' <<<"$BODY" 2>/dev/null || true)
  [ -n "$AGENT_TOKEN" ] && [ -n "$AGENT_ID" ] || die "agent $1 could not register (HTTP $STATUS)"
}

# revoked_count AGENT_ID: Operator persona. How many token_revoked_access
# events the audit log holds for that agent.
revoked_count() {
  "$AWRIT" audit events --event-type token_revoked_access --agent-id "$1" --limit 1 --json | jq -r .total
}

echo "FIX-1 regression (issue #97) against $BROKER_URL"

# ------------------------------------------------------------------ set-up
# Are the tools here? A missing tool is a failed run, never a silent skip.
command -v curl >/dev/null || die "curl is not installed"
command -v jq   >/dev/null || die "jq is not installed"
[ -x "$AWRIT" ]  || die "$AWRIT is missing; build it: go build -o bin/awrit ./cmd/awrit"
[ -x "$EDSIGN" ] || die "$EDSIGN is missing; build it: go build -o bin/edsign ./tests/sec-l2b/edsign"

# The operator's secret comes from the environment only. It is never echoed.
export AACTL_BROKER_URL="$BROKER_URL"
export AACTL_ADMIN_SECRET="${AACTL_ADMIN_SECRET:-${AA_ADMIN_SECRET:-}}"
[ -n "$AACTL_ADMIN_SECRET" ] || die "AA_ADMIN_SECRET is not set in the environment"

# Is a broker answering? If not, the run fails: no broker, no proof.
http GET /v1/health ""
[ "$STATUS" = "200" ] || die "no healthy broker at $BROKER_URL (HTTP $STATUS)"

# Operator registers a fresh test app (credentials stay in this shell only).
app_json=$("$AWRIT" app register --name "rt-regression-$RUN_ID" --scopes "$SCOPE" --json) \
  || die "operator could not register a test app (is AA_ADMIN_SECRET this broker's secret?)"
client_id=$(jq -r '.client_id // empty' <<<"$app_json")
client_secret=$(jq -r '.client_secret // empty' <<<"$app_json")
[ -n "$client_id" ] && [ -n "$client_secret" ] || die "app registration returned no credentials"

# App signs in.
http POST /v1/app/auth "" \
  "$(jq -nc --arg i "$client_id" --arg s "$client_secret" '{client_id:$i,client_secret:$s}')"
APP_TOKEN=$(jq -r '.access_token // empty' <<<"$BODY" 2>/dev/null || true)
[ -n "$APP_TOKEN" ] || die "app could not sign in (HTTP $STATUS)"

# Two agents, each on its own task so revoking one cannot touch the other.
new_agent "rt-reg-agent-a-$RUN_ID" "$TASK_A"; A_TOKEN="$AGENT_TOKEN"; A_ID="$AGENT_ID"
new_agent "rt-reg-agent-b-$RUN_ID" "$TASK_B"; B_TOKEN="$AGENT_TOKEN"

# Operator cancels agent A's token (token-level revocation).
a_jti=$(jwt_claim "$A_TOKEN" jti 2>/dev/null || true)
[ -n "$a_jti" ] && [ "$a_jti" != "null" ] || die "could not read agent A's token ID"
"$AWRIT" revoke --level token --target "$a_jti" >/dev/null || die "operator could not revoke agent A's token"

# Operator notes how many revoked-use events agent A has before the attempt.
r0=$(revoked_count "$A_ID") || die "operator could not read the audit log"
[[ "$r0" =~ ^[0-9]+$ ]] || die "audit log count was not a number"

echo "Checks:"

# ------------------------------------------------------------------ check 1
# The app keeps using the cancelled token. The broker must give the same plain
# refusal it gives any bad token: 401, "token verification failed", and the
# word "revoked" nowhere. If it said "revoked", a thief would learn they had
# been caught.
http POST /v1/token/renew "$A_TOKEN"
if [ "$STATUS" = "401" ]; then
  pass "1a revoked token is refused with 401"
else
  fail "1a revoked token is refused with 401 (got HTTP $STATUS)"
fi
detail=$(jq -r '.detail // empty' <<<"$BODY" 2>/dev/null || true)
if [ "$detail" = "token verification failed" ]; then
  pass "1b refusal says only \"token verification failed\""
else
  fail "1b refusal says only \"token verification failed\" (got \"$detail\")"
fi
if grep -qi revoked <<<"$BODY"; then
  fail "1c refusal does not contain the word \"revoked\""
else
  pass "1c refusal does not contain the word \"revoked\""
fi
if grep -q access_token <<<"$BODY"; then
  fail "1d no new token is handed out for a revoked token"
else
  pass "1d no new token is handed out for a revoked token"
fi

# ------------------------------------------------------------------ check 2
# The operator, by contrast, must be told everything: exactly one new
# token_revoked_access event naming agent A, its task and orchestrator, the
# door that was tried, and that it was denied.
r1=$(revoked_count "$A_ID") || die "operator could not read the audit log"
[[ "$r1" =~ ^[0-9]+$ ]] || die "audit log count was not a number"
if [ "$((r1 - r0))" -eq 1 ]; then
  pass "2a exactly one new token_revoked_access event for agent A"
else
  fail "2a exactly one new token_revoked_access event for agent A (before=$r0 after=$r1)"
fi
ev=$("$AWRIT" audit events --event-type token_revoked_access --agent-id "$A_ID" --json \
      | jq -c '.events[-1] // {}') || die "operator could not read the audit log"
expect_field() { # FIELD WANT LABEL
  local got
  got=$(jq -r --arg f "$1" '.[$f] // empty' <<<"$ev")
  if [ "$got" = "$2" ]; then pass "$3"; else fail "$3 (got \"$got\")"; fi
}
expect_field agent_id "$A_ID"                          "2b event names agent A"
expect_field task_id  "$TASK_A"                        "2c event names agent A's task"
expect_field orch_id  "$ORCH"                          "2d event names agent A's orchestrator"
expect_field outcome  "denied"                         "2e event outcome is denied"
expect_field resource "/v1/token/renew"                "2f event records the door that was tried"
expect_field detail   "revoked token used | path=/v1/token/renew" "2g event detail has the agreed wording"

# ------------------------------------------------------------------ check 3
# A token nobody cancelled must keep working exactly as before. Agent B
# renews its token and the new token checks out as valid.
http POST /v1/token/renew "$B_TOKEN"
new_b=$(jq -r '.access_token // empty' <<<"$BODY" 2>/dev/null || true)
if [ "$STATUS" = "200" ] && [ -n "$new_b" ]; then
  pass "3a a good token still renews (HTTP 200, new token issued)"
else
  fail "3a a good token still renews (got HTTP $STATUS)"
fi
http POST /v1/token/validate "" "$(jq -nc --arg t "$new_b" '{token:$t}')"
if [ "$STATUS" = "200" ] && [ "$(jq -r '.valid' <<<"$BODY" 2>/dev/null || true)" = "true" ]; then
  pass "3b the renewed good token is valid"
else
  fail "3b the renewed good token is valid (got HTTP $STATUS)"
fi

# ------------------------------------------------------------------ result
echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "FIX-1 regression: all checks passed."
  exit 0
fi
echo "FIX-1 regression: $FAILED check(s) failed."
exit 1
