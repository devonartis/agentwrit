# Demo Dashboard (M14)

## Purpose

Web-based dashboard for visualizing AgentAuth demo workflows in real time.
Provides a single-page interface where users can trigger the 3-agent demo
pipeline (Agent A, B, C) and the 5-attack simulator, then watch results
stream in via Server-Sent Events (SSE). Side-by-side comparison highlights
the security gap between insecure (shared API key) and secure (AgentAuth)
modes.

## Design decisions

- **HTMX + SSE over React/SPA framework**: No build step required. A single
  HTML file with HTMX handles DOM updates, while SSE provides real-time event
  delivery. This keeps the demo self-contained and easy to run without Node.js
  tooling.

- **In-memory state with asyncio.Lock**: The dashboard runs a single
  `DashboardState` instance with pub/sub for SSE subscribers. No external
  store (Redis, SQLite) is needed for a demo tool. The `asyncio.Lock`
  serializes mutations safely for concurrent SSE clients.

- **Background task pattern**: `POST /demo/run` returns immediately with
  `{"status": "started"}` and launches the orchestrator + simulator via
  `asyncio.create_task`. Events flow to connected SSE clients as they are
  produced. This keeps the HTTP response non-blocking.

- **Injectable demo runner**: The `create_app` factory accepts an optional
  `demo_runner` coroutine, allowing tests to substitute a mock that publishes
  canned events without hitting real broker/resource-server processes.

- **Dark theme**: Professional dark color scheme (`#1a1a2e` background,
  `#4ecca3` success green, `#e94560` danger red) chosen for presentation
  readability on projectors and large screens.

## Architecture

```
demo/dashboard/
  __init__.py         # Package marker
  main.py             # FastAPI app factory, default demo runner
  state.py            # DashboardState dataclass with pub/sub
  routes.py           # SSE stream, demo control, status endpoints
  templates/
    index.html        # HTMX single-page dashboard
  static/
    style.css         # Dark theme CSS (grid layout, transitions)
    app.js            # Vanilla JS: mode toggle, SSE handlers, DOM updates
  tests/
    conftest.py       # TestClient fixtures with no-op runner
    test_dashboard.py # Unit tests (10): health, status, reset, run, SSE
    test_integration.py # Integration tests (6): full flow, SSE ordering,
                        #   concurrent clients, reset, conflict detection
```

## Endpoints

| Method | Path             | Description                        |
|--------|------------------|------------------------------------|
| GET    | `/`              | Serve dashboard HTML               |
| GET    | `/health`        | Health check                       |
| GET    | `/events/stream` | SSE event stream                   |
| POST   | `/demo/run`      | Start demo (accepts `{"mode":...}`)  |
| POST   | `/demo/reset`    | Clear all state                    |
| GET    | `/demo/status`   | Current state snapshot             |

## SSE event types

- `status` -- lifecycle messages (started, running attacks, complete)
- `agent_event` -- per-agent completion with timing and detail
- `attack_result` -- per-attack outcome with attempts/successes/blocked

## Running

```bash
cd demo
python -m dashboard.main --port 8070
# Open http://localhost:8070 in a browser
```

## Testing

```bash
cd demo
python -m pytest dashboard/tests/ -v
```
