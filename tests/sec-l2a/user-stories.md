# SEC-L2a: Token Hardening — User Stories

**Branch:** `fix/sec-l2a`
**Docker stack:** `./scripts/stack_up.sh`

---

## Testing Scenarios — VPS First, Container Second

> **IMPORTANT:** Every broker story in this file is tested in TWO deployment
> modes. VPS mode runs FIRST to prove the application works. Container mode
> runs SECOND to prove the deployment works. If VPS passes but Container
> fails, the bug is in deployment config. If VPS fails, the bug is in the
> application — don't bother testing Container until VPS passes.

### VPS Mode (Application Test)

Run the compiled broker binary directly on the host machine. No Docker, no
containers, no volume mounts.

```bash
# Build binaries
go build -o bin/broker ./cmd/broker/
go build -o bin/aactl ./cmd/aactl/

# Generate config and start broker
./bin/aactl init --config-path /tmp/aa-test-l2a/config
AA_CONFIG_PATH=/tmp/aa-test-l2a/config \
AA_DB_PATH=/tmp/agentauth-l2a.db \
AA_SIGNING_KEY_PATH=/tmp/signing-l2a.key \
  ./bin/broker
```

### Container Mode (Deployment Test)

```bash
./scripts/stack_up.sh
```

### Story Categories

| Stories | Mode | Why |
|---------|------|-----|
| S1 | VPS + Container | Baseline — proves auth still works after hardening |
| S2, S3 | VPS | MaxTTL config — requires broker restart with different env vars |
| S4, S5 | VPS + Container | Revocation — tests runtime token lifecycle |
| S6 | VPS | Startup warning — check broker stdout |
| S7 | VPS + Container | Backward compat — runtime token behavior |
| N1, N2 | VPS + Container | JWT tampering — runtime security checks |
| N3, N5 | Unit test | Can't easily trigger from outside the broker |
| N4 | VPS + Container | Regression — wrong secret rejection |
| SEC1 | Code inspection | Manual code review |

---

## Positive Stories

---

## L2a-S1: Operator Authenticates and Lists Apps After Hardening [ACCEPTANCE]

**Tracker:** b4-s1
**Persona:** Operator
**Tool:** aactl + curl

The operator just deployed the token hardening update to the broker. Before
doing anything else, they want to confirm the basics still work — log in
with their admin secret, list the registered apps, and verify the JWT the
broker issued uses the correct algorithm and key ID. This is the first thing
an operator does after any security update: prove the system still works
before telling the team to proceed. If admin auth or app listing broke, the
hardening damaged something fundamental and needs to be rolled back.

**Route:** POST /v1/admin/auth → GET /v1/admin/apps
**Mode:** VPS + Container
**Steps:**
1. Source env.sh
2. Authenticate with the admin secret using aactl or curl, get a JWT
3. Decode the JWT header and check it says `alg: EdDSA` and has a `kid` field
4. Use the JWT to call GET /v1/admin/apps
5. Verify the call succeeds (200)

**Expected:**
- Admin auth returns 200 with a JWT
- JWT header contains `"alg":"EdDSA"` and a non-empty `kid`
- GET /v1/admin/apps returns 200 with valid JSON

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S1-auth-and-list-apps.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S1 — Operator Authenticates and Lists Apps After Hardening

Who: The operator.

What: The operator just deployed the token hardening update (B4). Before
doing anything else, they confirm the basics still work — admin auth, app
listing, and that the JWT uses EdDSA with a key ID. This is the first
check after any security update.

Why: If admin auth or app listing broke, the hardening damaged something
fundamental. The operator needs to know immediately so they can roll back.

How to run: Source the environment file. Authenticate with the admin secret.
Decode the JWT header to verify alg=EdDSA and kid is present. Then list apps.

Expected: Admin auth returns 200 with a JWT. JWT header has alg=EdDSA and
a non-empty kid. GET /v1/admin/apps returns 200 with valid JSON.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh
TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Token received: ${TOKEN:0:20}..." >> "$F"
echo "" >> "$F"
echo "JWT header:" >> "$F"
echo "$TOKEN" | cut -d. -f1 | base64 -d 2>/dev/null | jq . >> "$F" 2>&1
echo "" >> "$F"
echo "GET /v1/admin/apps:" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S2: Operator Caps Token Lifetime With MaxTTL [ACCEPTANCE]

