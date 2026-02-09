"""Tests for Attack 4: Privilege Escalation."""

from __future__ import annotations

import httpx
import pytest

from attacks.privilege_escalation import escalation_attack
from attacks.models import AttackResult


# -- helpers ----------------------------------------------------------------


def _insecure_transport():
    """Mock: insecure resource server accepts any API-Key."""

    def handler(request: httpx.Request) -> httpx.Response:
        if request.headers.get("api-key"):
            return httpx.Response(200, json={"customer_id": "12345", "name": "Acme"})
        return httpx.Response(401, json={"title": "Missing API key"})

    return httpx.MockTransport(handler)


def _secure_transport():
    """Mock: broker rejects escalation (403); resource server rejects scope (403)."""

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path
        auth = request.headers.get("authorization", "")

        if not auth.startswith("Bearer "):
            return httpx.Response(401, json={"title": "Missing bearer token"})

        # Broker delegation endpoint: reject scope escalation
        if path == "/v1/delegate" and request.method == "POST":
            return httpx.Response(
                403,
                json={
                    "type": "urn:agentauth:error:scope_attenuation",
                    "title": "Scope attenuation violation",
                    "status": 403,
                    "detail": "Cannot delegate scope not held by delegator",
                },
            )

        # Resource server: reject direct access (scope mismatch)
        if path.startswith("/customers/"):
            return httpx.Response(
                403,
                json={
                    "type": "urn:agentauth:resource:403",
                    "title": "Scope mismatch",
                    "status": 403,
                },
            )

        return httpx.Response(404, json={"title": "Not found"})

    return httpx.MockTransport(handler)


def _patched_client_class(transport):
    class Patched(httpx.AsyncClient):
        def __init__(self, **kw):
            kw["transport"] = transport
            super().__init__(**kw)

    return Patched


# -- insecure mode tests ----------------------------------------------------


class TestEscalationInsecure:
    """Insecure mode: no delegation system, direct access succeeds."""

    @pytest.mark.asyncio
    async def test_direct_access_succeeds(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await escalation_attack(
            agent_credential="shared-api-key",
            broker_url="http://test-broker",
            resource_url="http://test-resource",
            mode="insecure",
        )

        assert result.name == "privilege_escalation"
        assert result.mode == "insecure"
        assert result.attempts == 1  # Only direct access, no delegation
        assert result.successes == 1
        assert result.blocked == 0
        assert result.attack_succeeded is True

    @pytest.mark.asyncio
    async def test_delegation_skipped_in_insecure(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_insecure_transport())
        )

        result = await escalation_attack(
            agent_credential="shared-api-key",
            broker_url="http://test-broker",
            resource_url="http://test-resource",
            mode="insecure",
        )

        skip_details = [d for d in result.details if "SKIPPED" in d]
        assert len(skip_details) == 1


# -- secure mode tests ------------------------------------------------------


class TestEscalationSecure:
    """Secure mode: both delegation escalation and direct access blocked."""

    @pytest.mark.asyncio
    async def test_both_attempts_blocked(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await escalation_attack(
            agent_credential="agent-c-token",
            broker_url="http://test-broker",
            resource_url="http://test-resource",
            mode="secure",
        )

        assert result.attempts == 2
        assert result.successes == 0
        assert result.blocked == 2
        assert result.attack_succeeded is False

    @pytest.mark.asyncio
    async def test_delegation_denied_403(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await escalation_attack(
            agent_credential="agent-c-token",
            broker_url="http://test-broker",
            resource_url="http://test-resource",
            mode="secure",
        )

        deleg_details = [d for d in result.details if "DELEGATE" in d]
        assert len(deleg_details) == 1
        assert "DENIED" in deleg_details[0]
        assert "403" in deleg_details[0]

    @pytest.mark.asyncio
    async def test_direct_access_denied_403(self, monkeypatch) -> None:
        monkeypatch.setattr(
            httpx, "AsyncClient", _patched_client_class(_secure_transport())
        )

        result = await escalation_attack(
            agent_credential="agent-c-token",
            broker_url="http://test-broker",
            resource_url="http://test-resource",
            mode="secure",
        )

        access_details = [d for d in result.details if "GET /customers" in d]
        assert len(access_details) == 1
        assert "BLOCKED" in access_details[0]
        assert "403" in access_details[0]
