"""Demo ticketing MCP server (FastMCP, Apache-2.0). In-memory, seeded, Streamable HTTP.

Declared in tools/tickets/manifest.yaml. get_ticket returns the customer's message body, which
is why the manifest classifies its output as confidential: bodies carry email addresses and
phone numbers, and reading one taints the conversation with touched_pii.
"""

from __future__ import annotations

import itertools
import os
from datetime import datetime, timezone

from fastmcp import FastMCP

mcp = FastMCP("tickets")

_ids = itertools.count(4413)
TICKETS: dict[int, dict] = {
    4410: {"id": 4410, "title": "Cannot reset password", "status": "open", "customer": "Jane Doe <jane.doe@customer.example>", "body": "I tried the reset link twice. Call me on +1 415-555-0134 if needed."},
    4411: {"id": 4411, "title": "Invoice total looks wrong", "status": "open", "customer": "Omar Haddad <omar@customer.example>", "body": "Invoice 2026-10-0042 shows 1,200.50 but the quote said 1,100."},
    4412: {"id": 4412, "title": "Feature request: dark mode", "status": "closed", "customer": "Priya Nair <priya.nair@customer.example>", "body": "Would love a dark mode. Thanks!"},
}


@mcp.tool
def list_tickets(status: str = "open") -> list[dict]:
    """List tickets with id, title and status. status: open | closed | all."""
    return [
        {"id": t["id"], "title": t["title"], "status": t["status"]}
        for t in TICKETS.values()
        if status == "all" or t["status"] == status
    ]


@mcp.tool
def get_ticket(ticket_id: int) -> dict:
    """Fetch one ticket including the customer's message body."""
    t = TICKETS.get(ticket_id)
    if t is None:
        raise ValueError(f"no ticket {ticket_id}")
    return t


@mcp.tool
def create_ticket(title: str, body: str, customer: str = "unknown") -> dict:
    """Create a ticket."""
    tid = next(_ids)
    TICKETS[tid] = {"id": tid, "title": title, "status": "open", "customer": customer, "body": body, "created": datetime.now(timezone.utc).isoformat()}
    return {"id": tid, "status": "open"}


@mcp.tool
def close_ticket(ticket_id: int, resolution: str) -> dict:
    """Close a ticket with a resolution note."""
    t = TICKETS.get(ticket_id)
    if t is None:
        raise ValueError(f"no ticket {ticket_id}")
    t["status"] = "closed"
    t["resolution"] = resolution
    return {"id": ticket_id, "status": "closed"}


if __name__ == "__main__":
    mcp.run(transport="http", host="0.0.0.0", port=int(os.environ.get("PORT", "8000")), path="/mcp")
