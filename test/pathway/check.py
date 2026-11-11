"""Assertions of the pathway test. Driven by test/pathway/run.sh, which starts the components and
calls one phase of this script at a time:

    check.py pathway        the hook contract, listings, denials, audit, taint, tracing
    check.py audit-store    with --clickhouse: rows land in the real table under the real grants
    check.py opa-down       the policy engine is stopped: everything must fail closed
    check.py adapter-down   the adapter is stopped: the gateway must fail closed
    check.py recovered      both are back: calls work again
    check.py drive          probe mode: send one listing and one call through each route
    check.py show-probe     probe mode: print what the gateway sent to the hook
"""

from __future__ import annotations

import asyncio
import json
import os
import secrets
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path

import logging

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client
from mcp.shared.exceptions import McpError

sys.path.insert(0, str(Path(__file__).resolve().parent))
import mint  # noqa: E402

logging.getLogger("mcp").setLevel(logging.CRITICAL)  # the SDK logs noise when a session closes

ROOT = Path(__file__).resolve().parents[2]
W = Path(os.environ.get("EACP_PATHWAY_DIR", ROOT / "dist" / "pathway"))
GATEWAY = os.environ.get("EACP_PATHWAY_GATEWAY", "http://127.0.0.1:13000")
TICKETS_URL = os.environ.get("EACP_PATHWAY_TICKETS", "http://127.0.0.1:18001/mcp")
TICKET_TOOLS = {"list_tickets", "get_ticket", "create_ticket", "close_ticket"}


@dataclass
class Report:
    failed: int = 0
    total: int = 0

    def check(self, name: str, ok: bool, detail: object = "") -> bool:
        self.total += 1
        self.failed += 0 if ok else 1
        print(f"  {'PASS' if ok else 'FAIL'}  {name}" + (f"   [{detail}]" if detail != "" and not ok else ""))
        return ok

    def done(self) -> int:
        print(f"  {self.total - self.failed}/{self.total} passed")
        return 1 if self.failed else 0


@dataclass
class Outcome:
    """A tool call as the client experienced it."""

    error: bool
    text: str = ""
    code: int | None = None  # JSON-RPC error code, when the call failed at the protocol level
    data: dict = field(default_factory=dict)  # the error's data member

    @property
    def eacp_code(self) -> str | None:
        return self.data.get("eacp_code")

    @property
    def eacp_codes(self) -> list[str]:
        return self.data.get("eacp_codes") or ([self.eacp_code] if self.eacp_code else [])


class Caller:
    def __init__(
        self,
        sub: str = "alice",
        groups: tuple[str, ...] = ("support",),
        clearance: str | None = "restricted",
        act: str | None = "agent-support-assistant",
        path: str = "/mcp",
        conversation: str | None = "auto",
        token: str | None = "mint",
    ):
        self.conversation = f"pw-{secrets.token_hex(4)}" if conversation == "auto" else conversation
        self.trace_id = secrets.token_hex(16)
        self.headers = {"traceparent": f"00-{self.trace_id}-{secrets.token_hex(8)}-01"}
        if self.conversation:
            self.headers["x-eacp-conversation-id"] = self.conversation
        if token == "mint":
            token = mint.token(W / "keys", sub, list(groups), clearance=clearance, act=act)
        if token:
            self.headers["Authorization"] = f"Bearer {token}"
        self.url = f"{GATEWAY}{path}"

    async def _session(self, fn):
        async with streamablehttp_client(self.url, headers=self.headers) as (r, w, _):
            async with ClientSession(r, w) as s:
                await s.initialize()
                return await fn(s)

    async def tools(self) -> set[str] | Outcome:
        try:
            res = await self._session(lambda s: s.list_tools())
            return {t.name for t in res.tools}
        except BaseException as e:  # noqa: BLE001 - the SDK wraps protocol errors in exception groups
            return _protocol_error(e)

    async def call(self, name: str, args: dict | None = None) -> Outcome:
        try:
            res = await self._session(lambda s: s.call_tool(name, args or {}))
        except BaseException as e:  # noqa: BLE001
            return _protocol_error(e)
        text = "\n".join(getattr(c, "text", "") for c in res.content)
        if res.isError:
            # A policy denial of a tool call arrives as a tool result whose text is a JSON object
            # {"error": "eacp_policy", "message": ..., "data": {...}}; a tool's own error is prose.
            try:
                body = json.loads(text)
                if isinstance(body, dict) and body.get("error") == "eacp_policy":
                    return Outcome(True, body.get("message", ""), None, body.get("data", {}))
            except ValueError:
                pass
        return Outcome(bool(res.isError), text)


