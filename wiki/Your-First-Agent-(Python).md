# Your First Agent (Python)

Build a working AI agent with AgentAuth in 15 minutes. No prior AgentAuth experience needed.

> **What you'll build:** A Python agent that gets a temporary credential, uses it to access a resource, and then releases it when done.
>
> **What you'll learn:** How to get tokens, use tokens, renew tokens, and release tokens.
>
> **Prerequisites:** Python 3.8+, Docker, and a terminal.

---

## Step 1: Start AgentAuth

If you haven't already, start the broker and sidecar:

```bash
git clone https://github.com/devonartis/agentauth-core.git
cd agentauth-core
AA_ADMIN_SECRET="my-super-secret-key-change-me" docker compose up -d
```

Verify it's running:

```bash
curl http://localhost:8081/v1/health
```

You should see:
```json
{"status": "ok", "scope_ceiling": ["read:data:*", "write:data:*"]}
```

---

## Step 2: Install the Python Library

```bash
pip install requests
```

That's the only dependency you need. AgentAuth uses standard HTTP — no special SDK required.

---

## Step 3: Write Your First Agent

Create a file called `my_agent.py`:

```python
"""
My First AgentAuth Agent
========================
This agent demonstrates the complete token lifecycle:
1. Get a token from the sidecar
2. Use the token to access a resource
3. Renew the token if needed
4. Release the token when done
"""
import requests
import time

# The sidecar runs on port 8081 by default
SIDECAR_URL = "http://localhost:8081"

# The broker runs on port 8080 (we use it for validation and release)
BROKER_URL = "http://localhost:8080"


def get_token():
    """
    Step 1: Ask the sidecar for a token.
    
    This is the simplest way to get a credential. The sidecar handles
    all the complex crypto (key generation, challenge-response, registration)
    behind the scenes.
    """
    print("📋 Requesting a token from the sidecar...")
    
    response = requests.post(f"{SIDECAR_URL}/v1/token", json={
        "agent_name": "my-first-python-agent",   # A name for your agent
        "scope": ["read:data:*"],                 # What you want access to
        "ttl": 300,                               # How long (seconds) — 5 minutes
        "task_id": "tutorial-001"                 # Optional: tag for audit trail
    })
    
    # Check for errors
    if response.status_code != 200:
        print(f"   Error: {response.status_code} - {response.text}")
        return None
    
    data = response.json()
    
    print(f"   Token received!")
    print(f"   Agent ID:    {data['agent_id']}")
    print(f"   Scope:       {data['scope']}")
    print(f"   Expires in:  {data['expires_in']} seconds")
    print(f"   Token type:  {data['token_type']}")
    print()
    
    return data


def use_token(token):
    """
    Step 2: Use the token to access a protected resource.
    
    In a real application, you'd send this token to your database,
    API, or any service that validates AgentAuth tokens.
    
    Here we'll validate the token against the broker to prove it works.
    """
    print("🔍 Validating the token against the broker...")
    
    response = requests.post(f"{BROKER_URL}/v1/token/validate", json={
        "token": token
    })
    
    data = response.json()
    
    if data["valid"]:
        claims = data["claims"]
        print(f"   Token is VALID!")
        print(f"   Subject:  {claims['sub']}")
        print(f"   Scope:    {claims['scope']}")
        print(f"   Issued:   {claims['iat']} (Unix timestamp)")
        print(f"   Expires:  {claims['exp']} (Unix timestamp)")
        print(f"   Token ID: {claims['jti']}")
    else:
        print(f"   Token is INVALID: {data.get('error', 'unknown')}")
    
    print()
    return data["valid"]


def renew_token(token):
    """
    Step 3: Renew the token before it expires.
    
    If your task takes longer than the TTL, you can renew the token
    to get a fresh one with the same identity and scope.
    
    Pro tip: Renew at 80% of the TTL (e.g., at 4 minutes for a 5-minute token).
    """
    print("🔄 Renewing the token...")
    
    response = requests.post(f"{SIDECAR_URL}/v1/token/renew",
        headers={"Authorization": f"Bearer {token}"}
    )
    
    if response.status_code == 200:
        data = response.json()
        print(f"   Renewed! New expiry: {data['expires_in']} seconds")
        print()
        return data["access_token"]
    else:
        print(f"   Renewal failed: {response.status_code} - {response.text}")
        print()
        return None


def release_token(token):
    """
    Step 4: Release the token when you're done.
    
    This is optional but good practice. It:
    - Signals that your task is complete
    - Creates a clean audit trail entry
    - Frees up resources immediately (instead of waiting for expiry)
    """
    print("🏁 Releasing the token (task complete)...")
    
    response = requests.post(f"{BROKER_URL}/v1/token/release",
        headers={"Authorization": f"Bearer {token}"}
    )
    
    if response.status_code == 204:
        print("   Token released successfully!")
    elif response.status_code == 401:
        print("   Token was already expired or revoked")
    else:
        print(f"   Release failed: {response.status_code}")
    
    print()


def main():
    print("=" * 60)
    print("  My First AgentAuth Agent (Python)")
    print("=" * 60)
    print()
    
    # Step 1: Get a token
    token_data = get_token()
    if not token_data:
        print("Failed to get token. Is AgentAuth running?")
        print("Try: AA_ADMIN_SECRET='my-super-secret-key-change-me' docker compose up -d")
        return
    
    token = token_data["access_token"]
    
    # Step 2: Use the token
    is_valid = use_token(token)
    if not is_valid:
        print("Token validation failed!")
        return
    
    # Simulate doing some work...
    print("⏳ Simulating work (2 seconds)...")
    time.sleep(2)
    print()
    
    # Step 3: Renew the token (in case we need more time)
    new_token = renew_token(token)
    if new_token:
        token = new_token  # Use the renewed token going forward
    
    # Step 4: Release when done
    release_token(token)
    
    print("=" * 60)
    print("  Done! Your first agent lifecycle is complete.")
    print("=" * 60)


if __name__ == "__main__":
    main()
```

