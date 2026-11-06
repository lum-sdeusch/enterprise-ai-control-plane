#!/usr/bin/env python3
"""Pin a tool server: hash its tools/list, prompts/list and resources/list exactly the way the
adapter does (adapter/internal/pinning) and write the hashes into the manifest.

    scripts/pin-tools.py tools/tickets --url http://localhost:8000/mcp [--token ...] --write
    scripts/pin-tools.py tools/tickets --url ... --check      # exit 1 if the manifest differs

Canonical form shared with Go: items sorted by name (or uri), JSON with sorted keys, compact
separators, UTF-8, no HTML escaping. The cross-language test vector below is also in
adapter/internal/pinning/pinning_test.go; change both or neither.
"""

from __future__ import annotations

import argparse
import asyncio
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


async def listings(url: str, token: str | None) -> dict[str, list[dict]]:
    from mcp import ClientSession
    from mcp.client.streamable_http import streamablehttp_client

    headers = {"Authorization": f"Bearer {token}"} if token else {}
    out: dict[str, list[dict]] = {}
    async with streamablehttp_client(url, headers=headers) as (r, w, _):
        async with ClientSession(r, w) as s:
            await s.initialize()
            tools = await s.list_tools()
            out["tools"] = [t.model_dump(exclude_none=True, by_alias=True) for t in tools.tools]
            try:
                prompts = await s.list_prompts()
                out["prompts"] = [p.model_dump(exclude_none=True, by_alias=True) for p in prompts.prompts]
            except Exception:  # noqa: BLE001 - server may not support prompts
                out["prompts"] = []
            try:
                resources = await s.list_resources()
                out["resources"] = [x.model_dump(exclude_none=True, by_alias=True) for x in resources.resources]
            except Exception:  # noqa: BLE001
                out["resources"] = []
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
    lists = asyncio.run(listings(args.url, args.token))
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