def _protocol_error(exc: BaseException) -> Outcome:
    """The JSON-RPC error behind an exception from the SDK, which may be nested in task groups."""
    stack = [exc]
    while stack:
        e = stack.pop()
        if isinstance(e, McpError):
            return Outcome(True, e.error.message, e.error.code, e.error.data if isinstance(e.error.data, dict) else {})
        stack.extend(getattr(e, "exceptions", []))
    if isinstance(exc, (KeyboardInterrupt, SystemExit)):
        raise exc
    return Outcome(True, f"{type(exc).__name__}: {exc}")


def audit_rows(conversation: str | None = None, **match: str) -> list[dict]:
    rows = []
    for line in (W / "hook.log").read_text(errors="replace").splitlines():
        if '"kind":"eacp.audit"' not in line:
            continue
        row = json.loads(line)
        if conversation is not None and row.get("conversation_id") != conversation:
            continue
        if all(row.get(k) == v for k, v in match.items()):
            rows.append(row)
    return rows


def spans(trace_id: str) -> list[dict]:
    out = []
    for line in (W / "spans.jsonl").read_text(errors="replace").splitlines():
        line = line.strip("\x00 ")
        if line:
            s = json.loads(line)
            if s["trace_id"] == trace_id:
                out.append(s)
    return out


