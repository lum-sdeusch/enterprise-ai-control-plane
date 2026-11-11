#!/usr/bin/env python3
"""The six demo scenarios as an executable test against a running cluster (scripts/e2e.sh).

Each scenario drives the gateway the way a client would and asserts the control plane's
response: the eacp_code the adapter puts in the JSON-RPC error data, the tools visible in a
listing, or which model answered. Scenarios print PASS/FAIL and the script exits non-zero on
any failure, so CI cannot be green by accident.

Requires: a reachable gateway (EACP_GATEWAY_URL), Keycloak (EACP_KEYCLOAK_TOKEN_URL) with the
demo users from the generated realm, the demo-runner client secret (EACP_RUNNER_CLIENT_SECRET,
from `scripts/seed-openbao.sh runner-secret`), and the `mcp` and `httpx` packages.

`run.py --smoke` runs only the gateway and identity checks of ticket 03 (demo/scenarios/smoke.py).
`run.py --pathway` runs the adapter-in-the-pathway checks of ticket 04 (demo/scenarios/pathway.py).
TODO(verify): the `mcp` SDK v2 client API for spec 2026-07-28 (handshake-free sessions).
"""

from __future__ import annotations

import asyncio
import json
import os
import sys
import uuid
from dataclasses import dataclass, field

import httpx
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from mcp.shared.exceptions import McpError

GATEWAY = os.environ.get("EACP_GATEWAY_URL", "http://gw.127.0.0.1.sslip.io:8080").rstrip("/")
TOKEN_URL = os.environ.get("EACP_KEYCLOAK_TOKEN_URL", "https://auth.127.0.0.1.sslip.io:8081/realms/eacp/protocol/openid-connect/token")
RUNNER_CLIENT_ID = os.environ.get("EACP_RUNNER_CLIENT_ID", "demo-runner")
RUNNER_CLIENT_SECRET = os.environ.get("EACP_RUNNER_CLIENT_SECRET", "")
# Keycloak's certificate is signed by the laptop CA (scripts/up.sh writes .eacp/ca.pem); unset
# means the system trust store (cloud-gpu).
CA_FILE: str | bool = os.environ.get("EACP_CA_FILE") or True


@dataclass
class Result:
    name: str
    ok: bool
    detail: str = ""


@dataclass
class Report:
    results: list[Result] = field(default_factory=list)

    def add(self, name: str, ok: bool, detail: str = "") -> None:
        self.results.append(Result(name, ok, detail))
        print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")

    @property
    def failed(self) -> bool:
        return any(not r.ok for r in self.results)


def user_token(username: str, password: str, audiences: tuple[str, ...] = ("eacp-mcp", "eacp-llm")) -> str:
    """Resource-owner password grant on the demo-runner client (laptop profile only). The
    gateway audiences are optional scopes on that client, so the token carries exactly the
    audiences asked for."""
    scope = " ".join(["openid", "eacp", *(f"{a}-audience" for a in audiences)])
    resp = httpx.post(
        TOKEN_URL,
        data={"grant_type": "password", "client_id": RUNNER_CLIENT_ID, "client_secret": RUNNER_CLIENT_SECRET, "username": username, "password": password, "scope": scope},
        timeout=10,
        verify=CA_FILE,
    )
    resp.raise_for_status()
    return resp.json()["access_token"]


def _protocol_error(exc: BaseException) -> tuple[bool, str, dict]:
    """The JSON-RPC error behind an SDK exception, which may be nested in task groups."""
    stack = [exc]
    while stack:
        e = stack.pop()
        if isinstance(e, McpError):
            return True, e.error.message, e.error.data if isinstance(e.error.data, dict) else {}
        stack.extend(getattr(e, "exceptions", []))
    if isinstance(exc, (KeyboardInterrupt, SystemExit)):
        raise exc
    return True, f"{type(exc).__name__}: {exc}", {}


class Session:
    """One caller on one MCP endpoint: the aggregate /mcp by default, or a per-server /mcp/<name>.

    How a policy denial arrives depends on the method (agentgateway 1.7; test/pathway checks it):
    a denied tools/call is a tool result with isError true whose text is the JSON object
    {"error": "eacp_policy", "message": ..., "data": {"eacp_code": ..., ...}}; a denied listing is
    a JSON-RPC error whose data member is that data object. Both end up in the third element of
    what call() returns, or in the dict tools() returns instead of a list.
    """

    def __init__(self, token: str, conversation_id: str | None = None, path: str = "/mcp"):
        self.conversation = conversation_id or f"demo:{uuid.uuid4()}"
        self.trace_id = uuid.uuid4().hex
        self.headers = {
            "Authorization": f"Bearer {token}",
            "x-eacp-conversation-id": self.conversation,
            "traceparent": f"00-{self.trace_id}-{uuid.uuid4().hex[:16]}-01",
        }
        self.url = f"{GATEWAY}{path}"

    async def _with_session(self, fn):
        async with streamablehttp_client(self.url, headers=self.headers) as (r, w, _):
            async with ClientSession(r, w) as s:
                await s.initialize()
                return await fn(s)

    async def tools(self) -> list[str] | dict:
        """The listed tool names, or the denial's data when the listing was refused."""
        try:
            res = await self._with_session(lambda s: s.list_tools())
            return [t.name for t in res.tools]
        except BaseException as e:  # noqa: BLE001 - the SDK wraps protocol errors in groups
            _, message, data = _protocol_error(e)
            return {"message": message, **data}

    async def call(self, name: str, args: dict) -> tuple[bool, str, dict]:
        """Returns (is_error, text, eacp_data)."""
        try:
            res = await self._with_session(lambda s: s.call_tool(name, args))
        except BaseException as e:  # noqa: BLE001
            return _protocol_error(e)
        text = "\n".join(getattr(c, "text", "") for c in res.content)
        if res.isError:
            try:
                body = json.loads(text)
                if isinstance(body, dict) and body.get("error") == "eacp_policy":
                    return True, body.get("message", ""), body.get("data", {})
            except ValueError:
                pass
        return bool(res.isError), text, {}

    async def chat(self, prompt: str, model: str = "eacp-auto") -> tuple[int, dict]:
        async with httpx.AsyncClient(timeout=120) as c:
            resp = await c.post(f"{GATEWAY}/v1/chat/completions", headers=self.headers, json={"model": model, "messages": [{"role": "user", "content": prompt}]})
            try:
                return resp.status_code, resp.json()
            except ValueError:
                return resp.status_code, {"raw": resp.text}


