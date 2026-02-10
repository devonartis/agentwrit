"""Integration tests for the resource server.

These tests exercise full request flows through the middleware + routes,
testing both insecure and secure modes end-to-end. Secure mode uses a
mocked broker to simulate realistic validation responses.
"""

from __future__ import annotations

from unittest.mock import MagicMock, AsyncMock

import httpx
import pytest
from fastapi.testclient import TestClient

from resource_server.main import create_app
from resource_server.middleware import ServerMode


def _broker_ok(agent_id: str, scopes: list[str]) -> httpx.AsyncClient:
    """Mock broker that approves all tokens."""
    client = AsyncMock(spec=httpx.AsyncClient)
    resp = MagicMock()
    resp.status_code = 200
    resp.json.return_value = {
        "valid": True,
        "agent_id": agent_id,
        "scope": scopes,
        "expires_in": 290,
        "delegation_depth": 0,
    }
    client.post.return_value = resp
    return client


def _broker_deny_scope() -> httpx.AsyncClient:
    """Mock broker that rejects with 403 scope mismatch."""
    client = AsyncMock(spec=httpx.AsyncClient)
    resp = MagicMock()
    resp.status_code = 403
    resp.json.return_value = {
        "type": "urn:agentauth:error:scope-mismatch",
        "title": "Required scope not granted",
        "status": 403,
    }
    client.post.return_value = resp
    return client


class TestInsecureFullFlow:
    """End-to-end insecure mode: API key allows all access."""

    def setup_method(self) -> None:
        app = create_app(mode=ServerMode.insecure)
        self.client = TestClient(app)
        self.headers = {"API-Key": "demo-key"}

    def test_full_customer_support_workflow(self) -> None:
        """Simulate the demo workflow: fetch customer, fetch orders,
        update ticket, send notification."""
        # Agent A: fetch customer
        resp = self.client.get("/customers/12345", headers=self.headers)
        assert resp.status_code == 200
        assert resp.json()["name"] == "Alice Johnson"

        # Agent B: fetch orders for analysis
        resp = self.client.get("/orders/12345", headers=self.headers)
        assert resp.status_code == 200
        assert len(resp.json()) == 2

        # Agent C: update ticket
        resp = self.client.put(
            "/tickets/789",
            json={"status": "closed", "assignee": "agent-c"},
            headers=self.headers,
        )
        assert resp.status_code == 200
        assert resp.json()["status"] == "closed"

        # Agent C: send notification
        resp = self.client.post(
            "/notifications/send",
            json={"customer_id": 12345, "message": "Ticket #789 resolved"},
            headers=self.headers,
        )
        assert resp.status_code == 200
        assert resp.json()["sent"] is True

    def test_lateral_movement_succeeds(self) -> None:
        """In insecure mode, Agent A can access orders (not its scope)."""
        resp = self.client.get("/orders/12347", headers=self.headers)
        assert resp.status_code == 200


class TestSecureFullFlow:
    """End-to-end secure mode: tokens validated against broker."""

    def test_scoped_customer_access(self) -> None:
        """Agent A with read:Customers:12345 can access that customer."""
        mock = _broker_ok(
            "spiffe://agentauth.local/agent/orch1/task1/inst1",
            ["read:Customers:12345"],
        )
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer agent-a-token"},
        )
        assert resp.status_code == 200
        assert resp.json()["id"] == 12345

    def test_lateral_movement_blocked(self) -> None:
        """Agent A tries to read orders — broker rejects (scope mismatch)."""
        mock = _broker_deny_scope()
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.get(
            "/orders/12347",
            headers={"Authorization": "Bearer agent-a-token"},
        )
        assert resp.status_code == 403

    def test_delegation_workflow(self) -> None:
        """Agent B delegates to Agent C for ticket update + notification."""
        # Agent C has write:Tickets:789 and invoke:Notifications
        mock = _broker_ok(
            "spiffe://agentauth.local/agent/orch1/task1/inst-c",
            ["write:Tickets:789", "invoke:Notifications"],
        )
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)

        # Update ticket
        resp = client.put(
            "/tickets/789",
            json={"status": "closed"},
            headers={"Authorization": "Bearer agent-c-delegated"},
        )
        assert resp.status_code == 200

        # Send notification
        resp = client.post(
            "/notifications/send",
            json={"customer_id": 12345, "message": "Resolved"},
            headers={"Authorization": "Bearer agent-c-delegated"},
        )
        assert resp.status_code == 200

    def test_expired_token_rejected(self) -> None:
        """Broker rejects expired token with 401."""
        client_mock = AsyncMock(spec=httpx.AsyncClient)
        resp_mock = MagicMock()
        resp_mock.status_code = 401
        resp_mock.json.return_value = {
            "type": "urn:agentauth:error:invalid-token",
            "title": "Token expired",
            "status": 401,
        }
        client_mock.post.return_value = resp_mock
        app = create_app(mode=ServerMode.secure, http_client=client_mock)
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer expired-token"},
        )
        assert resp.status_code == 401

    def test_no_token_rejected(self) -> None:
        """Request without Authorization header is rejected."""
        app = create_app(mode=ServerMode.secure)
        client = TestClient(app)
        resp = client.get("/customers/12345")
        assert resp.status_code == 401
