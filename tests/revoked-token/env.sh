#!/usr/bin/env bash
# FIX-1 (issue #97) acceptance-test environment: revoked token audit.
#
# Source this ONCE per terminal before running any story:
#     source ./tests/revoked-token/env.sh
# Works in bash or zsh. Run it from the repository root.
#
# NOTHING SECRET LIVES IN THIS FILE. The admin secret must already be in
# your shell (export AA_ADMIN_SECRET='<a strong secret, 32+ characters>').
# App credentials created during the test are kept in a scratch folder
# outside the repository (RT_WORKDIR) and are never printed into evidence.

# ---------------------------------------------------------------- required
# The operator's admin secret. The broker refuses weak or empty secrets.
if [ -z "${AA_ADMIN_SECRET:-}" ]; then
  echo "env.sh: AA_ADMIN_SECRET is not set. Run: export AA_ADMIN_SECRET='<strong secret>' and source this file again." >&2
  return 1 2>/dev/null || exit 1
fi

# ---------------------------------------------------------------- run mode
# RT_MODE picks how the broker runs:
#   container  (default, run FIRST) broker inside Docker via ./scripts/stack_up.sh
#   vps        (run SECOND)         bare compiled binary ./bin/broker on this host
export RT_MODE="${RT_MODE:-container}"
case "$RT_MODE" in
  container|vps) ;;
  *) echo "env.sh: RT_MODE must be 'container' or 'vps' (got '$RT_MODE')" >&2; return 1 2>/dev/null || exit 1 ;;
esac

# Port the broker answers on (same variable stack_up.sh uses).
export AA_HOST_PORT="${AA_HOST_PORT:-8080}"

# Address every story talks to.
export BROKER_URL="http://127.0.0.1:${AA_HOST_PORT}"

# ---------------------------------------------------------------- scratch space
# Scratch folder for app credentials, response bodies and the VPS-mode
# database. Lives outside the repository so nothing secret can be committed.
export RT_WORKDIR="${RT_WORKDIR:-${TMPDIR:-/tmp}/revoked-token-test}"
mkdir -p "$RT_WORKDIR"
export RT_BODY="$RT_WORKDIR/last-response.json"   # body of the most recent rt_call

# ---------------------------------------------------------------- operator CLI (awrit)
# awrit reads these two variables. Operators never type raw HTTP.
export AACTL_BROKER_URL="$BROKER_URL"
export AACTL_ADMIN_SECRET="$AA_ADMIN_SECRET"

# ---------------------------------------------------------------- VPS-mode broker settings
# Only used when you start ./bin/broker yourself (RT_MODE=vps). Container mode
# gets its own data volume from docker-compose.yml.
export AA_BIND_ADDRESS="${AA_BIND_ADDRESS:-127.0.0.1}"
export AA_PORT="${AA_PORT:-$AA_HOST_PORT}"
if [ "$RT_MODE" = "vps" ]; then
  export AA_DB_PATH="$RT_WORKDIR/agentwrit.db"
  export AA_SIGNING_KEY_PATH="$RT_WORKDIR/signing.key"
fi

# ---------------------------------------------------------------- test knobs
# Compiled helper that creates an Ed25519 key and signs the registration nonce
# (Go stdlib, source in tests/sec-l2b/edsign). Built in story RT-PC1.
export RT_EDSIGN="./bin/edsign"
# Lifetime (seconds) of the deliberately short-lived app used to get an
# expired token, and how long to wait before using it.
export RT_SHORT_TTL="${RT_SHORT_TTL:-3}"
export RT_EXPIRY_WAIT="${RT_EXPIRY_WAIT:-6}"
# Scope used by every test app and agent.
export RT_SCOPE="read:data:*"

# ================================================================= helpers
# Small shell helpers so each story reads as a few plain steps. They only
# wrap curl/jq/awrit; they add no behaviour of their own.

# rt_call METHOD PATH TOKEN [JSON_BODY]
#   Sends one HTTP request. Prints ONLY the HTTP status code. The response
#   body is written to $RT_BODY (read it with: cat "$RT_BODY"). TOKEN may be
#   "" for no Authorization header. Persona: Developer / App (curl only).
rt_call() {
  local m="$1" p="$2" t="$3" b="${4:-}"
  local args=(-s -o "$RT_BODY" -w '%{http_code}' -X "$m" "$BROKER_URL$p")
  [ -n "$t" ] && args+=(-H "Authorization: Bearer $t")
  [ -n "$b" ] && args+=(-H "Content-Type: application/json" -d "$b")
  curl "${args[@]}"
}

