"""Tests for Agent C -- ActionTaker."""

from __future__ import annotations

import httpx
import pytest

from agents.agent_action import ActionTaker, ActionResult
from agents.broker_client import BrokerClient
from resource_server.middleware import ServerMode


# ── helpers ────────────────────────────────────────────────────────────────

TICKET_RESP = {"ticket_id": "789", "status": "resolved"}
NOTIF_RESP = {"sent": True, "message_id": "msg-001"}


def _resource_transport(
    ticket_status: int = 200,
    notif_status: int = 200,
) -> httpx.MockTransport:
    """Mock transport for ticket update + notification send."""

    captured: dict = {"calls": []}

    def handler(request: httpx.Request) -> httpx.Response:
        path = str(request.url.path)
        captured["calls"].append({"method": request.method, "path": path})

        if "/tickets/" in path and request.method == "PUT":
            return httpx.Response(ticket_status, json=TICKET_RESP)
        if "/notifications/send" in path and request.method == "POST":
            return httpx.Response(notif_status, json=NOTIF_RESP)
        return httpx.Response(404, json={"error": "not found"})

    transport = httpx.MockTransport(handler)
    transport._captured = captured  # type: ignore[attr-defined]
    return transport


def _make_patched_client(transport):
    """Create a patched AsyncClient class that uses the given transport."""

    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# ── secure mode ────────────────────────────────────────────────────────────


class TestActionTakerSecure:
    """Secure mode: uses delegation token, no registration."""

    @pytest.mark.asyncio
    async def test_run_closes_ticket_and_notifies(
        self, mock_broker_client: BrokerClient, monkeypatch,
    ) -> None:
        agent = ActionTaker(
            name="Agent-C",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(_resource_transport()))
        result = await agent.run(
            ticket_id=789,
            customer_id=12345,
            resolution="Issue resolved per analysis.",
            delegation_token="deleg-tok-from-B",
        )

        assert isinstance(result, ActionResult)
        assert result.ticket_updated is True
        assert result.notification_sent is True
        assert result.ticket_response == TICKET_RESP
        assert result.notification_response == NOTIF_RESP
        # Should use the delegation token, not register
        assert agent.access_token == "deleg-tok-from-B"
        assert agent.agent_instance_id == ""  # never registered

    @pytest.mark.asyncio
    async def test_bearer_header_uses_delegation_token(
        self, mock_broker_client: BrokerClient, monkeypatch,
    ) -> None:
        """Verify the actual HTTP header carries the delegated token."""
        captured_headers: list[str] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured_headers.append(request.headers.get("authorization", ""))
            return httpx.Response(200, json={"ok": True})

        agent = ActionTaker(
            name="Agent-C",
            broker=mock_broker_client,
            mode=ServerMode.secure,
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(httpx.MockTransport(handler)))
        await agent.run(
            ticket_id=789,
            customer_id=12345,
            resolution="fixed",
            delegation_token="my-deleg-token",
        )

        assert all(h == "Bearer my-deleg-token" for h in captured_headers)


# ── insecure mode ──────────────────────────────────────────────────────────


class TestActionTakerInsecure:
    """Insecure mode: uses API-Key header, same operations."""

    @pytest.mark.asyncio
    async def test_run_with_api_key(self, mock_broker_client: BrokerClient, monkeypatch) -> None:
        captured_headers: list[str] = []

        def handler(request: httpx.Request) -> httpx.Response:
            captured_headers.append(request.headers.get("api-key", ""))
            return httpx.Response(200, json={"ok": True})

        agent = ActionTaker(
            name="Agent-C",
            broker=mock_broker_client,
            mode=ServerMode.insecure,
            insecure_api_key="dev-key",
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(httpx.MockTransport(handler)))
        result = await agent.run(
            ticket_id=789,
            customer_id=12345,
            resolution="fixed",
        )

        assert result.ticket_updated is True
        assert result.notification_sent is True
        assert all(h == "dev-key" for h in captured_headers)


# ── error handling ─────────────────────────────────────────────────────────


class TestActionTakerErrors:
    """Verify errors from resource server propagate correctly."""

    @pytest.mark.asyncio
    async def test_ticket_not_found(self, mock_broker_client: BrokerClient, monkeypatch) -> None:
        agent = ActionTaker(
            name="Agent-C",
            broker=mock_broker_client,
            mode=ServerMode.insecure,
        )
        monkeypatch.setattr(httpx, "AsyncClient", _make_patched_client(_resource_transport(ticket_status=404)))
        with pytest.raises(httpx.HTTPStatusError) as exc_info:
            await agent.run(
                ticket_id=99999,
                customer_id=12345,
                resolution="fixed",
            )

        assert exc_info.value.response.status_code == 404
