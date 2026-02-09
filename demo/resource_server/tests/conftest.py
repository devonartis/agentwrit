"""Shared test fixtures for resource server tests."""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient

from resource_server.main import create_app
from resource_server.middleware import ServerMode
from resource_server.seed_data import TICKETS
from resource_server.models import TicketStatus


@pytest.fixture
def secure_client() -> TestClient:
    """TestClient running in secure mode."""
    app = create_app(mode=ServerMode.secure)
    return TestClient(app)


@pytest.fixture
def insecure_client() -> TestClient:
    """TestClient running in insecure mode."""
    app = create_app(mode=ServerMode.insecure)
    return TestClient(app)


@pytest.fixture(autouse=True)
def _reset_ticket_state():
    """Reset mutable ticket state between tests."""
    from resource_server.seed_data import TICKETS

    originals = {tid: t.model_copy() for tid, t in TICKETS.items()}
    yield
    TICKETS.clear()
    TICKETS.update(originals)
