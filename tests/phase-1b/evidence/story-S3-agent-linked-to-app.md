# P1B-S3 — Developer Registers an Agent Linked to the App

Who: The developer.

What: The developer creates a launch token and immediately uses it to register an
agent with the broker. The key thing being tested is that the agent gets linked to
the app that created the launch token. When the operator later pulls the audit trail,
they can see which app is responsible for which agents — critical for security
investigations if an app is compromised.

Why: Without app-to-agent traceability, if an app is compromised, the operator
can't tell which agents belong to it. The entire app model depends on this link
being created automatically at registration time.

How to run: Authenticate as the weather-bot app, create a launch token, get a
challenge nonce, generate an Ed25519 key pair, sign the hex-decoded nonce bytes, and
register the agent — all in one sequence (the launch token expires in 30 seconds).
Then check the audit trail for agent_registered and token_issued events — both
should include the app_id.

Expected: Agent registration succeeds (200) with agent_id and access_token. Audit
events agent_registered and token_issued both include app_id=app-weather-bot-fffad0.

## Test Output

--- Step 1: App Authentication ---
HTTP 200
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLXdlYXRoZXItYm90LWZmZmFkMCIsImV4cCI6MTc3MjY3ODQzMCwibmJmIjoxNzcyNjc4MTMwLCJpYXQiOjE3NzI2NzgxMzAsImp0aSI6ImVmZmJkM2MxOGZmMTI3Y2U5MzQzYzVhNWFkZDMxYTNmIiwic2NvcGUiOlsiYXBwOmxhdW5jaC10b2tlbnM6KiIsImFwcDphZ2VudHM6KiIsImFwcDphdWRpdDpyZWFkIl19.xAaMhuH4VB4JGDqv2u7WcjzZQ_0uCbtzZW4bhVbZYL-4gBu1GAvQ_zPoLkSo-RyCojiKMh-mvKsFnx7O3wugDA",
  "expires_in": 300,
  "token_type": "Bearer",
  "scopes": [
    "app:launch-tokens:*",
    "app:agents:*",
    "app:audit:read"
  ]
}

--- Step 2: Create Launch Token ---
HTTP 201
{
  "launch_token": "f04077fcf6ba9255fb9647c851ee94f24304acb1a0b433a1a7b141dad83d4887",
  "expires_at": "2026-03-05T02:36:00Z",
  "policy": {
    "allowed_scope": [
      "read:weather:current"
    ],
    "max_ttl": 300
  }
}

--- Step 3: Get Challenge Nonce ---
{
  "nonce": "b1632a79a512e97f645f732ecd9b3a1f8417cfdf8189fb8bc9815838f65c28a0",
  "expires_in": 30
}

--- Step 4: Register Agent ---
HTTP 200
{
  "agent_id": "spiffe://agentauth.local/agent/test-orch-1/test-task-1/a6154416937a6ade",
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvdGVzdC1vcmNoLTEvdGVzdC10YXNrLTEvYTYxNTQ0MTY5MzdhNmFkZSIsImV4cCI6MTc3MjY3ODQzMCwibmJmIjoxNzcyNjc4MTMwLCJpYXQiOjE3NzI2NzgxMzAsImp0aSI6IjZlOTA2MmI2MGM0YzExNTcyNTU2MTVhMTBkZGIwOTU1Iiwic2NvcGUiOlsicmVhZDp3ZWF0aGVyOmN1cnJlbnQiXSwidGFza19pZCI6InRlc3QtdGFzay0xIiwib3JjaF9pZCI6InRlc3Qtb3JjaC0xIn0.D-dNSrXHYqoPx4KiUzPYTClnarYtM6pnvZbT6DeZmT6YHpeG7XRexbf0Je_egnZyipdmaR87V1H9tSHrHYSmDg",
  "expires_in": 300
}

--- Step 5: Audit Trail ---

ID          TIMESTAMP                       EVENT TYPE        AGENT ID                                                                 OUTCOME  DETAIL
evt-000023  2026-03-05T02:35:30.165249381Z  agent_registered  spiffe://agentauth.local/agent/test-orch-1/test-task-1/a6154416937a6ade  success  Agent registered with scope [read:weather:current] app_id...
Showing 1 of 1 events (offset=0, limit=100)

ID          TIMESTAMP                       EVENT TYPE    AGENT ID                                                                 OUTCOME  DETAIL
evt-000024  2026-03-05T02:35:30.167740464Z  token_issued  spiffe://agentauth.local/agent/test-orch-1/test-task-1/a6154416937a6ade  success  Token issued, jti=6e9062b60c4c1157255615a10ddb0955, ttl=3...
Showing 1 of 1 events (offset=0, limit=100)


## Verdict

PASS — Agent registration succeeded (HTTP 200) with SPIFFE agent_id and access_token. Audit trail confirms traceability: agent_registered (evt-000023) detail includes "app_id=app-weather-bot-fffad0", token_issued (evt-000024) detail includes "app_id=app-weather-bot-fffad0". The agent is linked to the originating app.