async def pathway(rep: Report) -> None:
    # --- the manifest's pins describe the server that is running --------------------------------
    pin = subprocess.run([sys.executable, str(ROOT / "scripts/pin-tools.py"), "tools/tickets", "--url", TICKETS_URL, "--check"], capture_output=True, text=True, cwd=ROOT)
    rep.check("tickets manifest pins match the live server", pin.returncode == 0, pin.stdout.strip() or pin.stderr.strip()[-200:])

    # --- listings: entitlement, clearance, agent manifest, prefixing, unknown servers ----------
    alice = Caller()
    got = await alice.tools()
    rep.check("aggregate: alice sees exactly the four tickets tools, prefixed", got == {f"tickets_{t}" for t in TICKET_TOOLS}, got)
    rep.check("aggregate: the server without a manifest is invisible", isinstance(got, set) and not any(n.startswith("notes_") for n in got), got)
    got = await Caller(path="/mcp/tickets").tools()
    rep.check("per-server route: the same four tools, unprefixed, same pin", got == TICKET_TOOLS, got)
    got = await Caller(sub="erin", groups=("marketing",)).tools()
    rep.check("a caller outside the required group sees none of the server's tools", got == set(), got)
    got = await Caller(sub="bob", clearance="internal").tools()
    rep.check("clearance internal hides the tool that returns confidential data", got == {f"tickets_{t}" for t in TICKET_TOOLS - {"get_ticket"}}, got)
    got = await Caller(act="agent-nobody-registered", conversation="pw-unknown-agent").tools()
    rep.check("an agent without a manifest sees nothing", got == set(), got)
    rows = audit_rows(alice.conversation, phase="list", server="tickets")
    rep.check("listing is audited with the pin status", bool(rows) and rows[-1]["pin_status"] == "match" and rows[-1]["decision"] == "allow", rows[-1:] or "no row")
    rows = audit_rows(alice.conversation, phase="list", server="notes")
    rep.check("the hidden server's listing is audited as unknown", bool(rows) and rows[-1]["reasons"] == ["unknown_server"], rows[-1:] or "no row")

    # --- an allowed call: both phases, identity, audit, trace ----------------------------------
    out = await alice.call("tickets_list_tickets", {"status": "open"})
    rep.check("an entitled call succeeds through the aggregate", not out.error and "4410" in out.text, out)
    req = audit_rows(alice.conversation, phase="request", tool="list_tickets")
    res = audit_rows(alice.conversation, phase="response", tool="list_tickets")
    rep.check("request and response phases each write one audit row", len(req) == 1 and len(res) == 1, (len(req), len(res)))
    if req and res:
        r = req[0]
        rep.check(
            "the audit row names subject, agent, actor chain, server and tool",
            (r["subject"], r["agent"], r["actor_chain"], r["server"], r["team"]) == ("alice", "support-assistant", "alice -> agent-support-assistant", "tickets", "support"),
            r,
        )
        rep.check("the audit row carries decision, policy revision and reasons", r["decision"] == "allow" and r["policy_revision"] == "pathway" and r["reasons"] == [], r)
        rep.check("both phases carry the client's trace id", r["trace_id"] == alice.trace_id == res[0]["trace_id"], (alice.trace_id, r["trace_id"], res[0]["trace_id"]))
        rep.check("the response phase knows the tool the request phase stashed", res[0]["tool"] == "list_tickets" and res[0]["server"] == "tickets", res[0])
        rep.check("the response phase keeps the conversation", res[0]["conversation_id"] == alice.conversation, res[0]["conversation_id"])

    out = await Caller(path="/mcp/tickets", conversation="pw-per-server").call("list_tickets", {"status": "all"})
    rep.check("the same call succeeds on the per-server route", not out.error and "4412" in out.text, out)

    nocv = Caller(conversation=None)
    await nocv.call("tickets_list_tickets")
    rep.check("without a conversation id the subject-and-agent fallback is used", len(audit_rows("fallback:alice|support-assistant", phase="request")) >= 1)

    # --- denials reach the client with a machine-readable code ---------------------------------
    bob = Caller(sub="bob", clearance="internal")
    out = await bob.call("tickets_get_ticket", {"ticket_id": 4410})
    rep.check("a call above the caller's clearance is denied", out.error and out.eacp_code == "clearance_insufficient", out)
    rep.check("the denial carries the conversation id", out.data.get("conversation_id") == bob.conversation, out.data)
    rows = audit_rows(bob.conversation, phase="request", tool="get_ticket")
    rep.check("the denial is audited with its reason", bool(rows) and rows[0]["decision"] == "deny" and "clearance_insufficient" in rows[0]["reasons"], rows[:1] or "no row")
    rep.check("a denied call has no response phase", audit_rows(bob.conversation, phase="response") == [])

    out = await alice.call("notes_read_note", {"note_id": 1})
    rep.check("a call to the server without a manifest is denied", out.error and "unknown_server" in out.eacp_codes, out)
    out = await Caller(sub="erin", groups=("marketing",)).call("tickets_list_tickets")
    rep.check("a call from outside the required group is denied", out.error and "group_not_entitled" in out.eacp_codes, out)
    out = await alice.call("tickets_create_ticket", {"title": "key", "body": "my key is AKIAIOSFODNN7EXAMPLE"})
    rep.check("a secret in tool arguments is denied", out.error and out.eacp_code == "secret_in_arguments", out)

    # --- a tool's own error is not a policy denial ----------------------------------------------
    out = await alice.call("tickets_get_ticket", {"ticket_id": 99999})
    rep.check("a tool error passes through as the tool's error", out.error and out.eacp_code is None and "no ticket 99999" in out.text, out)

    # --- taint is recorded from what a result contains ------------------------------------------
    carol = Caller(conversation="pw-taint")
    out = await carol.call("tickets_get_ticket", {"ticket_id": 4410})
    rows = audit_rows("pw-taint", phase="response", tool="get_ticket")
    tainted = bool(rows) and json.loads(rows[0]["taint_after"]).get("touched_pii") is True and json.loads(rows[0]["taint_before"]).get("touched_pii") is False
    rep.check("reading a ticket with contact details taints the conversation", not out.error and tainted, rows[:1] or out)
    await carol.call("tickets_list_tickets")
    rows = audit_rows("pw-taint", phase="request", tool="list_tickets")
    rep.check("the taint is there on the conversation's next call", bool(rows) and json.loads(rows[0]["taint_before"]).get("touched_pii") is True, rows[:1] or "no row")

    # --- a route without a JWT policy gets nothing, whatever token it forwards ------------------
    forged = Caller(path="/open/tickets", conversation="pw-open")
    got = await forged.tools()
    rep.check("unverified route: listing is refused", isinstance(got, Outcome) and got.eacp_code == "unauthenticated", got)
    out = await forged.call("list_tickets")
    rep.check("unverified route: a call is refused although a signed token was sent", out.error and out.eacp_code == "unauthenticated", out)

    # --- one call, one trace ----------------------------------------------------------------------
    time.sleep(4)  # the adapter's span batcher flushes every 2 s
    got = spans(alice.trace_id)
    by_id = {s["span_id"]: s for s in got}
    services = {s["service"] for s in got}
    adapter_spans = [s for s in got if s["service"] == "eacp-adapter"]
    names = {s["name"] for s in adapter_spans}
    rep.check("gateway and adapter spans share the client's trace", {"eacp-adapter"} < services and len(services) >= 2, sorted(services))
    rep.check("the adapter has a request and a response span", {"eacp.tool.request", "eacp.tool.response"} <= names, sorted(names))

    def under_gateway(s: dict) -> bool:
        seen = set()
        while s and s["span_id"] not in seen:
            seen.add(s["span_id"])
            if s["service"] != "eacp-adapter":
                return True
            s = by_id.get(s["parent_span_id"])
        return False

    roots = [s for s in adapter_spans if s["name"] in {"eacp.tool.request", "eacp.tool.response"}]
    rep.check("the adapter's spans hang under a gateway span", bool(roots) and all(under_gateway(s) for s in roots), [(s["name"], s["parent_span_id"]) for s in roots])
    req_span = next((s for s in adapter_spans if s["name"] == "eacp.tool.request"), None)
    rep.check(
        "the adapter span carries the policy attributes",
        bool(req_span) and req_span["attributes"].get("eacp.policy.decision") == "allow" and req_span["attributes"].get("eacp.policy.revision") == "pathway",
        req_span and req_span["attributes"],
    )


