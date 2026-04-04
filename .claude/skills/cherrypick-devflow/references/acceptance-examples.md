# Acceptance Test Examples Reference

This file contains real examples of well-written acceptance test stories from this project. Skills and agents should reference this file when writing acceptance tests — it shows the expected quality bar.

See `LIVE-TEST-TEMPLATE.md` for the full process and rules. This file is the examples companion.

---

## Who Reads These?

Executives and QA testers. Every story must make sense to someone who has never seen a line of Go code. If you have to ask an engineer what the story means, it failed.

---

## Example 1: Simple App Story — One Curl Call (B5-S1)

This is the simplest pattern: banner + one curl + verdict. The entire story is created in one bash call, then the verdict is appended after reading the output.

**Step 1: Create the story file with banner + test output**

```bash
F=tests/sec-l2b/evidence/story-S1-generic-validate-error.md
cat > "$F" << 'BANNER'
# L2b-S1 — App Receives a Generic Error When Sending a Bad Token [ACCEPTANCE]

**Mode:** VPS

Who: The app. In production, an app receives tokens from agents and sends
them to the broker's validate endpoint to check whether the agent should be
trusted. This happens automatically, hundreds of times per minute.

What: The app sends a token to the broker that is clearly invalid — it's
not a real token at all, just garbage text. Before this fix, the broker
would tell the app exactly what was wrong with the token (e.g., "token
contains an invalid number of segments"). Now the broker gives a generic
message that reveals nothing about the token's structure.

Why: If the broker reveals why a token failed, an attacker who intercepts
error messages can use that information to craft better forged tokens. For
example, knowing "invalid signature" vs "expired" tells the attacker the
token format is correct and they just need a better signing key. A generic
message ("token is invalid or expired") gives the attacker nothing to work
with.

How to run: We emulate what the app does in production — it sends an HTTP
request to the broker's validate endpoint with a token it received. In this
case, the token is deliberately garbage to trigger an error.

Expected: The broker responds with `{"valid": false}` and a generic error
message: "token is invalid or expired". The response must NOT contain words
like "segment", "signature", "malformed", or any other internal detail.

## Test Output

BANNER
curl -s -X POST http://127.0.0.1:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d '{"token":"not-a-valid-token"}' | jq . >> "$F" 2>&1
echo "" >> "$F"
echo "## Verdict" >> "$F"
echo "" >> "$F"
cat "$F"
```

**Step 2: Read the output, then append the verdict (only after you see the result)**

```bash
echo 'PASS — The broker returned valid=false with the generic message "token is invalid or expired". No internal details (segment, signature, malformed, algorithm) appear in the response. An attacker learns nothing about why the token failed.' >> tests/sec-l2b/evidence/story-S1-generic-validate-error.md
```

### What makes this good:

- **Who** says "The app" and explains what the app does in production
- **What** explains the change in plain English — before vs. after
- **Why** gives a concrete attack scenario an executive can understand
- **How** says "we emulate what the app does" — grounded in reality
- **Verdict** is written AFTER seeing the output, explains the result in security terms

---

## Example 2: Multi-Step Story — Setup + Action + Verify (B5-S2)

When a story needs setup (admin auth, agent registration, revocation), everything goes in ONE bash call. Each step is labeled so the output tells a readable story.

**Step 1: Create the story file with banner + all steps + test output**

