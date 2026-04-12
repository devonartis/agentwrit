# awrit CLI Reference

The operator command-line tool for managing tokens, apps, revocation, and audit trails.

---

## Overview

**awrit** is the operator CLI for the AgentWrit broker. It provides full control over token lifecycle, revocation, app management, and audit trail inspection via a set of commands backed by the broker's admin API.

awrit uses the Cobra command framework and outputs formatted tables by default, with optional JSON output for scripting.

### Installation

Build awrit from source:

```bash
cd /path/to/agentwrit
go build -o awrit ./cmd/awrit
```

Or use the pre-built binary if already available in your AgentWrit distribution.

### Quick Start

Set the required environment variables:

```bash
export AACTL_BROKER_URL="http://localhost:8080"
export AACTL_ADMIN_SECRET="your-admin-secret-here"
```

Run a command:

```bash
awrit audit events
```

Output defaults to formatted tables. Add `--json` to any command for raw JSON:

```bash
awrit audit events --json
```

---

## Environment Configuration

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AACTL_BROKER_URL` | Broker base URL (scheme + host + optional port) | `http://localhost:8080` or `https://broker.example.com:9443` |
| `AACTL_ADMIN_SECRET` | Admin secret for authentication | See broker deployment docs for secret rotation |

### Authentication Flow

1. awrit reads `AACTL_ADMIN_SECRET` from the environment
2. On first command invocation, it sends a POST to `/v1/admin/auth` with `{"secret": "..."}`
3. The broker returns a short-lived JWT (`access_token`)
4. awrit caches the token for the session and uses Bearer auth for all subsequent requests
5. Token is automatically refreshed if it expires during command execution (transparent to the user)

### Security Notes

- **Never commit secrets to version control.** Use environment variable files, secret managers, or CI/CD secret stores.
- The admin secret grants full broker control. Restrict access to operators only.
- Tokens are cached in memory for the current shell session only; they are not persisted to disk.
- Token expiry is handled transparently; no manual refresh is required.

---

## Global Flags

All commands support the following global flag:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output raw JSON instead of formatted table. Useful for scripting and integration with other tools. |

Example:
```bash
awrit --json audit events
```

---

## Commands

### audit events

**Synopsis:**
```
awrit audit events [flags]
```

**Description:**

Query the broker's audit trail. Events record all meaningful actions: token issuance, revocation, scope changes, and administrative operations. Use filters to narrow results for investigation, compliance, or monitoring.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--agent-id` | string | (empty) | Filter by agent SPIFFE ID. Shows all events for a specific agent. |
| `--task-id` | string | (empty) | Filter by task ID. Shows all events for a specific delegated task. |
| `--event-type` | string | (empty) | Filter by event type (e.g., `token_issued`, `token_revoked`, `scope_narrowed`, `agent_denied`). |
| `--since` | string | (empty) | Events after this time in RFC3339 format (e.g., `2026-02-27T10:30:00Z`). |
| `--until` | string | (empty) | Events before this time in RFC3339 format. |
| `--limit` | int | 100 | Maximum number of results to return. Use for pagination. |
| `--offset` | int | 0 | Pagination offset. Skip first N results. |
| `--outcome` | string | (empty) | Filter by outcome: `success` or `denied`. Useful for finding failed operations or abuse attempts. |

**Output (Table):**

| Column | Description |
|--------|-------------|
| ID | Unique event identifier (UUID) |
| TIMESTAMP | RFC3339 timestamp when the event occurred |
| EVENT TYPE | Type of event (token_issued, token_revoked, etc.) |
| AGENT ID | SPIFFE ID of the agent involved (or empty for broker operations) |
| OUTCOME | success or denied |
| DETAIL | Additional context, truncated to 60 characters for readability |

**Output (JSON):**

```json
{
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "timestamp": "2026-02-27T15:32:10Z",
      "event_type": "token_issued",
      "agent_id": "spiffe://example.com/agent/crawler",
      "outcome": "success",
      "detail": "Token issued for scope admin:read"
    }
  ],
  "total": 42,
  "offset": 0,
  "limit": 100
}
```