async def audit_store(rep: Report) -> None:
    """With --clickhouse: the adapter's rows land in the real table, through the real grants."""
    import httpx

    base = os.environ["EACP_PATHWAY_CLICKHOUSE"]
    reader = ("eacp_audit_reader", os.environ["AUDIT_READER_PASSWORD"])
    writer = ("eacp_audit_writer", os.environ["AUDIT_WRITER_PASSWORD"])

    def query(sql: str, auth: tuple[str, str]) -> httpx.Response:
        return httpx.post(base, params={"query": sql}, auth=auth, timeout=10)

    alice = Caller(conversation="pw-clickhouse")
    out = await alice.call("tickets_list_tickets")
    rep.check("a call succeeds with ClickHouse as the required audit store", not out.error, out)
    # The client library lists tools before a call to learn the output schema, so the conversation
    # also has listing rows; the call itself is the request and response pair.
    sql = f"SELECT phase, subject, agent, server, tool, decision, policy_revision, trace_id, toString(event_id) AS event_id, toString(ts) AS ts FROM eacp_audit.events WHERE conversation_id = '{alice.conversation}' AND phase IN ('request', 'response') ORDER BY ts FORMAT JSONEachRow"
    resp = query(sql, reader)
    rows = [json.loads(line) for line in resp.text.splitlines() if line.strip()] if resp.status_code == 200 else []
    rep.check("the request and response rows are in eacp_audit.events", [r["phase"] for r in rows] == ["request", "response"], resp.text[:300])
    if rows:
        r = rows[0]
        rep.check(
            "the stored row has subject, agent, tool, decision, revision and trace id",
            (r["subject"], r["agent"], r["server"], r["tool"], r["decision"], r["policy_revision"], r["trace_id"]) == ("alice", "support-assistant", "tickets", "list_tickets", "allow", "pathway", alice.trace_id),
            r,
        )
    resp = query(f"SELECT count() FROM eacp_audit.events WHERE trace_id = '{alice.trace_id}' AND tool = 'list_tickets'", reader)
    rep.check("an auditor finds the call's decisions by trace id", resp.text.strip() == "2", resp.text[:200])
    resp = query("ALTER TABLE eacp_audit.events DELETE WHERE 1", writer)
    rep.check("the writer cannot delete audit rows", resp.status_code != 200 and "ACCESS_DENIED" in resp.text, resp.text[:200])
    resp = query("SELECT count() FROM eacp_audit.events", writer)
    rep.check("the writer cannot read audit rows", resp.status_code != 200 and "ACCESS_DENIED" in resp.text, resp.text[:200])
    resp = query("INSERT INTO eacp_audit.events (ts, subject) VALUES (now(), 'x')", reader)
    rep.check("the reader cannot write audit rows", resp.status_code != 200 and "ACCESS_DENIED" in resp.text, resp.text[:200])


