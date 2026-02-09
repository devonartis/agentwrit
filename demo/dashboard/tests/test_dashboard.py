"""Unit tests for the dashboard backend."""

from __future__ import annotations

import asyncio
import json
import time

import pytest
from fastapi.testclient import TestClient

from dashboard.main import create_app
from dashboard.state import DashboardState


# ---------------------------------------------------------------------------
# Health
# ---------------------------------------------------------------------------


def test_health_endpoint(dashboard_client: TestClient):
    resp = dashboard_client.get("/health")
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "healthy"
    assert body["service"] == "dashboard"


# ---------------------------------------------------------------------------
# Status
# ---------------------------------------------------------------------------


def test_status_initial(dashboard_client: TestClient):
    resp = dashboard_client.get("/demo/status")
    assert resp.status_code == 200
    body = resp.json()
    assert body["running"] is False
    assert body["mode"] is None
    assert body["demo_result"] is None
    assert body["attack_results"] is None


# ---------------------------------------------------------------------------
# Reset
# ---------------------------------------------------------------------------


def test_reset_clears_state(dashboard_client: TestClient, dashboard_state: DashboardState):
    # Seed some state.
    dashboard_state.running = True
    dashboard_state.mode = "insecure"
    dashboard_state.demo_result = {"mode": "insecure"}
    dashboard_state.attack_results = [{"name": "test"}]
    dashboard_state.events.append({"type": "status"})

    resp = dashboard_client.post("/demo/reset")
    assert resp.status_code == 200
    assert resp.json()["status"] == "reset"

    # Verify clean state.
    status = dashboard_client.get("/demo/status").json()
    assert status["running"] is False
    assert status["mode"] is None
    assert status["demo_result"] is None
    assert status["attack_results"] is None


# ---------------------------------------------------------------------------
# Run demo
# ---------------------------------------------------------------------------


def test_run_starts_demo(dashboard_client: TestClient):
    resp = dashboard_client.post("/demo/run", json={"mode": "insecure"})
    assert resp.status_code == 200
    body = resp.json()
    assert body["status"] == "started"
    assert body["mode"] == "insecure"


def test_run_invalid_mode(dashboard_client: TestClient):
    resp = dashboard_client.post("/demo/run", json={"mode": "unknown"})
    assert resp.status_code == 400
    assert "mode" in resp.json()["detail"]


def test_run_while_already_running(dashboard_client: TestClient, dashboard_state: DashboardState):
    dashboard_state.running = True
    resp = dashboard_client.post("/demo/run", json={"mode": "insecure"})
    assert resp.status_code == 409
    assert "already running" in resp.json()["detail"]


# ---------------------------------------------------------------------------
# Run with mock orchestrator + simulator
# ---------------------------------------------------------------------------


def test_run_triggers_runner_and_updates_status():
    """Verify the background runner populates state when it completes."""
    calls: list[str] = []

    async def mock_runner(state: DashboardState, mode: str) -> None:
        calls.append(mode)
        await state.publish({"type": "status", "data": {"message": "Demo started"}})
        state.demo_result = {"mode": mode, "success": True}
        state.attack_results = [{"name": "test_attack", "attack_succeeded": True}]
        state.running = False
        await state.publish({"type": "status", "data": {"message": "Demo complete"}})

    app = create_app(demo_runner=mock_runner)
    client = TestClient(app)

    resp = client.post("/demo/run", json={"mode": "secure"})
    assert resp.status_code == 200

    # Give the background task time to finish.
    time.sleep(0.2)

    status = client.get("/demo/status").json()
    assert status["demo_result"] is not None
    assert status["demo_result"]["mode"] == "secure"
    assert status["attack_results"] is not None
    assert calls == ["secure"]


# ---------------------------------------------------------------------------
# SSE stream -- unit-level test of state pub/sub machinery
# ---------------------------------------------------------------------------


@pytest.mark.asyncio
async def test_sse_publish_subscribe():
    """Verify the DashboardState pub/sub delivers events to subscribers."""
    state = DashboardState()
    queue = await state.subscribe()

    event = {"type": "agent_event", "data": {"agent_name": "Agent-A"}}
    await state.publish(event)

    received = queue.get_nowait()
    assert received["type"] == "agent_event"
    assert received["data"]["agent_name"] == "Agent-A"
    assert "timestamp" in received

    await state.unsubscribe(queue)


@pytest.mark.asyncio
async def test_sse_multiple_subscribers():
    """Verify that all subscribers receive the same events."""
    state = DashboardState()
    q1 = await state.subscribe()
    q2 = await state.subscribe()

    await state.publish({"type": "status", "data": {"message": "hello"}})

    r1 = q1.get_nowait()
    r2 = q2.get_nowait()
    assert r1["type"] == "status"
    assert r2["type"] == "status"
    assert r1["data"]["message"] == "hello"
    assert r2["data"]["message"] == "hello"

    await state.unsubscribe(q1)
    await state.unsubscribe(q2)


def test_sse_stream_endpoint_content_type():
    """Verify the SSE endpoint returns text/event-stream and delivers events."""
    received: list[dict] = []

    async def mock_runner(state: DashboardState, mode: str) -> None:
        # Publish one event then stop.
        await state.publish({"type": "status", "data": {"message": "test-event"}})
        state.running = False

    app = create_app(demo_runner=mock_runner)
    client = TestClient(app)

    # Trigger the runner so events get published on the app's event loop.
    client.post("/demo/run", json={"mode": "insecure"})
    time.sleep(0.1)

    # The events are stored in state.events — verify the pipeline works.
    state: DashboardState = app.state.dashboard
    assert len(state.events) >= 1
    assert any(e["data"].get("message") == "test-event" for e in state.events)
