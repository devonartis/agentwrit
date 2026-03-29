# P0-K4 — Token Renewal Works After Restart [ACCEPTANCE]

Who: The developer.

What: The developer has a long-lived agent that renews its token
periodically. After a broker restart, the renewal must still work because
the signing key is the same. The developer sends a renewal request with
a Bearer token that was issued before the restart.

Why: If renewal breaks after restart, every long-running agent needs manual
re-authentication after each broker restart — defeating the purpose of
token renewal entirely.

How to run: Get an admin token, restart the broker, renew the token using
the pre-restart Bearer token.

Expected: Renewal returns HTTP 200 with a new access_token and expires_in.

## Test Output

Step 1: Get token before restart
Token: eyJhbGciOiJFZERTQSIsInR5cCI6Ik...

Step 2-3: Restart and wait
 Container agentauth-core-broker-1 Restarting 
 Container agentauth-core-broker-1 Started 
{"status":"ok","version":"2.0.0","uptime":3,"db_connected":true,"audit_events_count":2}


Step 4: Renew with pre-restart token
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImV4cCI6MTc3NDc5MTcxNCwibmJmIjoxNzc0NzkxNDE0LCJpYXQiOjE3NzQ3OTE0MTQsImp0aSI6Ijg1MTU4NzI4ZjlkMDc0ODcyNWU3ZGU4NzlkNGY1N2QxIiwic2NvcGUiOlsiYWRtaW46bGF1bmNoLXRva2VuczoqIiwiYWRtaW46cmV2b2tlOioiLCJhZG1pbjphdWRpdDoqIl19.XQb_eDUZK8p-o0R4U4TxZ2Sh1UMh5zeiEGEtEds0Rtw71bUC_OzfMKNdmNpEDpgKAESHlSLqHwy4lEIF5oWaAA","expires_in":300}

HTTP 200

## Verdict

PASS — Renewal returned HTTP 200 with a fresh access_token and expires_in:300 after broker restart. The persistent signing key allows pre-restart tokens to be renewed without re-authentication.