**Examples:**

List all events (last 100):
```bash
awrit audit events
```

Find all token revocation events in the last hour:
```bash
awrit audit events --event-type token_revoked --since 2026-02-27T14:32:10Z
```

Show all failed authorization attempts:
```bash
awrit audit events --outcome denied
```

Audit a specific agent (with JSON output for parsing):
```bash
awrit audit events --agent-id spiffe://example.com/agent/crawler --json
```

Paginate through 1000 events in batches of 100:
```bash
awrit audit events --limit 100 --offset 0
awrit audit events --limit 100 --offset 100
```

**API Endpoint:**

`GET /v1/audit/events?agent_id=...&task_id=...&event_type=...&since=...&until=...&limit=...&offset=...&outcome=...`

---

### token release

**Synopsis:**
```
awrit token release --token <jwt>
```

**Description:**

Release (self-revoke) an agent token. The token being released must be provided via the `--token` flag. This endpoint allows agents to revoke their own tokens or operators to test the release flow.

When a token is released, it is added to the revocation list. Subsequent requests with that token are rejected (HTTP 403).

Calling this endpoint multiple times with the same token is idempotent: the second and subsequent calls succeed with no additional side effects.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--token` | string | (required) | The agent JWT to release. Must be a valid JWT; invalid tokens are rejected. |

**Output (Success):**

```
Token released successfully.
```

Or if already released:

```
Token already released (revoked).
```

**Output (JSON):**

JSON output mode is not applicable to this command. The response is a simple HTTP 204 No Content on success. Both fresh releases and already-revoked tokens exit with status 0.

**Examples:**

Release a token (stored in an environment variable):
```bash
AGENT_TOKEN="eyJhbGciOiJFZDI1NTE5IiwidHlwIjoiSldUIn0.eyJzY29wZXMi..."
awrit token release --token "$AGENT_TOKEN"
```

Release a token hardcoded in a script (not recommended for production):
```bash
awrit token release --token "eyJhbGciOiJFZDI1NTE5IiwidHlwIjoiSldUIn0...."
```

**API Endpoint:**

`POST /v1/token/release`

**Authentication:**

This endpoint uses the token being released as its credential (Bearer auth). Unlike other awrit commands, it does not use the admin secret. This allows agents to revoke their own tokens or operators to test revocation with an agent's token.

**Status Codes:**

- `204 No Content` — Token released successfully (or already revoked)
- `400 Bad Request` — Invalid or malformed token
- `403 Forbidden` — Token has been revoked (when trying to use a revoked token for authentication)

---

### revoke

**Synopsis:**
```
awrit revoke --level <level> --target <target>
```

**Description:**

Revoke tokens at various granularity levels. Supports four revocation scopes:

- **token** — Revoke a specific token by its JTI (JWT ID)
- **agent** — Revoke all tokens issued to an agent (by SPIFFE ID)
- **task** — Revoke all tokens for a specific task (by task ID)
- **chain** — Revoke all tokens in a delegation chain rooted at a given agent

Revocation is idempotent: revoking already-revoked tokens or targets returns success.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--level` | string | (required) | Revocation granularity: `token`, `agent`, `task`, or `chain`. |
| `--target` | string | (required) | Identifier based on level: JTI (token), SPIFFE ID (agent/chain), or task ID (task). |

**Output (Table):**

| Column | Description |
|--------|-------------|
| REVOKED | true if any tokens were revoked; false if none were found |
| LEVEL | Revocation level used |
| TARGET | The target identifier that was revoked |
| COUNT | Number of tokens revoked |

**Output (JSON):**

```json
{
  "revoked": true,
  "level": "agent",
  "target": "spiffe://example.com/agent/crawler",
  "count": 3
}
```

**Examples:**

