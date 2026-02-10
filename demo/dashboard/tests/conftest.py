"""Shared test fixtures for dashboard tests."""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient

from dashboard.main import create_app
from dashboard.state import DashboardState


async def _noop_runner(state: DashboardState, mode: str) -> None:
    """No-op demo runner for tests that don't need real execution."""
    pass


@pytest.fixture
def dashboard_client() -> TestClient:
    """TestClient backed by the dashboard app with no-op runner."""
    app = create_app(demo_runner=_noop_runner)
    return TestClient(app)


@pytest.fixture
def dashboard_state(dashboard_client: TestClient) -> DashboardState:
    """Direct access to the app's DashboardState for assertions."""
    return dashboard_client.app.state.dashboard
