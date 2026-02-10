"""Pydantic models for the resource server API."""

from __future__ import annotations

from datetime import datetime
from enum import Enum

from pydantic import BaseModel, Field


# ---------- Domain models ----------


class Customer(BaseModel):
    """A customer record."""

    id: int
    name: str
    email: str
    phone: str
    tier: str = "standard"


class Order(BaseModel):
    """An order belonging to a customer."""

    id: int
    customer_id: int
    product: str
    amount: float
    status: str = "completed"
    created_at: str = "2026-01-15T10:00:00Z"


class TicketStatus(str, Enum):
    """Status values for a support ticket."""

    open = "open"
    in_progress = "in_progress"
    closed = "closed"


class Ticket(BaseModel):
    """A support ticket."""

    id: int
    customer_id: int
    subject: str
    status: TicketStatus = TicketStatus.open
    assignee: str | None = None
    updated_at: str = "2026-01-15T10:00:00Z"


class TicketUpdate(BaseModel):
    """Request body for updating a ticket."""

    status: TicketStatus | None = None
    assignee: str | None = Field(default=None, max_length=200)


class NotificationRequest(BaseModel):
    """Request body for sending a notification."""

    customer_id: int
    message: str = Field(max_length=2000)
    channel: str = Field(default="email", max_length=100)


class NotificationResponse(BaseModel):
    """Response confirming notification delivery."""

    sent: bool = True
    notification_id: str
    customer_id: int
    channel: str
    timestamp: str


# ---------- RFC 7807 Problem ----------


class Problem(BaseModel):
    """RFC 7807 problem+json error response."""

    type: str = "about:blank"
    title: str
    status: int
    detail: str = ""
    instance: str = ""
