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

--- Step 1: Admin authenticates ---
Admin token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 2: Create launch token and register agent ---
Agent ID: spiffe://agentauth.local/agent/s2-orch/s2-task/d72568a5b27b3fab
Agent token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 3: Operator revokes the agent ---
{
  "revoked": true,
  "level": "agent",
  "target": "spiffe://agentauth.local/agent/s2-orch/s2-task/d72568a5b27b3fab",
  "count": 1
}

--- Step 4: App validates the revoked token ---
{
  "valid": false,
  "error": "token is invalid or expired"
}

## Verdict

PASS — The operator revoked agent spiffe://agentauth.local/agent/s2-orch/s2-task/d72568a5b27b3fab. When the app then validated the revoked token, the broker returned the generic message "token is invalid or expired" — identical to what it returns for any other bad token. The response does NOT say "token has been revoked". An attacker cannot tell whether the token was ever valid.
