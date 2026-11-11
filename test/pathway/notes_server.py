"""A second MCP server for the pathway test. It is deliberately NOT onboarded: no manifest, no
pin, no policy data. Through the aggregate it must stay invisible and uncallable."""

from __future__ import annotations

import os

from fastmcp import FastMCP

mcp = FastMCP("notes")


@mcp.tool
def read_note(note_id: int) -> str:
    """Read a note."""
    return f"note {note_id}: nothing to see"


if __name__ == "__main__":
    mcp.run(transport="http", host="127.0.0.1", port=int(os.environ.get("PORT", "18002")), path="/mcp")
