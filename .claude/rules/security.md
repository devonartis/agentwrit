# Security — Rules of Engagement

- Never leak internal state in error responses. No stack traces, no file paths, no internal identifiers.
- Never log secrets. No client_secret, no API keys, no tokens in audit records or log output.
- All crypto uses stdlib. No third-party crypto libraries unless explicitly approved.
- Secrets must be required at startup, not optional. Fail fast on missing secrets.
- Weak secrets are rejected. Empty secrets are rejected. No silent defaults.
- Tokens must expire. No indefinite tokens. Reject explicit TTL of 0 or negative.
- Constant-time comparison for all secret/token comparisons.
- Sanitize all error messages before returning to clients.
- Security-sensitive events (auth failures, scope violations, revocations) get audit entries.
- Request body size limits on all endpoints. No unbounded reads.
- Security headers on all HTTP responses.
- No hardcoded secrets outside of test fixtures.
- No `init()` functions — explicit initialization prevents hidden security setup.
