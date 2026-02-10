"""Shared fixtures for demo agent tests."""

from __future__ import annotations

import pytest
import httpx

from agents.broker_client import BrokerClient
from agents.agent_base import AgentBase
from resource_server.middleware import ServerMode


# ---------------------------------------------------------------------------
# Mock transport that returns canned broker responses
# ---------------------------------------------------------------------------


class MockTransport(httpx.MockTransport):
    """httpx mock transport with helpers for broker endpoints."""


def _make_mock_transport(
    challenge_nonce: str = "ab" * 32,
    register_resp: dict | None = None,
    validate_resp: dict | None = None,
    delegate_resp: dict | None = None,
    revoke_resp: dict | None = None,
) -> httpx.MockTransport:
    """Create a mock transport that returns canned JSON for each endpoint."""

    if register_resp is None:
        register_resp = {
            "agent_instance_id": "spiffe://agentauth.local/agent/orch1/task1/inst1",
            "access_token": "mock-access-token",
            "expires_in": 300,
            "refresh_after": 240,
        }
    if validate_resp is None:
        validate_resp = {
            "valid": True,
            "agent_id": "spiffe://agentauth.local/agent/orch1/task1/inst1",
            "scope": ["read:Customers:*"],
            "expires_in": 280,
            "delegation_depth": 0,
        }
    if delegate_resp is None:
        delegate_resp = {
            "delegation_token": "mock-deleg-token",
            "chain_hash": "abc123",
            "delegation_depth": 1,
        }
    if revoke_resp is None:
        revoke_resp = {
            "revoked": True,
            "level": "token",
            "target_id": "mock-target",
            "revoked_at": "2026-01-01T00:00:00Z",
        }

    def handler(request: httpx.Request) -> httpx.Response:
        path = request.url.path

        if path == "/v1/challenge" and request.method == "GET":
            return httpx.Response(
                200,
                json={"nonce": challenge_nonce, "expires_at": "2026-01-01T00:01:00Z"},
            )
        if path == "/v1/register" and request.method == "POST":
            return httpx.Response(201, json=register_resp)
        if path == "/v1/token/validate" and request.method == "POST":
            return httpx.Response(200, json=validate_resp)
        if path == "/v1/delegate" and request.method == "POST":
            return httpx.Response(201, json=delegate_resp)
        if path == "/v1/revoke" and request.method == "POST":
            return httpx.Response(200, json=revoke_resp)

        return httpx.Response(404, json={"error": "not found"})

    return httpx.MockTransport(handler)


@pytest.fixture()
def mock_broker_client() -> BrokerClient:
    """BrokerClient backed by a canned mock transport."""
    transport = _make_mock_transport()
    client = httpx.AsyncClient(transport=transport, base_url="http://test-broker")
    return BrokerClient(broker_url="http://test-broker", http_client=client)


@pytest.fixture()
def mock_agent(mock_broker_client: BrokerClient) -> AgentBase:
    """AgentBase in secure mode, wired to mock broker."""
    return AgentBase(
        name="test-agent",
        broker=mock_broker_client,
        resource_url="http://test-resource",
        mode=ServerMode.secure,
    )


@pytest.fixture()
def insecure_agent(mock_broker_client: BrokerClient) -> AgentBase:
    """AgentBase in insecure mode."""
    return AgentBase(
        name="test-agent-insecure",
        broker=mock_broker_client,
        resource_url="http://test-resource",
        mode=ServerMode.insecure,
        insecure_api_key="test-key",
    )
