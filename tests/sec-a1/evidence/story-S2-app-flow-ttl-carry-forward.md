# A1-S2 — App Creates Agent, Renewal Preserves TTL [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer, verifying the production flow end-to-end.

What: In production, apps — not admins — manage agents. An app registers
with the broker, authenticates, creates launch tokens for its agents, and
those agents register using the launch token. The app sets a max TTL of
120 seconds. After the agent registers and renews, the renewed token must
still be 120 seconds — not the broker's 300-second default.

Why: The admin flow (S1) bypasses scope ceiling enforcement. The app flow
is where real security enforcement happens. If TTL carry-forward only
works via admin but breaks via app, the fix is incomplete.

How to run: Admin registers an app. App authenticates, creates launch
token, agent registers, agent renews. Compare TTLs.

Expected: Both original and renewed token have expires_in=120.

## Test Output

--- Step 1: Admin authenticates ---
Admin token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 2: Admin registers app ---
{
  "app_id": "app-ttl-s2-app-b772e4",
  "name": "",
  "client_id": "tsa-b52870bdd197",
  "client_secret": "9d292bb9b72809abbde022b7a779418beabac853f4ddd404da9df726e9c17931",
  "scopes": [
    "read:data:*"
  ],
  "token_ttl": 1800,
  "status": ""
}

--- Step 3: App authenticates ---
{
  "expires_in": 1800,
  "token_type": "Bearer"
}
App token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

--- Step 4: App creates launch token ---
{
  "launch_token": "865d970765971b5034f4c3f4c1c7179755dc5cb4d9f562cc72f9356e9496ac91",
  "expires_at": "2026-03-30T18:16:28Z",
  "policy": {
    "allowed_scope": [
      "read:data:*"
    ],
    "max_ttl": 120
  }
}

--- Step 5: Agent registers ---
{
  "agent_id": "spiffe://agentauth.local/agent/s2-orch/s2-task/19d3e9a9e14281ab",
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6ImltNDBHRzAwbXM2R2NWZnBpdUttSmJoZDF5MUJUNEpHTEczQVhKOVRXWUkifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvczItb3JjaC9zMi10YXNrLzE5ZDNlOWE5ZTE0MjgxYWIiLCJhdWQiOlsiYWdlbnRhdXRoIl0sImV4cCI6MTc3NDg5NDY3OCwibmJmIjoxNzc0ODk0NTU4LCJpYXQiOjE3NzQ4OTQ1NTgsImp0aSI6IjBhNTMwOTE3Yjc3NTdhM2ZkMGE4MDZhYjc1MTg0YzhkIiwic2NvcGUiOlsicmVhZDpkYXRhOioiXSwidGFza19pZCI6InMyLXRhc2siLCJvcmNoX2lkIjoiczItb3JjaCJ9.rtVBkx_b1g3h30v1Tb01u0nQAkGgCytuVz112UC_OPnn-kd3iv628646xhk34-VXfVjg98fAT_2Hu9q3lHViBg",
  "expires_in": 120
}
Original TTL: 120

--- Step 6: Agent renews ---
{
  "access_token": "eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6ImltNDBHRzAwbXM2R2NWZnBpdUttSmJoZDF5MUJUNEpHTEczQVhKOVRXWUkifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQvczItb3JjaC9zMi10YXNrLzE5ZDNlOWE5ZTE0MjgxYWIiLCJhdWQiOlsiYWdlbnRhdXRoIl0sImV4cCI6MTc3NDg5NDY3OSwibmJmIjoxNzc0ODk0NTU5LCJpYXQiOjE3NzQ4OTQ1NTksImp0aSI6ImVmZTQ1YzZmOTNiMWJiMWM2ZTM5ZWM3OGM0NTRiZmVmIiwic2NvcGUiOlsicmVhZDpkYXRhOioiXSwidGFza19pZCI6InMyLXRhc2siLCJvcmNoX2lkIjoiczItb3JjaCJ9.trDcJi1l-Nn0t82J_IoiUqxG5_8dUyyNEqkXFhj5QYL2mAy5xjd_JbVewuvfP3eIepKACcVKIVVmKuxWO6pmAg",
  "expires_in": 120
}
Renewed TTL: 120

--- Compare ---
Original: 120s, Renewed: 120s, Default: 300s

## Verdict

PASS — Full app flow: admin registered app, app authenticated, app created launch token with max_ttl=120, agent registered (expires_in=120), agent renewed (expires_in=120). TTL carry-forward works through the production app flow, not just the admin shortcut.
