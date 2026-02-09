# Resource Server (M11)

## Purpose

The resource server is a Python FastAPI application that simulates a customer database API for AgentAuth demos. It provides four endpoints representing typical enterprise resources (customers, orders, tickets, notifications) and supports two operating modes: **insecure** (shared API keys) and **secure** (AgentAuth token validation). This allows the demo to show the security gap and how AgentAuth closes it.

## Design decisions

**FastAPI over Flask**: FastAPI was chosen for automatic request validation via Pydantic, built-in OpenAPI docs, and async support for non-blocking broker calls in secure mode. The async middleware can call the broker's `/v1/token/validate` endpoint without blocking the event loop.

**Dual-mode middleware**: A single `AuthMiddleware` class handles both modes rather than separate middleware stacks. The mode is set at app startup via CLI flag or environment variable. This keeps the codebase simple and makes mode-switching a single configuration change.

**Scope mapping via regex rules**: Each endpoint maps to a required scope (e.g., `GET /customers/12345` requires `read:Customers:12345`). The mapping is defined as a list of `(method, regex, template)` tuples in `middleware.py`. Path parameters are extracted from regex named groups and interpolated into the scope template. This is declarative and easy to extend.

**httpx client injection**: The middleware accepts an optional `httpx.AsyncClient` for testing. Production creates a fresh client per request; tests inject a mock. This avoids monkey-patching and keeps tests deterministic.

**In-memory seed data**: Sample data (5 customers, 10 orders, 3 tickets) is defined as module-level dicts in `seed_data.py`. No database — the data exists only in process memory. Tickets are mutable (PUT updates modify the dict), which is reset between tests via an autouse fixture.

## Endpoints

| Method | Path | Scope (Secure Mode) | Description |
|--------|------|---------------------|-------------|
| GET | `/customers/{id}` | `read:Customers:{id}` | Retrieve a customer record |
| GET | `/orders/{customer_id}` | `read:Orders:{customer_id}` | List orders for a customer |
| PUT | `/tickets/{id}` | `write:Tickets:{id}` | Update ticket status/assignee |
| POST | `/notifications/send` | `invoke:Notifications` | Simulate sending a notification |
| GET | `/health` | (none — skips auth) | Health check with mode label |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RESOURCE_SERVER_MODE` | `secure` | Operating mode: `secure` or `insecure` |
| `BROKER_URL` | `http://localhost:8080` | AgentAuth broker URL for token validation |
| `--mode` CLI flag | (env var) | Override mode from command line |
| `--port` CLI flag | `8090` | HTTP listen port |

## File layout

```
demo/
  requirements.txt          — Python dependencies
  conftest.py               — root pytest config
  resource_server/
    __init__.py
    main.py                 — FastAPI app factory + CLI entrypoint
    routes.py               — 4 endpoint handlers
    seed_data.py            — pre-seeded customers, orders, tickets
    models.py               — Pydantic models + RFC 7807 Problem
    middleware.py            — AuthMiddleware (secure/insecure)
    tests/
      __init__.py
      conftest.py           — shared fixtures
      test_routes.py        — route handler tests (20)
      test_middleware.py     — middleware unit tests (19)
      test_integration.py   — end-to-end flow tests (7)
```

## Running

```bash
cd demo
pip install -r requirements.txt

# Start in insecure mode (no broker needed)
python -m resource_server.main --mode insecure

# Start in secure mode (broker must be running on :8080)
python -m resource_server.main --mode secure

# Run tests
pytest resource_server/tests/ -v

# Check formatting
black --check .
```
