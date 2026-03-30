# L2b-S6 — Broker Rejects Oversized Requests Before Processing Them [ACCEPTANCE]

**Mode:** VPS

Who: The security reviewer. They are verifying that the broker has a global
limit on how much data a client can send in a single request.

What: The broker now enforces a 1 MB limit on all incoming request bodies,
on every endpoint. Before this fix, each endpoint had to remember to add
its own body limit — if one was missed, an attacker could send a massive
request and consume memory or CPU. Now the limit is global: no matter which
endpoint an attacker targets, the broker cuts them off at 1 MB.

Why: Without a body limit, an attacker can send a 1 GB JSON payload to any
endpoint and the broker will try to read and parse it all. This is a denial
of service attack — it exhausts the broker's memory and makes it unavailable
to legitimate agents and apps. A 1 MB global limit stops this class of
attack entirely.

How to run: We send a request body that is just over 1 MB (1,048,577 bytes)
to the validate endpoint. The broker should reject it before trying to
parse the JSON.

Expected: HTTP 413 (Request Entity Too Large). The broker should not crash,
hang, or return a 500.

## Test Output

Sending 1 MB + 1 byte payload to /v1/token/validate...
HTTP status: 413

## Verdict

PASS — The broker returned HTTP 413 (Request Entity Too Large) when sent a 1 MB + 1 byte payload. It did not crash, hang, or return a 500. The global body limit is working — any endpoint is protected against oversized requests regardless of whether the endpoint handler remembers to check.