# rt_b64url_decode STRING : decode base64url (JWT segments).
rt_b64url_decode() {
  local s="$1"
  s="${s//-/+}"; s="${s//_/\/}"
  case $(( ${#s} % 4 )) in 2) s="$s==";; 3) s="$s=";; esac
  printf '%s' "$s" | base64 -d 2>/dev/null
}

# jwt_claim TOKEN FIELD : print one field from a token's (unsigned) payload,
#   e.g. jwt_claim "$AGENT_TOKEN" jti . Read-only, for building test inputs.
jwt_claim() {
  rt_b64url_decode "$(printf '%s' "$1" | cut -d. -f2)" | jq -r ".$2"
}

# rt_app_login FILE : sign the test app in (FILE is app.json or app-short.json
#   in $RT_WORKDIR, written by RT-PC3). Sets APP_TOKEN. Persona: App.
rt_app_login() {
  local id sec
  id=$(jq -r .client_id "$RT_WORKDIR/$1"); sec=$(jq -r .client_secret "$RT_WORKDIR/$1")
  APP_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/app/auth" -H "Content-Type: application/json" \
    -d "$(jq -nc --arg i "$id" --arg s "$sec" '{client_id:$i,client_secret:$s}')" | jq -r .access_token)
  export APP_TOKEN
}

# rt_new_agent NAME ORCH TASK : the app (APP_TOKEN) asks for a launch token and
#   the agent registers itself, exactly as in production. Sets AGENT_TOKEN,
#   AGENT_ID, AGENT_ORCH, AGENT_TASK. Use a UNIQUE TASK per story so a
#   task-level revocation in one story never touches another. Persona: App.
rt_new_agent() {
  local lt nonce sj resp
  lt=$(curl -s -X POST "$BROKER_URL/v1/app/launch-tokens" -H "Authorization: Bearer $APP_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$(jq -nc --arg n "$1" --arg s "$RT_SCOPE" '{agent_name:$n,allowed_scope:[$s]}')" | jq -r .launch_token)
  nonce=$(curl -s "$BROKER_URL/v1/challenge" | jq -r .nonce)
  sj=$("$RT_EDSIGN" "$nonce")
  resp=$(curl -s -X POST "$BROKER_URL/v1/register" -H "Content-Type: application/json" \
    -d "$(jq -nc --arg lt "$lt" --arg n "$nonce" --arg o "$2" --arg t "$3" --arg s "$RT_SCOPE" \
          --argjson k "$sj" '{launch_token:$lt,nonce:$n,public_key:$k.public_key,signature:$k.signature,orch_id:$o,task_id:$t,requested_scope:[$s]}')")
  AGENT_TOKEN=$(printf '%s' "$resp" | jq -r .access_token)
  AGENT_ID=$(printf '%s' "$resp" | jq -r .agent_id)
  AGENT_ORCH="$2"; AGENT_TASK="$3"
  export AGENT_TOKEN AGENT_ID AGENT_ORCH AGENT_TASK
}

# rt_audit_count EVENT_TYPE [extra awrit flags] : print how many audit events
#   of that type exist (optionally filtered, e.g. --agent-id "$AGENT_ID").
#   Persona: Operator (awrit; jq only reads awrit's JSON output).
rt_audit_count() {
  ./bin/awrit audit events --event-type "$1" --limit 1 "${@:2}" --json | jq -r .total
}

# rt_forge_token SUBJECT : a well-formed three-part token that nobody at the
#   broker ever issued (random signature). Used for "never valid" cases.
rt_forge_token() {
  local h p s
  h=$(printf '{"alg":"EdDSA","typ":"JWT"}' | base64 | tr -d '=\n' | tr '+/' '-_')
  p=$(printf '{"sub":"%s","exp":4102444800}' "$1" | base64 | tr -d '=\n' | tr '+/' '-_')
  s=$(head -c 64 /dev/urandom | base64 | tr -d '=\n' | tr '+/' '-_')
  printf '%s.%s.%s' "$h" "$p" "$s"
}

# rt_tamper_token TOKEN : the same token with one character of the signature
#   changed, so it looks real but the signature no longer matches.
rt_tamper_token() {
  local h p s n
  IFS=. read -r h p s <<<"$1"
  if [ "${s:0:1}" = "A" ]; then n="B"; else n="A"; fi
  printf '%s.%s.%s%s' "$h" "$p" "$n" "${s:1}"
}

# rt_forge_for TOKEN : a token carrying the SAME payload (same owner) as TOKEN
#   but with a random signature, i.e. a forgery naming a real agent.
rt_forge_for() {
  local h p s
  IFS=. read -r h p s <<<"$1"
  s=$(head -c 64 /dev/urandom | base64 | tr -d '=\n' | tr '+/' '-_')
  printf '%s.%s.%s' "$h" "$p" "$s"
}

echo "revoked-token env loaded: mode=$RT_MODE broker=$BROKER_URL workdir=$RT_WORKDIR"
