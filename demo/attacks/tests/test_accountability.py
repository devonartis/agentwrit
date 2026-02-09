"""Tests for Attack 5: Accountability Check."""

from __future__ import annotations

import httpx
import pytest

from attacks.accountability import accountability_check
from attacks.models import AttackResult


# -- helpers ----------------------------------------------------------------


def _audit_transport_with_events():
    """Mock: broker returns audit events with agent attribution."""

    events = [
        {
            "event_type": "token_issued",
            "agent_id": "spiffe://agentauth.local/agent/orch1/task1/inst1",
            "action": "token_issued",
            "timestamp": "2026-01-15T10:30:00Z",
        },
        {
            "event_type": "resource_accessed",
            "agent_id": "spiffe://agentauth.local/agent/orch1/task1/inst1",
            "action": "read:Customers:12345",
            "timestamp": "2026-01-15T10:30:05Z",
        },
    ]

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/v1/audit/events":
            auth = request.headers.get("authorization", "")
            if auth.startswith("Bearer "):
                return httpx.Response(200, json={"events": events})
            return httpx.Response(401, json={"title": "Missing bearer token"})
        return httpx.Response(404, json={"title": "Not found"})

    return httpx.MockTransport(handler)


def _audit_transport_empty():
    """Mock: broker returns empty audit log."""

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path == "/v1/audit/events":
            return httpx.Response(200, json={"events": []})
        return httpx.Response(404, json={"title": "Not found"})

    return httpx.MockTransport(handler)


def _audit_transport_error():
    """Mock: broker returns 500 for audit queries."""

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"title": "Internal error"})

    return httpx.MockTransport(handler)


def _patched_client_class(transport):
    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# -- insecure mode tests ----------------------------------------------------


class TestAccountabilityInsecure:
    """Insecure mode: no audit trail, attacker evades attribution."""

    @pytest.mark.asyncio
    async def test_no_attribution(self) -> None:
        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token=None,
            mode="insecure",
        )

        assert result.name == "accountability"
        assert result.mode == "insecure"
        assert result.attempts == 1
        assert result.successes == 1  # evasion succeeded
        assert result.blocked == 0
        assert result.attack_succeeded is True

    @pytest.mark.asyncio
    async def test_details_explain_gap(self) -> None:
        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token=None,
            mode="insecure",
        )

        assert any("No audit trail" in d for d in result.details)
        assert any("Cannot determine" in d for d in result.details)


# -- secure mode tests -----------------------------------------------------


class TestAccountabilitySecure:
    """Secure mode: broker audit trail provides full attribution."""

    @pytest.mark.asyncio
    async def test_attribution_found(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_audit_transport_with_events())
        )

        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token="admin-token",
            mode="secure",
        )

        assert result.attempts == 1
        assert result.successes == 0
        assert result.blocked == 1  # evasion blocked
        assert result.attack_succeeded is False

    @pytest.mark.asyncio
    async def test_details_include_events(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_audit_transport_with_events())
        )

        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token="admin-token",
            mode="secure",
        )

        assert any("2 event(s)" in d for d in result.details)
        assert any("spiffe://" in d for d in result.details)

    @pytest.mark.asyncio
    async def test_empty_audit_means_evasion(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_audit_transport_empty())
        )

        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token="admin-token",
            mode="secure",
        )

        assert result.successes == 1  # evasion succeeded (no events)
        assert result.attack_succeeded is True
        assert any("empty" in d for d in result.details)

    @pytest.mark.asyncio
    async def test_audit_error_means_evasion(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_audit_transport_error())
        )

        result = await accountability_check(
            broker_url="http://test-broker",
            admin_token="admin-token",
            mode="secure",
        )

        assert result.successes == 1
        assert result.attack_succeeded is True
        assert any("500" in d for d in result.details)