Revoke a specific token by JTI:
```bash
awrit revoke --level token --target "jti-abc123"
```

Revoke all tokens issued to an agent (e.g., after compromise):
```bash
awrit revoke --level agent --target "spiffe://example.com/agent/crawler"
```

Revoke all tokens for a specific delegated task:
```bash
awrit revoke --level task --target "task-456"
```

Revoke an entire delegation chain (agent + all delegates):
```bash
awrit revoke --level chain --target "spiffe://example.com/agent/root-agent"
```

Revoke and check the result in JSON:
```bash
awrit revoke --level agent --target "spiffe://example.com/agent/crawler" --json
```

**API Endpoint:**

`POST /v1/revoke`

**Request Body:**

```json
{
  "level": "agent",
  "target": "spiffe://example.com/agent/crawler"
}
```

**Status Codes:**

- `200 OK` — Revocation processed (may have revoked 0 or more tokens)
- `400 Bad Request` — Invalid level or malformed target
- `401 Unauthorized` — Admin secret missing or invalid

---

### app register

**Synopsis:**
```
awrit app register --name <name> --scopes <scope1>,<scope2>,... [--token-ttl N]
```

**Description:**

Register a new app and receive generated credentials (client_id and client_secret). The app can then authenticate via the `/v1/app/auth` endpoint to obtain access tokens for agents.

The scope ceiling defines the maximum set of scopes that app's agents can request. The client_secret is shown only once and cannot be retrieved later — save it immediately.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | (required) | Human-readable app name for identification |
| `--scopes` | string | (required) | Comma-separated scope ceiling (e.g., `admin:read,admin:write,read`) |
| `--token-ttl` | int | (global AA_APP_TOKEN_TTL) | App JWT TTL in seconds (overrides global default) |

**Output (Table):**

| Column | Description |
|--------|-------------|
| APP_ID | Unique app identifier |
| CLIENT_ID | Client identifier for app authentication |
| CLIENT_SECRET | Secret credential (shown once only) |
| SCOPES | Scope ceiling |
| TOKEN_TTL | App JWT TTL in seconds |

**Output (JSON):**

```json
{
  "app_id": "app-12345",
  "client_id": "app_crawler",
  "client_secret": "secret_abc123def456...",
  "scopes": ["admin:read", "read"],
  "token_ttl": 1800
}
```

**Examples:**

Register an app with read-only scopes:
```bash
awrit app register --name "crawler" --scopes read
```

Register an app with admin scopes and custom TTL:
```bash
awrit app register --name "admin-tool" --scopes "admin:read,admin:write,read" --token-ttl 3600
```

Get the result as JSON:
```bash
awrit app register --name "webhook" --scopes "write" --json
```

**API Endpoint:**

`POST /v1/admin/apps`

**Request Body:**

```json
{
  "name": "crawler",
  "scopes": ["admin:read", "read"],
  "token_ttl": 1800
}
```

---

### app list

**Synopsis:**
```
awrit app list
```

**Description:**

List all registered apps. Shows app name, ID, client ID, scope ceiling, TTL, status, and creation time.

**Flags:**

None specific to this command. Supports global `--json` flag.

**Output (Table):**

| Column | Description |
|--------|-------------|
| NAME | App name |
| APP_ID | Unique app identifier |
| CLIENT_ID | Client identifier |
| STATUS | Operational status (active, inactive) |
| SCOPES | Scope ceiling (comma-separated) |
| TOKEN_TTL | App JWT TTL in seconds |
| CREATED | RFC3339 timestamp of registration |

**Output (JSON):**

```json
{
  "apps": [
    {
      "app_id": "app-12345",
      "name": "crawler",
      "client_id": "app_crawler",
      "scopes": ["admin:read", "read"],
      "token_ttl": 1800,
      "status": "active",
      "created_at": "2026-02-20T08:15:00Z"
    }
  ],
  "total": 1
}
```

**Examples:**

List all apps:
```bash
awrit app list
```

