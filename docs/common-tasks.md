# Common Tasks

Enterprise-grade step-by-step instructions for AgentAuth workflows, organized by role.

**Document metadata:**
- **Audience:** Developers (building AI agents), Platform Operators (managing AgentAuth deployments)
- **Version:** 2.0 (Enterprise)
- **Prerequisites:** Broker running. See [Getting Started: Developer](getting-started-developer.md) or [Getting Started: Operator](getting-started-operator.md).
- **Next steps:** For advanced topics, see [Concepts](concepts.md), [API Reference](api.md), [Architecture](architecture.md), or [Troubleshooting](troubleshooting.md).

---

## Quick Reference

| Task | Endpoint | Role | HTTP Method |
|------|----------|------|-------------|
| Register Agent | `POST /v1/register` | Developer | Broker |
| Validate a Token | `POST /v1/token/validate` | Developer | Broker |
| Renew a Token | `POST /v1/token/renew` | Developer | Broker |
| Release a Token | `POST /v1/token/release` | Developer | Broker |
| Delegate a Token | `POST /v1/delegate` | Developer | Broker |
| Check Broker Health | `GET /v1/health` | Developer | Broker |
| Authenticate as Admin | `POST /v1/admin/auth` | Operator | Broker |
| Register App | `POST /v1/admin/apps` | Operator | Broker |
| List Apps | `GET /v1/admin/apps` | Operator | Broker |
| Create Launch Token | `POST /v1/admin/launch-tokens` | Operator | Broker |
| App Authentication | `POST /v1/app/auth` | Operator | Broker |
| Revoke Tokens | `POST /v1/revoke` | Operator | Broker |
| Query Audit Trail | `GET /v1/audit/events` | Operator | Broker |
| Get Broker Metrics | `GET /v1/metrics` | Operator | Broker |
| Register App (aactl) | `aactl app register --name NAME --scopes SCOPES` | Operator | CLI |
| List Apps (aactl) | `aactl app list` | Operator | CLI |
| Revoke Tokens (aactl) | `aactl revoke --level <level> --target <target>` | Operator | CLI |
| Query Audit Trail (aactl) | `aactl audit events [--filters]` | Operator | CLI |

---

## Developer Tasks

> **Persona:** Developer building an AI agent in Python or TypeScript. You interact with the broker directly. No admin credentials required.
>
> **Prerequisite:** [Getting Started: Developer](getting-started-developer.md), [API Reference](api.md)

### Register an Agent

Register an agent with the broker using a launch token. The launch token is provided by your operator and grants permission to register a single agent.

**What's happening:** The registration flow is challenge-response based. You request a challenge (nonce) from the broker, sign it with the launch token, and return the signature along with your agent's public key. The broker validates the signature and issues your agent an access token.

**Python example:**

```python
import requests
import json
import hmac
import hashlib

BROKER = "http://localhost:8080"

def register_agent(broker, launch_token, agent_name, task_id=None):
    """Register an agent with the broker."""
    # Step 1: Get a challenge nonce
    challenge_resp = requests.get(f"{broker}/v1/challenge")
    challenge_resp.raise_for_status()
    challenge_data = challenge_resp.json()
    nonce = challenge_data["nonce"]

    # Step 2: Sign the nonce with the launch token
    signature = hmac.new(
        launch_token.encode(),
        nonce.encode(),
        hashlib.sha256
    ).hexdigest()

    # Step 3: Register with the broker
    reg_payload = {
        "agent_name": agent_name,
        "nonce": nonce,
        "signature": signature,
        "launch_token": launch_token,
    }
    if task_id:
        reg_payload["task_id"] = task_id

    reg_resp = requests.post(
        f"{broker}/v1/register",
        json=reg_payload
    )
    reg_resp.raise_for_status()
    return reg_resp.json()

# Register agent
try:
    launch_token = "launch_token_from_operator"
    data = register_agent(
        BROKER,
        launch_token=launch_token,
        agent_name="data-processor",
        task_id="task-analyze-q4"
    )

    token = data["access_token"]
    agent_id = data["agent_id"]
    expires_in = data["expires_in"]

    print(f"Agent:       {agent_id}")
    print(f"Expires in:  {expires_in}s")
    print(f"Token:       {token}")
except requests.exceptions.HTTPError as e:
    print(f"Registration failed: {e.response.status_code} - {e.response.text}")
```

**Expected response (201 Created):**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvb3JjaC8uLi4iLCJleHAiOjE3NDU0MDU2MzAsImlhdCI6MTc0NTQwNTMzMCwic2NvcGUiOlsicmVhZDpkYXRhOioiXX0.SIGNATURE",
  "agent_id": "spiffe://agentauth.local/agent/orch-001/task-analyze-q4/proc-abc123",
  "expires_in": 300
}
```

**If this fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Invalid request (malformed JSON, missing fields, invalid signature) | Check payload syntax; ensure signature is correct |
| 401 | Launch token invalid or expired | Request a fresh launch token from your operator |
| 409 | Agent already registered | Use a different agent name or task ID |

---

### Release a Token

Signal task completion by releasing your token. This is optional but creates an explicit audit trail entry.

**What's happening:** Token release records the exact moment your task completed. This allows the broker to track task duration precisely and enables resource cleanup. Released tokens are immediately marked as completed in the audit trail.

**Python example:**

```python
import requests

BROKER = "http://localhost:8080"

def release_token(broker, token):
    """Release a token when task is complete."""
    resp = requests.post(
        f"{broker}/v1/token/release",
        headers={"Authorization": f"Bearer {token}"}
    )

    if resp.status_code == 204:
        return True
    elif resp.status_code == 401:
        raise RuntimeError("Token invalid or expired")
    else:
        raise RuntimeError(f"Release failed: {resp.status_code}")

# When task completes
try:
    release_token(BROKER, your_token)
    print("Task completed and token released")
except RuntimeError as e:
    print(f"Release failed: {e}")
```

**Expected response:** 204 No Content

**If this fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 204 | Success | Token has been released |
| 401 | Token invalid or expired | Token cannot be released (already expired or revoked) |

---

### Validate a Token

Check whether a token is valid and inspect its claims. This endpoint does not require authentication and can be called by any client.

**What's happening:** The broker validates the JWT signature and checks if the token has been revoked. No authentication needed, so you can validate tokens from upstream systems or for audit purposes. The endpoint always returns 200 with a `valid` boolean rather than using HTTP status for validity.

**Python example:**

```python
import requests
import json
from datetime import datetime

BROKER = "http://localhost:8080"

def validate_token(broker, token):
    """Validate a token and return its claims if valid."""
    resp = requests.post(
        f"{broker}/v1/token/validate",
        json={"token": token},
    )
    resp.raise_for_status()
    result = resp.json()

    if result["valid"]:
        return result["claims"]
    else:
        raise ValueError(f"Token invalid: {result.get('error', 'unknown')}")

# Validate a token
token = "<token_to_validate>"
try:
    claims = validate_token(BROKER, token)

    exp_timestamp = claims['exp']
    exp_datetime = datetime.utcfromtimestamp(exp_timestamp).isoformat()

    print(f"Subject:    {claims['sub']}")
    print(f"Scope:      {', '.join(claims['scope'])}")
    print(f"Task ID:    {claims.get('task_id', 'N/A')}")
    print(f"Orch ID:    {claims.get('orch_id', 'N/A')}")
    print(f"Expires:    {exp_datetime} (UTC)")
    print(f"JTI:        {claims['jti']}")
    print(f"Issued at:  {datetime.utcfromtimestamp(claims['iat']).isoformat()} (UTC)")
except ValueError as e:
    print(f"Validation failed: {e}")
except requests.exceptions.RequestException as e:
    print(f"Network error: {e}")
