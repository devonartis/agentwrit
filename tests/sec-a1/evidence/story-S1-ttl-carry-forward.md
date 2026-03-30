# A1-S1 — Renewal Preserves the Original Token TTL [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer. They are verifying that a critical security
fix works: when an agent's token is renewed, the new token must have the
same lifetime as the original.

What: Before this fix, when an agent renewed its token, the broker gave
the new token the default lifetime (300 seconds) instead of the original
lifetime. This meant an agent that was given a short-lived 120-second
token could renew it and get a 300-second token — nearly tripling its
access window. Now the broker remembers how long the original token was
supposed to last and gives the renewed token the same lifetime.

Why: If an agent can extend its token's lifetime through renewal, it
defeats the purpose of short-lived tokens. A security team that issues
120-second tokens expects those tokens to stay 120 seconds — even after
renewal. Without this fix, an attacker who compromises an agent can
keep renewing to get longer and longer access windows.

How to run: We use the admin to create a launch token with a specific
short TTL (120 seconds), register an agent with it, then renew the
agent's token and check whether the new token has the same TTL.

Expected: The renewed token's expires_in should be 120 (same as original),
NOT 300 (the broker default).

## Test Output

--- Step 1: Admin authenticates ---
Admin token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 2: Create launch token with TTL=120 ---
{
  "launch_token": "1cbabe95fd7395d06a0db6f545e2ae50d59a977493f0c986ed25c4099b5413d4",
  "expires_at": "2026-03-30T17:56:22Z",
  "policy": {
    "allowed_scope": [
      "read:data:*"
    ],
    "max_ttl": 120
  }
}

--- Step 3: Register agent ---
{
  "agent_id": "spiffe://agentauth.local/agent/s1-orch/s1-task/a318a4ed83e85562",
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6ImltNDBHRzAwbXM2R2NWZnBpdUttSmJoZDF5MUJUNEpHTEczQVhKOVRXWUkifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvczEtb3JjaC9zMS10YXNrL2EzMThhNGVkODNlODU1NjIiLCJhdWQiOlsiYWdlbnRhdXRoIl0sImV4cCI6MTc3NDg5MzQ3MywibmJmIjoxNzc0ODkzMzUzLCJpYXQiOjE3NzQ4OTMzNTMsImp0aSI6ImY1ZmMzMGNlZWIzZDAzMTAxY2NhNjQwMGJkZDJkYmZkIiwic2NvcGUiOlsicmVhZDpkYXRhOioiXSwidGFza19pZCI6InMxLXRhc2siLCJvcmNoX2lkIjoiczEtb3JjaCJ9.Jz1IcDhDUVf6wCjxGZ4n0F_lPN5RdDqNKgNtyb50DHPLGYjemP5pGY6d8yuXnP_8tsygKbz1MkajSm56gsJXCA",
  "expires_in": 120
}
Original TTL: 120

--- Step 4: Renew the token ---
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6ImltNDBHRzAwbXM2R2NWZnBpdUttSmJoZDF5MUJUNEpHTEczQVhKOVRXWUkifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvczEtb3JjaC9zMS10YXNrL2EzMThhNGVkODNlODU1NjIiLCJhdWQiOlsiYWdlbnRhdXRoIl0sImV4cCI6MTc3NDg5MzQ3MywibmJmIjoxNzc0ODkzMzUzLCJpYXQiOjE3NzQ4OTMzNTMsImp0aSI6ImMxYjMzOTQ3ZTUzYzE0MjkwOWQ1ZmZjNTQyNTlkMTkzIiwic2NvcGUiOlsicmVhZDpkYXRhOioiXSwidGFza19pZCI6InMxLXRhc2siLCJvcmNoX2lkIjoiczEtb3JjaCJ9.AKM72PzN9iktzTxvdcnf-Ui87_w-4A3PmiWJIbPq8BYqQN-QFlMbnz0mSRlyDL20OVDb6eZFNFVNddYChQceAA",
  "expires_in": 120
}
Renewed TTL: 120

--- Step 5: Compare ---
Original: 120s, Renewed: 120s, Default: 300s

## Verdict

PASS — Agent issued with TTL=120 via admin launch token. After renewal, expires_in=120 (not 300 default). SEC-A1 TTL carry-forward works. Note: this used the admin flow — see S2 for the correct production app flow.