**Tracker:** b4-s2
**Persona:** Operator
**Tool:** broker binary + curl

The operator runs AgentAuth at a financial services company. Their compliance
team requires that no credential lives longer than 60 seconds. The operator
sets `AA_MAX_TTL=60` when starting the broker. Even though the default TTL
is 300 seconds, the broker should clamp every issued token to the 60-second
ceiling. This prevents anyone — including admins — from accidentally or
maliciously issuing long-lived tokens that violate the compliance policy.

**Route:** POST /v1/admin/auth
**Mode:** VPS (requires broker restart with custom env var)
**Steps:**
1. Start the broker with `AA_MAX_TTL=60`
2. Authenticate and get a JWT
3. Decode the JWT payload (base64url decode the middle segment)
4. Calculate `exp - iat` (or `exp - now`)
5. Verify the difference is ≤ 60 seconds, NOT the default 300

**Expected:**
- Token is issued successfully
- `exp - iat` is approximately 60 seconds (not 300)
- The token works for authentication during its shortened lifetime

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S2-maxttl-cap.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S2 — Operator Caps Token Lifetime With MaxTTL

Who: The operator.

What: The operator sets AA_MAX_TTL=60 to enforce a 60-second ceiling on all
tokens. Even though the default TTL is 300 seconds, the broker should clamp
every issued token down to the 60-second ceiling. This is a compliance
requirement at a financial services company.

Why: Without MaxTTL enforcement, anyone — including admins — could issue
long-lived tokens that violate the compliance policy. If this test fails,
the compliance ceiling doesn't work and the operator can't safely deploy
to regulated environments.

How to run: Start the broker with AA_MAX_TTL=60. Authenticate, decode the
JWT payload, and check that exp - iat is approximately 60, not 300.

Expected: Token issued successfully. exp - iat is approximately 60 seconds.

## Test Output

BANNER
pkill -f './bin/broker' 2>/dev/null; sleep 1
AA_MAX_TTL=60 AA_ADMIN_SECRET=live-test-secret-32bytes-long-ok \
AA_DB_PATH=/tmp/agentauth-l2a.db AA_SIGNING_KEY_PATH=/tmp/signing-l2a.key \
  ./bin/broker &
BROKER_PID=$!
sleep 2

