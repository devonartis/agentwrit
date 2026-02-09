"""Tests for the Demo Orchestrator."""

from __future__ import annotations

import httpx
import pytest

from agents.broker_client import BrokerClient
from agents.orchestrator import run_demo, DemoResult
from resource_server.middleware import ServerMode


# ── helpers ────────────────────────────────────────────────────────────────

CUSTOMER_DATA = {"customer_id": "12345", "name": "Acme Corp", "tier": "enterprise"}
ORDERS_DATA = {
    "customer_id": "12345",
    "orders": [{"order_id": "ORD-001", "amount": 500}],
}
TICKET_RESP = {"ticket_id": "789", "status": "resolved"}
NOTIF_RESP = {"sent": True, "message_id": "msg-001"}


def _resource_transport() -> httpx.MockTransport:
    """Mock resource server that handles all four endpoints."""

    def handler(request: httpx.Request) -> httpx.Response:
        path = str(request.url.path)
        if "/customers/" in path and request.method == "GET":
            return httpx.Response(200, json=CUSTOMER_DATA)
        if "/orders/" in path and request.method == "GET":
            return httpx.Response(200, json=ORDERS_DATA)
        if "/tickets/" in path and request.method == "PUT":
            return httpx.Response(200, json=TICKET_RESP)
        if "/notifications/send" in path and request.method == "POST":
            return httpx.Response(200, json=NOTIF_RESP)
        return httpx.Response(404, json={"error": "not found"})

    return httpx.MockTransport(handler)


def _broker_transport() -> httpx.MockTransport:
    """Mock broker that handles challenge, register, and delegate."""
    nonce = "ab" * 32

    def handler(request: httpx.Request) -> httpx.Response:
        path = str(request.url.path)
        if path == "/v1/challenge":
            return httpx.Response(200, json={"nonce": nonce, "expires_at": "2026-01-01T00:01:00Z"})
        if path == "/v1/register":
            return httpx.Response(201, json={
                "agent_instance_id": "spiffe://agentauth.local/agent/demo-orch/ticket-789/inst",
                "access_token": "mock-access-token",
                "expires_in": 300,
                "refresh_after": 240,
            })
        if path == "/v1/delegate":
            return httpx.Response(201, json={
                "delegation_token": "mock-deleg-token",
                "chain_hash": "abc123",
                "delegation_depth": 1,
            })
        return httpx.Response(404, json={"error": "not found"})

    return httpx.MockTransport(handler)


def _routing_transport(resource_transport, broker_transport) -> httpx.MockTransport:
    """Create a single transport that routes by URL pattern."""

    def handler(request: httpx.Request) -> httpx.Response:
        url_str = str(request.url)
        if "8080" in url_str or "broker" in url_str:
            return broker_transport.handle_request(request)
        return resource_transport.handle_request(request)

    return httpx.MockTransport(handler)


def _patch_httpx_client(resource_transport, broker_transport):
    """Patch httpx.AsyncClient to use a routing mock transport."""
    import httpx as _httpx

    orig = _httpx.AsyncClient
    transport = _routing_transport(resource_transport, broker_transport)

    class Patched(_httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    _httpx.AsyncClient = Patched
    return orig


# ── tests ──────────────────────────────────────────────────────────────────


class TestOrchestratorInsecure:
    """Test the full A -> B -> C pipeline in insecure mode."""

    @pytest.mark.asyncio
    async def test_run_demo_insecure_success(self) -> None:
        resource_t = _resource_transport()
        broker_t = _broker_transport()
        orig = _patch_httpx_client(resource_t, broker_t)
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                ticket_id=789,
                customer_id=12345,
                insecure_api_key="test-key",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert isinstance(result, DemoResult)
        assert result.success is True
        assert result.mode == "insecure"
        assert len(result.agents) == 3
        assert result.total_time_ms > 0

    @pytest.mark.asyncio
    async def test_agent_sequence_abc(self) -> None:
        """Verify agents run in A, B, C order."""
        resource_t = _resource_transport()
        broker_t = _broker_transport()
        orig = _patch_httpx_client(resource_t, broker_t)
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        names = [a.agent_name for a in result.agents]
        assert names == ["Agent-A", "Agent-B", "Agent-C"]


class TestOrchestratorSecure:
    """Test the full pipeline in secure mode with mocked broker."""

    @pytest.mark.asyncio
    async def test_run_demo_secure_success(self) -> None:
        resource_t = _resource_transport()
        broker_t = _broker_transport()
        orig = _patch_httpx_client(resource_t, broker_t)
        try:
            result = await run_demo(
                mode=ServerMode.secure,
                ticket_id=789,
                customer_id=12345,
                launch_token="seed-lt",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert result.success is True
        assert result.mode == "secure"
        assert len(result.agents) == 3


class TestDemoResultCapture:
    """Verify DemoResult captures all agent outcomes."""

    @pytest.mark.asyncio
    async def test_all_fields_populated(self) -> None:
        resource_t = _resource_transport()
        broker_t = _broker_transport()
        orig = _patch_httpx_client(resource_t, broker_t)
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        for agent in result.agents:
            assert agent.agent_name != ""
            assert agent.elapsed_ms >= 0
            assert agent.detail != ""
            assert agent.success is True