```

**Expected response (200 OK, valid token):**

```json
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/orch-001/task-analyze-q4/proc-abc123",
    "exp": 1745405630,
    "iat": 1745405330,
    "jti": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6",
    "scope": ["read:data:*"],
    "task_id": "task-analyze-q4",
    "orch_id": "orch-001"
  }
}
```

**Expected response (200 OK, invalid token):**

```json
{
  "valid": false,
  "error": "token_revoked",
  "detail": "Token JTI a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6 was revoked at 2026-02-15T10:00:00Z"
}
```

**TypeScript/Node.js example:**

```typescript
import fetch from 'node-fetch';

const BROKER = "http://localhost:8080";

interface ClaimSet {
  iss: string;
  sub: string;
  exp: number;
  iat: number;
  jti: string;
  scope: string[];
  task_id?: string;
  orch_id?: string;
}

interface ValidationResponse {
  valid: boolean;
  claims?: ClaimSet;
  error?: string;
  detail?: string;
}

async function validateToken(broker: string, token: string): Promise<ClaimSet> {
  const response = await fetch(`${broker}/v1/token/validate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  });

  if (!response.ok) {
    throw new Error(`Validation request failed: ${response.status}`);
  }

  const result = (await response.json()) as ValidationResponse;

  if (!result.valid) {
    throw new Error(`Token invalid: ${result.error} - ${result.detail}`);
  }

  return result.claims!;
}

// Usage
try {
  const claims = await validateToken(BROKER, "<token_to_validate>");

  const expDateTime = new Date(claims.exp * 1000).toISOString();
  const iatDateTime = new Date(claims.iat * 1000).toISOString();

  console.log(`Subject:    ${claims.sub}`);
  console.log(`Scope:      ${claims.scope.join(', ')}`);
  console.log(`Task ID:    ${claims.task_id || 'N/A'}`);
  console.log(`Orch ID:    ${claims.orch_id || 'N/A'}`);
  console.log(`Expires:    ${expDateTime}`);
  console.log(`JTI:        ${claims.jti}`);
  console.log(`Issued at:  ${iatDateTime}`);
} catch (error) {
  console.error(`Validation failed: ${error.message}`);
}
```

**Claims returned when valid:**

| Claim | Type | Description |
|-------|------|-------------|
| `iss` | string | Always `"agentauth"` |
| `sub` | string | Agent SPIFFE ID (subject) |
| `exp` | int | Expiration timestamp (Unix seconds) |
| `iat` | int | Issued-at timestamp (Unix seconds) |
| `jti` | string | Unique token identifier (32 hex chars) |
| `scope` | string[] | Granted permissions |
| `task_id` | string | Associated task ID |
| `orch_id` | string | Associated orchestrator ID |

**If this fails:**

| Issue | Meaning | Action |
|-------|---------|--------|
| Broker unreachable | Network connectivity issue | Verify broker is running; check firewall rules |
| Invalid token format | Token is malformed | Ensure you're passing the full JWT string |
| `token_expired` | Token timestamp has passed | Renew the token or re-register to get a fresh one |
| `token_revoked` | Token was revoked by operator | Re-register to get a fresh token; investigate with operator |
| `invalid_signature` | JWT signature verification failed | Token may be corrupted or forged; obtain new one |

---

### Renew a Token

Renew a token before it expires. The renewed token has fresh timestamps but maintains the same identity and scope.

**What's happening:** Renewal extends your token's lifetime without needing to go back through the full registration flow. The broker validates your current token and issues a fresh one. This is efficient for long-running tasks. A renewal loop pattern keeps tokens fresh automatically.

**Python example (simple renewal):**

```python
import requests

BROKER = "http://localhost:8080"

def renew_token(broker, token):
    """Renew a token. Returns new token data or raises on failure."""
    resp = requests.post(
        f"{broker}/v1/token/renew",
        headers={"Authorization": f"Bearer {token}"},
    )

    if resp.status_code == 401:
        raise RuntimeError("Token expired -- re-register to get a new one")
    if resp.status_code == 403:
        raise RuntimeError("Token revoked -- re-register to get a new one")

    resp.raise_for_status()
    return resp.json()

# Simple renewal
token = "<your_current_access_token>"
try:
    data = renew_token(BROKER, token)
    new_token = data["access_token"]
    new_ttl = data["expires_in"]
    print(f"Renewed. New TTL: {new_ttl}s")
except RuntimeError as e:
    print(f"Renewal failed: {e}")
```

**Python example (renewal loop for long-running tasks):**

```python
import requests
import time
import threading

BROKER = "http://localhost:8080"

class TokenManager:
    """Manages token renewal for long-running tasks."""

    def __init__(self, broker, token):
        self.broker = broker
        self.token = token
        self.expires_at = 0
        self.lock = threading.Lock()
        self._renew_loop_thread = None
        self._stop_flag = False

    def acquire(self):
        """Get the current token, renewing if needed."""
        with self.lock:
            now = time.time()

            # If token is within 80% TTL, renew it
            if now >= self.expires_at * 0.8:
                self._refresh_token()

            return self.token

    def _refresh_token(self):
        """Refresh the token via renewal."""
        try:
            resp = requests.post(
                f"{self.broker}/v1/token/renew",
                headers={"Authorization": f"Bearer {self.token}"},
                timeout=5
            )
            if resp.status_code in (401, 403):
                # Token expired or revoked; cannot auto-renew
                raise RuntimeError("Token cannot be renewed; agent must re-register")

            resp.raise_for_status()
            data = resp.json()
            self.token = data["access_token"]
            self.expires_at = time.time() + data["expires_in"]
        except requests.exceptions.RequestException as e:
            raise RuntimeError(f"Renewal failed: {e}")

    def start_renewal_loop(self):
        """Start a background thread that renews the token periodically."""
        if self._renew_loop_thread is not None:
            return  # Already running

        self._stop_flag = False
        self._renew_loop_thread = threading.Thread(target=self._renewal_loop, daemon=True)
        self._renew_loop_thread.start()

    def _renewal_loop(self):
        """Background loop: renew token at 80% TTL."""
        while not self._stop_flag:
            time.sleep(1)  # Check every second

            try:
                with self.lock:
                    now = time.time()
                    if now >= self.expires_at * 0.8:
                        self._refresh_token()
            except Exception as e:
                print(f"Renewal loop error: {e}")

    def stop(self):
        """Stop the renewal loop."""
        self._stop_flag = True
        if self._renew_loop_thread:
            self._renew_loop_thread.join(timeout=2)

# Usage
# Assuming you already have a token from registration
current_token = "<your_access_token>"
manager = TokenManager(BROKER, current_token)
manager.start_renewal_loop()

# In your main loop, get the token whenever needed
active_token = manager.acquire()
print(f"Using token: {active_token[:40]}...")

# Cleanup
manager.stop()
```

**Expected response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "scope": ["read:data:*"],
  "token_type": "Bearer"
}
```

**TypeScript/Node.js example (renewal loop):**

```typescript
import fetch from 'node-fetch';

const BROKER = "http://localhost:8080";

interface TokenData {
  access_token: string;
  expires_in: number;
  scope: string[];
  token_type: string;
}

class TokenManager {
  private broker: string;
  private agentName: string;
  private scope: string[];
  private ttl: number;
  private token: string | null = null;
  private expiresAt: number = 0;
  private renewalTimer: NodeJS.Timeout | null = null;

  constructor(broker: string, agentName: string, scope: string[], ttl: number = 300) {
    this.broker = broker;
    this.agentName = agentName;
    this.scope = scope;
    this.ttl = ttl;
  }

  async acquire(): Promise<string> {
    const now = Date.now() / 1000;

    // If no token or within 80% of TTL, refresh
    if (!this.token || now >= this.expiresAt * 0.8) {
      await this.refresh();
    }

    return this.token!;
  }

  private async refresh(): Promise<void> {
    if (this.token) {
      try {
        const response = await fetch(`${this.broker}/v1/token/renew`, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${this.token}`,
            'Content-Type': 'application/json',
          },
          signal: AbortSignal.timeout(5000),
        });

        if (response.status === 401 || response.status === 403) {
          // Token expired or revoked; re-acquire
          await this.acquireFresh();
        } else if (response.ok) {
          const data = (await response.json()) as TokenData;
          this.token = data.access_token;
          this.expiresAt = Date.now() / 1000 + data.expires_in;
        } else {
          throw new Error(`Renewal failed: ${response.status}`);
        }
      } catch (error) {
        // Network error; try to re-acquire
        await this.acquireFresh();
      }
    } else {
      await this.acquireFresh();
    }
  }

  private async acquireFresh(): Promise<void> {
    const response = await fetch(`${this.broker}/v1/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        agent_name: this.agentName,
        scope: this.scope,
        ttl: this.ttl,
      }),
      signal: AbortSignal.timeout(5000),
    });

    if (!response.ok) {
      throw new Error(`Token acquisition failed: ${response.status}`);
    }

    const data = (await response.json()) as TokenData;
    this.token = data.access_token;
    this.expiresAt = Date.now() / 1000 + data.expires_in;
  }

  startRenewalLoop(): void {
    if (this.renewalTimer) {
      return;  // Already running
    }

    // Renewal every second; check if refresh needed
    this.renewalTimer = setInterval(async () => {
      const now = Date.now() / 1000;
      if (this.token && now >= this.expiresAt * 0.8) {
        try {
          await this.refresh();
        } catch (error) {
          console.error(`Renewal loop error: ${error}`);
        }
      }
    }, 1000);
  }

  stop(): void {
    if (this.renewalTimer) {
      clearInterval(this.renewalTimer);
      this.renewalTimer = null;
    }
  }
}

