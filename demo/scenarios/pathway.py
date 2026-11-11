"""Adapter-in-the-pathway checks on a running cluster (ticket 04): `scripts/e2e.sh --pathway`,
or `make pathway-cluster`.

The same contract test/pathway proves against a bare agentgateway, here through everything the
cluster adds: Keycloak-issued tokens, the charts' gateway policies, the adapter with its OPA
sidecar, the guardrail service, ClickHouse as the audit store and Jaeger behind the collector.

  - listings follow entitlement: alice sees the tickets tools, a user outside the group none,
    a user with lower clearance not the tool that returns confidential data;
  - a denied call reaches the client with its eacp_code and the conversation id;
  - each phase of a call writes one row to eacp_audit.events;
  - one call is one trace in Jaeger, with gateway and adapter spans;
  - with the OPA sidecar stopped, calls fail closed and the audit row says so.

Needs kubectl against the cluster (audit rows are read inside the ClickHouse pod, and the OPA
process is paused on the kind node), EACP_AUDIT_READER_PASSWORD (scripts/e2e.sh fetches it from
OpenBao) and, for the trace check, Jaeger's query API (EACP_JAEGER_URL, default the laptop's
http://127.0.0.1:8084).
"""

from __future__ import annotations

import asyncio
import json
import os
import subprocess

import httpx
from run import Report, Session, user_token

TOOLS = {"list_tickets", "get_ticket", "create_ticket", "close_ticket"}
PREFIXED = {f"tickets_{t}" for t in TOOLS}
JAEGER = os.environ.get("EACP_JAEGER_URL", "http://127.0.0.1:8084").rstrip("/")
KIND_NODE = os.environ.get("EACP_KIND_NODE", "eacp-control-plane")


def password(user: str) -> str:
    return os.environ.get(f"EACP_{user.upper()}_PASSWORD", f"changeme-{user}")


def audit(where: str) -> list[dict]:
    """Rows of eacp_audit.events, read as the read-only audit user inside the ClickHouse pod."""
    sql = (
        "SELECT phase, subject, agent, server, tool, decision, reasons, policy_revision, trace_id "
        f"FROM eacp_audit.events WHERE {where} ORDER BY ts FORMAT JSONEachRow"
    )
    out = subprocess.run(
        ["kubectl", "exec", "-n", "telemetry", "clickhouse-0", "--", "clickhouse-client",
         "--user", "eacp_audit_reader", "--password", os.environ.get("EACP_AUDIT_READER_PASSWORD", ""), "--query", sql],
        capture_output=True, text=True, timeout=60,
    )
    if out.returncode != 0:
        raise RuntimeError(f"audit query failed: {out.stderr.strip()[-300:]}")
    return [json.loads(line) for line in out.stdout.splitlines() if line.strip()]


def on_node(script: str) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", "exec", KIND_NODE, "sh", "-c", script], capture_output=True, text=True, timeout=60)


def opa_signal(sig: str) -> bool:
    """Pause (STOP) or resume (CONT) the adapter pod's OPA process on the kind node. The OPA image
    has no shell to exec into, and deleting the container would only make the kubelet restart it."""
    script = (
        "for c in $(crictl ps --name '^opa$' -q); do "
        "pid=$(crictl inspect --output go-template --template '{{.info.pid}}' \"$c\"); "
        f"kill -{sig} \"$pid\" && echo \"$pid\"; done"
    )
    done = on_node(script)
    return done.returncode == 0 and done.stdout.strip() != ""


