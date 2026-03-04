# P0-S8 — Old Admin Login Format Is Rejected With a Helpful Error

Who: A developer using the old admin login format.

What: The developer sends the old admin login format — client_id and client_secret fields — directly to the broker's admin auth endpoint. Before Phase 0, the broker accepted this shape. Now it should reject it with a clear error message that tells the caller exactly what changed and what format to use instead.

Why: Developers or scripts using the old format shouldn't get a cryptic "unauthorized" error. They should get a helpful message that tells them how to fix their code. This prevents wasted debugging time.

How to run: Source the environment file. Then send a POST to /v1/admin/auth with the old format (client_id and client_secret fields). The broker should return a 400 error with guidance about the new format.

Expected: HTTP 400 with a message telling the caller to use {"secret": "..."} instead.

## Test Output

{"type":"urn:agentauth:error:invalid_request","title":"Bad Request","status":400,"detail":"Admin auth format changed. Use {\"secret\": \"...\"} instead of client_id/client_secret","instance":"/v1/admin/auth","error_code":"invalid_request","request_id":"ce30616028d719a9"}

HTTP 400

## Verdict

PASS — The broker returned 400 with a clear migration message: "Admin auth format changed. Use {\"secret\": \"...\"} instead of client_id/client_secret"