TOKEN=$(curl -s -X POST "http://127.0.0.1:8080/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"live-test-secret-32bytes-long-ok"}' | jq -r '.access_token')
echo "Token received: ${TOKEN:0:20}..." >> "$F"
echo "" >> "$F"
echo "Decoded payload:" >> "$F"
echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq '{exp, iat, ttl_seconds: (.exp - .iat)}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
kill $BROKER_PID 2>/dev/null
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S3: Operator Disables MaxTTL Ceiling [ACCEPTANCE]

**Tracker:** b4-s3
**Persona:** Operator
**Tool:** broker binary + curl

The operator runs a development environment where long-lived tokens are fine.
They set `AA_MAX_TTL=0` to disable the ceiling entirely. The default TTL
(300 seconds) should be used without clamping. This proves the ceiling is
optional and doesn't interfere when disabled.

**Route:** POST /v1/admin/auth
**Mode:** VPS
**Steps:**
1. Start the broker with `AA_MAX_TTL=0`
2. Authenticate and get a JWT
3. Decode the payload and check `exp - iat`
4. Verify it matches the default TTL (300 seconds), not clamped

**Expected:**
- Token TTL is approximately 300 seconds (the default)
- No clamping applied

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S3-maxttl-disabled.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S3 — Operator Disables MaxTTL Ceiling

Who: The operator.

What: The operator sets AA_MAX_TTL=0 to disable the ceiling in a dev
environment. The default TTL of 300 seconds should apply without any
clamping.

Why: If disabling the ceiling still clamps tokens, the feature interferes
with dev environments. The ceiling must be optional.

How to run: Start the broker with AA_MAX_TTL=0. Authenticate, decode the
JWT payload, and check that exp - iat is approximately 300, not clamped.

Expected: Token TTL is approximately 300 seconds. No clamping applied.

## Test Output

BANNER
pkill -f './bin/broker' 2>/dev/null; sleep 1
AA_MAX_TTL=0 AA_ADMIN_SECRET=live-test-secret-32bytes-long-ok \
AA_DB_PATH=/tmp/agentauth-l2a.db AA_SIGNING_KEY_PATH=/tmp/signing-l2a.key \
  ./bin/broker &
BROKER_PID=$!
sleep 2

TOKEN=$(curl -s -X POST "http://127.0.0.1:8080/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"live-test-secret-32bytes-long-ok"}' | jq -r '.access_token')
echo "Token received: ${TOKEN:0:20}..." >> "$F"
echo "" >> "$F"
echo "Decoded payload:" >> "$F"
echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq '{exp, iat, ttl_seconds: (.exp - .iat)}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
kill $BROKER_PID 2>/dev/null
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S4: Revoked Token Is Rejected Everywhere [ACCEPTANCE]

**Tracker:** b4-s4
**Persona:** Security Reviewer
**Tool:** curl

The security reviewer is verifying that token revocation actually works end
to end. Before this security hardening, revocation was only checked in HTTP
middleware — if any code path inside the broker skipped the middleware
(internal calls, future endpoints), a revoked token could sneak through.
Now the revocation check is inside `Verify()` itself, so every code path
that validates a token also checks revocation. The reviewer issues a token,
confirms it works, then revokes it by renewing (which revokes the
predecessor), and confirms the old token is dead.

**Route:** POST /v1/admin/auth → POST /v1/token/renew → GET /v1/admin/apps (with old token)
**Mode:** VPS + Container
**Steps:**
1. Authenticate and get token A
2. Use token A on GET /v1/admin/apps — should succeed (200)
3. Renew token A → get token B (this revokes token A)
4. Use token A on GET /v1/admin/apps again — should fail (401)
5. Use token B on GET /v1/admin/apps — should succeed (200)

**Expected:**
- Token A works before renewal (200)
- After renewal, token A returns 401
- Token B (the new token) returns 200

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S4-revoked-token-rejected.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S4 — Revoked Token Is Rejected Everywhere

Who: The security reviewer.

What: The reviewer verifies that token revocation works end to end. Before
this hardening, revocation was only checked in HTTP middleware. Now the check
is inside Verify() itself, so every code path that validates a token also
checks revocation. The reviewer issues a token, confirms it works, renews it
(which revokes the old one), and confirms the old token is dead.

Why: If a revoked token still works on any endpoint, an attacker who stole a
token can keep using it indefinitely even after the legitimate user renewed.
This is a critical security gap.

How to run: Source env. Get token A. Use it on /v1/admin/apps (should work).
Renew it to get token B (revokes A). Try token A again (should fail 401).
Try token B (should work 200).

Expected: Token A returns 200 before renewal, 401 after renewal. Token B
returns 200.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

TOKEN_A=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Token A: ${TOKEN_A:0:20}..." >> "$F"
echo "" >> "$F"

echo "--- Token A before renewal ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN_A" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"

TOKEN_B=$(curl -s -X POST "$BROKER_URL/v1/token/renew" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_A" | jq -r '.access_token')
echo "Token B (after renewal): ${TOKEN_B:0:20}..." >> "$F"
echo "" >> "$F"

echo "--- Token A after renewal (should be 401) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN_A" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"

echo "--- Token B (should be 200) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN_B" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S5: Token Renewal Issues New Token and Kills Old One [ACCEPTANCE]

**Tracker:** b4-s5
**Persona:** Operator
**Tool:** curl

The operator verifies that token renewal works correctly after deploying
the hardening. When a token is renewed, the old token must be revoked as
part of the renewal — otherwise both old and new tokens remain valid, and
if one is stolen, there's no way to limit the blast radius. After this
hardening, renewal is transactional: new token issued only if old token
is successfully revoked. The operator tests this by renewing a token and
confirming the old one is dead.

**Route:** POST /v1/admin/auth → POST /v1/token/renew
**Mode:** VPS + Container
**Steps:**
1. Get token A
2. Renew it → get token B
3. Decode both JWTs and compare their `jti` claims — they should be different
4. Use token B on an authenticated endpoint — should work
5. Use token A on an authenticated endpoint — should fail (revoked)

**Expected:**
- Renewal returns 200 with a new JWT
- New token has a different `jti` than the old one
- New token works, old token is rejected

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S5-token-renewal.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S5 — Token Renewal Issues New Token and Kills Old One

Who: The operator.

What: The operator verifies that token renewal is transactional. Renewing a
token must revoke the old one and issue a new one with a different JTI. If
both tokens remain valid after renewal, a stolen token can't be contained.

Why: Without transactional renewal, an attacker who stole a token keeps a
valid credential even after the legitimate user renewed. The blast radius
is unlimited.

How to run: Source env. Get token A. Renew it to get token B. Decode both
JWTs and compare jti claims. Use token B (should work). Use token A (should
fail — it was revoked by the renewal).

Expected: Renewal returns 200. Token B has a different jti than token A.
Token B works, token A returns 401.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

TOKEN_A=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')

TOKEN_B=$(curl -s -X POST "$BROKER_URL/v1/token/renew" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN_A" | jq -r '.access_token')

echo "Token A jti:" >> "$F"
echo "$TOKEN_A" | cut -d. -f2 | base64 -d 2>/dev/null | jq '.jti' >> "$F" 2>&1
echo "Token B jti:" >> "$F"
echo "$TOKEN_B" | cut -d. -f2 | base64 -d 2>/dev/null | jq '.jti' >> "$F" 2>&1
echo "" >> "$F"

echo "--- Token B on /v1/admin/apps ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN_B" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"

echo "--- Token A on /v1/admin/apps (should be 401) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN_A" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S6: Broker Warns When DefaultTTL Exceeds MaxTTL [ACCEPTANCE]

**Tracker:** b4-s6
**Persona:** Operator
**Tool:** broker binary startup logs

The operator accidentally misconfigures the broker with `AA_DEFAULT_TTL=7200`
(2 hours) but `AA_MAX_TTL=3600` (1 hour). This means every token will be
silently clamped from the 2-hour default down to 1 hour. Without a warning,
the operator would spend hours debugging why tokens expire sooner than
expected. The broker should start successfully (not crash) but log a WARN
so the operator notices the misconfiguration immediately.

**Route:** N/A (startup behavior)
**Mode:** VPS
**Steps:**
1. Start the broker with `AA_DEFAULT_TTL=7200 AA_MAX_TTL=3600`
2. Read the startup logs
3. Look for a WARN line containing both TTL values
4. Verify the broker started (didn't crash)
5. Verify admin auth still works

**Expected:**
- Broker starts successfully
- Startup log contains WARN with `default_ttl=7200` and `max_ttl=3600`
- Admin auth returns 200 (broker functions normally)

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S6-ttl-warning.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S6 — Broker Warns When DefaultTTL Exceeds MaxTTL

Who: The operator.

What: The operator accidentally sets AA_DEFAULT_TTL=7200 but AA_MAX_TTL=3600.
Every token would be silently clamped. The broker should warn on startup so
the operator notices the misconfiguration immediately.

Why: Without the warning, the operator would spend hours debugging why tokens
expire at 1 hour instead of the expected 2 hours. Silent clamping is a trap.

How to run: Start the broker with the misconfigured TTL values. Check the
startup logs for a WARN line. Verify admin auth still works.

Expected: Broker starts. Startup log has WARN with default_ttl=7200 and
max_ttl=3600. Admin auth returns 200.

## Test Output

BANNER
pkill -f './bin/broker' 2>/dev/null; sleep 1
AA_DEFAULT_TTL=7200 AA_MAX_TTL=3600 \
AA_ADMIN_SECRET=live-test-secret-32bytes-long-ok \
AA_DB_PATH=/tmp/agentauth-l2a.db AA_SIGNING_KEY_PATH=/tmp/signing-l2a.key \
  ./bin/broker > /tmp/broker-s6.log 2>&1 &
BROKER_PID=$!
sleep 2

echo "Broker startup log:" >> "$F"
cat /tmp/broker-s6.log >> "$F"
echo "" >> "$F"

echo "Admin auth test:" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "http://127.0.0.1:8080/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"live-test-secret-32bytes-long-ok"}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
kill $BROKER_PID 2>/dev/null
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-S7: Tokens With Empty kid Are Still Accepted [ACCEPTANCE]

**Tracker:** b4-s7
**Persona:** Operator
**Tool:** curl + unit test

The operator verifies backward compatibility after deploying the hardening.
Tokens issued before this update don't have a `kid` (key ID) in their JWT
header — the `kid` field was added as part of this hardening. The broker
must still accept tokens with an empty or missing `kid`. If it rejected
them, every agent and pipeline would need to re-authenticate immediately
after the upgrade, causing an outage.

**Route:** Any authenticated endpoint
**Mode:** VPS + Container
**Steps:**
1. Get a fresh token (will have kid in header)
2. Verify it works on an authenticated endpoint
3. Note: We can't easily create a token without kid from outside, so we
   verify via unit test `TestVerify_AcceptsEmptyKid` that the code path
   handles empty kid correctly

**Expected:**
- Fresh tokens (with kid) work normally
- Unit test `TestVerify_AcceptsEmptyKid` PASSES
- The verify logic explicitly allows empty kid (code inspection)

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-S7-empty-kid-accepted.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-S7 — Tokens With Empty kid Are Still Accepted

Who: The operator.

What: The operator verifies backward compatibility. Tokens issued before the
B4 hardening don't have a kid field in their JWT header. The broker must
still accept them. If it rejected them, every agent and pipeline would break
immediately after the upgrade.

Why: Backward compatibility prevents outages during upgrades. If old tokens
stop working, every agent must re-authenticate simultaneously.

How to run: Source env. Get a fresh token (will have kid). Verify it works.
Then run the unit test TestVerify_AcceptsEmptyKid to prove the empty-kid
code path is correct.

Expected: Fresh tokens work. Unit test passes. Empty kid is allowed.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')

echo "JWT header:" >> "$F"
echo "$TOKEN" | cut -d. -f1 | base64 -d 2>/dev/null | jq . >> "$F" 2>&1
echo "" >> "$F"

echo "--- Using token on /v1/admin/apps ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TOKEN" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"

echo "--- Unit test: TestVerify_AcceptsEmptyKid ---" >> "$F"
go test ./internal/token/ -run TestVerify_AcceptsEmptyKid -v >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## Negative Stories

---

## L2a-N1: Broker Rejects Token With Tampered Algorithm [ACCEPTANCE]

**Tracker:** b4-n1
**Persona:** Security Reviewer
**Tool:** curl + base64

The security reviewer is testing for algorithm confusion attacks. This is a
real attack vector (CVE-2015-9235 and others): the attacker takes a valid
EdDSA-signed JWT, changes the `alg` header to `HS256`, then signs it with
the public key as the HMAC secret. If the broker doesn't validate the `alg`
field, it might verify the token using HMAC instead of EdDSA, and the
attacker gets in with a forged token. The reviewer tampers with the header
and confirms the broker catches it.

**Route:** GET /v1/admin/apps (with tampered token)
**Mode:** VPS + Container
**Steps:**
1. Get a valid token
2. Split the JWT on dots: `header.payload.signature`
3. Base64url decode the header
4. Change `"alg":"EdDSA"` to `"alg":"HS256"`
5. Base64url re-encode the header
6. Reconstruct the JWT: `tampered_header.original_payload.original_signature`
7. Present the tampered JWT to an authenticated endpoint

**Expected:**
- Broker returns 401
- The tampered token is NOT accepted
- Error response does not reveal which check failed (no information leakage)

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-N1-tampered-alg.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-N1 — Broker Rejects Token With Tampered Algorithm

Who: The security reviewer.

What: The reviewer tests for algorithm confusion attacks (CVE-2015-9235).
They take a valid EdDSA-signed JWT, change the alg header to HS256, and
present it to the broker. If the broker doesn't validate alg, it might
verify using HMAC instead of EdDSA, letting forged tokens through.

Why: Algorithm confusion is a real-world JWT attack. If the broker accepts
tokens with a tampered algorithm, an attacker can forge valid-looking tokens
using the public key as an HMAC secret.

How to run: Source env. Get a valid token. Decode the JWT header, change
alg from EdDSA to HS256, re-encode, and present to the broker.

Expected: Broker returns 401. The tampered token is rejected.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')

HEADER=$(echo "$TOKEN" | cut -d. -f1)
PAYLOAD=$(echo "$TOKEN" | cut -d. -f2)
SIGNATURE=$(echo "$TOKEN" | cut -d. -f3)

TAMPERED_HEADER=$(echo "$HEADER" | base64 -d 2>/dev/null \
  | sed 's/"alg":"EdDSA"/"alg":"HS256"/' \
  | base64 | tr '+/' '-_' | tr -d '=')

TAMPERED_TOKEN="${TAMPERED_HEADER}.${PAYLOAD}.${SIGNATURE}"

echo "Original alg:" >> "$F"
echo "$HEADER" | base64 -d 2>/dev/null | jq '.alg' >> "$F" 2>&1
echo "Tampered alg:" >> "$F"
echo "$TAMPERED_HEADER" | base64 -d 2>/dev/null | jq '.alg' >> "$F" 2>&1
echo "" >> "$F"

echo "--- Presenting tampered token (should be 401) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TAMPERED_TOKEN" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-N2: Broker Rejects Token With Wrong Key ID [ACCEPTANCE]

**Tracker:** b4-n2
**Persona:** Security Reviewer
**Tool:** curl + base64

The security reviewer tampers with the `kid` (key ID) in the JWT header.
In a multi-broker deployment, each broker has its own signing key with a
unique ID. If an attacker steals a token from broker A and presents it to
broker B, the kid won't match. Before this hardening, the broker didn't
check kid at all — any token with a valid signature was accepted regardless
of which key supposedly signed it. Now the broker rejects mismatched kids.

**Route:** GET /v1/admin/apps (with tampered kid)
**Mode:** VPS + Container
**Steps:**
1. Get a valid token
2. Decode the JWT header and note the real kid
3. Change the kid to `"wrong-key-id-12345"`
4. Re-encode and reconstruct the JWT
5. Present to an authenticated endpoint

**Expected:**
- Broker returns 401
- The wrong-kid token is NOT accepted

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-N2-wrong-kid.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-N2 — Broker Rejects Token With Wrong Key ID

Who: The security reviewer.

What: The reviewer tampers with the kid (key ID) in the JWT header. In a
multi-broker deployment, each broker has its own signing key with a unique
ID. If an attacker steals a token from broker A and presents it to broker B,
the kid won't match. The broker should reject it.

Why: Before this hardening, the broker didn't check kid at all. Any token
with a valid signature was accepted regardless of which key supposedly signed
it. This is a cross-broker replay attack vector.

How to run: Source env. Get a valid token. Decode the header, change kid to
a fake value, re-encode, and present to the broker.

Expected: Broker returns 401. The wrong-kid token is rejected.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')

HEADER=$(echo "$TOKEN" | cut -d. -f1)
PAYLOAD=$(echo "$TOKEN" | cut -d. -f2)
SIGNATURE=$(echo "$TOKEN" | cut -d. -f3)

echo "Original kid:" >> "$F"
echo "$HEADER" | base64 -d 2>/dev/null | jq '.kid' >> "$F" 2>&1
echo "" >> "$F"

TAMPERED_HEADER=$(echo "$HEADER" | base64 -d 2>/dev/null \
  | jq -c '.kid = "wrong-key-id-12345"' \
  | base64 | tr '+/' '-_' | tr -d '=')

TAMPERED_TOKEN="${TAMPERED_HEADER}.${PAYLOAD}.${SIGNATURE}"

echo "--- Presenting wrong-kid token (should be 401) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" \
  -H "Authorization: Bearer $TAMPERED_TOKEN" "$BROKER_URL/v1/admin/apps" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-N3: Tokens Without Expiry Are Rejected [ACCEPTANCE]

**Tracker:** b4-n3
**Persona:** Security Reviewer
**Tool:** unit test

The security reviewer is verifying that the broker rejects tokens with no
expiry (`exp=0` or missing). Before this hardening, a token with `exp=0`
was treated as "never expires" — a permanent credential. If an attacker
obtained such a token, it could never be rotated out by simply waiting for
expiration. This is a critical gap: permanent tokens violate the principle
of least privilege and make breach recovery much harder.

This cannot be easily tested from outside the broker because the broker
always sets a positive `exp` when issuing tokens. We verify via unit test.

**Route:** N/A (unit test)
**Mode:** N/A
**Steps:**
1. Run `go test ./internal/token/ -run TestVerify_RejectsZeroExpiry -v`
2. Verify the test passes
3. Confirm the test asserts `ErrNoExpiry` is returned

**Expected:**
- Unit test PASSES
- Test confirms tokens with `exp <= 0` return `ErrNoExpiry`

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-N3-no-expiry-rejected.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-N3 — Tokens Without Expiry Are Rejected

Who: The security reviewer.

What: The reviewer verifies that tokens with no expiry (exp=0 or missing) are
rejected. Before this hardening, such tokens were treated as "never expires"
— a permanent credential that can never be rotated out by waiting.

Why: Permanent tokens violate the principle of least privilege. If an attacker
obtains one, it never expires. Breach recovery becomes much harder because you
can't wait for credentials to rotate.

How to run: Run the unit test TestVerify_RejectsZeroExpiry.

Expected: Unit test passes. Tokens with exp <= 0 return ErrNoExpiry.

## Test Output

BANNER
go test ./internal/token/ -run TestVerify_RejectsZeroExpiry -v >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-N4: Wrong Admin Secret Is Still Rejected [ACCEPTANCE]

**Tracker:** b4-n4
**Persona:** Security Reviewer
**Tool:** curl

Regression check. The security reviewer sends an admin auth request with
a wrong secret. After all the token hardening changes, the basic
authentication flow must still work correctly — wrong secrets rejected,
correct secrets accepted. If this fails, the hardening broke something in
the auth path.

**Route:** POST /v1/admin/auth
**Mode:** VPS + Container
**Steps:**
1. Send admin auth with a wrong secret
2. Verify 401 returned
3. Send admin auth with the correct secret
4. Verify 200 returned

**Expected:**
- Wrong secret: 401
- Correct secret: 200 with JWT

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-N4-wrong-secret-rejected.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-N4 — Wrong Admin Secret Is Still Rejected

Who: The security reviewer.

What: Regression check after the token hardening. The reviewer sends a wrong
admin secret and then a correct one. The basic auth flow must still work.

Why: If the hardening broke the auth path, wrong secrets might be accepted
or correct secrets rejected. Either would be a critical regression.

How to run: Source env. Send admin auth with a wrong secret (expect 401).
Send admin auth with the correct secret (expect 200).

Expected: Wrong secret returns 401. Correct secret returns 200 with JWT.

## Test Output

BANNER
source ./tests/sec-l2a/env.sh

echo "--- Wrong secret (should be 401) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d '{"secret":"this-is-the-wrong-secret-entirely"}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"

echo "--- Correct secret (should be 200) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## L2a-N5: Renewal Fails When Predecessor Revocation Fails [ACCEPTANCE]

**Tracker:** b4-n5
**Persona:** Security Reviewer
**Tool:** unit test

The security reviewer is verifying a critical transactional safety check.
When a token is renewed, the old token must be revoked first. If the
revocation fails (database error, storage issue), the renewal must also
fail — otherwise both old and new tokens remain valid, and the attacker
who stole the old token still has a working credential even after the
legitimate user renewed. Before this hardening, revocation errors were
silently ignored and renewal always succeeded.

This cannot be tested from outside because you can't force a revocation
failure through the HTTP API. We verify via unit test.

**Route:** N/A (unit test)
**Mode:** N/A
**Steps:**
1. Run `go test ./internal/token/ -run TestRenew_RevokeFailureBlocksRenewal -v`
2. Verify the test passes
3. Confirm the test asserts the error contains "revoke predecessor"

**Expected:**
- Unit test PASSES
- Renewal fails when revocation fails
- Error message contains "revoke predecessor"

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-N5-renewal-fails-on-revoke-error.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-N5 — Renewal Fails When Predecessor Revocation Fails

Who: The security reviewer.

What: The reviewer verifies that renewal fails if the old token's revocation
fails. Before this hardening, revocation errors were silently ignored and
renewal always succeeded — leaving both old and new tokens valid.

Why: If revocation is silently skipped, an attacker who stole the old token
keeps a valid credential even after the legitimate user renewed. The blast
radius is unlimited.

How to run: Run the unit test TestRenew_RevokeFailureBlocksRenewal.

Expected: Unit test passes. Renewal fails when revocation fails. Error
contains "revoke predecessor".

## Test Output

BANNER
go test ./internal/token/ -run TestRenew_RevokeFailureBlocksRenewal -v >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the output, append the verdict:
```bash
echo "PASS — or FAIL — <reason>" >> "$F"
```

---

## Security Review

---

## L2a-SEC1: Code Review — Verify() Check Order [ACCEPTANCE]

**Tracker:** b4-sec1
**Persona:** Security Reviewer
**Tool:** code inspection

The security reviewer reads `internal/token/tkn_svc.go` and traces the
`Verify()` function line by line. The verification checks must happen in
a specific order — each check acts as a gate. If any check fails, the
function must return immediately without proceeding to more expensive checks.
The order matters for both security and performance:

1. **Format** — Split JWT, basic structure check (cheap)
2. **Algorithm** — Validate `alg == EdDSA` (cheap, prevents algorithm confusion)
3. **Key ID** — Validate `kid` matches or is empty (cheap, prevents cross-broker replay)
4. **Signature** — Verify Ed25519 signature (expensive crypto operation)
5. **Claims** — Validate `exp > 0` and `exp > now` (cheap, post-signature)
6. **Revocation** — Check if JTI is revoked (database lookup, most expensive)

The reviewer confirms no early returns skip a security check, and that
the expensive checks (signature, revocation) happen only after the cheap
checks pass.

**Route:** N/A (code inspection)
**Mode:** N/A
**Steps:**
1. Read `internal/token/tkn_svc.go` `Verify()` function
2. Trace the check order from top to bottom
3. Verify no check is skipped on any code path
4. Verify algorithm confusion is caught before signature verification

**Expected:**
- Checks follow the order: format → alg → kid → signature → claims → revocation
- Algorithm and kid checks happen BEFORE signature verification
- Revocation check happens AFTER claims validation
- Every failure returns an appropriate error (ErrInvalidToken, ErrTokenExpired, ErrTokenRevoked, ErrNoExpiry)
- No code path allows a token to pass Verify() without going through all 6 checks

**Test command:**
```bash
F=tests/sec-l2a/evidence/story-SEC1-verify-check-order.md
mkdir -p tests/sec-l2a/evidence
cat > "$F" << 'BANNER'
# L2a-SEC1 — Code Review: Verify() Check Order

Who: The security reviewer.

What: The reviewer reads internal/token/tkn_svc.go and traces the Verify()
function line by line. The checks must happen in a specific order: format,
algorithm, kid, signature, claims, revocation. Each check is a gate — if it
fails, the function returns immediately.

Why: The order matters for security and performance. Cheap checks (alg, kid)
must happen before expensive checks (signature, revocation). Algorithm
confusion must be caught before signature verification. If checks are out of
order, an attacker could exploit gaps.

How to run: Read the Verify() function. Trace the check order. Verify no
code path skips a security check.

Expected: Checks in order: format → alg → kid → signature → claims →
revocation. No path skips a check. Appropriate errors for each failure.

## Code Review

BANNER
echo "Verify() function source:" >> "$F"
echo '```go' >> "$F"
sed -n '/^func.*Verify(/,/^}/p' internal/token/tkn_svc.go >> "$F" 2>&1
echo '```' >> "$F"
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

After reading the code, append the verdict with the check order analysis:
```bash
echo "PASS — or FAIL — <detailed analysis of check order>" >> "$F"
```
