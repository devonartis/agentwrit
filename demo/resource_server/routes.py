"""Route handlers for the resource server endpoints.

Endpoints:
  GET  /customers/{id}           — read:Customers:{id}
  GET  /orders/{customer_id}     — read:Orders:{customer_id}
  PUT  /tickets/{id}             — write:Tickets:{id}
  POST /notifications/send       — invoke:Notifications
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone

from fastapi import APIRouter, HTTPException

from resource_server.models import (
    Customer,
    NotificationRequest,
    NotificationResponse,
    Order,
    Problem,
    Ticket,
    TicketUpdate,
)
from resource_server.seed_data import CUSTOMERS, ORDERS, TICKETS

router = APIRouter()


def _problem(status: int, title: str, detail: str = "") -> dict:
    """Return an RFC 7807 problem dict."""
    return Problem(
        type=f"urn:agentauth:resource:{status}",
        title=title,
        status=status,
        detail=detail,
    ).model_dump()


# ---------- GET /customers/{id} ----------


@router.get("/customers/{customer_id}")
def get_customer(customer_id: int) -> Customer:
    """Retrieve a customer record by ID."""
    customer = CUSTOMERS.get(customer_id)
    if customer is None:
        raise HTTPException(
            status_code=404,
            detail=_problem(
                404, "Customer not found", f"No customer with id {customer_id}"
            ),
        )
    return customer


# ---------- GET /orders/{customer_id} ----------


@router.get("/orders/{customer_id}")
def get_orders(customer_id: int) -> list[Order]:
    """Retrieve all orders for a customer."""
    if customer_id not in CUSTOMERS:
        raise HTTPException(
            status_code=404,
            detail=_problem(
                404, "Customer not found", f"No customer with id {customer_id}"
            ),
        )
    return ORDERS.get(customer_id, [])


# ---------- PUT /tickets/{id} ----------


@router.put("/tickets/{ticket_id}")
def update_ticket(ticket_id: int, body: TicketUpdate) -> Ticket:
    """Update a support ticket's status or assignee."""
    ticket = TICKETS.get(ticket_id)
    if ticket is None:
        raise HTTPException(
            status_code=404,
            detail=_problem(404, "Ticket not found", f"No ticket with id {ticket_id}"),
        )
    if body.status is not None:
        ticket.status = body.status
    if body.assignee is not None:
        ticket.assignee = body.assignee
    ticket.updated_at = datetime.now(timezone.utc).isoformat()
    return ticket


# ---------- POST /notifications/send ----------


@router.post("/notifications/send")
def send_notification(body: NotificationRequest) -> NotificationResponse:
    """Simulate sending a notification to a customer."""
    if body.customer_id not in CUSTOMERS:
        raise HTTPException(
            status_code=404,
            detail=_problem(
                404, "Customer not found", f"No customer with id {body.customer_id}"
            ),
        )
    return NotificationResponse(
        sent=True,
        notification_id=str(uuid.uuid4()),
        customer_id=body.customer_id,
        channel=body.channel,
        timestamp=datetime.now(timezone.utc).isoformat(),
    )
