"""Unit tests for token validation middleware.

Tests cover:
- Insecure mode: API-Key checks
- Secure mode: Bearer token extraction, broker interaction (mocked)
- Scope mapping: URL -> required scope resolution
- Error responses: RFC 7807 format
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import httpx
import pytest
from fastapi.testclient import TestClient

from resource_server.main import create_app
from resource_server.middleware import ServerMode, _required_scope


# ---------- Scope resolution unit tests ----------


class TestScopeResolution:
    """Verify _required_scope maps URL patterns correctly."""

    def test_customers_scope(self) -> None:
        assert _required_scope("GET", "/customers/12345") == "read:Customers:12345"

    def test_orders_scope(self) -> None:
        assert _required_scope("GET", "/orders/12347") == "read:Orders:12347"

    def test_tickets_scope(self) -> None:
        assert _required_scope("PUT", "/tickets/789") == "write:Tickets:789"

    def test_notifications_scope(self) -> None:
        assert _required_scope("POST", "/notifications/send") == "invoke:Notifications"

    def test_unknown_path_returns_none(self) -> None:
        assert _required_scope("GET", "/unknown") is None

    def test_wrong_method_returns_none(self) -> None:
        assert _required_scope("POST", "/customers/12345") is None


# ---------- Insecure mode tests ----------


class TestInsecureMode:
    """Middleware behaviour in insecure mode."""

    def test_valid_api_key(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/customers/12345", headers={"API-Key": "any-key"})
        assert resp.status_code == 200

    def test_missing_api_key_401(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/customers/12345")
        assert resp.status_code == 401
        body = resp.json()
        assert body["title"] == "Missing API key"
        assert body["status"] == 401

    def test_empty_api_key_401(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/customers/12345", headers={"API-Key": "  "})
        assert resp.status_code == 401

    def test_health_skips_auth(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/health")
        assert resp.status_code == 200


# ---------- Secure mode tests (mocked broker) ----------


def _mock_broker_client(
    status_code: int = 200,
    json_body: dict | None = None,
    raise_error: bool = False,
) -> httpx.AsyncClient:
    """Create a mock httpx.AsyncClient with a canned POST response."""
    client = AsyncMock(spec=httpx.AsyncClient)
    if raise_error:
        client.post.side_effect = httpx.ConnectError("Connection refused")
    else:
        # Use MagicMock for response so .json() is sync (like real httpx.Response).
        mock_resp = MagicMock()
        mock_resp.status_code = status_code
        mock_resp.json.return_value = json_body or {}
        client.post.return_value = mock_resp
    return client


class TestSecureMode:
    """Middleware behaviour in secure mode with mocked broker."""

    def test_valid_token_200(self) -> None:
        mock = _mock_broker_client(
            200,
            {
                "valid": True,
                "agent_id": "spiffe://agentauth.local/agent/orch/task/inst",
                "scope": ["read:Customers:12345"],
                "expires_in": 290,
                "delegation_depth": 0,
            },
        )
        app = create_app(
            mode=ServerMode.secure,
            broker_url="http://broker:8080",
            http_client=mock,
        )
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer valid-token-here"},
        )
        assert resp.status_code == 200
        # Verify the broker was called with correct payload.
        mock.post.assert_called_once()
        call_kwargs = mock.post.call_args
        assert call_kwargs[1]["json"]["token"] == "valid-token-here"
        assert call_kwargs[1]["json"]["required_scope"] == "read:Customers:12345"

    def test_missing_bearer_401(self) -> None:
        app = create_app(mode=ServerMode.secure)
        client = TestClient(app)
        resp = client.get("/customers/12345")
        assert resp.status_code == 401
        assert resp.json()["title"] == "Missing bearer token"

    def test_invalid_token_401(self) -> None:
        mock = _mock_broker_client(401, {"type": "urn:agentauth:error:invalid-token"})
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer bad-token"},
        )
        assert resp.status_code == 401
        assert resp.json()["title"] == "Token validation failed"

    def test_scope_mismatch_403(self) -> None:
        mock = _mock_broker_client(403, {"type": "urn:agentauth:error:scope-mismatch"})
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer scoped-token"},
        )
        assert resp.status_code == 403
        assert resp.json()["title"] == "Scope mismatch"

    def test_broker_unreachable_502(self) -> None:
        mock = _mock_broker_client(raise_error=True)
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.get(
            "/customers/12345",
            headers={"Authorization": "Bearer some-token"},
        )
        assert resp.status_code == 502
        assert resp.json()["title"] == "Broker unreachable"

    def test_health_skips_auth_in_secure_mode(self) -> None:
        app = create_app(mode=ServerMode.secure)
        client = TestClient(app)
        resp = client.get("/health")
        assert resp.status_code == 200

    def test_notifications_scope_sent_to_broker(self) -> None:
        mock = _mock_broker_client(
            200,
            {
                "valid": True,
                "agent_id": "agent-c",
                "scope": ["invoke:Notifications"],
                "expires_in": 100,
                "delegation_depth": 1,
            },
        )
        app = create_app(mode=ServerMode.secure, http_client=mock)
        client = TestClient(app)
        resp = client.post(
            "/notifications/send",
            json={"customer_id": 12345, "message": "Hello"},
            headers={"Authorization": "Bearer notif-token"},
        )
        assert resp.status_code == 200
        call_kwargs = mock.post.call_args
        assert call_kwargs[1]["json"]["required_scope"] == "invoke:Notifications"


# ---------- RFC 7807 response format ----------


class TestProblemFormat:
    """All error responses should follow RFC 7807."""

    def test_insecure_401_is_problem_json(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/customers/12345")
        assert resp.status_code == 401
        body = resp.json()
        assert "type" in body
        assert "title" in body
        assert "status" in body
        assert body["status"] == 401
        assert "application/problem+json" in resp.headers["content-type"]

    def test_secure_401_is_problem_json(self) -> None:
        app = create_app(mode=ServerMode.secure)
        client = TestClient(app)
        resp = client.get("/customers/12345")
        assert resp.status_code == 401
        body = resp.json()
        assert "type" in body
        assert "title" in body
        assert "status" in body
        assert "application/problem+json" in resp.headers["content-type"]
