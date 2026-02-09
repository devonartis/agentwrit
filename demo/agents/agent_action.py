"""Agent C -- ActionTaker: closes ticket and sends notification via delegation."""

from __future__ import annotations

import logging
from dataclasses import dataclass, field

from agents.agent_base import AgentBase
from resource_server.middleware import ServerMode

logger = logging.getLogger(__name__)


@dataclass
class ActionResult:
    """Outcome produced by the ActionTaker agent."""

    ticket_updated: bool = False
    notification_sent: bool = False
    ticket_response: dict = field(default_factory=dict)
    notification_response: dict = field(default_factory=dict)


@dataclass
class ActionTaker(AgentBase):
    """Closes a ticket and sends a notification using a delegated token.

    In secure mode, Agent C does NOT register with the broker itself.
    Instead it uses the delegation token issued by Agent B, which
    carries only ``write:Tickets:{id}`` and ``invoke:Notifications``
    scope -- demonstrating least-privilege delegation.
    """

    async def run(
        self,
        ticket_id: int,
        customer_id: int,
        resolution: str,
        delegation_token: str = "",
        orch_id: str = "",
        task_id: str = "",
    ) -> ActionResult:
        """Execute the action workflow.

        Parameters
        ----------
        ticket_id:
            The ticket to close.
        customer_id:
            Customer to notify.
        resolution:
            Resolution text produced by Agent B.
        delegation_token:
            Token delegated from Agent B (secure mode).
        """
        # In secure mode, use the delegation token directly (no fresh registration)
        if self.mode == ServerMode.secure:
            self.access_token = delegation_token
            logger.info("[%s] Using delegated token (no registration)", self.name)

        # 1 -- Update ticket
        logger.info("[%s] Closing ticket #%d...", self.name, ticket_id)
        ticket_resp = await self.call_resource(
            "PUT",
            f"/tickets/{ticket_id}",
            json={"status": "resolved", "assignee": self.name, "resolution": resolution},
        )
        logger.info("[%s] Ticket #%d closed", self.name, ticket_id)

        # 2 -- Send notification
        logger.info("[%s] Sending notification to customer #%d...", self.name, customer_id)
        notif_resp = await self.call_resource(
            "POST",
            "/notifications/send",
            json={"customer_id": customer_id, "message": resolution},
        )
        logger.info("[%s] Notification sent", self.name)

        return ActionResult(
            ticket_updated=True,
            notification_sent=True,
            ticket_response=ticket_resp,
            notification_response=notif_resp,
        )
