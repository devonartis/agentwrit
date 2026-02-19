# AgentAuth Backlog — Features, Fixes, and Usability Gaps

Everything we've run into during cowork sessions, what we actually saw, and what
needs to happen next. This is the living document — update it as things get fixed
or new issues surface.

Last updated: 2026-02-17

---

## P0 — Blocking for Demo / Testing

### 0. Audit log is in-memory only — events lost on broker restart

**What we found:** The broker's audit system (`broker/internal/audit/audit_log.go`)
stores ALL audit events in a Go slice (`[]AuditEvent`) protected by a mutex. There is
NO persistence to disk, database, or external service. When the broker restarts, every
audit event is gone.

**What this means:**
- `agentauth security audit list` returns nothing after a broker restart
- `agentauth security audit verify-chain` can't verify anything
- The hash-chained tamper-evidence is useless if events disappear
- The dashboard audit table (`/htmx/security/audit`) is empty after restart
- For the demo: if you restart docker compose between runs, all audit history is lost

**Where it lives in code:**
- `broker/internal/audit/audit_log.go` — `events []AuditEvent` (line ~30)
- No call to any store/database for persistence
- The `SqlStore` in `broker/internal/store/` handles tokens, agents, ceilings — but NOT audit

**What needs to happen:**
1. Add `SaveAuditEvent()` and `QueryAuditEvents()` to `SqlStore`
2. Create SQLite table: `audit_events (id, timestamp, event_type, agent_id, task_id, orch_id, detail, hash, prev_hash)`
3. `AuditLog.Record()` should write to both memory (for fast query) and SQLite (for durability)
4. On broker startup, load existing events from SQLite to rebuild the hash chain

**Repo:** agentAuth (Go change)
**Impact:** Without this, the audit system is a facade. It looks professional but loses
everything on restart. For the demo this is tolerable (restart between shows). For
production use, this is a hard blocker.

---

### 1. Sidecar health endpoint doesn't return its own ID

**What we saw:** When we built the CLI ceiling management commands (`agentauth operator
sidecar update-ceiling`), we realized the command requires `--sidecar-id`. But there's
no programmatic way to discover the sidecar ID. The health endpoint (`GET /v1/health`
on the sidecar) returns `scope_ceiling`, `broker_connected`, `agents_registered`,
`uptime_seconds` — but NOT `sidecar_id`.

**How we found the ID today:** We had to search the sidecar's stdout logs for the
startup message: `[SIDECAR] [MAIN] ready addr=:8081 sidecar_id=sc-xxx`. That's not
acceptable for production use.

**Where the ID lives in code:** The sidecar stores it in `sidecarState.sidecarID`
(file: `broker/cmd/sidecar/bootstrap.go`, line 21). The health handler has access
to it via `h.state` (file: `broker/cmd/sidecar/handler.go`, line 280) but doesn't
include it in the response (line 318-336).

**Exact fix:**
```go
// broker/cmd/sidecar/handler.go, around line 326
// Add after the existing resp map:
if h.state != nil {
    resp["sidecar_id"] = h.state.sidecarID  // <-- ADD THIS
    if lr := h.state.getLastRenewal(); !lr.IsZero() {
        resp["last_renewal"] = lr.Format(time.RFC3339)
    }
    // ...existing code...
}
```

**Repo:** agentAuth (Go change)
**Impact:** Without this, the CLI `update-ceiling` and `get-ceiling` commands are
unusable without log-diving. The operator has no way to manage ceilings programmatically.

---

### 2. CLI `update-ceiling` should auto-discover sidecar ID (depends on #1)

**What we want:** If the operator doesn't pass `--sidecar-id`, the CLI should query
the sidecar health endpoint and get the ID automatically. Single-sidecar deployments
(which is the common case) shouldn't require the operator to know the ID at all.

