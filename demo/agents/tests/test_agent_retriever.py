"""Tests for Agent A -- DataRetriever."""

from __future__ import annotations

import httpx
import pytest

from agents.agent_retriever import DataRetriever
from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode


# ── helpers ────────────────────────────────────────────────────────────────

CUSTOMER_DATA = {
    "customer_id": "12345",
    "name": "Acme Corp",
    "tier": "enterprise",
}


def _resource_transport(status: int = 200, body: dict | None = None):
    """Mock transport for the resource server."""
    body = body if body is not None else CUSTOMER_DATA

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(status, json=body)

    return httpx.MockTransport(handler)


def _make_patched_client(transport):
    """Create a patched AsyncClient class that uses the given transport."""

    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# ── secure mode ────────────────────────────────────────────────────────────


class TestDataRetrieverSecure:
    """Secure mode: register then fetch."""

    @pytest.mark.asyncio
    async def test_run_returns_customer_data(self, mock_broker_client: BrokerClient, monkeypatch) -> None:
        agent = DataRetriever(
            name="Agent-A",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(_resource_transport()))
        data = await agent.run(
            customer_id=12345,
            launch_token="lt-123",
            orch_id="orch1",
            task_id="task1",
        )

        assert data["customer_id"] == "12345"
        assert agent.agent_instance_id.startswith("spiffe://")
        assert agent.access_token == "mock-access-token"


# ── insecure mode ──────────────────────────────────────────────────────────


class TestDataRetrieverInsecure:
    """Insecure mode: skip registration, use API-Key."""

    @pytest.mark.asyncio
    async def test_run_skips_registration(self, mock_broker_client: BrokerClient, monkeypatch) -> None:
        agent = DataRetriever(
            name="Agent-A",
            broker=mock_broker_client,
            mode=ServerMode.insecure,
            insecure_api_key="dev-key",
        )
        captured: dict = {}

        def handler(request: httpx.Request) -> httpx.Response:
            captured["api_key"] = request.headers.get("api-key", "")
            return httpx.Response(200, json=CUSTOMER_DATA)

        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(httpx.MockTransport(handler)))
        data = await agent.run(customer_id=12345)

        assert data["customer_id"] == "12345"
        assert captured["api_key"] == "dev-key"
        # Should NOT have registered
        assert agent.agent_instance_id == ""


# ── error handling ─────────────────────────────────────────────────────────


class TestDataRetrieverErrors:
    """Resource server errors should propagate as HTTPStatusError."""

    @pytest.mark.asyncio
    async def test_404_raises(self, mock_broker_client: BrokerClient, monkeypatch) -> None:
        agent = DataRetriever(
            name="Agent-A",
            broker=mock_broker_client,
            mode=ServerMode.insecure,
        )
        transport = _resource_transport(
            status=404,
            body={"type": "urn:agentauth:resource:404", "title": "Not found", "status": 404, "detail": "", "instance": ""},
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(transport))
        with pytest.raises(httpx.HTTPStatusError) as exc_info:
            await agent.run(customer_id=99999)

        assert exc_info.value.response.status_code == 404