Count registered apps:
```bash
awrit app list --json | jq '.total'
```

**API Endpoint:**

`GET /v1/admin/apps`

---

### app get

**Synopsis:**
```
awrit app get <app-id>
```

**Description:**

Retrieve full details of a specific registered app, including scope ceiling, TTL, status, and timestamps.

**Arguments:**

| Argument | Description |
|----------|-------------|
| `app-id` | App identifier (required, positional) |

**Flags:**

None specific to this command. Supports global `--json` flag.

**Output (Table):**

| Column | Description |
|--------|-------------|
| APP_ID | App identifier |
| NAME | App name |
| CLIENT_ID | Client identifier |
| STATUS | Operational status |
| SCOPES | Scope ceiling (comma-separated) |
| TOKEN_TTL | App JWT TTL in seconds |
| CREATED | Creation timestamp |
| UPDATED | Last update timestamp |

**Output (JSON):**

```json
{
  "app_id": "app-12345",
  "name": "crawler",
  "client_id": "app_crawler",
  "scopes": ["admin:read", "read"],
  "token_ttl": 1800,
  "status": "active",
  "created_at": "2026-02-20T08:15:00Z",
  "updated_at": "2026-02-27T15:32:10Z"
}
```

**Examples:**

Get app details:
```bash
awrit app get app-12345
```

**API Endpoint:**

`GET /v1/admin/apps/{app-id}`

---

### app update

**Synopsis:**
```
awrit app update --id <app-id> [--scopes <scope1>,<scope2>,...] [--token-ttl N]
```

**Description:**

Update an app's scope ceiling and/or token TTL. At least one of `--scopes` or `--token-ttl` is required.

If scopes are narrowed (removed), any tokens that exceed the new ceiling are not automatically revoked — they remain valid until expiration. Use `awrit revoke` separately if immediate revocation is needed.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--id` | string | (required) | App ID to update |
| `--scopes` | string | (empty) | New comma-separated scope ceiling |
| `--token-ttl` | int | (none) | New app JWT TTL in seconds |

**Output (Table):**

| Column | Description |
|--------|-------------|
| APP_ID | App identifier |
| SCOPES | Updated scope ceiling |
| TOKEN_TTL | Updated TTL in seconds |
| UPDATED_AT | Update timestamp |

**Output (JSON):**

```json
{
  "app_id": "app-12345",
  "scopes": ["read"],
  "token_ttl": 3600,
  "updated_at": "2026-02-27T15:32:10Z"
}
```

**Examples:**

Widen scope ceiling:
```bash
awrit app update --id app-12345 --scopes "admin:read,admin:write,read"
```

Update TTL only:
```bash
awrit app update --id app-12345 --token-ttl 7200
```

Narrow both scopes and TTL:
```bash
awrit app update --id app-12345 --scopes "read" --token-ttl 1800
```

**API Endpoint:**

`PUT /v1/admin/apps/{app-id}`

**Request Body:**

```json
{
  "scopes": ["read"],
  "token_ttl": 3600
}
```

---

### app remove

**Synopsis:**
```
awrit app remove --id <app-id>
```

**Description:**

Deregister an app (soft delete). The app record is retained for audit purposes, but credentials are revoked and the app can no longer authenticate.

Existing agent tokens issued by this app remain valid until expiration; use `awrit revoke` if immediate token revocation is needed.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--id` | string | (required) | App ID to deregister |

**Output (Table):**

| Column | Description |
|--------|-------------|
| APP_ID | App identifier |
| STATUS | Status after deregistration (deregistered) |
| DEREGISTERED_AT | Deregistration timestamp |

**Output (JSON):**

```json
{
  "app_id": "app-12345",
  "status": "deregistered",
  "deregistered_at": "2026-02-27T15:32:10Z"
}
```

**Examples:**

Deregister an app:
```bash
awrit app remove --id app-12345
```

**API Endpoint:**

`DELETE /v1/admin/apps/{app-id}`

---