// Usage
const manager = new TokenManager(BROKER, "my-agent", ["read:data:*"], 300);
manager.startRenewalLoop();

// Get token whenever needed
const token = await manager.acquire();
console.log(`Using token: ${token.substring(0, 40)}...`);

// Cleanup
manager.stop();
```

**When to renew:** At 80% of the TTL. For a 300-second token, renew at the 240-second mark.

**If renewal fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 401 | Token expired | Re-acquire via `POST /v1/token` |
| 403 | Token revoked | Re-acquire via `POST /v1/token`; investigate with operator |
| 502 | Broker unreachable | Retry with exponential backoff |
| 503 | Broker unavailable | Retry with exponential backoff; use stale token if available locally |

---

### Validate and Inspect Token Claims

Programmatically check token expiration and scope before making requests.

**Python example:**

```python
import requests
from datetime import datetime, timedelta
import time

BROKER = "http://localhost:8080"

def token_is_valid_and_has_scope(broker, token, required_scope):
    """Check if token is valid and has required scope."""
    resp = requests.post(
        f"{broker}/v1/token/validate",
        json={"token": token},
    )
    resp.raise_for_status()
    result = resp.json()

    if not result["valid"]:
        return False, f"Token invalid: {result.get('error')}"

    claims = result["claims"]

    # Check expiration
    now = time.time()
    if claims["exp"] <= now:
        return False, "Token expired"

    # Check scope
    token_scopes = set(claims.get("scope", []))
    if not required_scope.issubset(token_scopes):
        return False, f"Token lacks required scope: {required_scope}"

    return True, "Token valid"

# Usage
token = "<token_to_check>"
required_scope = {"read:data:users"}

valid, message = token_is_valid_and_has_scope(BROKER, token, required_scope)
if valid:
    print("✓ Token is valid and has required scope")
else:
    print(f"✗ {message}")
```

---

### Delegate a Token to Another Agent

Delegation lets your agent issue a narrower-scoped token to another registered agent. Scopes can only narrow (attenuate), never expand.

**What's happening:** Delegation creates a new token signed by the broker, with a reference back to your token in the delegation chain. This allows you to hand off work to a less-privileged agent without exposing your full token. The chain is cryptographically verified; each entry must narrow the scope further.

**Python example:**

```python
import requests

BROKER = "http://localhost:8080"

def delegate_token(broker, my_token, delegate_agent_id, scope, ttl=60):
    """Delegate a narrower-scoped token to another agent."""
    resp = requests.post(
        f"{broker}/v1/delegate",
        headers={"Authorization": f"Bearer {my_token}"},
        json={
            "delegate_to": delegate_agent_id,
            "scope": scope,
            "ttl": ttl,
        },
    )

    if resp.status_code == 400:
        error = resp.json()
        raise ValueError(f"Invalid delegation request: {error.get('detail')}")
    if resp.status_code == 403:
        error = resp.json()
        raise ValueError(f"Scope escalation attempted: {error.get('detail')}")
    if resp.status_code == 404:
        raise ValueError("Delegate agent not found")

    resp.raise_for_status()
    return resp.json()

# Delegate a token
my_token = "<your_access_token_with_read:data:*>"
delegate_agent_id = "spiffe://agentauth.local/agent/orch-001/task-002/abc123"

try:
    result = delegate_token(
        BROKER,
        my_token,
        delegate_agent_id,
        scope=["read:data:users"],  # Narrower than read:data:*
        ttl=60
    )

    delegated_token = result["access_token"]
    chain = result["delegation_chain"]

    print(f"Delegated token acquired (expires in {result['expires_in']}s)")
    print(f"Delegation chain depth: {len(chain)}")
    for i, entry in enumerate(chain):
        print(f"  [{i}] {entry['agent']} -> scope: {entry['scope']}")
except ValueError as e:
    print(f"Delegation failed: {e}")
except requests.exceptions.RequestException as e:
    print(f"Network error: {e}")
```

**Expected response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "expires_in": 60,
  "delegation_chain": [
    {
      "agent": "spiffe://agentauth.local/agent/orch-001/task-001/abc",
      "scope": ["read:data:*"]
    },
    {
      "agent": "spiffe://agentauth.local/agent/orch-001/task-002/def",
      "scope": ["read:data:users"]
    }
  ],
  "chain_hash": "a1b2c3d4e5f6..."
}
```

**TypeScript/Node.js example:**

```typescript
import fetch from 'node-fetch';

const BROKER = "http://localhost:8080";

interface DelegationChainEntry {
  agent: string;
  scope: string[];
}

interface DelegationResponse {
  access_token: string;
  expires_in: number;
  delegation_chain: DelegationChainEntry[];
  chain_hash: string;
}

async function delegateToken(
  broker: string,
  myToken: string,
  delegateAgentId: string,
  scope: string[],
  ttl: number = 60
): Promise<DelegationResponse> {
  const response = await fetch(`${broker}/v1/delegate`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${myToken}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      delegate_to: delegateAgentId,
      scope,
      ttl,
    }),
  });

  if (!response.ok) {
    const error = await response.json();
    if (response.status === 400) {
      throw new Error(`Invalid delegation request: ${error.detail}`);
    }
    if (response.status === 403) {
      throw new Error(`Scope escalation attempted: ${error.detail}`);
    }
    if (response.status === 404) {
      throw new Error('Delegate agent not found');
    }
    throw new Error(`Delegation failed: ${response.status}`);
  }

  return response.json() as Promise<DelegationResponse>;
}

// Usage
try {
  const result = await delegateToken(
    BROKER,
    "<your_access_token_with_read:data:*>",
    "spiffe://agentauth.local/agent/orch-001/task-002/abc123",
    ["read:data:users"],
    60
  );

  console.log(`Delegated token acquired (expires in ${result.expires_in}s)`);
  console.log(`Delegation chain depth: ${result.delegation_chain.length}`);
  result.delegation_chain.forEach((entry, i) => {
    console.log(`  [${i}] ${entry.agent} -> scope: ${entry.scope.join(', ')}`);
  });
} catch (error) {
  console.error(`Delegation failed: ${error.message}`);
}
```

**Delegation rules:**

