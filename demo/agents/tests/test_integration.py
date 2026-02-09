"""Integration tests for the full Agent A -> B -> C demo workflow.

These tests use mocked broker and resource server transports to validate
the complete agent pipeline without requiring live services.
"""

from __future__ import annotations

import httpx
import pytest

from agents.orchestrator import run_demo, DemoResult
from resource_server.middleware import ServerMode


# ── shared mock data ───────────────────────────────────────────────────────

CUSTOMER_DATA = {"customer_id": "12345", "name": "Acme Corp", "tier": "enterprise"}
ORDERS_DATA = {
    "customer_id": "12345",
    "orders": [
        {"order_id": "ORD-001", "amount": 500},
        {"order_id": "ORD-002", "amount": 1200},
    ],
}
TICKET_RESP = {"ticket_id": "789", "status": "resolved"}
NOTIF_RESP = {"sent": True, "message_id": "msg-001"}


def _resource_transport() -> httpx.MockTransport:
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


def _patch_clients(resource_t, broker_t):
    """Patch httpx.AsyncClient to route broker vs resource calls."""
    import httpx as _httpx

    orig = _httpx.AsyncClient

    def routing_handler(request: httpx.Request) -> httpx.Response:
        url_str = str(request.url)
        if "8080" in url_str or "broker" in url_str:
            return broker_t.handle_request(request)
        return resource_t.handle_request(request)

    transport = httpx.MockTransport(routing_handler)

    class Patched(_httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    _httpx.AsyncClient = Patched
    return orig


# ── integration tests ──────────────────────────────────────────────────────


class TestFullWorkflowInsecure:
    """Complete insecure pipeline: Orchestrator -> A -> B -> C."""

    @pytest.mark.asyncio
    async def test_full_pipeline_success(self) -> None:
        orig = _patch_clients(_resource_transport(), _broker_transport())
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

        assert result.success is True
        assert result.mode == "insecure"
        assert len(result.agents) == 3

    @pytest.mark.asyncio
    async def test_agent_a_feeds_agent_b(self) -> None:
        """Agent B receives customer data produced by Agent A."""
        orig = _patch_clients(_resource_transport(), _broker_transport())
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                ticket_id=789,
                customer_id=12345,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        # Agent B's detail should reference the customer analysis
        agent_b = result.agents[1]
        assert agent_b.agent_name == "Agent-B"
        assert "12345" in agent_b.detail
        assert "enterprise" in agent_b.detail


class TestFullWorkflowSecure:
    """Complete secure pipeline with mocked broker."""

    @pytest.mark.asyncio
    async def test_secure_pipeline_with_delegation(self) -> None:
        orig = _patch_clients(_resource_transport(), _broker_transport())
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
        # Agent C should have used delegation
        agent_c = result.agents[2]
        assert agent_c.agent_name == "Agent-C"
        assert agent_c.success is True


class TestAgentSequenceIntegrity:
    """Verify the A -> B -> C ordering and data flow."""

    @pytest.mark.asyncio
    async def test_sequence_ordering(self) -> None:
        orig = _patch_clients(_resource_transport(), _broker_transport())
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

    @pytest.mark.asyncio
    async def test_all_agents_succeed(self) -> None:
        orig = _patch_clients(_resource_transport(), _broker_transport())
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        for agent in result.agents:
            assert agent.success is True, f"{agent.agent_name} failed: {agent.detail}"


class TestDemoResultCompleteness:
    """Verify DemoResult captures all expected fields."""

    @pytest.mark.asyncio
    async def test_all_fields_present(self) -> None:
        orig = _patch_clients(_resource_transport(), _broker_transport())
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert isinstance(result, DemoResult)
        assert result.mode in ("secure", "insecure")
        assert result.total_time_ms > 0
        assert len(result.agents) == 3
        for agent in result.agents:
            assert agent.agent_name != ""
            assert agent.elapsed_ms >= 0
            assert agent.detail != ""

    @pytest.mark.asyncio
    async def test_timing_is_consistent(self) -> None:
        """Total time should be >= sum of individual agent times."""
        orig = _patch_clients(_resource_transport(), _broker_transport())
        try:
            result = await run_demo(
                mode=ServerMode.insecure,
                insecure_api_key="k",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        agent_sum = sum(a.elapsed_ms for a in result.agents)
        assert result.total_time_ms >= agent_sum * 0.9  # allow small float variance