**Exact fix:**
```python
# app/cli/operator.py — in sidecar_update_ceiling()
# Change sidecar_id from required to optional:
sidecar_id: str = typer.Option(None, "--sidecar-id", help="Sidecar ID (auto-discovered if omitted)")

# If not provided, discover it:
if not sidecar_id:
    health = client.operator.get_sidecar_health()
    sidecar_id = health.get("sidecar_id")
    if not sidecar_id:
        typer.echo("Error: Sidecar health endpoint doesn't return sidecar_id. "
                    "Upgrade the sidecar or pass --sidecar-id manually.")
        raise typer.Exit(1)
    typer.echo(f"Auto-discovered sidecar: {sidecar_id}")
```

**Repo:** agentauth-app (Python change, but depends on Go fix #1)
**Impact:** Makes the CLI actually usable for the common single-sidecar case.

---

### 3. Operator docs don't mention runtime ceiling management

**What we saw:** `docs/getting-started-operator.md` (line 85) documents the env var
`AA_SIDECAR_SCOPE_CEILING` and says it's required. But it never mentions:
- That the env var is just the bootstrap seed
- That the ceiling can be updated at runtime via the broker admin API
- That the sidecar picks up changes on its renewal cycle (no restart needed)
- How to use the CLI commands we just built

This is why the previous developer was managing ceilings by editing docker-compose.yml
and restarting containers — the docs told them that's how it works.

**What needs to be added to `docs/getting-started-operator.md`:**
1. Section: "Runtime Ceiling Management" after the current env var section
2. CLI examples: `show-ceiling`, `get-ceiling`, `update-ceiling`
3. Explanation of the renewal cycle (4-12 minutes for changes to take effect)
4. Emergency narrowing and auto-revocation behavior
5. Reference to the User Stories doc for full persona walkthroughs

**Repo:** agentauth-app

---

## P1 — Required for Production-Ready Demo

### 4. Developer → Operator ceiling request workflow

**What we discussed:** When we were building the scope narrowing feature, we realized
there's no way for a developer to formally request new scopes from the operator. The
conversation went like this:

> "The developer needs to be able to tell the operator, who is also the admin, to change
> the ceiling." — Divine, 2026-02-17

In AWS, you file an IAM request. Here, the developer just tells the operator out-of-band
(Slack message, email, etc.). There's no audit trail of who requested what and why.

**What this would look like:**
```bash
# Developer requests a new scope
agentauth dev ceiling request \
  --scope "delete:customer:*" \
  --reason "New feature: account deletion per JIRA-456"

# Operator reviews pending requests
agentauth operator ceiling requests list

# Operator approves (updates ceiling automatically)
agentauth operator ceiling requests approve --id REQ-123
```

**What needs to happen:**
1. New broker endpoint: `POST /v1/admin/ceiling-requests` (create request)
2. New broker endpoint: `GET /v1/admin/ceiling-requests` (list pending)
3. New broker endpoint: `PUT /v1/admin/ceiling-requests/{id}` (approve/deny)
4. On approval: broker calls its own `UpdateSidecarCeiling()` internally
5. Audit event: `ceiling_request_created`, `ceiling_request_approved`, `ceiling_request_denied`
6. SDK methods + CLI commands for both developer and operator

**Repo:** Both (Go for broker endpoints, Python for SDK/CLI)
**Impact:** Without this, ceiling management is ad-hoc and unaudited.

---

### 5. No way to list active sidecars

**What we saw:** When trying to use the `get-ceiling` CLI command, we needed the sidecar
ID. Even after fixing #1, an operator managing multiple sidecars needs to list them all.

**What exists today:** The broker stores sidecar records in its database (created during
activation in `admin_svc.go`), but there's no list endpoint. You can get a specific
sidecar's ceiling (`GET /v1/admin/sidecars/{id}/ceiling`) but not list all sidecars.

**Exact fix:** Add `GET /v1/admin/sidecars` to `broker/internal/admin/admin_hdl.go`.
Return: sidecar_id, scope_ceiling, activation_time, last_renewal, status (active/expired).
Add corresponding SDK method and CLI command.

**Repo:** agentAuth (Go) + agentauth-app (Python SDK/CLI)

---

### 6. No way to list registered agents

**What we saw:** During testing, when an agent got revoked, we had no way to check what
other agents were still registered or what scopes they had. The sidecar health endpoint
shows `agents_registered: N` (a count) but not the actual agent list.

**What exists today:**
- Sidecar has an in-memory `agentRegistry` (`broker/cmd/sidecar/handler.go`) with a
  `count()` method but no `list()` method
- Broker stores agent records in its database but has no list endpoint

**Exact fix:**
1. Add `list()` to sidecar's `agentRegistry` (returns SPIFFE IDs, scopes, creation time)
2. Add `GET /v1/agents` to sidecar's HTTP handler
3. Add `GET /v1/admin/agents` to broker's admin handler (optionally filtered by sidecar_id)
4. SDK method: `client.operator.list_agents(sidecar_id=None)`
5. CLI: `agentauth operator agent list [--sidecar-id ID]`

**Repo:** Both

---

### 7. Automated test suite for scope narrowing

**What we saw:** Every time we made a change to the scope narrowing logic, we had to
run the full app and manually test with Lewis Smith tickets. When FIX-002 broke
`find_customer_by_name`, we only found it by running the demo end-to-end. There are
zero automated tests for the scope narrowing implementation.

**Tests needed (specific):**
```python
# test_scope_narrowing.py

# 1. _scope_matches_any() unit tests
def test_exact_match():
    assert _scope_matches_any("read:customer:contact", {"read:customer:contact"})

def test_narrowed_match():
    # Narrowed scope in set should match base scope
    assert _scope_matches_any("read:customer:contact", {"read:customer:contact:cust-001"})

def test_wildcard_match():
    assert _scope_matches_any("read:customer:contact", {"read:customer:*"})

def test_no_match():
    assert not _scope_matches_any("read:customer:payment", {"read:customer:contact"})

# 2. Registration scope building
def test_authenticated_user_gets_both_broad_and_narrow():
    # Agent should have: read:customer:contact AND read:customer:contact:cust-001

def test_anonymous_user_gets_only_broad():
    # Agent should have: read:customer:contact (no narrowed scopes)

# 3. _enforce_tool_call() integration tests (mock broker)
def test_customer_bound_tool_builds_narrowed_scope():
    # get_customer_payment(customer_id="cust-001") should check read:customer:payment:cust-001

def test_non_customer_bound_tool_uses_broad_scope():
    # find_customer_by_name(name="Lewis") should check read:customer:contact

def test_cross_customer_denied_by_scope_mismatch():
    # Token has read:customer:payment:cust-001
    # Tool call for cust-002 should be denied
```

**Repo:** agentauth-app
**How to run:** `uv run pytest tests/test_scope_narrowing.py -v`

---

## P2 — Important for Real-World Use

### 8. `_authenticate_user()` is a mock

**What it does today:** Pattern-matches customer names from ticket text. This is fine
for the demo but completely wrong for production.

```python
# app/web/routes.py — current mock
def _authenticate_user(ticket_text):
    text_lower = ticket_text.lower()
    for cust_id, cust in CUSTOMERS.items():
        if cust["name"].lower() in text_lower:
            return cust_id, cust
    return None, None
```

**Why it matters:** The whole point of scope narrowing is that identity is a verified
fact from the auth layer, not something extracted from user input. If the LLM can
influence which customer ID gets used, the security model breaks.

**What production looks like:**
```python
def _authenticate_user(request):
    # Extract from session/JWT/SSO — never from ticket text
    session = request.session
    user_id = session.get("user_id")
    if user_id and user_id in CUSTOMERS:
        return user_id, CUSTOMERS[user_id]
    return None, None
```

**Repo:** agentauth-app (Python — but this is a deployment decision, not a code fix)

---

### 9. Ceiling change audit history not exposed via CLI

**What exists:** The broker already logs `EventScopesCeilingUpdated` audit events
(in `admin_svc.go`, line ~430). The security CLI can query audit events with
`agentauth security audit list --event-type TYPE`. But nobody documented that you
can use `--event-type scope_ceiling_updated` to see ceiling change history.

**What we need:** Add this to the operator docs and the CLI help text. Also verify
that the audit event includes old_ceiling, new_ceiling, who changed it, and when.

**Repo:** agentauth-app (documentation + CLI help text)

---

### 10. Dashboard doesn't show scope narrowing

**What we saw:** The web dashboard shows the SSE event stream with agent registrations,
tool calls, and broker decisions. But it doesn't visually show:
- That scopes were narrowed to a specific customer
- The difference between broad and narrowed scopes
- Which scope was checked for each tool call (broad vs narrowed)
- Why a cross-customer access was denied (scope mismatch detail)

**Where to fix:** `app/web/templates/` — the SSE event rendering. Add a "Scope Binding"
section to the run detail view that shows the full scope chain:
```
Ceiling:    read:customer:*
Registered: read:customer:contact, read:customer:contact:cust-001, read:customer:payment:cust-001
Tool call:  get_customer_payment(customer_id=cust-001)
Required:   read:customer:payment:cust-001
Result:     ALLOWED (exact match in agent scope set)
```

**Repo:** agentauth-app

---

### 11. No rate limiting on ceiling updates

**What could happen:** A misconfigured automation or script could spam
`PUT /v1/admin/sidecars/{id}/ceiling` and cause constant token revocations.

**Where to fix:** `broker/internal/admin/admin_hdl.go` — the ceiling PUT handler.
Add a rate limiter (e.g., max 10 updates per minute per sidecar). The broker already
has rate limiting on `/v1/admin/auth` — use the same pattern.

**Repo:** agentAuth (Go change)

---

## P3 — Nice to Have / Future

### 12. Ceiling diff preview (dry-run)

**Use case:** Operator wants to see what would happen if they narrow the ceiling
before actually doing it — how many tokens would be revoked, which agents affected.

**Fix:** Add `--dry-run` flag to `update-ceiling` CLI. Broker would need a new
query endpoint or a dry-run parameter on the existing PUT.

### 13. Scope playground / simulator

**Use case:** Developer wants to test scope configurations without running the full
system. "If my ceiling is X and my token has Y and the tool requires Z, will it work?"

**Fix:** CLI command or web UI that takes ceiling + token scopes + tool call and
shows the full matching chain with pass/fail at each step.

### 14. Webhook on ceiling change

**Use case:** Slack notification when a ceiling changes. CI/CD pipeline triggers
when scopes are updated.

**Fix:** Broker stores webhook URLs per sidecar. Fires POST on ceiling change
with old/new ceiling and revocation info.

### 15. Multi-sidecar management

**Use case:** Production with multiple sidecars (one per service, one per environment).
Need bulk ceiling operations and cross-sidecar comparison.

**Fix:** `agentauth operator sidecar list` + `agentauth operator sidecar diff --sid1 X --sid2 Y`

---

## Completed Fixes (Reference)

### FIX-001: Sidecar ceiling requires wildcards for customer-bound scopes
**Date:** 2026-02-17 | **Commit:** 33fb0b2
**Problem:** Ceiling had `read:customer:payment` but narrowed scopes are
`read:customer:payment:cust-001`. Broker's `SplitN(":", 3)` sees different identifiers.
**Fix:** Changed ceiling to use wildcards: `read:customer:*`, `delete:customer:*`, etc.
**Detail:** See `docs/cowork/fixes/FIX-001-scope-ceiling-wildcard.md`

### FIX-002: Non-customer-bound tools need broad scopes at registration
**Date:** 2026-02-17 | **Commit:** (pending)
**Problem:** `find_customer_by_name` requires `read:customer:contact` (broad) but agent
only had `read:customer:contact:cust-001` (narrowed). All tool calls failed and agent
was immediately revoked.
**Fix:** Registration now includes BOTH broad and narrowed scopes.
**Detail:** See `docs/cowork/fixes/FIX-002-broad-scopes-for-lookup-tools.md`

---

## Already Built (This Session)

- **Runtime scope narrowing** — commit c79df77
- **CLI ceiling management** — commit 33fb0b2
- **SDK ceiling methods** — commit 33fb0b2
- **Wildcard ceiling** — commit 33fb0b2
- **User stories doc** — 5 personas with CLI examples
- **CHANGELOG.md** — full history from design through all commits
- **Backlog** — this document
- **Fix documentation** — detailed fix logs in `docs/cowork/fixes/`
