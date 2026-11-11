#!/usr/bin/env python3
"""Pin a tool server: hash its tools/list, prompts/list and resources/list exactly the way the
adapter does (adapter/internal/pinning) and write the hashes into the manifest.

    scripts/pin-tools.py tools/tickets --url http://localhost:8000/mcp [--token ...] --write
    scripts/pin-tools.py tools/tickets --url ... --check      # exit 1 if the manifest differs

Canonical form shared with Go: items sorted by name (or uri), JSON with sorted keys, compact
separators, UTF-8, no HTML escaping. The items are the server's own, with the names it
advertises; the adapter removes the aggregate endpoint's "<server>_" prefix before hashing, so
one pin holds on every route. The cross-language test vector below is also in
adapter/internal/pinning/pinning_test.go; change both or neither.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path

import yaml

VECTOR_ITEMS = [
    {"name": "fetch", "description": "Fetch a URL & return <markdown>", "inputSchema": {"type": "object", "properties": {"url": {"type": "string"}}, "required": ["url"]}},
    {"name": "ping", "description": "Ping ü"},
]
VECTOR_HASH = "sha256:313593aef9970794928f9d855ce03b662b615ea1d3d342dddc2f0b30b6d5ff3f"


def canonical_hash(items: list[dict]) -> str:
    ordered = sorted(items, key=lambda m: str(m.get("name", m.get("uri", ""))))
    canon = json.dumps(ordered, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    return "sha256:" + hashlib.sha256(canon).hexdigest()


def _rpc(client, url: str, headers: dict, payload: dict) -> dict:
    """One JSON-RPC exchange over Streamable HTTP; the answer may arrive as JSON or as one SSE event."""
    resp = client.post(url, headers=headers, json=payload)
    if resp.status_code == 202 or not resp.content:
        return {}
    resp.raise_for_status()
    body = resp.text
    if "text/event-stream" in resp.headers.get("content-type", ""):
        events = [line[5:].strip() for line in body.splitlines() if line.startswith("data:")]
        body = events[-1] if events else "{}"
    return json.loads(body)


def listings(url: str, token: str | None) -> dict[str, list[dict]]:
    """The server's three listings exactly as it sends them.

    Raw JSON-RPC on purpose, not an MCP SDK: an SDK parses items into its own models and drops or
    renames what it does not know, and the pin must be the hash of what the adapter will see. The
    gateway passes listing items through unchanged (checked by test/pathway), so the raw items
    from the server are that.
    """
    import httpx

    headers = {"Accept": "application/json, text/event-stream", "Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    out: dict[str, list[dict]] = {}
    with httpx.Client(timeout=20) as client:
        # Servers on the handshake revisions need initialize; stateless ones (2026-07-28) answer
        # without it, so a refused initialize is not fatal.
        init = {"jsonrpc": "2.0", "id": 0, "method": "initialize", "params": {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "eacp-pin", "version": "1"}}}
        resp = client.post(url, headers=headers, json=init)
        if resp.status_code < 400:
            if sid := resp.headers.get("mcp-session-id"):
                headers["Mcp-Session-Id"] = sid
            client.post(url, headers=headers, json={"jsonrpc": "2.0", "method": "notifications/initialized"})
        for n, kind in enumerate(("tools", "prompts", "resources"), start=1):
            items: list[dict] = []
            cursor = None
            while True:
                payload = {"jsonrpc": "2.0", "id": n, "method": f"{kind}/list"}
                if cursor:
                    payload["params"] = {"cursor": cursor}
                answer = _rpc(client, url, headers, payload)
                if "error" in answer:  # a server without prompts or resources: an empty listing
                    break
                result = answer.get("result", {})
                items.extend(result.get(kind, []))
                cursor = result.get("nextCursor")
                if not cursor:
                    break
            out[kind] = items
    return out


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("tool_dir", help="tools/<name>")
    ap.add_argument("--url", required=False, help="Streamable HTTP URL of the running server")
    ap.add_argument("--token", default=None)
    ap.add_argument("--write", action="store_true", help="write the hashes into the manifest")
    ap.add_argument("--check", action="store_true", help="exit 1 if the manifest's pins differ")
    ap.add_argument("--self-test", action="store_true", help="verify the cross-language vector")
    args = ap.parse_args()

    if args.self_test:
        got = canonical_hash(VECTOR_ITEMS)
        print(got)
        return 0 if got == VECTOR_HASH else 1

    if not args.url:
        ap.error("--url is required unless --self-test")
    manifest_path = Path(args.tool_dir) / "manifest.yaml"
    doc = yaml.safe_load(manifest_path.read_text())
    lists = listings(args.url, args.token)
    pins = {kind: canonical_hash(items) for kind, items in lists.items()}

    declared = {t["name"] for t in doc["spec"]["tools"]}
    advertised = {t["name"] for t in lists["tools"]}
    for extra in sorted(advertised - declared):
        print(f"warning: server advertises undeclared tool {extra!r}; it will be denied by policy")
    for missing in sorted(declared - advertised):
        print(f"warning: manifest declares {missing!r} but the server does not advertise it")

    current = doc["spec"].get("pins", {})
    if args.check:
        diff = {k: (current.get(k), v) for k, v in pins.items() if current.get(k) != v}
        if diff:
            for k, (old, new) in diff.items():
                print(f"pins.{k}: manifest {old} != live {new}")
            return 1
        print("pins match")
        return 0
    if args.write:
        doc["spec"]["pins"] = pins
        manifest_path.write_text(yaml.safe_dump(doc, sort_keys=False, width=120))
        print(f"wrote pins to {manifest_path}")
    else:
        print(json.dumps(pins, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
