# TD-006: Per-App JWT TTL — User Stories

## Personas and Tools

| Persona | Tool | Stories |
|---------|------|---------|
| Operator | `aactl` | 1–4 |
| Developer | `curl` | 5 |
| Security reviewer | Both | 6–7 |

---

## Operator Stories

### S1 — Operator registers app with default TTL

**As an operator**, I want to register an app without specifying a TTL and have it get the global default (30 min).

**Precondition:** Broker running with default config (no AA_APP_TOKEN_TTL set).

**Acceptance criteria:**
- `aactl app register --name ttl-default --scopes "read:data:*"` returns 201
- Response shows `token_ttl: 1800`
- `aactl app get <app_id>` confirms `token_ttl: 1800`

### S2 — Operator registers app with custom TTL

**As an operator**, I want to specify a TTL when registering an app so that long-running apps get longer tokens.

**Acceptance criteria:**
- `aactl app register --name ttl-custom --scopes "read:data:*" --token-ttl 3600` returns 201
- Response shows `token_ttl: 3600`
- Developer authenticates with this app's credentials → JWT `exp` is ~3600s from `iat`

### S3 — Operator updates existing app's TTL

**As an operator**, I want to change an app's TTL without re-registering it.

**Acceptance criteria:**
- `aactl app update --id <app_id> --token-ttl 7200` succeeds
- `aactl app get <app_id>` shows `token_ttl: 7200`
- Next developer auth uses new TTL
- Audit trail shows `app_updated` event with old and new TTL values

### S4 — Operator is rejected for out-of-bounds TTL

**As an operator**, I want the broker to reject TTL values outside safe bounds.

**Acceptance criteria:**
- `aactl app register --name ttl-low --scopes "read:data:*" --token-ttl 30` → error (< 60s minimum)
- `aactl app register --name ttl-high --scopes "read:data:*" --token-ttl 100000` → error (> 86400s maximum)
- `aactl app update --id <app_id> --token-ttl 5` → error (< 60s minimum)

## Developer Stories

### S5 — Developer gets a token that lasts long enough

**As a developer**, I want my app JWT to reflect the configured TTL.

**Precondition:** App registered with `--token-ttl 3600`.

**Acceptance criteria:**
- `POST /v1/app/auth` with valid credentials returns 200
- Response `expires_in` field is 3600
- JWT `exp` claim is approximately `iat + 3600`

## Security Stories

### S6 — TTL bounds prevent misconfiguration

**As a security reviewer**, I want TTL bounded between 60s and 86400s.

**Acceptance criteria:**
- API rejects `token_ttl: 0` at registration (if explicitly provided; omitted=default is ok)
- API rejects `token_ttl: -1`
- API rejects `token_ttl: 86401`
- API accepts `token_ttl: 60` (minimum)
- API accepts `token_ttl: 86400` (maximum)

### S7 — TTL changes are audited

**As a security reviewer**, I want TTL changes recorded in audit.

**Acceptance criteria:**
- After `aactl app update --token-ttl`, `aactl audit events --event-type app_updated` shows the change
- Audit detail includes old TTL and new TTL values
