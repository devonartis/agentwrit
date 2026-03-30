# L2b-S4 — Every Response Includes Security Headers [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer. They are checking that the broker sets
protective HTTP headers on every response, not just on specific endpoints.

What: Modern browsers and HTTP clients use security headers to protect
against common attacks. The broker now adds three headers to every single
response it sends, regardless of which endpoint was called:

- X-Content-Type-Options: nosniff — prevents browsers from guessing the
  content type, which stops a class of attacks where a malicious file is
  served with the wrong type.
- X-Frame-Options: DENY — prevents the broker's responses from being
  embedded in an iframe, which stops clickjacking attacks.
- Cache-Control: no-store — prevents sensitive responses (like tokens)
  from being cached by browsers or proxies.

Why: Without these headers, a browser or proxy could cache tokens, frame
the broker UI for clickjacking, or misinterpret response types. These are
standard security best practices — OWASP recommends all three.

How to run: The reviewer checks the response headers on three different
endpoints: health (public), metrics (public), and token validate (POST).
All three must include all three security headers.

Expected: All three headers present on every endpoint.

## Test Output

--- /v1/health ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY

--- /v1/metrics ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY

--- /v1/token/validate (POST) ---
Cache-Control: no-store
X-Content-Type-Options: nosniff
X-Frame-Options: DENY

## Verdict

PASS — All three security headers (X-Content-Type-Options: nosniff, X-Frame-Options: DENY, Cache-Control: no-store) are present on every endpoint tested: /v1/health, /v1/metrics, and /v1/token/validate. The SecurityHeaders middleware is working globally — it does not matter which endpoint is called.
