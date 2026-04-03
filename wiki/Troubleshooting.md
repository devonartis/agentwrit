# Troubleshooting

Fix common AgentAuth errors. Organized by HTTP status code and role.

> **Start here:** Find your HTTP status code in the [Diagnostic Flowchart](#diagnostic-flowchart), then jump to the specific error.

---

## Diagnostic Flowchart

```
What HTTP status did you get?

400 → Missing or malformed request fields
      → Fix: Check JSON body for required fields

401 → Which endpoint?
      /v1/register → See Developer Errors below
      /v1/token/renew → Token expired → Renew at 80% TTL
      /v1/admin/auth → Invalid credentials → Check AA_ADMIN_SECRET

403 → What error code?
      scope_violation → Request narrower scope
      insufficient_scope → Token doesn't have required scope
      token revoked → Re-acquire from sidecar

429 → Rate limited → Wait Retry-After seconds

502 → Sidecar can't reach broker → Check broker health

503 → Circuit breaker open → Check broker, wait for recovery
```

---

## Developer Errors

### 401 at /v1/register: "nonce signature verification failed"

**Cause:** You signed the nonce as text instead of hex-decoding it to bytes.

**Fix:**

```python
# WRONG — signs the ASCII hex string (64 bytes of text)
signature = private_key.sign(nonce_hex.encode("utf-8"))

# RIGHT — signs the decoded 32 bytes
signature = private_key.sign(bytes.fromhex(nonce_hex))
```

> **If you use the sidecar**, you'll never hit this error. The sidecar handles nonce signing automatically.

---

### 401 at /v1/register: "invalid Ed25519 public key: wrong key size"

**Cause:** You encoded a DER/PEM-format key instead of the raw 32-byte key.

**Fix:**

```python
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

# WRONG — DER encoding adds headers, producing 44 bytes
pub_der = key.public_key().public_bytes(Encoding.DER, PublicFormat.SubjectPublicKeyInfo)

# RIGHT — raw encoding produces exactly 32 bytes
pub_raw = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
pub_b64 = base64.b64encode(pub_raw).decode()
```

---

### 401 at /v1/register: "launch token not found"

**Cause:** Launch token expired (default 30s TTL), was already used (single-use), or was copied incorrectly.

**Fix:** Get a new launch token from your operator. If using the sidecar, restart it: `docker compose restart sidecar`.

---

### 401 at /v1/register: "nonce not found or expired"

**Cause:** More than 30 seconds passed between `GET /v1/challenge` and `POST /v1/register`.

**Fix:** Get a fresh nonce and register immediately — no delay:

```python
# Get nonce and register in one sequence
challenge = requests.get(f"{BROKER}/v1/challenge")
nonce_hex = challenge.json()["nonce"]
signature = private_key.sign(bytes.fromhex(nonce_hex))
# ... register immediately ...
```

---

### 403 at /v1/register: "requested scope exceeds allowed scope"

**Cause:** Your scope isn't a subset of the launch token's allowed scope.

**Fix:**

```python
# If allowed_scope is ["read:data:*"], these WORK:
scope = ["read:data:*"]          # exact match
scope = ["read:data:customers"]  # narrower

# These FAIL:
scope = ["write:data:*"]         # different action
scope = ["read:data:*", "admin:*"]  # includes unauthorized scope
```

---

### 403 at /v1/token/renew: "token has been revoked"

**Cause:** An admin revoked your token (at token, agent, task, or chain level).

**Fix:** Re-acquire from the sidecar:

```python
data = requests.post(f"{SIDECAR}/v1/token", json={
    "agent_name": "my-agent",
    "scope": ["read:data:*"],
}).json()
new_token = data["access_token"]
```

---

### 401 at /v1/token/renew: "token expired"

**Cause:** Token's TTL elapsed. Default is 300 seconds (5 minutes).

**Fix:** Renew earlier — at 80% of the TTL:

```python
ttl = 300
time.sleep(ttl * 0.8)  # Renew at 240 seconds, not 300
# ... renew here ...
```

---

### 403 at sidecar /v1/token: "requested scope exceeds sidecar ceiling"

**Cause:** Scope exceeds the sidecar's `AA_SIDECAR_SCOPE_CEILING`.

**Fix:** Check what's available:

```bash
curl http://localhost:8081/v1/health
# Shows: {"status":"ok","scope_ceiling":["read:data:*","write:data:*"]}
```

Then request a scope within that ceiling.

---

## Operator Errors

### Broker exits: "AA_ADMIN_SECRET must be set"

**Fix:**

```bash
export AA_ADMIN_SECRET="$(openssl rand -hex 32)"
# Then restart the broker
```

---

### Sidecar exits: "AA_ADMIN_SECRET must be set" or "AA_SIDECAR_SCOPE_CEILING must be set"

**Fix:**

```bash
export AA_ADMIN_SECRET="same-as-broker"
export AA_SIDECAR_SCOPE_CEILING="read:data:*,write:data:*"
# Then restart the sidecar
```

---

### 401 at /v1/admin/auth: "invalid credentials"

**Cause:** `client_secret` doesn't match `AA_ADMIN_SECRET`.

**Fix:**

```bash
# Verify it's set
env | grep AA_ADMIN_SECRET | wc -l

# Use it correctly
curl -X POST http://localhost:8080/v1/admin/auth \
  -H "Content-Type: application/json" \
  -d "{\"client_id\":\"admin\",\"client_secret\":\"$AA_ADMIN_SECRET\"}"
```

---

### 429 at /v1/admin/auth: "rate limit exceeded"

**Cause:** More than 5 requests/second to admin auth.

**Fix:** Cache admin tokens (they last 300 seconds). Don't re-authenticate for every request.

```python
# Cache the admin token
admin_token = None
admin_token_expiry = 0

def get_admin_token():
    global admin_token, admin_token_expiry
    if admin_token and time.time() < admin_token_expiry:
        return admin_token
    resp = requests.post(f"{BROKER}/v1/admin/auth", json={
        "client_id": "admin",
        "client_secret": os.environ["AA_ADMIN_SECRET"]
    })
    data = resp.json()
    admin_token = data["access_token"]
    admin_token_expiry = time.time() + data["expires_in"] * 0.8
    return admin_token
```

---

### Sidecar can't reach broker (502)

**Cause:** Network connectivity between sidecar and broker.

**Fix:**

```bash
# Check broker is running
curl http://localhost:8080/v1/health

# Check sidecar can reach broker
docker compose logs sidecar | tail -20

# If using Docker, ensure both are on the same network
docker network ls
```

---

## Error Response Format

All broker errors use RFC 7807:

```json
{
  "type": "urn:agentauth:error:scope_violation",
  "title": "Forbidden",
  "status": 403,
  "detail": "requested scope exceeds allowed scope",
  "instance": "/v1/register",
  "error_code": "scope_violation",
  "request_id": "a1b2c3d4e5f67890",
  "hint": "requested scope must be a subset of allowed scope"
}
```

**Always include `request_id` when reporting issues to your operator.**

Sidecar errors are simpler:
```json
{
  "error": "Forbidden",
  "detail": "requested scope exceeds sidecar ceiling"
}
```

---

## Still Stuck?

1. Check broker logs: `docker compose logs broker`
2. Check sidecar logs: `docker compose logs sidecar`
3. Validate your token: `POST /v1/token/validate` (no auth required)
4. Check sidecar ceiling: `GET /v1/health` on sidecar port
5. File an issue: https://github.com/devonartis/agentauth-core/issues

---

## Next Steps

- [[API Reference]] — Endpoint details
- [[Common Tasks]] — Working examples
- [[Developer Guide]] — Integration guide