### init

**Synopsis:**
```
awrit init [--mode {dev|prod}] [--config-path PATH] [--force]
```

**Description:**

Initialize AgentWrit by generating a cryptographically secure admin secret and writing a configuration file.

In **dev mode**, the plaintext secret is stored in the config file for easy retrieval during development. In **prod mode**, only the bcrypt hash is stored — the plaintext is shown once on stdout and never saved to disk.

Use the generated secret to set `AACTL_ADMIN_SECRET` for subsequent awrit commands.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | dev | Initialization mode: `dev` (plaintext stored) or `prod` (bcrypt hash only) |
| `--config-path` | string | (default) | Explicit path to write config file (default: `~/.broker/config`) |
| `--force` | bool | false | Overwrite existing config file without confirmation |

**Output:**

```
Config written to: /home/user/.broker/config

Admin secret: dGVzdC1zZWNyZXQtdmFsdWUtYmFzZTY0LWVuY29kZWQtdGV4dA...

Dev mode: secret is also stored in the config file.
```

Or in prod mode:

```
Config written to: /etc/broker/config

Admin secret: dGVzdC1zZWNyZXQtdmFsdWUtYmFzZTY0LWVuY29kZWQtdGV4dA...

WARNING: Save this secret now. It will not be shown again.
Store it in your secrets manager (Vault, AWS Secrets Manager, etc.).
```

**Examples:**

Initialize in dev mode (default):
```bash
awrit init
```

Initialize in prod mode with explicit path:
```bash
awrit init --mode prod --config-path /etc/broker/config
```

Reinitialize with force:
```bash
awrit init --force
```

**Output File Format:**

The config file is YAML containing the admin secret (plaintext in dev, bcrypt hash in prod):

```yaml
mode: development
admin_secret: dGVzdC1zZWNyZXQtdmFsdWU=
```

or in prod:

```yaml
mode: production
admin_secret: $2a$12$...bcrypt.hash.here...
```

---

## Common Workflows

### Incident Response: Audit, Revoke, and Verify

**Scenario:** An agent token may have been compromised. Investigate, revoke it, and confirm.

**Steps:**

1. **Audit the agent's recent activity:**
   ```bash
   awrit audit events --agent-id "spiffe://example.com/agent/crawler" --since 2026-02-27T10:00:00Z
   ```
   Review the event detail column for suspicious operations (e.g., unexpected scopes requested, denials).

2. **Revoke all tokens for the agent:**
   ```bash
   awrit revoke --level agent --target "spiffe://example.com/agent/crawler"
   ```
   This immediately invalidates the agent's tokens and forces re-authentication.

3. **Verify revocation by checking recent events:**
   ```bash
   awrit audit events --agent-id "spiffe://example.com/agent/crawler" --event-type token_revoked
   ```
   Confirm that revocation events appear in the audit trail.

4. **(Optional) Check if the agent can still authenticate:**
   Attempt to use the agent. It should receive a 403 Unauthorized on the next request.

### Token Lifecycle Monitoring

**Scenario:** Understand token issuance and revocation patterns for compliance or troubleshooting.

**Steps:**

1. **Count all tokens issued today:**
   ```bash
   awrit audit events --event-type token_issued --since 2026-02-27T00:00:00Z --limit 1000 --json | jq '.total'
   ```

2. **Find revocation events by outcome:**
   ```bash
   awrit audit events --event-type token_revoked --outcome success --limit 100
   ```

3. **Identify failed token requests:**
   ```bash
   awrit audit events --event-type token_issue_failed --outcome denied --limit 100
   ```

4. **Export audit data for external analysis:**
   ```bash
   awrit audit events --limit 1000 --json > audit_export.json
   ```
   Process the JSON with external tools (jq, Python, etc.) for further analysis.

---

## Exit Codes

| Code | Meaning | Example |
|------|---------|---------|
| 0 | Command succeeded | Token released successfully, events listed |
| 1 | Command failed | Missing required flag, invalid broker URL, network error, HTTP 4xx/5xx response |