async def opa_down(rep: Report) -> None:
    alice = Caller(conversation="pw-opa-down")
    out = await alice.call("tickets_list_tickets")
    rep.check("with OPA stopped a call fails closed", out.error and out.eacp_code == "fail_closed", out)
    rows = audit_rows("pw-opa-down", phase="request")
    rep.check("the audit row records fail_closed", bool(rows) and rows[0]["decision"] == "fail_closed", rows[:1] or "no row")
    got = await alice.tools()
    rep.check("with OPA stopped a listing is refused, not passed through", isinstance(got, Outcome) and got.eacp_code == "fail_closed", got)


async def adapter_down(rep: Report) -> None:
    alice = Caller(conversation="pw-adapter-down")
    out = await alice.call("tickets_list_tickets")
    rep.check("with the adapter stopped the gateway refuses the call", out.error and "4410" not in out.text, out)
    got = await alice.tools()
    rep.check("with the adapter stopped the gateway refuses the listing", isinstance(got, Outcome), got)


async def recovered(rep: Report) -> None:
    alice = Caller(conversation="pw-recovered")
    got = await alice.tools()
    rep.check("listing works again", got == {f"tickets_{t}" for t in TICKET_TOOLS}, got)
    out = await alice.call("tickets_list_tickets")
    rep.check("calls work again", not out.error and "4410" in out.text, out)


async def drive(_: Report) -> None:
    for path, name in (("/mcp", "tickets_list_tickets"), ("/mcp/tickets", "list_tickets")):
        c = Caller(path=path, conversation=f"probe{path.replace('/', '-')}")
        print(f"  {path}: tools = {sorted(await c.tools())}")
        out = await c.call(name, {"status": "open"})
        print(f"  {path}: call error={out.error} text={out.text[:60]!r}")


def show_probe() -> None:
    def short(x: object, n: int = 300) -> str:
        s = json.dumps(x)
        return s if len(s) <= n else f"{s[:n]}... <{len(s)} chars>"

    for line in (W / "hook.log").read_text(errors="replace").splitlines():
        if not line.startswith("{"):
            continue
        d = json.loads(line)
        meta = d["metadata_context"]
        print(f"\n  {d['phase'].upper()} {d['method']}  service_names={d['service_names']}")
        if "headers" in d:
            print(f"    headers          {sorted(d['headers'])}")
        print(f"    metadata_context {short({k: v for k, v in meta.items() if k != 'jwt'})}")
        if "jwt" in meta:
            print(f"    jwt claims       {short(meta['jwt'], 200)}")
        print(f"    grpc metadata    {sorted(d['grpc_metadata'])}")
        print(f"    body             {short(d['body'], 200)}")


def main() -> int:
    phase = sys.argv[1] if len(sys.argv) > 1 else "pathway"
    if phase == "show-probe":
        show_probe()
        return 0
    fn = {"pathway": pathway, "audit-store": audit_store, "opa-down": opa_down, "adapter-down": adapter_down, "recovered": recovered, "drive": drive}[phase]
    rep = Report()
    asyncio.run(fn(rep))
    return 0 if phase == "drive" else rep.done()


if __name__ == "__main__":
    sys.exit(main())