async def scenario_1_rugpull(rep: Report, alice: Session) -> None:
    tools = await alice.tools()
    rep.add("1a benign poisoned-demo is listed before the rug-pull", "poisoned-demo_summarize" in tools)
    print("     -> redeploy tools/poisoned-demo with POISON=1 now (scripts/e2e.sh does this), then:")
    tools = await alice.tools()
    rep.add("1b poisoned server disappears from tools/list", "poisoned-demo_summarize" not in tools)
    err, _, data = await alice.call("poisoned-demo_summarize", {"text": "hello"})
    rep.add("1c calls to the quarantined server are denied", err and data.get("eacp_code") == "server_quarantined", str(data))


async def scenario_2_injection(rep: Report, alice: Session) -> None:
    err, text, data = await alice.call("web-fetch_fetch", {"url": "https://docs.example.com/kb/injected"})
    rep.add("2 injected tool result is blocked", err and data.get("eacp_code") == "injection_in_tool_result", str(data))


async def scenario_3_taint(rep: Report, alice: Session) -> None:
    err, _, _ = await alice.call("customer-db_execute_sql", {"sql": "select name, email from customers limit 1"})
    rep.add("3a reading a restricted record is allowed", not err)
    err, _, data = await alice.call("web-fetch_fetch", {"url": "https://docs.example.com/kb/reset"})
    rep.add("3b exfil-capable tool denied after restricted read", err and data.get("eacp_code") == "exfil_after_restricted", str(data))


async def scenario_4_routing(rep: Report, alice_fresh: Session, alice_tainted: Session) -> None:
    status, body = await alice_fresh.chat("Write a friendly two-line greeting for a newsletter.")
    model = body.get("model", "")
    rep.add("4a public prompt is answered by the external model", status == 200 and "claude" in model, model)
    status, body = await alice_tainted.chat("Summarise what we know about this customer.")
    model = body.get("model", "")
    rep.add("4b restricted conversation stays in-house", status == 200 and "claude" not in model, model)


async def scenario_5_approval(rep: Report, alice: Session) -> None:
    args = {"to": "jane.doe@customer.example", "subject": "Your ticket 4410", "body": "We reset your password."}
    err, _, data = await alice.call("email-send_send_email", args)
    ok = err and data.get("eacp_code") == "approval_required" and data.get("approval_url")
    rep.add("5a high-risk tool denied with an approval link", bool(ok), str(data.get("approval_url")))
    if ok:
        print(f"     -> approve as bob at {data['approval_url']} (scripts/e2e.sh drives this with bob's session), then:")
        err, _, data = await alice.call("email-send_send_email", args)
        rep.add("5b retry succeeds after approval", not err, str(data))


async def scenario_6_budget(rep: Report, carol: Session) -> None:
    status = 200
    for _ in range(80):
        status, _ = await carol.chat("Say hi.")
        if status == 429:
            break
    rep.add("6 per-user token budget exhausts with 429", status == 429, f"last status {status}")


async def main() -> int:
    rep = Report()
    alice = Session(user_token("alice", os.environ.get("EACP_ALICE_PASSWORD", "changeme-alice")))
    carol = Session(user_token("carol", os.environ.get("EACP_CAROL_PASSWORD", "changeme-carol")))
    await scenario_1_rugpull(rep, alice)
    await scenario_2_injection(rep, Session(alice.headers["Authorization"][7:]))
    tainted = Session(alice.headers["Authorization"][7:])
    await scenario_3_taint(rep, tainted)
    await scenario_4_routing(rep, Session(alice.headers["Authorization"][7:]), tainted)
    await scenario_5_approval(rep, Session(alice.headers["Authorization"][7:]))
    await scenario_6_budget(rep, carol)
    print(f"\n{sum(r.ok for r in rep.results)}/{len(rep.results)} passed")
    return 1 if rep.failed else 0


if __name__ == "__main__":
    if "--smoke" in sys.argv[1:]:
        from smoke import smoke

        sys.exit(asyncio.run(smoke()))
    if "--pathway" in sys.argv[1:]:
        from pathway import pathway

        sys.exit(asyncio.run(pathway()))
    sys.exit(asyncio.run(main()))
