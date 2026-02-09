"""Integration tests for the dashboard backend.

All tests use mocked orchestrator/simulator to avoid real HTTP calls.
"""

from __future__ import annotations

import asyncio
import json
import time
from dataclasses import asdict

import pytest
from fastapi.testclient import TestClient

from dashboard.main import create_app
from dashboard.state import DashboardState


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _make_mock_runner(delay: float = 0.0):
    """Return a mock demo runner that publishes canned events.

    Args:
        delay: Seconds to sleep before publishing (simulates work).
    """

    async def runner(state: DashboardState, mode: str) -> None:
        if delay:
            await asyncio.sleep(delay)

        # Agent events.
        for name in ("Agent-A", "Agent-B", "Agent-C"):
            await state.publish({
                "type": "agent_event",
                "data": {
                    "agent_name": name,
                    "success": True,
                    "elapsed_ms": 42.0,
                    "detail": f"{name} completed",
                },
            })

        state.demo_result = {
            "mode": mode,
            "agents": [
                {"agent_name": n, "success": True, "elapsed_ms": 42.0, "detail": f"{n} completed"}
                for n in ("Agent-A", "Agent-B", "Agent-C")
            ],
            "total_time_ms": 126.0,
            "success": True,
        }

        # Attack events.
        attacks = [
            "credential_theft",
            "lateral_movement",
            "impersonation",
            "escalation",
            "accountability",
        ]
        attack_results = []
        for attack_name in attacks:
            succeeded = mode == "insecure"
            result = {
                "name": attack_name,
                "mode": mode,
                "attempts": 3,
                "successes": 3 if succeeded else 0,
                "blocked": 0 if succeeded else 3,
                "attack_succeeded": succeeded,
                "details": [],
            }
            attack_results.append(result)
            await state.publish({
                "type": "attack_result",
                "data": result,
            })

        state.attack_results = attack_results
        state.running = False

        await state.publish({
            "type": "status",
            "data": {"message": "Demo complete", "mode": mode},
        })

    return runner


def _wait_for_done(client: TestClient, max_wait: float = 5.0) -> dict:
    """Poll /demo/status until running is False or timeout."""
    deadline = time.monotonic() + max_wait
    while time.monotonic() < deadline:
        status = client.get("/demo/status").json()
        if not status["running"]:
            return status
        time.sleep(0.05)
    return client.get("/demo/status").json()


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_full_demo_flow():
    """POST /demo/run, wait for completion, verify results populated."""
    app = create_app(demo_runner=_make_mock_runner())
    client = TestClient(app)

    resp = client.post("/demo/run", json={"mode": "insecure"})
    assert resp.status_code == 200
    assert resp.json()["status"] == "started"

    status = _wait_for_done(client)
    assert status["running"] is False
    assert status["demo_result"] is not None
    assert status["demo_result"]["mode"] == "insecure"
    assert status["demo_result"]["success"] is True
    assert len(status["demo_result"]["agents"]) == 3

    assert status["attack_results"] is not None
    assert len(status["attack_results"]) == 5
    # In insecure mode, all attacks succeed.
    for attack in status["attack_results"]:
        assert attack["attack_succeeded"] is True


def test_full_demo_flow_secure():
    """Same as above but in secure mode -- attacks should be blocked."""
    app = create_app(demo_runner=_make_mock_runner())
    client = TestClient(app)

    resp = client.post("/demo/run", json={"mode": "secure"})
    assert resp.status_code == 200

    status = _wait_for_done(client)
    assert status["running"] is False
    assert status["attack_results"] is not None
    for attack in status["attack_results"]:
        assert attack["attack_succeeded"] is False


def test_sse_events_during_demo():
    """Trigger demo, wait for completion, verify events arrive in order.

    Verifies event ordering through the state's events list rather than
    the SSE streaming endpoint (TestClient + SSE streaming can deadlock
    because the background task and iter_lines share the same event loop).
    """
    app = create_app(demo_runner=_make_mock_runner())
    client = TestClient(app)

    client.post("/demo/run", json={"mode": "insecure"})
    _wait_for_done(client)

    state: DashboardState = app.state.dashboard
    events = state.events

    # Should have agent events, attack events, and status events.
    agent_events = [e for e in events if e["type"] == "agent_event"]
    attack_events = [e for e in events if e["type"] == "attack_result"]
    assert len(agent_events) == 3
    assert len(attack_events) == 5

    # Agent events in order: A, B, C.
    assert [e["data"]["agent_name"] for e in agent_events] == ["Agent-A", "Agent-B", "Agent-C"]

    # All events have timestamps.
    for event in events:
        assert "timestamp" in event


def test_reset_during_idle():
    """Run demo, let it complete, reset, verify clean state."""
    app = create_app(demo_runner=_make_mock_runner())
    client = TestClient(app)

    client.post("/demo/run", json={"mode": "insecure"})
    _wait_for_done(client)

    # Verify some state exists.
    status = client.get("/demo/status").json()
    assert status["demo_result"] is not None

    # Reset.
    resp = client.post("/demo/reset")
    assert resp.status_code == 200

    status = client.get("/demo/status").json()
    assert status["running"] is False
    assert status["mode"] is None
    assert status["demo_result"] is None
    assert status["attack_results"] is None


@pytest.mark.asyncio
async def test_concurrent_sse_clients():
    """Two SSE subscribers both receive the same events."""
    state = DashboardState()
    q1 = await state.subscribe()
    q2 = await state.subscribe()

    for i in range(5):
        await state.publish({"type": "status", "data": {"seq": i}})

    events1 = []
    events2 = []
    for _ in range(5):
        events1.append(q1.get_nowait())
        events2.append(q2.get_nowait())

    assert len(events1) == 5
    assert len(events2) == 5
    for i in range(5):
        assert events1[i]["data"]["seq"] == i
        assert events2[i]["data"]["seq"] == i

    await state.unsubscribe(q1)
    await state.unsubscribe(q2)


def test_run_while_already_running():
    """POST /demo/run while demo is running returns 409."""
    async def slow_runner(state: DashboardState, mode: str) -> None:
        await asyncio.sleep(10)  # Never finishes during test.
        state.running = False

    app = create_app(demo_runner=slow_runner)
    client = TestClient(app)

    resp1 = client.post("/demo/run", json={"mode": "insecure"})
    assert resp1.status_code == 200

    # Second request while still running.
    resp2 = client.post("/demo/run", json={"mode": "secure"})
    assert resp2.status_code == 409
    assert "already running" in resp2.json()["detail"]