- The delegated `scope` must be a subset of your token's scope.
- You cannot escalate: `read:data:*` cannot delegate `write:data:*`.
- You cannot widen resources: `read:data:users` cannot delegate `read:data:*`.
- Maximum delegation chain depth is 5.
- The `delegate_to` must be the SPIFFE ID of a registered agent.
- Default TTL is 60 seconds if not specified.
- Each chain entry is cryptographically signed; the token includes a `chain_hash` claim for integrity verification.

**If delegation fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Invalid request (bad format, missing fields) | Verify `delegate_to` is a valid SPIFFE ID and `scope` is an array |
| 403 | Scope escalation or widening attempted | Ensure delegated scope is a strict subset of your scope |
| 404 | Delegate agent not found in broker | Verify the agent's SPIFFE ID is correct and it has registered |
| 401 | Your token is invalid or expired | Renew or re-register to get a fresh token |

---

### Handle Token Expiration Gracefully

Implement graceful token expiration with automatic re-acquisition.

**Python example:**

```python
import requests
import time
from functools import wraps

BROKER = "http://localhost:8080"

def acquire_token(broker, agent_name, scope, ttl=300):
    """Acquire a fresh token from the broker."""
    resp = requests.post(
        f"{broker}/v1/register",
        json={
            "agent_name": agent_name,
            "scope": scope,
            "ttl": ttl,
        }
    )
    resp.raise_for_status()
    return resp.json()

def handle_token_expiration(broker_url, agent_name, scope):
    """Decorator: handle token expiration and retry requests."""
    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            # Get token from kwargs or acquire fresh
            token = kwargs.get("token")
            if not token:
                token_data = acquire_token(broker_url, agent_name, scope)
                token = token_data["access_token"]

            kwargs["token"] = token
            max_retries = 3

            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except requests.exceptions.HTTPError as e:
                    if e.response.status_code == 401:
                        # Token expired; re-acquire and retry
                        if attempt < max_retries - 1:
                            token_data = acquire_token(broker_url, agent_name, scope)
                            kwargs["token"] = token_data["access_token"]
                            continue
                    raise

            raise RuntimeError("Max retries exceeded")
        return wrapper
    return decorator

@handle_token_expiration(BROKER, "my-agent", ["read:data:*"])
def make_api_call(endpoint, token):
    """Example API call using token."""
    headers = {"Authorization": f"Bearer {token}"}
    resp = requests.get(endpoint, headers=headers)
    resp.raise_for_status()
    return resp.json()

# Usage
try:
    result = make_api_call("http://api.example.com/data", token=None)
    print(result)
except Exception as e:
    print(f"API call failed: {e}")
```

---

### Check Broker Health

Verify that the broker is running and ready to serve requests.

**What's happening:** The health endpoint reports on broker status, database connectivity, and audit event count. Use this for startup checks and monitoring.

**Python example:**

```python
import requests
import json

BROKER = "http://localhost:8080"

def check_broker_health(broker):
    """Check broker health status."""
    resp = requests.get(f"{broker}/v1/health")
    resp.raise_for_status()
    return resp.json()

try:
    health = check_broker_health(BROKER)

    print(f"Status:              {health['status']}")
    print(f"Version:             {health['version']}")
    print(f"Database connected:  {health['db_connected']}")
    print(f"Audit events:        {health['audit_events_count']}")
    print(f"Uptime:              {health['uptime']}s")

    if health['status'] != 'ok':
        print(f"⚠ Warning: Broker status is {health['status']}")
except requests.exceptions.ConnectionError:
    print("✗ Broker unreachable at", BROKER)
except requests.exceptions.HTTPError as e:
    print(f"✗ Health check failed: {e.response.status_code}")
```

**Expected response (200 OK):**

```json
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 1234,
  "db_connected": true,
  "audit_events_count": 42
}
```

**TypeScript/Node.js example:**

```typescript
import fetch from 'node-fetch';

const BROKER = "http://localhost:8080";

interface HealthStatus {
  status: string;
  version: string;
  uptime: number;
  db_connected: boolean;
  audit_events_count: number;
}

async function checkBrokerHealth(broker: string): Promise<HealthStatus> {
  try {
    const response = await fetch(`${broker}/v1/health`, {
      method: 'GET',
      signal: AbortSignal.timeout(5000),
    });

    if (!response.ok) {
      throw new Error(`Health check failed: ${response.status}`);
    }

    return response.json() as Promise<HealthStatus>;
  } catch (error) {
    throw new Error(`Broker unreachable: ${error}`);
  }
}

// Usage
try {
  const health = await checkBrokerHealth(BROKER);

  console.log(`Status:              ${health.status}`);
  console.log(`Version:             ${health.version}`);
  console.log(`Database connected:  ${health.db_connected}`);
  console.log(`Audit events:        ${health.audit_events_count}`);
  console.log(`Uptime:              ${health.uptime}s`);

  if (health.status !== 'ok') {
    console.warn(`⚠ Warning: Broker status is ${health.status}`);
  }
} catch (error) {
  console.error(`✗ ${error.message}`);
}
```

**If this fails:**

| Issue | Meaning | Action |
|-------|---------|--------|
| Connection refused | Broker is not running | Start the broker container or process |
| `db_connected: false` | Broker cannot reach database | Check database is running; verify data path |
| HTTP 503 | Broker is starting up | Retry in a few seconds |

---

## Operator Tasks

> **Persona:** Platform Operator managing AgentAuth deployments.
>
> These tasks cover administrative operations: authentication, launch token management, revocation, audit, and monitoring. All examples use curl or similar.
>
> **Prerequisite:** [Getting Started: Operator](getting-started-operator.md), Broker running, `AA_ADMIN_SECRET` set.

### Authenticate as Admin

Every admin operation requires a Bearer token obtained from the admin auth endpoint. Admin tokens have a 300-second TTL and include all admin scopes.

**What's happening:** Admin authentication is separate from agent authentication. You provide your admin secret, get back a short-lived admin token, and use that Bearer token for all subsequent admin operations. Cache and reuse the token within its TTL.

**Using aactl (recommended):**

aactl reads `AA_ADMIN_SECRET` from environment variables and authenticates automatically before every command. No explicit login step is needed:

```bash
export AA_ADMIN_SECRET=change-me-in-production

# aactl then auto-authenticates on each invocation
aactl app list
```

**Bash/curl example:**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
ADMIN_SECRET="change-me-in-production"

# Get admin token
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

echo "Admin token acquired (first 40 chars): ${ADMIN_TOKEN:0:40}..."
echo "TTL: 300s"
```

**Expected response (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "token_type": "Bearer"
}
```

**Python example:**

```python
import requests
import os
import time

BROKER = "http://localhost:8080"

class AdminTokenManager:
    """Manage admin token acquisition and caching."""

    def __init__(self, broker, admin_secret):
        self.broker = broker
        self.admin_secret = admin_secret
        self.token = None
        self.expires_at = 0

    def get_token(self):
        """Get a cached admin token or acquire a fresh one."""
        now = time.time()

        if self.token and now < self.expires_at:
            return self.token

        # Acquire fresh
        resp = requests.post(
            f"{self.broker}/v1/admin/auth",
            json={"secret": self.admin_secret}
        )
        resp.raise_for_status()
        data = resp.json()

        self.token = data["access_token"]
        self.expires_at = now + data["expires_in"] - 10  # 10s buffer

        return self.token

# Usage
admin_secret = os.getenv("AA_ADMIN_SECRET")
token_mgr = AdminTokenManager(BROKER, admin_secret)

try:
    token = token_mgr.get_token()
    print(f"Admin token acquired: {token[:40]}...")
except requests.exceptions.RequestException as e:
    print(f"Authentication failed: {e}")
```

**Admin token scopes:** `admin:launch-tokens:*`, `admin:revoke:*`, `admin:audit:*`

**Rate limit:** 5 requests/second, burst 10, per IP address. Exceeding returns 429 with `Retry-After: 1`.

**If authentication fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 401 | Bad credentials | Verify `AA_ADMIN_SECRET` is correct and matches broker |
| 400 | Missing `secret` field | Ensure `secret` field is present in the request |
| 429 | Rate limited | Wait for `Retry-After` seconds before retrying |