On error, awrit prints a descriptive message to stderr and exits with code 1.

---

## Error Handling

### Common Errors

**AACTL_BROKER_URL is not set**
```
Error: AACTL_BROKER_URL is not set
```
Set the environment variable:
```bash
export AACTL_BROKER_URL="http://localhost:8080"
```

**AACTL_ADMIN_SECRET is not set**
```
Error: AACTL_ADMIN_SECRET is not set
```
Set the environment variable:
```bash
export AACTL_ADMIN_SECRET="your-secret"
```

**auth failed (HTTP 401)**
```
Error: auth failed (HTTP 401): invalid admin secret
```
Check that `AACTL_ADMIN_SECRET` matches the broker's configured secret. See broker deployment docs for secret rotation.

**HTTP 404: endpoint not found**
```
Error: HTTP 404: endpoint not found
```
Verify `AACTL_BROKER_URL` points to a running broker. Check that the broker port matches (default 8080).

**Invalid RFC3339 timestamp**
```
Error: invalid timestamp format
```
Ensure `--since` and `--until` use RFC3339 format: `2026-02-27T15:32:10Z` or `2026-02-27T15:32:10-05:00`.

**--limit or --offset is negative**
```
Error: limit must be > 0
```
Use positive integers only. Default limit is 100 if not specified.

---

## Output Modes

### Table Output (Default)

Human-readable formatted tables with tab-separated columns:

```bash
awrit audit events
```

Output:
```
ID                                    TIMESTAMP              EVENT TYPE      AGENT ID                                      OUTCOME  DETAIL
550e8400-e29b-41d4-a716-446655440000  2026-02-27T15:32:10Z  token_issued    spiffe://example.com/agent/crawler              success  Token issued for scope read
```

**Advantages:**
- Readable in terminal
- No parsing required for human inspection
- Progress and metadata printed to stderr

**Disadvantages:**
- Hard to parse programmatically
- Fixed column widths may truncate long values

### JSON Output

Structured JSON output suitable for scripting and automation:

```bash
awrit audit events --json
```

Output:
```json
{
  "events": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "timestamp": "2026-02-27T15:32:10Z",
      "event_type": "token_issued",
      "agent_id": "spiffe://example.com/agent/crawler",
      "outcome": "success",
      "detail": "Token issued for scope read"
    }
  ],
  "total": 1,
  "offset": 0,
  "limit": 100
}
```

**Advantages:**
- Fully structured and parseable
- No truncation
- Suitable for piping to tools like jq, Python, or Go
- Machine-readable for automation

**Disadvantages:**
- Less human-readable
- Requires tools to parse and filter

### Combining with Unix Tools

**Count events:**
```bash
awrit audit events --json | jq '.total'
```

**Filter by outcome:**
```bash
awrit audit events --json | jq '.events[] | select(.outcome == "denied")'
```

**Extract agent IDs:**
```bash
awrit audit events --json | jq '.events[].agent_id' | sort | uniq
```

**Export to CSV (using jq):**
```bash
awrit audit events --json | jq -r '.events[] | [.id, .timestamp, .event_type, .agent_id, .outcome] | @csv' > audit.csv
```

---

## Security Best Practices

### Admin Secret Management

1. **Never hardcode secrets** in scripts, containers, or version control.
2. **Use environment variable files** (`.env`, `.envrc`) and `.gitignore` them.
3. **Rotate secrets regularly** according to your security policy.
4. **Restrict access** to the environment where `AACTL_ADMIN_SECRET` is set (CI/CD runners, bastion hosts, operator workstations).
5. **Audit secret usage** by enabling verbose logging on the broker side.

### Token Handling

1. **Never log or display agent tokens** in full unless necessary for debugging.
2. **Use `--token` with environment variables**, not hardcoded strings:
   ```bash
   awrit token release --token "$AGENT_TOKEN"
   ```
