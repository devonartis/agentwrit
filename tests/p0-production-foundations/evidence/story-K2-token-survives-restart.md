# P0-K2 — Token Survives Broker Restart [ACCEPTANCE]

Who: The developer.

What: The developer gets an admin token from the broker, then the broker
restarts. After restart, the developer validates the same token. Before
this fix, every restart generated a new signing key, which meant every
token issued before the restart became invalid. Now the key is loaded
from disk, so the token's signature still checks out.

Why: If tokens don't survive restart, every broker restart invalidates
all active sessions — agents, apps, and admins all lose their tokens
simultaneously. That's an outage, not a restart.

How to run: Get an admin token, validate it, restart the broker, validate
the same token again.

Expected: Token validates successfully both before and after restart.

## Test Output

Step 1: Get admin token
Token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

Step 2: Validate before restart
{"valid":true,"claims":{"iss":"agentauth","sub":"admin","exp":1774791675,"nbf":1774791375,"iat":1774791375,"jti":"3ab1d750a8264af2c127ba26896e755c","scope":["admin:launch-tokens:*","admin:revoke:*","admin:audit:*"]}}


Step 3: Restart broker
 Container agentauth-core-broker-1 Restarting 
 Container agentauth-core-broker-1 Started 

Step 4: Wait for healthy
{"status":"ok","version":"2.0.0","uptime":3,"db_connected":true,"audit_events_count":1}


Step 5: Validate after restart
{"valid":true,"claims":{"iss":"agentauth","sub":"admin","exp":1774791675,"nbf":1774791375,"iat":1774791375,"jti":"3ab1d750a8264af2c127ba26896e755c","scope":["admin:launch-tokens:*","admin:revoke:*","admin:audit:*"]}}


## Verdict

PASS — Token validated successfully before restart (valid:true) and after restart (valid:true). The persistent signing key ensures tokens survive broker restarts. Same JTI (3ab1d750...) confirmed in both responses.