---

### Create Launch Tokens

Launch tokens are the "secret zero" that bootstraps agent identity. Design the launch token policy carefully—it defines the maximum scope and TTL that any agent using this token can receive.

**What's happening:** When you create a launch token, the broker generates a single-use (or multi-use) bootstrap credential. Agents use this token to register and receive their SPIFFE identity. The policy attached to the launch token is enforced during agent registration, preventing agents from requesting excessive scopes.

**Bash/curl example:**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"

# Acquire admin token first
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Create launch token
RESPONSE=$(curl -s -X POST "$BROKER/v1/admin/launch-tokens" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "agent_name": "data-processor",
    "allowed_scope": ["read:data:*"],
    "max_ttl": 300,
    "single_use": true,
    "ttl": 30
  }')

echo "$RESPONSE" | python3 -m json.tool

LAUNCH_TOKEN=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['launch_token'])")
echo ""
echo "Launch token: $LAUNCH_TOKEN"
```

**Expected response (201 Created):**

```json
{
  "launch_token": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0",
  "expires_at": "2026-02-15T12:00:30Z",
  "policy": {
    "allowed_scope": ["read:data:*"],
    "max_ttl": 300,
    "single_use": true
  }
}
```

**Python example:**

```python
import requests
import json
from datetime import datetime

BROKER = "http://localhost:8080"

def create_launch_token(broker, admin_token, agent_name, allowed_scope, max_ttl, single_use=True, ttl=30):
    """Create a launch token for agent registration."""
    resp = requests.post(
        f"{broker}/v1/admin/launch-tokens",
        headers={
            "Authorization": f"Bearer {admin_token}",
            "Content-Type": "application/json",
        },
        json={
            "agent_name": agent_name,
            "allowed_scope": allowed_scope,
            "max_ttl": max_ttl,
            "single_use": single_use,
            "ttl": ttl,
        }
    )

    if resp.status_code == 400:
        raise ValueError(f"Invalid request: {resp.json().get('detail')}")
    if resp.status_code == 401:
        raise ValueError("Admin token invalid or expired")

    resp.raise_for_status()
    return resp.json()

# Create launch token
try:
    result = create_launch_token(
        BROKER,
        admin_token="<admin_token>",
        agent_name="data-processor",
        allowed_scope=["read:data:*"],
        max_ttl=300,
        single_use=True,
        ttl=30
    )

    launch_token = result["launch_token"]
    expires_at = result["expires_at"]
    policy = result["policy"]

    print(f"Launch token created")
    print(f"  Token:        {launch_token}")
    print(f"  Expires at:   {expires_at}")
    print(f"  Allowed scope: {', '.join(policy['allowed_scope'])}")
    print(f"  Max TTL:      {policy['max_ttl']}s")
    print(f"  Single-use:   {policy.get('single_use', False)}")
except ValueError as e:
    print(f"Failed to create launch token: {e}")
except requests.exceptions.RequestException as e:
    print(f"Network error: {e}")
```

**Request fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_name` | string | Yes | Agent identifier |
| `allowed_scope` | string[] | Yes | Maximum scope agents can request (principle of least privilege) |
| `max_ttl` | int | Yes | Maximum agent token lifetime in seconds (typically 300-900) |
| `single_use` | boolean | No | If `true`, token is consumed on first registration (default: `true`) |
| `ttl` | int | No | Launch token lifetime in seconds (default: 30) |

**Policy design guidance:**

| Decision | Recommendation |
|----------|---------------|
| **`single_use`** | Set `true` for one-shot agents; set `false` for orchestrators that register multiple agents with the same token |
| **`ttl`** (launch token lifetime) | Keep short (30s default). This controls how long the token is valid for initiating registration, not the agent token lifetime. |
| **`max_ttl`** (agent token cap) | Match to expected task duration: 300s (5 min) for short tasks, up to 900s (15 min) for longer workflows. |
| **`allowed_scope`** | Principle of least privilege. A data-reader agent gets `["read:data:*"]`, not `["read:data:*", "write:data:*"]`. |

**Key distinction:** `ttl` controls how long the launch token itself is valid (default 30s). `max_ttl` caps the TTL of the agent token issued during registration (default 300s). These are independent.

**If creation fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Invalid policy (e.g., empty allowed_scope) | Verify `allowed_scope` is not empty and contains valid scope strings |
| 401 | Admin token invalid or expired | Re-authenticate and get a fresh admin token |
| 403 | Policy violates constraints | Check that `max_ttl` is not excessive (max: 900s) |

---

### Revoke Tokens at Different Levels

AgentAuth provides four revocation levels, each with a different blast radius. Use the narrowest level that addresses the incident.

**What's happening:** Revocation is cryptographically verified: revoked tokens are immediately rejected on the next validation. The four levels allow you to target the exact scope of compromise—single token, all tokens for one agent, all tokens for one task, or an entire delegation chain.

**Using aactl (recommended):**

> **Note:** aactl is available for demo and development use. Production auth will be added in a future release.

```bash
# Token-level: revoke a single token by JTI
aactl revoke --level token --target a1b2c3d4e5f6...

# Agent-level: revoke all tokens for one agent
aactl revoke --level agent --target spiffe://agentauth.local/agent/orch/task/instance

# Task-level: revoke all tokens for a task
aactl revoke --level task --target task-001

# Chain-level: revoke an entire delegation chain
aactl revoke --level chain --target spiffe://agentauth.local/agent/orch/task/instance
```

**Bash/curl example (token-level revocation):**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"

