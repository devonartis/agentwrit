# SEC-A1 — TTL Carry-Forward on Renewal

## Infrastructure Prerequisites

| Prerequisite | Purpose | Smoke Test Story | Status |
|-------------|---------|-----------------|--------|
| Go 1.24+ compiled broker binary | VPS mode testing | Precondition (build) | NOT VERIFIED |
| Docker + docker-compose | Container mode testing | Precondition (stack_up) | NOT VERIFIED |

## Stories

### A1-S1: Admin Flow — Renewal Preserves the Original Token TTL [ACCEPTANCE]

The security reviewer verifies that when a token is renewed via the admin
flow, the new token gets the same TTL as the original — not the broker's
default. Admin creates launch token with max_ttl=120, agent registers,
agent renews. Renewed token must have expires_in=120.

**Route:** POST /v1/admin/launch-tokens, POST /v1/register, POST /v1/token/renew
**Persona:** Security Reviewer
**Mode:** VPS

### A1-S2: App Flow — Renewal Preserves TTL Through Production Path [ACCEPTANCE]

The security reviewer verifies the production flow end-to-end. Admin
registers app, app authenticates, app creates launch token with max_ttl=120,
agent registers, agent renews. This is the correct production path — apps
manage agents, not admin.

**Route:** POST /v1/admin/apps, POST /v1/app/auth, POST /v1/app/launch-tokens, POST /v1/register, POST /v1/token/renew
**Persona:** Security Reviewer
**Mode:** VPS

### A1-S3: App Cannot Create Launch Tokens via Admin Endpoint [ACCEPTANCE]

The security reviewer verifies the scope boundary between app and admin.
An app token (scope app:launch-tokens:*) must be rejected on the admin
endpoint (requires admin:launch-tokens:*). Same token must succeed on the
app endpoint. This proves the scope model is enforced.

**Route:** POST /v1/admin/launch-tokens, POST /v1/app/launch-tokens
**Persona:** Security Reviewer
**Mode:** VPS

### A1-R1: Standard Issue-Validate-Renew Flow Still Works [ACCEPTANCE]

Regression test. App creates launch token, agent registers, app validates
agent token, agent renews, renewed token validated. Full lifecycle must
work after B6 TTL changes.

**Route:** POST /v1/app/launch-tokens, POST /v1/register, POST /v1/token/validate, POST /v1/token/renew
**Persona:** App
**Mode:** VPS
