# App Launch Tokens — Route Separation Acceptance Tests

**Branch:** `fix/app-launch-tokens-endpoint`
**Cherry-pick:** `393d376` from agentauth-internal

---

## Infrastructure Prerequisites

| Prerequisite | Purpose | Smoke Test Story | Status |
|-------------|---------|-----------------|--------|
| Go 1.24+ compiled broker binary | VPS mode testing | N/A (built before tests) | NOT VERIFIED |

---

## Context

The broker has two endpoints for creating launch tokens. They serve different
operational purposes and must be separate routes with separate scopes:

- `POST /v1/admin/launch-tokens` — operator/platform endpoint for broker
  bootstrap, admin-only deployments, and break-glass issuance. Requires
  `admin:launch-tokens:*` scope.

- `POST /v1/app/launch-tokens` — application/runtime endpoint for normal
  agent lifecycle management. Requires `app:launch-tokens:*` scope. Enforces
  the app's registered scope ceiling.

Previously these were collapsed into one endpoint. This fix separates them.

---

## ALT-S1: App Creates Launch Token on App Route [ACCEPTANCE]

**Tracker:** alt-s1
**Persona:** Developer (via app credentials)
**Tool:** curl
**Mode:** VPS

The developer's application authenticates with the broker using its client
credentials, then creates a launch token for an agent using the app route.
This is the normal production path — apps manage their own agents.

**Route:** POST /v1/app/auth -> POST /v1/app/launch-tokens
**Steps:**
1. Authenticate as admin, register an app with scope ceiling `read:data:*`
2. Authenticate as the app using client_id/client_secret
3. Create a launch token via POST /v1/app/launch-tokens
4. Verify the launch token is returned (201)

**Expected:**
- App auth returns 200 with app token
- POST /v1/app/launch-tokens returns 201 with launch token
- The launch token's allowed_scope is within the app's ceiling

**Test command:**
```bash
F=tests/app-launch-tokens/evidence/story-S1-app-creates-launch-token.md
mkdir -p tests/app-launch-tokens/evidence
cat > "$F" << 'BANNER'
# ALT-S1 — App Creates Launch Token on App Route

Who: The developer's application.

What: The app authenticates with the broker using its client credentials,
then creates a launch token for one of its agents using the dedicated app
route (POST /v1/app/launch-tokens). This is the normal production path.
Apps manage their own agents within their scope ceiling.

Why: If the app can't create launch tokens through its own route, the entire
app-driven agent lifecycle is broken. Every app would need an operator to
manually create launch tokens, which doesn't scale.

How to run: Authenticate as admin. Register an app with a scope ceiling.
Authenticate as the app. Create a launch token via the app route.

Expected: App auth returns 200. Launch token creation returns 201.

## Test Output

BANNER
source ./tests/app-launch-tokens/env.sh

# Step 1: Get admin token
ADMIN_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Admin token: ${ADMIN_TOKEN:0:20}..." >> "$F"
echo "" >> "$F"

# Step 2: Register an app
echo "--- Register app ---" >> "$F"
APP_RESP=$(curl -s -X POST "$BROKER_URL/v1/admin/apps" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-pipeline","scopes":["read:data:*"],"token_ttl":1800}')
echo "$APP_RESP" | jq . >> "$F" 2>&1
CLIENT_ID=$(echo "$APP_RESP" | jq -r '.client_id')
CLIENT_SECRET=$(echo "$APP_RESP" | jq -r '.client_secret')
echo "" >> "$F"

# Step 3: Authenticate as app
echo "--- App auth ---" >> "$F"
APP_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/app/auth" \
  -H "Content-Type: application/json" \
  -d "{\"client_id\":\"$CLIENT_ID\",\"client_secret\":\"$CLIENT_SECRET\"}" | jq -r '.access_token')
echo "App token: ${APP_TOKEN:0:20}..." >> "$F"
echo "" >> "$F"

# Step 4: Create launch token via app route
echo "--- POST /v1/app/launch-tokens ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/app/launch-tokens" \
  -H "Authorization: Bearer $APP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"data-reader","allowed_scope":["read:data:*"],"max_ttl":300,"ttl":30}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

---

## ALT-S2: App Cannot Call Admin Launch Token Route [ACCEPTANCE]

**Tracker:** alt-s2
**Persona:** Security Reviewer
**Tool:** curl
**Mode:** VPS

The security reviewer verifies that an app token cannot call the admin
launch token endpoint. Before this fix, both admin and app tokens could
hit the same endpoint. Now they're separated — an app token on the admin
route should get 403.

**Route:** POST /v1/admin/launch-tokens (with app token)
**Steps:**
1. Use the app token from S1
2. Try to create a launch token via POST /v1/admin/launch-tokens
3. Verify 403 is returned

**Expected:**
- POST /v1/admin/launch-tokens with app token returns 403

**Test command:**
```bash
F=tests/app-launch-tokens/evidence/story-S2-app-blocked-from-admin-route.md
mkdir -p tests/app-launch-tokens/evidence
cat > "$F" << 'BANNER'
# ALT-S2 — App Cannot Call Admin Launch Token Route

Who: The security reviewer.

What: The reviewer verifies that an app token is rejected when it tries to
call the admin launch token endpoint (POST /v1/admin/launch-tokens). Before
this fix, both admin and app tokens hit the same endpoint. Now they're
separated — the admin route only accepts admin:launch-tokens:* scope.

