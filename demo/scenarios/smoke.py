"""Gateway and identity smoke test (ticket 03): `python3 demo/scenarios/run.py --smoke`.

Checks, for the tickets server's own route and for the aggregate endpoint:
  - protected-resource metadata is served and names this resource and Keycloak;
  - a request without a token gets 401 with a resource_metadata pointer;
  - a token for the MCP audience is accepted; one for the LLM audience only, or for no gateway
    audience, is rejected;
  - tools/list shows the tickets tools (prefixed with the server name on the aggregate) and a
    tools/call returns the seeded tickets.
"""

from __future__ import annotations

import os

import httpx
from run import GATEWAY, Report, Session, user_token

SERVER = "tickets"
TOOLS = {"list_tickets", "get_ticket", "create_ticket", "close_ticket"}
# The aggregate prefixes tool names with the target name, which the chart sets to the server
# name (Service annotation agentgateway.dev/mcp-target-name; prefixMode Always).
PREFIX = f"{SERVER}_"
INIT = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "eacp-smoke", "version": "0"}},
}


def initialize_status(path: str, token: str | None) -> httpx.Response:
    headers = {"Accept": "application/json, text/event-stream", "Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return httpx.post(f"{GATEWAY}{path}", headers=headers, json=INIT, timeout=10)


def check_metadata(rep: Report, path: str) -> None:
    resp = httpx.get(f"{GATEWAY}/.well-known/oauth-protected-resource{path}", timeout=10)
    body = resp.json() if resp.status_code == 200 else {}
    ok = body.get("resource") == f"{GATEWAY}{path}" and bool(body.get("authorization_servers"))
    rep.add(f"{path}: protected-resource metadata", ok, f"{resp.status_code} {body.get('authorization_servers')}")
    resp = initialize_status(path, None)
    challenge = resp.headers.get("www-authenticate", "")
    rep.add(f"{path}: no token is 401 with resource_metadata", resp.status_code == 401 and "resource_metadata" in challenge, challenge)


def check_audiences(rep: Report, path: str, tokens: dict[str, str]) -> None:
    for name, token in tokens.items():
        status = initialize_status(path, token).status_code
        want = 200 if name == "eacp-mcp" else 401
        rep.add(f"{path}: token for audience {name} gets {want}", status == want, f"got {status}")


async def check_tools(rep: Report, path: str, token: str, prefix: str) -> None:
    session = Session(token, path=path)
    names = set(await session.tools())
    want = {prefix + t for t in TOOLS}
    rep.add(f"{path}: tools/list shows the tickets tools", want <= names, ", ".join(sorted(names)))
    err, text, _ = await session.call(f"{prefix}list_tickets", {"status": "all"})
    rep.add(f"{path}: tools/call {prefix}list_tickets", not err and "4410" in text, text[:80].replace("\n", " "))


async def smoke() -> int:
    rep = Report()
    password = os.environ.get("EACP_ALICE_PASSWORD", "changeme-alice")
    tokens = {
        "eacp-mcp": user_token("alice", password, ("eacp-mcp",)),
        "eacp-llm": user_token("alice", password, ("eacp-llm",)),
        "none": user_token("alice", password, ()),
    }
    for path, prefix in ((f"/mcp/{SERVER}", ""), ("/mcp", PREFIX)):
        check_metadata(rep, path)
        check_audiences(rep, path, tokens)
        await check_tools(rep, path, tokens["eacp-mcp"], prefix)
    print(f"\n{sum(r.ok for r in rep.results)}/{len(rep.results)} passed")
    return 1 if rep.failed else 0
