"""Tests for Agent B -- Analyzer."""

from __future__ import annotations

import httpx
import pytest

from agents.agent_analyzer import Analyzer, AnalyzerResult
from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode


# ── helpers ────────────────────────────────────────────────────────────────

CUSTOMER_DATA = {"customer_id": "12345", "name": "Acme Corp", "tier": "enterprise"}
ORDERS_DATA = {
    "customer_id": "12345",
    "orders": [
        {"order_id": "ORD-001", "amount": 500},
        {"order_id": "ORD-002", "amount": 1200},
    ],
}


def _resource_transport():
    def handler(request: httpx.Request) -> httpx.Response:
        if "/orders/" in str(request.url):
            return httpx.Response(200, json=ORDERS_DATA)
        return httpx.Response(404, json={"error": "not found"})

    return httpx.MockTransport(handler)


def _patch_httpx_client(transport):
    import httpx as _httpx

    orig = _httpx.AsyncClient

    class Patched(_httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    _httpx.AsyncClient = Patched
    return orig


# ── secure mode ────────────────────────────────────────────────────────────


class TestAnalyzerSecure:
    """Secure mode: register, fetch orders, delegate, return result."""

    @pytest.mark.asyncio
    async def test_run_with_delegation(self, mock_broker_client: BrokerClient) -> None:
        agent = Analyzer(
            name="Agent-B",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        orig = _patch_httpx_client(_resource_transport())
        try:
            result = await agent.run(
                customer_data=CUSTOMER_DATA,
                customer_id=12345,
                ticket_id=789,
                launch_token="lt-123",
                orch_id="orch1",
                task_id="task1",
                agent_c_id="spiffe://agentauth.local/agent/orch1/task1/agent-c",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert isinstance(result, AnalyzerResult)
        assert "12345" in result.resolution_text
        assert "enterprise" in result.resolution_text
        assert len(result.orders) == 2
        assert result.delegation_token == "mock-deleg-token"
        assert result.delegation_depth == 1

    @pytest.mark.asyncio
    async def test_run_without_agent_c_skips_delegation(
        self, mock_broker_client: BrokerClient
    ) -> None:
        agent = Analyzer(
            name="Agent-B",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        orig = _patch_httpx_client(_resource_transport())
        try:
            result = await agent.run(
                customer_data=CUSTOMER_DATA,
                customer_id=12345,
                ticket_id=789,
                launch_token="lt-123",
                orch_id="orch1",
                task_id="task1",
                agent_c_id="",  # no delegation target
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert result.delegation_token == ""
        assert result.delegation_depth == 0


# ── insecure mode ──────────────────────────────────────────────────────────


class TestAnalyzerInsecure:
    """Insecure mode: skip registration and delegation."""

    @pytest.mark.asyncio
    async def test_run_no_registration(self, mock_broker_client: BrokerClient) -> None:
        agent = Analyzer(
            name="Agent-B",
            broker=mock_broker_client,
            mode=ServerMode.insecure,
            insecure_api_key="dev-key",
        )
        orig = _patch_httpx_client(_resource_transport())
        try:
            result = await agent.run(
                customer_data=CUSTOMER_DATA,
                customer_id=12345,
                ticket_id=789,
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        assert result.resolution_text != ""
        assert result.delegation_token == ""
        # Should NOT have registered
        assert agent.agent_instance_id == ""


# ── scope delegation subset ───────────────────────────────────────────────


class TestAnalyzerScopeAttenuation:
    """Verify delegated scope is a subset of the agent's own scope."""

    @pytest.mark.asyncio
    async def test_delegated_scope_is_attenuated(
        self, mock_broker_client: BrokerClient
    ) -> None:
        """The delegation call should request only write:Tickets + invoke:Notifications,
        which is a strict subset of the agent's own read + write + invoke scope."""
        captured_body: dict = {}

        # Wrap the broker client to capture the delegate call body
        orig_delegate = mock_broker_client.delegate

        async def spy_delegate(**kw):
            captured_body.update(kw)
            return await orig_delegate(**kw)

        mock_broker_client.delegate = spy_delegate  # type: ignore[assignment]

        agent = Analyzer(
            name="Agent-B",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        orig = _patch_httpx_client(_resource_transport())
        try:
            await agent.run(
                customer_data=CUSTOMER_DATA,
                customer_id=12345,
                ticket_id=789,
                launch_token="lt",
                orch_id="o",
                task_id="t",
                agent_c_id="spiffe://agentauth.local/agent/o/t/c",
            )
        finally:
            import httpx as _httpx
            _httpx.AsyncClient = orig

        # The delegated scope should be write + invoke only
        delegated = set(captured_body["scopes"])
        assert delegated == {"write:Tickets:789", "invoke:Notifications"}