Why: If an app can call the admin route, the separation is meaningless. The
admin route has no scope ceiling enforcement — an app could bypass its own
ceiling by calling the admin endpoint instead of the app endpoint. This is a
privilege escalation.

How to run: Use the app token from S1. Try to create a launch token via the
admin route instead of the app route.

Expected: HTTP 403 — insufficient scope.

## Test Output

BANNER
source ./tests/app-launch-tokens/env.sh

# Reuse app token from S1 (or re-authenticate)
ADMIN_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')

# Get app credentials
APP_RESP=$(curl -s -X POST "$BROKER_URL/v1/admin/apps" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-blocked","scopes":["read:data:*"],"token_ttl":1800}')
CLIENT_ID=$(echo "$APP_RESP" | jq -r '.client_id')
CLIENT_SECRET=$(echo "$APP_RESP" | jq -r '.client_secret')

APP_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/app/auth" \
  -H "Content-Type: application/json" \
  -d "{\"client_id\":\"$CLIENT_ID\",\"client_secret\":\"$CLIENT_SECRET\"}" | jq -r '.access_token')
echo "App token: ${APP_TOKEN:0:20}..." >> "$F"
echo "" >> "$F"

echo "--- POST /v1/admin/launch-tokens with app token (should be 403) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $APP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"sneaky-agent","allowed_scope":["read:data:*"],"max_ttl":300,"ttl":30}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

---

## ALT-S3: Admin Creates Launch Token on Admin Route [ACCEPTANCE]

**Tracker:** alt-s3
**Persona:** Operator
**Tool:** curl
**Mode:** VPS

The operator creates a launch token using the admin route. This is the
platform/operator path — used for broker bootstrap, dev/testing, and
break-glass issuance.

**Route:** POST /v1/admin/launch-tokens (with admin token)
**Steps:**
1. Authenticate as admin
2. Create a launch token via POST /v1/admin/launch-tokens
3. Verify 201 is returned

**Expected:**
- POST /v1/admin/launch-tokens returns 201 with launch token

**Test command:**
```bash
F=tests/app-launch-tokens/evidence/story-S3-admin-creates-launch-token.md
mkdir -p tests/app-launch-tokens/evidence
cat > "$F" << 'BANNER'
# ALT-S3 — Admin Creates Launch Token on Admin Route

Who: The operator.

What: The operator authenticates with the admin secret and creates a launch
token using the admin route (POST /v1/admin/launch-tokens). This is the
platform management path — used for broker bootstrap, initial app setup,
dev/testing environments, and emergency break-glass scenarios.

Why: Without the admin path, there's no way to bootstrap the system. Before
any apps are registered, someone needs to create the first launch token so
the first agent can register. The operator is that someone.

How to run: Authenticate as admin. Create a launch token via the admin route.

Expected: HTTP 201 with a launch token.

## Test Output

BANNER
source ./tests/app-launch-tokens/env.sh

ADMIN_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Admin token: ${ADMIN_TOKEN:0:20}..." >> "$F"
echo "" >> "$F"

echo "--- POST /v1/admin/launch-tokens with admin token ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/admin/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"bootstrap-agent","allowed_scope":["read:data:*","write:logs:*"],"max_ttl":600,"ttl":60}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```

---

## ALT-S4: Admin Cannot Call App Launch Token Route [ACCEPTANCE]

**Tracker:** alt-s4
**Persona:** Security Reviewer
**Tool:** curl
**Mode:** VPS

The security reviewer verifies that an admin token is rejected on the app
route. The routes are fully separated — each accepts only its own scope.

**Route:** POST /v1/app/launch-tokens (with admin token)
**Steps:**
1. Use the admin token
2. Try to create a launch token via POST /v1/app/launch-tokens
3. Verify 403 is returned

**Expected:**
- POST /v1/app/launch-tokens with admin token returns 403

**Test command:**
```bash
F=tests/app-launch-tokens/evidence/story-S4-admin-blocked-from-app-route.md
mkdir -p tests/app-launch-tokens/evidence
cat > "$F" << 'BANNER'
# ALT-S4 — Admin Cannot Call App Launch Token Route

Who: The security reviewer.

What: The reviewer verifies that an admin token is rejected when it tries
to call the app launch token endpoint (POST /v1/app/launch-tokens). The
routes are fully separated — each only accepts its own scope type.

Why: If admin tokens could call the app route, it would bypass the app scope
ceiling enforcement that the app route provides. More importantly, it would
mean the routes aren't truly separated — they're just aliases. True
separation means each route enforces its own authorization boundary.

How to run: Authenticate as admin. Try to create a launch token via the app
route instead of the admin route.

Expected: HTTP 403 — insufficient scope.

## Test Output

BANNER
source ./tests/app-launch-tokens/env.sh

ADMIN_TOKEN=$(curl -s -X POST "$BROKER_URL/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\":\"$AACTL_ADMIN_SECRET\"}" | jq -r '.access_token')
echo "Admin token: ${ADMIN_TOKEN:0:20}..." >> "$F"
echo "" >> "$F"

echo "--- POST /v1/app/launch-tokens with admin token (should be 403) ---" >> "$F"
curl -s -w "\nHTTP %{http_code}" -X POST "$BROKER_URL/v1/app/launch-tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"agent_name":"sneaky-admin","allowed_scope":["read:data:*"],"max_ttl":300,"ttl":30}' >> "$F" 2>&1
echo "" >> "$F"; echo "" >> "$F"
echo "## Verdict" >> "$F"; echo "" >> "$F"
cat "$F"
```
