# A1-R1 — Standard Issue-Validate-Renew Flow Still Works [ACCEPTANCE]

**Mode:** VPS

Who: The app. In production, an app creates agents, validates their
tokens, and agents renew before expiry. This is the core token lifecycle.

What: Regression test. The B6 TTL changes modified the Renew method.
This story proves the full lifecycle still works: app creates launch
token, agent registers, app validates the agent's token, agent renews.

Why: If B6 broke the basic token lifecycle, nothing works. This is the
most critical regression.

How to run: App flow — register app, authenticate, create launch token,
register agent, validate token, renew token. All steps must succeed.

Expected: All steps return 200/201. Validate returns valid=true. Renew
returns a new token.

## Test Output

--- Step 1: Admin auth + register app ---
App registered: app-r1-regression-app-467479

--- Step 2: App authenticates ---
App token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 3: App creates launch token ---
{
  "launch_token": "4c323673084dd7c4095ccc9c11e906c41d46114fc5e8164b89c1600f2a928f62",
  "expires_at": "2026-03-30T18:17:25Z",
  "policy": {
    "allowed_scope": [
      "read:data:*"
    ],
    "max_ttl": 300
  }
}

--- Step 4: Agent registers ---
Agent: spiffe://agentauth.local/agent/r1-orch/r1-task/dc1fd492298271c4
TTL: 300

--- Step 5: App validates agent token ---
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/r1-orch/r1-task/dc1fd492298271c4",
    "aud": [
      "agentauth"
    ],
    "exp": 1774894916,
    "nbf": 1774894616,
    "iat": 1774894616,
    "jti": "4ca99ae63f08a3a7bcea8786850494be",
    "scope": [
      "read:data:*"
    ],
    "task_id": "r1-task",
    "orch_id": "r1-orch"
  }
}

--- Step 6: Agent renews ---
{
  "expires_in": 300,
  "token_type": null
}
New token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 7: Validate renewed token ---
{
  "valid": true,
  "claims": {
    "iss": "agentauth",
    "sub": "spiffe://agentauth.local/agent/r1-orch/r1-task/dc1fd492298271c4",
    "aud": [
      "agentauth"
    ],
    "exp": 1774894917,
    "nbf": 1774894617,
    "iat": 1774894617,
    "jti": "56c670e16346e1efe4d1647222354886",
    "scope": [
      "read:data:*"
    ],
    "task_id": "r1-task",
    "orch_id": "r1-orch"
  }
}

## Verdict

PASS — Full lifecycle works: app registered, authenticated, created launch token, agent registered (TTL=300), app validated (valid=true), agent renewed (TTL=300, new JTI), renewed token validated (valid=true). B6 TTL changes did not break the standard flow.