# Authenticate
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Validate the token to get its JTI
JTI=$(curl -s -X POST "$BROKER/v1/token/validate" \
  -H "Content-Type: application/json" \
  -d '{"token": "TOKEN_TO_REVOKE"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['claims']['jti'])")

# Revoke by JTI (token-level)
curl -s -X POST "$BROKER/v1/revoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"token\", \"target\": \"$JTI\"}" \
  | python3 -m json.tool

echo "Token revoked: $JTI"
```

**Expected response (200 OK, all revocation types):**

```json
{
  "revoked": true,
  "level": "token",
  "target": "a1b2c3d4e5f6...",
  "count": 1
}
```

**Bash/curl example (agent-level revocation):**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
SPIFFE_ID="spiffe://agentauth.local/agent/orch/task/instance"

# Authenticate
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Revoke all tokens for this agent
curl -s -X POST "$BROKER/v1/revoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"agent\", \"target\": \"$SPIFFE_ID\"}" \
  | python3 -m json.tool
```

**Bash/curl example (task-level revocation):**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
TASK_ID="task-001"

# Authenticate
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Revoke all tokens for this task
curl -s -X POST "$BROKER/v1/revoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"task\", \"target\": \"$TASK_ID\"}" \
  | python3 -m json.tool
```

**Bash/curl example (chain-level revocation):**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
ROOT_DELEGATOR="spiffe://agentauth.local/agent/orch/task/instance"

# Authenticate
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# Revoke entire delegation chain rooted at this agent
curl -s -X POST "$BROKER/v1/revoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"chain\", \"target\": \"$ROOT_DELEGATOR\"}" \
  | python3 -m json.tool
```

**Python example (all revocation levels):**

```python
import requests

BROKER = "http://localhost:8080"

def revoke_tokens(broker, admin_token, level, target):
    """Revoke tokens at the specified level."""
    resp = requests.post(
        f"{broker}/v1/revoke",
        headers={
            "Authorization": f"Bearer {admin_token}",
            "Content-Type": "application/json",
        },
        json={
            "level": level,
            "target": target,
        }
    )

    if resp.status_code == 400:
        raise ValueError(f"Invalid request: {resp.json().get('detail')}")
    if resp.status_code == 401:
        raise ValueError("Admin token invalid or expired")
    if resp.status_code == 404:
        raise ValueError(f"Target not found: {target}")

    resp.raise_for_status()
    return resp.json()

# Example: revoke a single token by JTI
try:
    result = revoke_tokens(
        BROKER,
        admin_token="<admin_token>",
        level="token",
        target="a1b2c3d4e5f6..."
    )
    print(f"✓ Revoked {result['count']} token(s) at level '{result['level']}'")
except ValueError as e:
    print(f"✗ Revocation failed: {e}")

# Example: revoke all tokens for an agent
try:
    result = revoke_tokens(
        BROKER,
        admin_token="<admin_token>",
        level="agent",
        target="spiffe://agentauth.local/agent/orch/task/instance"
    )
    print(f"✓ Revoked {result['count']} token(s) for agent")
except ValueError as e:
    print(f"✗ Revocation failed: {e}")

# Example: revoke all tokens for a task
try:
    result = revoke_tokens(
        BROKER,
        admin_token="<admin_token>",
        level="task",
        target="task-001"
    )
    print(f"✓ Revoked {result['count']} token(s) for task")
except ValueError as e:
    print(f"✗ Revocation failed: {e}")

# Example: revoke entire delegation chain
try:
    result = revoke_tokens(
        BROKER,
        admin_token="<admin_token>",
        level="chain",
        target="spiffe://agentauth.local/agent/orch/task/root"
    )
    print(f"✓ Revoked {result['count']} token(s) in delegation chain")
except ValueError as e:
    print(f"✗ Revocation failed: {e}")
```

**Revocation decision tree:**

```
What happened?
├─ Single token leaked
│  └─ Token-level revocation (target = JTI)
├─ Agent instance compromised
│  └─ Agent-level revocation (target = SPIFFE ID)
├─ Entire task suspect
│  └─ Task-level revocation (target = task_id)
└─ Delegation chain exploited
   └─ Chain-level revocation (target = root delegator SPIFFE ID)
```

**If revocation fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Invalid target or level | Verify target format matches the level (JTI for token, SPIFFE ID for agent/chain, task_id for task) |
| 401 | Admin token invalid | Re-authenticate and get a fresh admin token |
| 404 | Target not found | Verify the target exists in the system |
| 409 | Already revoked | The target was previously revoked; operation is idempotent (safe to retry) |

---

### Query the Audit Trail

The audit trail is an append-only, hash-chained log of every significant operation. Use it for forensics, compliance, and incident investigation.

**What's happening:** Every token acquisition, revocation, delegation, and admin action is logged with a cryptographic hash chain. You can query by agent, task, time range, or event type. Hash chaining allows you to detect if any audit record has been tampered with.

**Using aactl (recommended):**

> **Note:** aactl is available for demo and development use. Production auth will be added in a future release.

```bash
# All events (table output)
aactl audit events

# Filter by event type
aactl audit events --event-type token_revoked

# Filter by agent
aactl audit events --agent-id spiffe://agentauth.local/agent/orch/task/instance

# Filter by time range with limit
aactl audit events --since 2026-02-19T00:00:00Z --limit 50

# Raw JSON output
aactl audit events --json
```

**Bash/curl examples:**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"

# Authenticate
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$AA_ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

# All events (default: last 100)
echo "=== Last 100 events ==="
curl -s "http://localhost:8080/v1/audit/events" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Filter by agent
echo "=== Events for specific agent ==="
curl -s "http://localhost:8080/v1/audit/events?agent_id=spiffe://agentauth.local/agent/orch/task/instance" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Filter by event type
echo "=== Token revocation events ==="
curl -s "http://localhost:8080/v1/audit/events?event_type=token_revoked" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Filter by time range
echo "=== Events from past hour ==="
curl -s "http://localhost:8080/v1/audit/events?since=2026-02-15T11:00:00Z&until=2026-02-15T12:00:00Z" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Filter by task
echo "=== Events for specific task ==="
curl -s "http://localhost:8080/v1/audit/events?task_id=task-001" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Filter by outcome (success, failure, or completed)
echo "=== Failed operations ==="
curl -s "http://localhost:8080/v1/audit/events?outcome=failure" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

# Paginate through large result sets
echo "=== Paginated results (50 per page, skip first 100) ==="
curl -s "http://localhost:8080/v1/audit/events?limit=50&offset=100" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool
```

**Expected response (200 OK):**

```json
{
  "events": [
    {
      "id": "event-001",
      "timestamp": "2026-02-15T11:00:00Z",
      "event_type": "token_acquired",
      "agent_id": "spiffe://agentauth.local/agent/orch/task/instance",
      "task_id": "task-001",
      "orch_id": "orch-001",
      "detail": {
        "scope": ["read:data:*"],
        "ttl": 300
      },
      "hash": "a1b2c3d4...",
      "prev_hash": "0000000..."
    }
  ],
  "total": 1,
  "limit": 100,
  "offset": 0
}
```

**Python example:**

```python
import requests
from datetime import datetime, timedelta
import json

BROKER = "http://localhost:8080"

def query_audit_trail(broker, admin_token, **filters):
    """Query the audit trail with optional filters."""
    resp = requests.get(
        f"{broker}/v1/audit/events",
        headers={"Authorization": f"Bearer {admin_token}"},
        params=filters
    )
    resp.raise_for_status()
    return resp.json()

# Example 1: Get all events
try:
    result = query_audit_trail(BROKER, admin_token="<admin_token>")
    print(f"Total events: {result['total']}")
    for event in result["events"]:
        print(f"  {event['timestamp']} {event['event_type']}: {event['agent_id']}")
except requests.exceptions.RequestException as e:
    print(f"Query failed: {e}")

# Example 2: Filter by agent
try:
    result = query_audit_trail(
        BROKER,
        admin_token="<admin_token>",
        agent_id="spiffe://agentauth.local/agent/orch/task/instance"
    )
    print(f"Events for agent: {result['total']}")
except requests.exceptions.RequestException as e:
    print(f"Query failed: {e}")

# Example 3: Filter by event type
try:
    result = query_audit_trail(
        BROKER,
        admin_token="<admin_token>",
        event_type="token_revoked"
    )
    print(f"Revocation events: {result['total']}")
except requests.exceptions.RequestException as e:
    print(f"Query failed: {e}")

# Example 4: Filter by time range (past 24 hours)
try:
    now = datetime.utcnow()
    yesterday = now - timedelta(hours=24)

    result = query_audit_trail(
        BROKER,
        admin_token="<admin_token>",
        since=yesterday.isoformat() + "Z",
        until=now.isoformat() + "Z"
    )
    print(f"Events in past 24h: {result['total']}")
except requests.exceptions.RequestException as e:
    print(f"Query failed: {e}")

# Example 5: Paginate through large result sets
try:
    all_events = []
    limit = 100
    offset = 0

    while True:
        result = query_audit_trail(
            BROKER,
            admin_token="<admin_token>",
            limit=limit,
            offset=offset
        )
        all_events.extend(result["events"])

        if len(result["events"]) < limit:
            break

        offset += limit

    print(f"Retrieved all {len(all_events)} events")
except requests.exceptions.RequestException as e:
    print(f"Query failed: {e}")

# Example 6: Verify hash chain integrity
def verify_hash_chain(events):
    """Verify that the audit trail hash chain is intact."""
    import hashlib

    for i, event in enumerate(events):
        # Re-compute hash: SHA256(prev_hash | event_data)
        prev_hash = event.get("prev_hash", "")

        event_data = f"{event['id']}|{event['timestamp']}|{event['event_type']}|{event['agent_id']}|{event['task_id']}|{event['orch_id']}|{json.dumps(event['detail'])}"
        computed_hash = hashlib.sha256((prev_hash + event_data).encode()).hexdigest()

        if computed_hash != event["hash"]:
            print(f"✗ Hash mismatch at event {i}: expected {event['hash']}, got {computed_hash}")
            return False

    print("✓ Hash chain verified")
    return True

try:
    result = query_audit_trail(BROKER, admin_token="<admin_token>", limit=50)
    verify_hash_chain(result["events"])
except Exception as e:
    print(f"Verification failed: {e}")
```

**Available filter parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `agent_id` | string | Filter by agent SPIFFE ID |
| `task_id` | string | Filter by task ID |
| `event_type` | string | Filter by event type (e.g., `token_acquired`, `token_revoked`, `delegation_issued`) |
| `outcome` | string | Filter by outcome: `success`, `failure`, or `completed` |
| `since` | string | Start time (RFC 3339 format, e.g., `2026-02-15T00:00:00Z`) |
| `until` | string | End time (RFC 3339 format) |
| `limit` | int | Max results (default: 100, max: 1000) |
| `offset` | int | Skip N results for pagination |

**Common event types:**

| Type | Meaning |
|------|---------|
| `token_acquired` | Token issued to agent |
| `token_renewed` | Token renewed by agent |
| `token_revoked` | Token revoked by operator |
| `token_validated` | Token validated (may be frequent; filter carefully) |
| `delegation_issued` | Token delegated from one agent to another |
| `agent_registered` | New agent registered with broker |
| `launch_token_created` | Launch token created by operator |
| `admin_auth` | Admin authentication occurred |

**Hash chain verification:** Each event has a `hash` and `prev_hash` field. To verify integrity, recompute `SHA256(prev_hash | id | timestamp | event_type | agent_id | task_id | orch_id | detail)` for each event and confirm it matches the recorded `hash`. The first event's `prev_hash` is all zeros.

**If audit query fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Invalid filter parameters | Check date format (RFC 3339), numeric limits, and enum values |
| 401 | Admin token invalid | Re-authenticate and get a fresh admin token |
| 500 | Audit database error | Check broker logs; may be transient |

---

### Register and Manage Apps

Apps are third-party services that call AgentAuth to create launch tokens and manage agents.

**What's happening:** Each app gets a `client_id` and `client_secret` for API authentication. The app can then authenticate to the broker via `POST /v1/app/auth`, which returns an app token with restricted scopes. The app can use this token to create launch tokens for agents it manages. The broker enforces a scope ceiling—the app cannot request scopes beyond the scope ceiling assigned during app registration.

**Using aactl (recommended):**

```bash
# Register a new app with specific scope ceiling
aactl app register --name "my-pipeline" --scopes "read:data:*,write:results:*"

# List all registered apps
aactl app list

# Get details of a specific app
aactl app get my-app-id

# Update an app's scopes
aactl app update --id my-app-id --scopes "read:data:reports"

# Deregister an app
aactl app remove --id my-app-id
```

**Bash/curl example for app authentication:**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
CLIENT_ID="app-pipeline-123"
CLIENT_SECRET="sk_app_abc123..."

# Step 1: Get an app token using client credentials
APP_TOKEN=$(curl -s -X POST "$BROKER/v1/app/auth" \
  -H "Content-Type: application/json" \
  -d "{\"client_id\": \"$CLIENT_ID\", \"client_secret\": \"$CLIENT_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

echo "App token acquired: ${APP_TOKEN:0:40}..."

# Step 2: Use the app token to create launch tokens
curl -s -X POST "$BROKER/v1/admin/launch-tokens" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $APP_TOKEN" \
  -d '{
    "agent_name": "processor-1",
    "allowed_scope": ["read:data:*"],
    "ttl": 1800
  }' | python3 -m json.tool
```

**Expected response for app auth (200 OK):**

```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9...",
  "expires_in": 300,
  "token_type": "Bearer",
  "scopes": ["read:data:*", "write:results:*"]
}
```

**Python example:**

```python
import requests

BROKER = "http://localhost:8080"

def authenticate_app(broker, client_id, client_secret):
    """Authenticate as an app and get a short-lived token."""
    resp = requests.post(
        f"{broker}/v1/app/auth",
        json={
            "client_id": client_id,
            "client_secret": client_secret,
        }
    )

    if resp.status_code == 401:
        raise ValueError("Invalid app credentials")
    if resp.status_code == 429:
        raise ValueError("Rate limited; retry later")

    resp.raise_for_status()
    return resp.json()

# Get app token
try:
    result = authenticate_app(
        BROKER,
        client_id="app-pipeline-123",
        client_secret="sk_app_abc123..."
    )

    print(f"App authenticated")
    print(f"  Token: {result['access_token'][:40]}...")
    print(f"  Scopes: {', '.join(result['scopes'])}")
    print(f"  TTL: {result['expires_in']}s")
except ValueError as e:
    print(f"Authentication failed: {e}")
except requests.exceptions.RequestException as e:
    print(f"Network error: {e}")
```

**If app auth fails:**

| Status | Meaning | Action |
|--------|---------|--------|
| 400 | Missing client credentials | Verify both `client_id` and `client_secret` are present |
| 401 | Invalid credentials | Verify credentials match the registered app |
| 429 | Rate limited | Wait before retrying; apps are limited to 10 requests/minute per client_id |

---

### Monitor Health and Metrics

Track broker health, request latency, and token activity.

**What's happening:** The broker exposes a health endpoint and Prometheus-compatible metrics. These let you build dashboards, set up alerts, and investigate performance issues in real time.

**Bash/curl examples:**

```bash
#!/bin/bash

# Check broker health
curl -s http://localhost:8080/v1/health | python3 -m json.tool

# Scrape Prometheus metrics from broker
curl -s http://localhost:8080/v1/metrics | head -20
```

**Expected broker health response:**

```json
{
  "status": "ok",
  "version": "2.0.0",
  "uptime": 12345,
  "db_connected": true,
  "audit_events_count": 42
}
```

**Key metrics to watch:**

| What to monitor | Metric | Alert when |
|-----------------|--------|------------|
| Broker availability | `agentauth_broker_up` | Value is 0 |
| Failed admin auth | `agentauth_admin_auth_total{status="failure"}` | Sustained increase |
| Failed app auth | `agentauth_app_auth_total{status="failure"}` | Sustained increase |
| Registration failures | `agentauth_registrations_total{status="failure"}` | Unexpected failures |
| Revocation activity | `agentauth_tokens_revoked_total` | Unexpected spike |
| Token expiration | `agentauth_tokens_expired_total` | Monitor trends |
| Request latency | `agentauth_request_duration_seconds` | p99 exceeds acceptable threshold |

**Python example (health checks):**

```python
import requests
import json

BROKER = "http://localhost:8080"

def check_broker_health(broker):
    """Check broker health."""
    resp = requests.get(f"{broker}/v1/health")
    resp.raise_for_status()
    return resp.json()

def fetch_metrics(broker):
    """Fetch Prometheus metrics."""
    resp = requests.get(f"{broker}/v1/metrics")
    resp.raise_for_status()
    return resp.text

# Check health
try:
    broker_health = check_broker_health(BROKER)
    print(f"Broker status: {broker_health['status']}")
    print(f"Version: {broker_health['version']}")
    print(f"Database connected: {broker_health['db_connected']}")
    print(f"Audit events: {broker_health['audit_events_count']}")
except requests.exceptions.RequestException as e:
    print(f"Health check failed: {e}")

# Fetch and parse metrics
try:
    metrics = fetch_metrics(BROKER)

    # Simple example: look for specific metrics
    for line in metrics.split('\n'):
        if 'agentauth_tokens_revoked_total' in line and not line.startswith('#'):
            print(f"Revocation metric: {line}")
except requests.exceptions.RequestException as e:
    print(f"Metrics fetch failed: {e}")
```

**Prometheus scrape configuration:**

```yaml
scrape_configs:
  - job_name: 'agentauth-broker'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /v1/metrics
    scrape_interval: 15s
```

**Docker health check:**

```dockerfile
HEALTHCHECK --interval=2s --timeout=3s --retries=10 \
  CMD curl -f http://localhost:8080/v1/health || exit 1
```

**Kubernetes liveness and readiness probes:**

```yaml
livenessProbe:
  httpGet:
    path: /v1/health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /v1/health
    port: 8080
  initialDelaySeconds: 2
  periodSeconds: 5
```

**If health checks fail:**

| Issue | Meaning | Action |
|-------|---------|--------|
| Connection refused | Broker is not running | Start broker process |
| `status` != "ok" | Broker degraded | Check logs for errors; may recover automatically |
| Metrics missing | Metrics endpoint disabled | Ensure metrics are enabled in configuration |

---

### Emergency Response: Token Revocation

In a security incident, immediately revoke compromised tokens or all tokens for a specific task.

**What's happening:** If you detect a compromise, you can revoke tokens at the agent, task, or chain level. This immediately invalidates all affected tokens. Combined with targeted scope restrictions for apps, this limits further damage while you investigate.

**Bash/curl example:**

```bash
#!/bin/bash
set -e

BROKER="http://localhost:8080"
ADMIN_SECRET="change-me-in-production"

# EMERGENCY: Authenticate as admin
ADMIN_TOKEN=$(curl -s -X POST "$BROKER/v1/admin/auth" \
  -H "Content-Type: application/json" \
  -d "{\"secret\": \"$ADMIN_SECRET\"}" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")

echo "[INCIDENT] Revoking all tokens for compromised task..."

# Step 1: Revoke all tokens for the affected task
TASK_ID="task-compromised"
curl -s -X POST "$BROKER/v1/revoke" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"level\": \"task\", \"target\": \"$TASK_ID\"}" | python3 -m json.tool

echo "[INCIDENT] Revoked all tokens for $TASK_ID"

# Step 2: Restrict app scope ceiling to minimal permissions
APP_ID="affected-app"
curl -s -X PUT "$BROKER/v1/admin/apps/$APP_ID" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "scopes": ["read:data:*"]
  }' | python3 -m json.tool

echo "[INCIDENT] Restricted app $APP_ID to read-only scope"

# Step 3: Query audit trail for evidence
curl -s "http://localhost:8080/v1/audit/events?task_id=$TASK_ID&limit=20" \
  -H "Authorization: Bearer $ADMIN_TOKEN" | python3 -m json.tool

echo "[INCIDENT] Revocation plan executed. Investigate and restore scope when safe."
```

**Python emergency response script:**

```python
import requests
import os
import sys
from datetime import datetime, timedelta

BROKER = "http://localhost:8080"

def emergency_incident_response(broker, admin_secret, compromised_task_id, affected_app_id=None):
    """Execute emergency incident response: revoke tokens and restrict app scope."""

    print(f"[{datetime.utcnow().isoformat()}] EMERGENCY INCIDENT RESPONSE INITIATED")

    # Step 1: Authenticate
    try:
        auth_resp = requests.post(
            f"{broker}/v1/admin/auth",
            json={"secret": admin_secret}
        )
        auth_resp.raise_for_status()
        admin_token = auth_resp.json()["access_token"]
        print("[✓] Admin authenticated")
    except requests.exceptions.RequestException as e:
        print(f"[✗] Authentication failed: {e}")
        return False

    # Step 2: Revoke all tokens for the compromised task
    try:
        revoke_resp = requests.post(
            f"{broker}/v1/revoke",
            headers={"Authorization": f"Bearer {admin_token}"},
            json={"level": "task", "target": compromised_task_id}
        )
        revoke_resp.raise_for_status()
        revocation_count = revoke_resp.json()["count"]
        print(f"[✓] Revoked {revocation_count} token(s) for task {compromised_task_id}")
    except requests.exceptions.RequestException as e:
        print(f"[✗] Revocation failed: {e}")
        return False

    # Step 3: Restrict affected app scope to read-only
    if affected_app_id:
        try:
            app_resp = requests.put(
                f"{broker}/v1/admin/apps/{affected_app_id}",
                headers={"Authorization": f"Bearer {admin_token}"},
                json={"scopes": ["read:data:*"]}
            )
            app_resp.raise_for_status()
            print(f"[✓] Restricted app {affected_app_id} to read-only scope")
        except requests.exceptions.RequestException as e:
            print(f"[✗] App scope restriction failed: {e}")
            return False

    # Step 4: Audit trail check
    try:
        now = datetime.utcnow()
        one_hour_ago = now - timedelta(hours=1)

        audit_resp = requests.get(
            f"{broker}/v1/audit/events",
            headers={"Authorization": f"Bearer {admin_token}"},
            params={
                "task_id": compromised_task_id,
                "since": one_hour_ago.isoformat() + "Z",
                "until": now.isoformat() + "Z",
                "limit": 20
            }
        )
        audit_resp.raise_for_status()
        events = audit_resp.json()["events"]
        print(f"[✓] Audit trail retrieved: {len(events)} event(s) in past hour")

        for event in events:
            print(f"    {event['timestamp']} {event['event_type']}: {event['agent_id']}")
    except requests.exceptions.RequestException as e:
        print(f"[✗] Audit trail query failed: {e}")
        return False

    print(f"[{datetime.utcnow().isoformat()}] EMERGENCY RESPONSE COMPLETE")
    print("[!] Tokens for the compromised task have been revoked.")
    if affected_app_id:
        print(f"[!] App {affected_app_id} is now read-only.")
    print("[!] Investigate and restore normal scope settings once the incident is resolved.")

    return True

# Usage
admin_secret = os.getenv("AA_ADMIN_SECRET")
if not admin_secret:
    print("Error: AA_ADMIN_SECRET not set")
    sys.exit(1)

success = emergency_incident_response(BROKER, admin_secret, "task-compromised", "affected-app")
sys.exit(0 if success else 1)
```

**Incident response checklist:**

1. ✓ Authenticate with admin credentials
2. ✓ Revoke all tokens for the compromised task
3. ✓ Restrict affected app scope to minimal permissions
4. ✓ Query audit trail to identify affected agents and users
5. ✓ Document the timeline and scope of compromise
6. ✓ Notify relevant teams
7. ⧗ Wait for investigation to complete
8. ⧗ Restore normal scope settings once safe

---

## Error Handling Reference

AgentAuth uses RFC 7807 `application/problem+json` error responses.

**Broker error format (RFC 7807):**

```json
{
  "type": "urn:agentauth:error:scope_violation",
  "title": "Forbidden",
  "status": 403,
  "detail": "requested scope exceeds allowed scope",
  "instance": "/v1/register",
  "error_code": "scope_violation",
  "request_id": "a1b2c3d4",
  "hint": "requested scope must be a subset of allowed scope"
}
```

**Broker error format:**

```json
{
  "error": "Forbidden",
  "detail": "requested scope exceeds scope ceiling"
}
```

**Universal status code reference:**

| Status | Meaning | Retry? | Action |
|--------|---------|--------|--------|
| 200–299 | Success | N/A | Proceed |
| 400 | Bad request | No | Fix the request body |
| 401 | Unauthorized | Yes (after re-auth) | Re-acquire token; check credentials |
| 403 | Forbidden | No | Check scope or permissions |
| 404 | Not found | No | Verify resource ID |
| 429 | Rate limited | Yes | Wait for `Retry-After` header |
| 500 | Server error | Yes (with backoff) | Check service logs; may be transient |
| 502 | Bad gateway | Yes (with backoff) | Broker unreachable |
| 503 | Service unavailable | Yes (with backoff) | Service starting up or recovering |

---

## Cross-References

- **API Reference:** [api.md](api.md) — Complete endpoint documentation
- **Developer Guide:** [getting-started-developer.md](getting-started-developer.md) — Setup and first agent
- **Operator Guide:** [getting-started-operator.md](getting-started-operator.md) — Deployment and administration
- **Concepts:** [concepts.md](concepts.md) — SPIFFE, token lifetime, delegation, scopes
- **Architecture:** [architecture.md](architecture.md) — Broker, key management, trust model
- **Troubleshooting:** [troubleshooting.md](troubleshooting.md) — Common problems and solutions
