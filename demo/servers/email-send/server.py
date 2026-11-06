"""Demo email MCP server (FastMCP). Delivers to Mailpit over SMTP so the demo stays offline.

Declared in tools/email-send/manifest.yaml as high risk and exfiltration-capable with approval
required. The server itself does no checking: that is the point. Every control is in the
control plane, which is why this server can be third-party code.
"""

from __future__ import annotations

import os
import smtplib
import uuid
from email.message import EmailMessage

from fastmcp import FastMCP

mcp = FastMCP("email-send")

SMTP_HOST = os.environ.get("SMTP_HOST", "mailpit")
SMTP_PORT = int(os.environ.get("SMTP_PORT", "1025"))
FROM = os.environ.get("SMTP_FROM", "support@example.com")


@mcp.tool
def send_email(to: str, subject: str, body: str) -> dict:
    """Send an email to a customer."""
    msg = EmailMessage()
    msg["From"] = FROM
    msg["To"] = to
    msg["Subject"] = subject
    msg["Message-ID"] = f"<{uuid.uuid4()}@example.com>"
    msg.set_content(body)
    with smtplib.SMTP(SMTP_HOST, SMTP_PORT, timeout=10) as s:
        s.send_message(msg)
    return {"message_id": msg["Message-ID"], "to": to}


if __name__ == "__main__":
    mcp.run(transport="http", host="0.0.0.0", port=int(os.environ.get("PORT", "8000")), path="/mcp")
