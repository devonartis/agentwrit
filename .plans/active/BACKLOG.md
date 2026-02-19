# AgentAuth Backlog — Features, Fixes, and Usability Gaps

Everything we've run into during cowork sessions, what we actually saw, and what
needs to happen next. This is the living document — update it as things get fixed
or new issues surface.

Last updated: 2026-02-19

---

## P0 — Blocking for Demo / Testing

### ~~0. Audit log is in-memory only — events lost on broker restart~~

**Status: DONE** | Commit: `4c2733d` | Merged: `9290e9d` | Date: 2026-02-18

Implemented SQLite-backed audit persistence with write-through (`AuditStore` interface).
`AA_DB_PATH` configurable (default `./agentauth.db`). On startup, broker loads existing
events from SQLite to rebuild the hash chain. Health endpoint returns `db_connected` and
`audit_events_count`. Prometheus metrics: `agentauth_audit_events_total`,
`agentauth_audit_write_duration_seconds`, `agentauth_db_errors_total`,
`agentauth_audit_events_loaded`.

---

### ~~1. Sidecar health endpoint doesn't return its own ID~~

**Status: DONE** | Commit: `50a2809` | Date: 2026-02-18

Sidecar `GET /v1/health` now returns `sidecar_id` in the response JSON. Also added
`lastRenewal` and `startTime` to sidecar state (`9eaf773`), and structured logging +
Prometheus metrics (`3a0677a`).

---

### 2. CLI `update-ceiling` should auto-discover sidecar ID (depends on #1)

**Status: NEEDS VERIFICATION** — Go dependency (#1) is done. Check if agentauth-app
has the Python-side change.

**What we want:** If the operator doesn't pass `--sidecar-id`, the CLI should query
the sidecar health endpoint and get the ID automatically. Single-sidecar deployments
(which is the common case) shouldn't require the operator to know the ID at all.

**Repo:** agentauth-app (Python change)
**Impact:** Makes the CLI actually usable for the common single-sidecar case.

---

### ~~3. Operator docs don't mention runtime ceiling management~~

**Status: DONE** | Date: 2026-02-18

Operator docs updated with runtime ceiling management section, `AA_DB_PATH` env var,
renewal cycle explanation, and emergency narrowing behavior. Updated in
`docs/getting-started-operator.md`.

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

### 16. Go CLI for admin endpoints (`cmd/cli/`)

**Status: NEW** | Priority: HIGH | Date: 2026-02-19

**What we saw:** Built `GET /v1/admin/sidecars` endpoint but there's no CLI anywhere
to access it. The only way to call admin endpoints is manual curl + JWT management.
The Python CLI in agentauth-app is a demo app CLI — it can change or be replaced.
Operator tooling must live in the Go repo alongside the broker and sidecar.

**User feedback:**
> "there is no cli for this in that repo and it should not be in that repo" (re: agentauth-app)
> "why would we write this without a cli to access it"

**What this would look like:**
```bash
# Authenticate as admin
agentauth admin auth --secret <admin-secret> --broker http://localhost:8080

# List sidecars
agentauth admin sidecars list

# Get ceiling for a sidecar
agentauth admin sidecars ceiling --id sc-abc123

# Update ceiling
agentauth admin sidecars ceiling update --id sc-abc123 --scope "read:customer:*"

# Query audit log
agentauth admin audit list [--event-type TYPE]
```

**What needs to happen:**
1. New `cmd/cli/` directory — third binary alongside broker and sidecar
2. Admin auth command (POST /v1/admin/auth, store token locally)
3. Sidecar list command (GET /v1/admin/sidecars)
4. Ceiling CRUD commands (GET/PUT /v1/admin/sidecars/{id}/ceiling)
5. Audit query commands (GET /v1/admin/audit)
6. Health check command (GET /v1/health)
7. Token inspection (decode JWT, show claims, expiry)

**Repo:** agentAuth (Go — this is the operator's tool, not the demo app's)
**Impact:** Without this, every admin endpoint we build is unusable by operators.

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

## Already Built

- **Runtime scope narrowing** — agentauth-app: `c79df77`; agentAuth: `c9e3eae`, `c024cad`
- **CLI ceiling management** — agentauth-app: `33fb0b2`; agentAuth: `c024cad`
- **SDK ceiling methods** — agentauth-app: `33fb0b2`
- **Wildcard ceiling fix** — agentauth-app: `33fb0b2`; agentAuth: `8970a02`
- **Audit persistence to SQLite** — agentAuth: `4c2733d` (P0-0)
- **Sidecar ID in health endpoint** — agentAuth: `50a2809` (P0-1)
- **Operator docs for ceiling management** — agentAuth: docs updated (P0-3)
- **Doc reorganization** — agentAuth: `c67f7c9` (moved old docs to internal_use_docs/)
- **CONTRIBUTING.md + SECURITY.md** — agentAuth: `c67f7c9`
- **Godoc comments** — agentAuth: `571203f` (sidecar config, problemdetails)
- **User stories doc** — 5 personas with CLI examples
- **CHANGELOG.md** — full history from design through all commits
- **Backlog** — this document
- **Fix documentation** — detailed fix logs in `docs/cowork/fixes/`
