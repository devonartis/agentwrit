
═══════════════════════════════════════════════════════════════
  SEC-L2b Acceptance + Regression Tests (agentauth-core)
═══════════════════════════════════════════════════════════════
  Broker: http://127.0.0.1:8080
  Date:   2026-03-30T16:14:43Z

── Setup: Admin auth + Agent registration ──
  Admin token: eyJhbGciOiJFZERTQSIs...
  Launch token: d520da45ea930a25d077...
  Nonce: 59df5e4fdf3d42b7386c...
  Agent token:  eyJhbGciOiJFZERTQSIs...
  Agent ID:     spiffe://agentauth.local/agent/l2b-orch/l2b-task/c7e5cb488033687d

═══════════════════════════════════════════════════════════════
  B5 (SEC-L2b) Acceptance Stories
═══════════════════════════════════════════════════════════════

── S1: Validate generic error on invalid token (H3) ──
  Response: {"valid":false,"error":"token is invalid or expired"}
  ✅ PASS: S1: Generic error message returned

── S2: Validate generic error on revoked token (H3) ──
  Revoking agent: spiffe://agentauth.local/agent/l2b-rev-orch/l2b-rev-task/54de1b3bdb41b6d2
  Revoke response: {"revoked":true,"level":"agent","target":"spiffe://agentauth.local/agent/l2b-rev-orch/l2b-rev-task/54de1b3bdb41b6d2","count":1}
  Validate revoked: {"valid":false,"error":"token is invalid or expired"}
  ✅ PASS: S2: Revoked token returns generic error

── S3: Renew rejects tampered token without leaking details (H4) ──
  HTTP status: 401
  Body: {"type":"urn:agentauth:error:unauthorized","title":"Unauthorized","status":401,"detail":"token verification failed","instance":"/v1/token/renew","error_code":"unauthorized","request_id":"06c30e708ca100d3"}
  ✅ PASS: S3: Tampered token rejected with generic error

── S4: Security headers on all responses (H1) ──
  --- /v1/health ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
  --- /v1/metrics ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
  --- /v1/token/validate ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
  ✅ PASS: S4: Security headers present on all endpoints

── S5: HSTS present when TLS enabled (H1) ──
  ⏭  SKIP: S5: Requires TLS cert — not available in Docker test mode

── S6: Oversized body returns 413 (H7) ──
  HTTP status: 413
  ✅ PASS: S6: Oversized body returns 413

═══════════════════════════════════════════════════════════════
  Regression Tests (B0-B4)
═══════════════════════════════════════════════════════════════

── R1: Admin auth works (B2) ──
  Already tested in setup — admin token obtained.
  ✅ PASS: R1: Admin auth works with hashed secret

── R2: Agent registration works (B0) ──
  Already tested in setup — agent registered via launch token.
  ✅ PASS: R2: Agent registration works

── R3: Token renewal works (B1) ──
  Renew response: {"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCIsImtpZCI6IkdkcWJJbGk1MG00TnRrUWcyWERPRnBuOUdpYkdDbjdzSTY2Q202SW82ZkkifQ.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJzcGlmZmU6Ly9hZ2VudGF1dGgubG9jYWwvYWdlbnQv...
  ✅ PASS: R3: Token renewal works

── R4: Token revocation + validate (B4) ──
  ✅ PASS: R4: Revoked token correctly rejected

═══════════════════════════════════════════════════════════════
  Results
═══════════════════════════════════════════════════════════════
  PASS: 9
  FAIL: 0
  SKIP: 1
  TOTAL: 10

  ✅ ALL TESTS PASSED
