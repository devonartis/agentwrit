"""Unit tests for resource server route handlers.

These tests exercise the four endpoints with the app in insecure mode
(API-Key header required). They validate request/response contracts,
seed data correctness, and error handling.
"""

from __future__ import annotations

import pytest
from fastapi.testclient import TestClient


class TestHealthEndpoint:
    """GET /health — no auth required."""

    def test_returns_healthy(self, insecure_client: TestClient) -> None:
        resp = insecure_client.get("/health")
        assert resp.status_code == 200
        body = resp.json()
        assert body["status"] == "healthy"
        assert body["mode"] == "insecure"

    def test_secure_mode_label(self, secure_client: TestClient) -> None:
        resp = secure_client.get("/health")
        assert resp.json()["mode"] == "secure"


class TestGetCustomer:
    """GET /customers/{id}."""

    def test_existing_customer(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.get("/customers/12345", headers=api_key_headers)
        assert resp.status_code == 200
        body = resp.json()
        assert body["id"] == 12345
        assert body["name"] == "Alice Johnson"
        assert body["tier"] == "premium"

    def test_all_seed_customers(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        for cid in [12345, 12346, 12347, 12348, 12349]:
            resp = insecure_client.get(f"/customers/{cid}", headers=api_key_headers)
            assert resp.status_code == 200
            assert resp.json()["id"] == cid

    def test_nonexistent_customer_404(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.get("/customers/99999", headers=api_key_headers)
        assert resp.status_code == 404
        assert "application/problem+json" in resp.headers["content-type"]
        body = resp.json()
        assert body["type"].startswith("urn:agentauth:resource:")
        assert body["status"] == 404


class TestGetOrders:
    """GET /orders/{customer_id}."""

    def test_customer_with_orders(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.get("/orders/12345", headers=api_key_headers)
        assert resp.status_code == 200
        orders = resp.json()
        assert len(orders) == 2
        assert all(o["customer_id"] == 12345 for o in orders)

    def test_all_customers_have_orders(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        for cid in [12345, 12346, 12347, 12348, 12349]:
            resp = insecure_client.get(f"/orders/{cid}", headers=api_key_headers)
            assert resp.status_code == 200
            assert len(resp.json()) >= 2

    def test_nonexistent_customer_404(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.get("/orders/99999", headers=api_key_headers)
        assert resp.status_code == 404


class TestUpdateTicket:
    """PUT /tickets/{id}."""

    def test_update_status(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.put(
            "/tickets/789", json={"status": "closed"}, headers=api_key_headers
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["id"] == 789
        assert body["status"] == "closed"

    def test_update_assignee(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.put(
            "/tickets/789", json={"assignee": "agent-c"}, headers=api_key_headers
        )
        assert resp.status_code == 200
        assert resp.json()["assignee"] == "agent-c"

    def test_partial_update(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        """Updating only assignee should not change status."""
        resp = insecure_client.put(
            "/tickets/789", json={"assignee": "x"}, headers=api_key_headers
        )
        assert resp.json()["status"] == "open"

    def test_nonexistent_ticket_404(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.put(
            "/tickets/99999", json={"status": "closed"}, headers=api_key_headers
        )
        assert resp.status_code == 404

    def test_invalid_status_422(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.put(
            "/tickets/789", json={"status": "invalid_status"}, headers=api_key_headers
        )
        assert resp.status_code == 422


class TestSendNotification:
    """POST /notifications/send."""

    def test_send_success(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.post(
            "/notifications/send",
            json={"customer_id": 12345, "message": "Your ticket is resolved"},
            headers=api_key_headers,
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["sent"] is True
        assert body["customer_id"] == 12345
        assert body["channel"] == "email"
        assert "notification_id" in body

    def test_custom_channel(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.post(
            "/notifications/send",
            json={"customer_id": 12345, "message": "Hello", "channel": "sms"},
            headers=api_key_headers,
        )
        assert resp.json()["channel"] == "sms"

    def test_nonexistent_customer_404(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.post(
            "/notifications/send",
            json={"customer_id": 99999, "message": "Hello"},
            headers=api_key_headers,
        )
        assert resp.status_code == 404

    def test_missing_required_field_422(
        self, insecure_client: TestClient, api_key_headers: dict
    ) -> None:
        resp = insecure_client.post(
            "/notifications/send",
            json={"customer_id": 12345},
            headers=api_key_headers,
        )
        assert resp.status_code == 422


class TestSeedData:
    """Verify seed data completeness per MVP Requirements 5.4."""

    def test_five_customers(self) -> None:
        from resource_server.seed_data import CUSTOMERS

        assert len(CUSTOMERS) == 5

    def test_ten_orders(self) -> None:
        from resource_server.seed_data import ORDERS

        total = sum(len(v) for v in ORDERS.values())
        assert total == 10

    def test_three_tickets(self) -> None:
        from resource_server.seed_data import TICKETS

        assert len(TICKETS) == 3