```bash
F=tests/sec-l2b/evidence/story-S2-generic-revoked-error.md
cat > "$F" << 'BANNER'
# L2b-S2 — App Receives a Generic Error When Checking a Revoked Token [ACCEPTANCE]

**Mode:** VPS

Who: The app. In production, an app validates tokens from agents before
granting access to resources. Sometimes an operator has revoked an agent's
access — maybe the agent was compromised, maybe the task is done. The app
doesn't know this yet and sends the now-revoked token to the broker.

What: The app sends a previously valid token to the broker's validate
endpoint. The token was revoked by the operator moments ago. Before this
fix, the broker would say "token has been revoked" — which tells an
attacker that the token WAS valid and is now specifically revoked. Now the
broker gives the same generic message as any other invalid token.

Why: An attacker who stole a revoked token and gets told "this was revoked"
knows the token was real. They can try to find the signing key or look for
other tokens from the same agent. A generic message ("token is invalid or
expired") gives them nothing — they can't tell if the token was ever valid.

How to run: We emulate the full production flow. First, get admin access.
Then create a launch token, register an agent (with challenge-response),
and get the agent's token. Then the operator revokes the agent. Finally,
the app tries to validate the revoked token.

Expected: The broker responds with `{"valid": false}` and the generic
message "token is invalid or expired" — NOT "token has been revoked".

## Test Output

BANNER

echo "--- Step 1: Admin authenticates ---" >> "$F"
ADMIN_TOKEN=$(curl -sf -X POST http://127.0.0.1:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d '{"secret":"live-test-secret-32bytes-long-ok"}' | jq -r .access_token)
echo "Admin token: ${ADMIN_TOKEN:0:30}..." >> "$F"

echo "" >> "$F"
echo "--- Step 2: Create launch token and register agent ---" >> "$F"
LT=$(curl -sf -X POST http://127.0.0.1:8080/v1/admin/launch-tokens \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"s2-revoke-test","allowed_scope":["read:data:*"],"max_ttl":300}' | jq -r .launch_token)
NONCE=$(curl -sf http://127.0.0.1:8080/v1/challenge | jq -r .nonce)
REG=$(python3 -c "
import json, base64, urllib.request
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
key = Ed25519PrivateKey.generate()
pub = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
sig = key.sign(bytes.fromhex('${NONCE}'))
body = json.dumps({'launch_token':'${LT}','nonce':'${NONCE}','public_key':base64.b64encode(pub).decode(),'signature':base64.b64encode(sig).decode(),'orch_id':'s2-orch','task_id':'s2-task','requested_scope':['read:data:*']}).encode()
req = urllib.request.Request('http://127.0.0.1:8080/v1/register', data=body, headers={'Content-Type':'application/json'})
print(urllib.request.urlopen(req).read().decode())
")
AGENT_TOKEN=$(echo "$REG" | jq -r .access_token)
AGENT_ID=$(echo "$REG" | jq -r .agent_id)
echo "Agent ID: $AGENT_ID" >> "$F"
echo "Agent token: ${AGENT_TOKEN:0:30}..." >> "$F"

echo "" >> "$F"
echo "--- Step 3: Operator revokes the agent ---" >> "$F"
curl -sf -X POST http://127.0.0.1:8080/v1/revoke \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"level\":\"agent\",\"target\":\"${AGENT_ID}\"}" | jq . >> "$F" 2>&1

echo "" >> "$F"
echo "--- Step 4: App validates the revoked token ---" >> "$F"
curl -s -X POST http://127.0.0.1:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"${AGENT_TOKEN}\"}" | jq . >> "$F" 2>&1

echo "" >> "$F"
echo "## Verdict" >> "$F"
echo "" >> "$F"
cat "$F"
```

**Step 2: Read the output, then append the verdict**

```bash
echo 'PASS — The operator revoked agent spiffe://agentauth.local/agent/s2-orch/s2-task/d72568a5b27b3fab. When the app validated the revoked token, the broker returned "token is invalid or expired" — identical to any other bad token. An attacker cannot tell whether the token was ever valid.' >> tests/sec-l2b/evidence/story-S2-generic-revoked-error.md
```

### What makes this good:

- **All setup inside the same bash call** — admin auth, launch token, registration, revocation, then the actual test
- **Labeled steps** so the output reads like a story
- **Agent registration uses challenge-response** — the real flow, not a shortcut
- **Verdict references the actual agent ID** from the output — it's real, not templated

---

## Example 3: Security Reviewer Checks Headers (B5-S4)

This shows checking response headers across multiple endpoints using a loop. Note: use `curl -s -D - -o /dev/null` for POST endpoints (not `curl -sI` which returns empty headers on POST).

**Step 1: Create the story with a loop over endpoints**

