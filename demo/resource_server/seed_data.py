"""Pre-seeded sample data for the demo resource server.

Contains 5 customers, 10 orders, and 3 tickets as specified in MVP Requirements 5.4.
"""

from __future__ import annotations

from resource_server.models import Customer, Order, Ticket, TicketStatus

# ---------- Customers (5) ----------

CUSTOMERS: dict[int, Customer] = {
    c.id: c
    for c in [
        Customer(
            id=12345,
            name="Alice Johnson",
            email="alice@example.com",
            phone="+1-555-0101",
            tier="premium",
        ),
        Customer(
            id=12346,
            name="Bob Smith",
            email="bob@example.com",
            phone="+1-555-0102",
            tier="standard",
        ),
        Customer(
            id=12347,
            name="Carol Davis",
            email="carol@example.com",
            phone="+1-555-0103",
            tier="enterprise",
        ),
        Customer(
            id=12348,
            name="Dan Wilson",
            email="dan@example.com",
            phone="+1-555-0104",
            tier="standard",
        ),
        Customer(
            id=12349,
            name="Eve Martinez",
            email="eve@example.com",
            phone="+1-555-0105",
            tier="premium",
        ),
    ]
}

# ---------- Orders (10) ----------

ORDERS: dict[int, list[Order]] = {}

_raw_orders = [
    Order(id=1001, customer_id=12345, product="Widget A", amount=29.99),
    Order(id=1002, customer_id=12345, product="Widget B", amount=49.99),
    Order(id=1003, customer_id=12346, product="Gadget X", amount=99.50),
    Order(id=1004, customer_id=12346, product="Gadget Y", amount=149.00),
    Order(id=1005, customer_id=12347, product="Service Plan Pro", amount=299.00),
    Order(id=1006, customer_id=12347, product="Add-On Pack", amount=59.00),
    Order(
        id=1007, customer_id=12348, product="Widget A", amount=29.99, status="pending"
    ),
    Order(id=1008, customer_id=12348, product="Widget C", amount=39.99),
    Order(id=1009, customer_id=12349, product="Enterprise License", amount=999.00),
    Order(id=1010, customer_id=12349, product="Support Upgrade", amount=199.00),
]

for _o in _raw_orders:
    ORDERS.setdefault(_o.customer_id, []).append(_o)

# ---------- Tickets (3) ----------

TICKETS: dict[int, Ticket] = {
    t.id: t
    for t in [
        Ticket(
            id=789,
            customer_id=12345,
            subject="Billing discrepancy on last invoice",
            status=TicketStatus.open,
        ),
        Ticket(
            id=790,
            customer_id=12347,
            subject="Service Plan upgrade request",
            status=TicketStatus.in_progress,
            assignee="support-team",
        ),
        Ticket(
            id=791,
            customer_id=12349,
            subject="License activation issue",
            status=TicketStatus.open,
        ),
    ]
}