---

## Step 4: Run It

```bash
python my_agent.py
```

You should see output like:

```
============================================================
  My First AgentAuth Agent (Python)
============================================================

📋 Requesting a token from the sidecar...
   Token received!
   Agent ID:    spiffe://agentauth.local/agent/orch/tutorial-001/proc-a1b2c3
   Scope:       ['read:data:*']
   Expires in:  300 seconds
   Token type:  Bearer

🔍 Validating the token against the broker...
   Token is VALID!
   Subject:  spiffe://agentauth.local/agent/orch/tutorial-001/proc-a1b2c3
   Scope:    ['read:data:*']
   Issued:   1745405330 (Unix timestamp)
   Expires:  1745405630 (Unix timestamp)
   Token ID: a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6

⏳ Simulating work (2 seconds)...

🔄 Renewing the token...
   Renewed! New expiry: 300 seconds

🏁 Releasing the token (task complete)...
   Token released successfully!

============================================================
  Done! Your first agent lifecycle is complete.
============================================================
```

---

## What Just Happened?

Here's the timeline of what your agent did:

```
Time 0s    → Agent asks sidecar for a token
             (Sidecar generates Ed25519 keys, does challenge-response with broker)
Time 0.1s  → Agent receives a JWT token valid for 300 seconds
Time 0.2s  → Agent validates the token with the broker (proves it works)
Time 2.2s  → Agent renews the token (gets a fresh 300-second window)
Time 2.3s  → Agent releases the token (signals "I'm done")
```

Behind the scenes, the sidecar did the heavy lifting:
1. Generated an Ed25519 key pair
2. Got a challenge nonce from the broker
3. Signed the nonce with the private key
4. Registered the agent with the broker
5. Received a signed JWT token
6. Returned it to your agent

**Your code only needed to make HTTP calls.** No crypto, no key management, no complexity.

---

## Challenge: Try These Modifications

### 1. Change the Scope
Try requesting different scopes:

```python
# Read only customer data
"scope": ["read:data:customers"]

# Read and write
"scope": ["read:data:*", "write:data:*"]
```

### 2. Use a Shorter TTL
See what happens with a 30-second token:

```python
"ttl": 30  # 30 seconds instead of 300
```

### 3. Auto-Renewal Loop
Add automatic renewal for long-running tasks:

```python
import threading

class TokenManager:
    """Automatically renews tokens before they expire."""
    
    def __init__(self, sidecar_url, agent_name, scope, ttl=300):
        self.sidecar_url = sidecar_url
        self.agent_name = agent_name
        self.scope = scope
        self.ttl = ttl
        self.token = None
        self._stop = False
    
    def start(self):
        """Get initial token and start renewal loop."""
        self._get_token()
        self._renewal_thread = threading.Thread(target=self._renewal_loop, daemon=True)
        self._renewal_thread.start()
        return self.token
    
    def stop(self):
        """Stop renewal and release the token."""
        self._stop = True
        if self.token:
            requests.post(f"{BROKER_URL}/v1/token/release",
                headers={"Authorization": f"Bearer {self.token}"})
    
    def _get_token(self):
        resp = requests.post(f"{self.sidecar_url}/v1/token", json={
            "agent_name": self.agent_name,
            "scope": self.scope,
            "ttl": self.ttl
        })
        data = resp.json()
        self.token = data["access_token"]
    
    def _renewal_loop(self):
        """Renew at 80% of TTL."""
        while not self._stop:
            time.sleep(self.ttl * 0.8)
            if self._stop:
                break
            resp = requests.post(f"{self.sidecar_url}/v1/token/renew",
                headers={"Authorization": f"Bearer {self.token}"})
            if resp.status_code == 200:
                self.token = resp.json()["access_token"]
                print(f"Token renewed automatically")
            else:
                self._get_token()  # Re-acquire if renewal fails
                print(f"Token re-acquired")

# Usage:
manager = TokenManager(SIDECAR_URL, "long-running-agent", ["read:data:*"])
token = manager.start()

# ... do work using manager.token ...

manager.stop()  # Clean up when done
```

---

## Common Errors and Fixes

| Error | Cause | Fix |
|-------|-------|-----|
| `Connection refused` | AgentAuth isn't running | Run `docker compose up -d` |
| `403: scope exceeds ceiling` | Requested scope is too broad | Check sidecar ceiling with `GET /v1/health` |
| `401: token expired` | Token TTL elapsed | Renew earlier (at 80% of TTL) or re-acquire |
| `401: launch token not found` | Internal sidecar bootstrap issue | Restart sidecar: `docker compose restart sidecar` |

---

## Next Steps

- [[Your First Agent (TypeScript)]] — Same tutorial in TypeScript
- [[Key Concepts Explained]] — Understand scopes, SPIFFE IDs, and delegation
- [[Common Tasks]] — Recipes for real-world workflows
- [[Developer Guide]] — Full developer integration guide