```bash
F=tests/sec-l2b/evidence/story-S4-security-headers.md
cat > "$F" << 'BANNER'
# L2b-S4 — Every Response Includes Security Headers [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer. They are checking that the broker sets
protective HTTP headers on every response, not just on specific endpoints.

What: Modern browsers and HTTP clients use security headers to protect
against common attacks. The broker now adds three headers to every single
response it sends, regardless of which endpoint was called:

- X-Content-Type-Options: nosniff — prevents browsers from guessing the
  content type, which stops a class of attacks where a malicious file is
  served with the wrong type.
- X-Frame-Options: DENY — prevents the broker's responses from being
  embedded in an iframe, which stops clickjacking attacks.
- Cache-Control: no-store — prevents sensitive responses (like tokens)
  from being cached by browsers or proxies.

Why: Without these headers, a browser or proxy could cache tokens, frame
the broker UI for clickjacking, or misinterpret response types. These are
standard security best practices — OWASP recommends all three.

How to run: The reviewer checks the response headers on three different
endpoints: health (public), metrics (public), and token validate (POST).
All three must include all three security headers.

Expected: All three headers present on every endpoint.

## Test Output

BANNER

for EP in "/v1/health" "/v1/metrics"; do
  echo "--- $EP ---" >> "$F"
  curl -s -D - -o /dev/null "http://127.0.0.1:8080${EP}" | grep -iE "x-content-type|x-frame|cache-control" >> "$F" 2>&1
  echo "" >> "$F"
done
echo "--- /v1/token/validate (POST) ---" >> "$F"
curl -s -D - -o /dev/null -X POST http://127.0.0.1:8080/v1/token/validate \
  -H "Content-Type: application/json" \
  -d '{"token":"x"}' | grep -iE "x-content-type|x-frame|cache-control" >> "$F" 2>&1
echo "" >> "$F"

echo "## Verdict" >> "$F"
echo "" >> "$F"
cat "$F"
```

**Step 2: Append verdict after seeing the output**

```bash
echo 'PASS — All three security headers present on every endpoint tested: /v1/health, /v1/metrics, and /v1/token/validate. The SecurityHeaders middleware is working globally.' >> tests/sec-l2b/evidence/story-S4-security-headers.md
```

### What makes this good:

- **Uses `curl -s -D - -o /dev/null`** instead of `curl -sI` for POST endpoints
- **What** explains each header in plain English — what it prevents
- **Why** connects to OWASP — an executive recognizes that name

---

## Example 4: Operator Story (B2 — P1)

This is an **Operator** story using `aactl`. Operators use CLI tools, not curl.

```markdown
# P1-S1 — Operator Generates Admin Secret in Dev Mode [ACCEPTANCE]

Who: The operator.

What: The operator runs aactl init with no flags to generate a config file
in development mode. The plaintext secret is stored in the file for convenience.

Why: If aactl init doesn't work, the operator can't bootstrap the broker
at all — there's no admin secret, no config file, no way to authenticate.

How to run: Clean temp dir, run aactl init, check output and file contents.

Expected: Config file with MODE=development, plaintext secret, 0600 perms, 0700 dir.
```

### What makes this good:

- **Who** is the operator — they use `aactl`, not curl
- **Why** is stark: "the operator can't bootstrap the broker at all"
- **Expected** includes security detail (file permissions) that matters to operators

---

## Common Mistakes

| Mistake | Why it's wrong | Fix |
|---------|---------------|-----|
| `Who: Developer (curl)` when the real actor is an app | In production, software calls the API, not a person | `Who: The app. In production, the app validates tokens...` |
| `What: POST /v1/token/validate with invalid token` | This is the HTTP call, not the business action | `What: The app sends a token to check if an agent should be trusted...` |
| `Why: H3 — JWT errors must not leak` | Jargon. An executive doesn't know what H3 means | `Why: If the broker reveals why a token failed, an attacker can craft better forgeries...` |
| `Expected: 200 OK with valid=false` | Technical without context | `Expected: The broker says the token is invalid but does NOT reveal why...` |
| Pre-writing PASS before seeing output | The verdict must be based on observed results | Run the test, see the output, then write the verdict |
| Bulk script dump as evidence | Not individual stories, not readable by QA | One `story-*.md` file per story with banner + output + verdict |
| Using `curl -sI` for POST endpoints | Returns empty headers | Use `curl -s -D - -o /dev/null` to dump headers on POST |