async def pathway() -> int:
    rep = Report()
    alice = Session(user_token("alice", password("alice")))
    carol_token = user_token("carol", password("carol"))
    erin_token = user_token("erin", password("erin"))

    # --- listings ---------------------------------------------------------------------------------
    got = await alice.tools()
    rep.add("alice sees exactly the tickets tools on the aggregate", isinstance(got, list) and set(got) == PREFIXED, str(got))
    got = await Session(user_token("alice", password("alice")), path="/mcp/tickets").tools()
    rep.add("the per-server route lists the same tools unprefixed", isinstance(got, list) and set(got) == TOOLS, str(got))
    got = await Session(erin_token).tools()
    rep.add("a user outside the required group sees none of the server's tools", got == [], str(got))
    got = await Session(carol_token).tools()
    rep.add("clearance internal hides the tool that returns confidential data", isinstance(got, list) and set(got) == PREFIXED - {"tickets_get_ticket"}, str(got))

    # --- an allowed call, audited and traced ------------------------------------------------------
    err, text, _ = await alice.call("tickets_list_tickets", {"status": "open"})
    rep.add("an entitled call succeeds", not err and "4410" in text, text[:120])
    rows = audit(f"conversation_id = '{alice.conversation}' AND tool = 'list_tickets'")
    rep.add("request and response phases each wrote one audit row", [r["phase"] for r in rows] == ["request", "response"], str([r["phase"] for r in rows]))
    if rows:
        r = rows[0]
        rep.add(
            "the row has subject, agent, decision, policy revision and trace id",
            r["subject"] != "" and r["agent"] == "support-assistant" and r["decision"] == "allow" and r["policy_revision"] not in ("", "unbundled") and r["trace_id"] == alice.trace_id,
            json.dumps(r),
        )

    # --- a denied call ----------------------------------------------------------------------------
    carol = Session(carol_token)
    err, text, data = await carol.call("tickets_get_ticket", {"ticket_id": 4410})
    rep.add("a call above the caller's clearance is denied with its code", err and data.get("eacp_code") == "clearance_insufficient", f"{text[:100]} {data}")
    rep.add("the denial carries the conversation id", data.get("conversation_id") == carol.conversation, str(data))
    rows = audit(f"conversation_id = '{carol.conversation}' AND phase = 'request' AND tool = 'get_ticket'")
    rep.add("the denial is audited with its reason", bool(rows) and rows[0]["decision"] == "deny" and "clearance_insufficient" in rows[0]["reasons"], str(rows[:1]))

    # --- one call, one trace ------------------------------------------------------------------------
    await asyncio.sleep(8)  # span batches reach Jaeger through the collector
    services: set[str] = set()
    adapter_spans: set[str] = set()
    try:
        resp = httpx.get(f"{JAEGER}/api/traces/{alice.trace_id}", timeout=15)
        for trace in resp.json().get("data", []) if resp.status_code == 200 else []:
            procs = {pid: p.get("serviceName", "") for pid, p in trace.get("processes", {}).items()}
            for span in trace.get("spans", []):
                svc = procs.get(span.get("processID"), "")
                services.add(svc)
                if svc == "eacp-adapter":
                    adapter_spans.add(span.get("operationName", ""))
    except (httpx.HTTPError, ValueError) as exc:
        services = {f"jaeger query failed: {exc}"}
    rep.add("Jaeger has one trace with gateway and adapter spans", "eacp-adapter" in services and len(services) >= 2, str(sorted(services)))
    rep.add("the adapter's request and response spans are in it", {"eacp.tool.request", "eacp.tool.response"} <= adapter_spans, str(sorted(adapter_spans)))

    # --- the OPA sidecar stopped --------------------------------------------------------------------
    if opa_signal("STOP"):
        try:
            frozen = Session(user_token("alice", password("alice")))
            err, text, data = await frozen.call("tickets_list_tickets", {"status": "open"})
            # The adapter answers fail_closed until the pod turns unready (the sidecar's readiness
            # probe), after which the gateway has no adapter endpoint and closes the gate itself.
            rep.add("with OPA stopped the call fails closed", err and "4410" not in text, f"{text[:100]} {data}")
            rows = audit(f"conversation_id = '{frozen.conversation}' AND phase = 'request'")
            rep.add("the audit row records fail_closed", bool(rows) and rows[0]["decision"] == "fail_closed", str(rows[:1]) or str(data))
        finally:
            opa_signal("CONT")
        await asyncio.sleep(20)  # the pod turns ready again
        err, text, _ = await Session(user_token("alice", password("alice"))).call("tickets_list_tickets", {"status": "open"})
        rep.add("calls work again once OPA resumes", not err and "4410" in text, text[:120])
    else:
        rep.add("pause the OPA sidecar on the kind node", False, f"no opa container found on {KIND_NODE} (EACP_KIND_NODE)")

    print(f"\n{sum(r.ok for r in rep.results)}/{len(rep.results)} passed")
    return 1 if rep.failed else 0