3. **Revoke tokens immediately** if accidentally exposed.
4. **Test revocation** in staging before using in production.

### Audit Trail

1. **Review audit events regularly** for suspicious patterns (failed auth, unusual revocations).
2. **Export and archive audit logs** for compliance and forensic analysis.
3. **Set up alerts** for high-risk events (e.g., mass revocations, failed auth attempts).
4. **Correlate awrit operations** with application logs to understand impact.

---

## Advanced Usage

### Integration with Monitoring

Export recent denied events to a monitoring system:

```bash
awrit audit events --outcome denied --limit 100 --json | jq '.events' | \
  curl -X POST \
    -H "Content-Type: application/json" \
    -d @- \
    https://monitoring.example.com/api/events
```

### Batch Revocation

Revoke all tokens for multiple agents from a list:

```bash
cat agents.txt | while read agent_id; do
  awrit revoke --level agent --target "$agent_id"
done
```

---

## Troubleshooting

### Slow Audit Queries

**Problem:** `awrit audit events` takes a long time to return.

**Solutions:**
- Reduce the `--limit` (default 100, but 1000+ can be slow).
- Use `--since` and `--until` to narrow the time window.
- Index frequently-filtered columns in the audit database (broker-side optimization).
- Use `--offset` and `--limit` for pagination rather than fetching all events at once.

### Revocation Not Taking Effect

**Problem:** Agents still authenticate after revocation.

**Solutions:**
- Confirm the revocation succeeded: `awrit audit events --event-type token_revoked` should show a recent event.
- Check broker logs for errors during revocation.
- Verify agents are reading the revocation list (not caching tokens locally).
- Confirm the broker is running and connected to the revocation backend (Redis, SQLite, etc.).

---

## Related Documentation

- **[Concepts](concepts.md)** — Understand agents, tokens, scopes, and delegation.
- **[Getting Started: Operator](getting-started-operator.md)** — Initial broker setup and awrit configuration.
- **[Common Tasks](common-tasks.md)** — Step-by-step guides for operations.
- **[API Reference](api.md)** — Full HTTP API specification (what awrit uses under the hood).
- **[Troubleshooting](troubleshooting.md)** — Resolve broker issues.

---

## FAQ

**Q: Can awrit be used remotely?**

A: Yes. Set `AACTL_BROKER_URL` to any reachable broker URL (e.g., `https://broker.example.com:9443`). The broker must have network access from your machine.

**Q: Can I run awrit commands in a container or CI/CD pipeline?**

A: Yes. Pass `AACTL_BROKER_URL` and `AACTL_ADMIN_SECRET` as environment variables. Use JSON output (`--json`) for reliable parsing.

**Q: What happens if the broker is down?**

A: awrit will fail with a network error. Retry after the broker recovers.

**Q: Can I revoke all tokens at once?**

A: Use `awrit revoke --level agent --target <agent-id>` to revoke all tokens for an agent, or `--level chain` for an entire delegation tree. There is no global "revoke all" command by design (safety).

**Q: How do I rotate the admin secret?**

A: See the broker deployment documentation. Once rotated, update `AACTL_ADMIN_SECRET` in your environment before running awrit again.

**Q: Can awrit run without the broker's admin secret?**

A: No. Admin operations (audit, revoke, app management) require authentication. Agent operations like `awrit token release` require the agent's own token, not the admin secret.

**Q: How do I automate daily audit exports?**

A: Use cron or systemd timer to run:
   ```bash
   awrit audit events --limit 10000 --json > "/backups/audit-$(date +%Y-%m-%d).json"
   ```

---

## What's Next?

| If you want to... | Read this |
|-------------------|-----------|
| Look up HTTP endpoints | [API Reference](api.md) |
| See the internal architecture | [Architecture](architecture.md) |
| Find where features live in code | [Implementation Map](implementation-map.md) |

---

*Previous: [API Reference](api.md) · Next: [Architecture](architecture.md)*
