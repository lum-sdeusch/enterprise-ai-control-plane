#!/usr/bin/env python3
"""Generate docs/adr/NNNN-<slug>.md and docs/adr/README.md from docs/adr/decisions.yaml.

One ADR per decision taken in the design rounds (docs/design.md section 12). The YAML is the
source; the Markdown is generated so the index, numbering and format never drift. CI checks
that the generated files are committed (`git diff --exit-code docs/adr/`).
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "docs" / "adr" / "decisions.yaml"
OUT = ROOT / "docs" / "adr"


def text(x) -> str:
    """Bullets are strings; a bullet with an unquoted colon parses as a one-key mapping, so
    accept that shape too instead of failing the whole generation."""
    if isinstance(x, dict) and len(x) == 1:
        k, v = next(iter(x.items()))
        return f"{k}: {v}".strip()
    return str(x).strip()


def slug(title: str) -> str:
    s = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
    return s[:60].rstrip("-")


def render(d: dict) -> str:
    num = f"{d['id']:04d}"
    lines = [f"# ADR {num}: {d['title']}", ""]
    lines.append(f"- Status: {d.get('status', 'accepted')}")
    lines.append(f"- Date: {d.get('date', '2026-11-06')}")
    if d.get("round"):
        lines.append(f"- Decided in: design round {d['round']}, question Q{d['id']}")
    if d.get("related"):
        rel = ", ".join(f"[ADR {r:04d}]({r:04d}-{slug(BY_ID[r]['title'])}.md)" for r in d["related"])
        lines.append(f"- Related: {rel}")
    lines += ["", "## Context", "", d["context"].strip(), "", "## Decision", "", d["decision"].strip(), ""]
    if d.get("alternatives"):
        lines += ["## Alternatives considered", ""]
        for alt in d["alternatives"]:
            lines.append(f"- **{alt['option']}.** {alt['reason'].strip()}")
        lines.append("")
    if d.get("consequences"):
        lines += ["## Consequences", ""]
        for c in d["consequences"]:
            lines.append(f"- {text(c)}")
        lines.append("")
    if d.get("verify"):
        lines += verify_section(d["verify"])
    return "\n".join(lines)


def verify_section(items: list) -> list[str]:
    """Open questions render as bullets. An answered one is a mapping with `item`, `answer` and
    `verified` (where and how it was checked); once every item is answered the heading says so."""
    answered = [v for v in items if isinstance(v, dict) and "answer" in v]
    title = "Verified at build time" if len(answered) == len(items) else "To verify at build time"
    lines = [f"## {title}", ""]
    for v in items:
        if v in answered:
            lines.append(f"- {str(v['item']).strip()}")
            lines.append(f"  **Answer ({str(v['verified']).strip()}):** {str(v['answer']).strip()}")
        else:
            lines.append(f"- {text(v)}")
    lines.append("")
    return lines


def main() -> int:
    decisions = yaml.safe_load(SRC.read_text())
    global BY_ID
    BY_ID = {d["id"]: d for d in decisions}
    ids = [d["id"] for d in decisions]
    if ids != sorted(ids) or len(set(ids)) != len(ids):
        print("decisions.yaml ids must be unique and ascending", file=sys.stderr)
        return 1

    # Remove stale generated files, keep the source and the index.
    for old in OUT.glob("[0-9][0-9][0-9][0-9]-*.md"):
        old.unlink()

    index = ["# Architecture decision records", "", "Generated from [decisions.yaml](decisions.yaml) by `scripts/gen-adrs.py`; edit the YAML, not these files.", "", "Each record is one decision from the design rounds recorded in [docs/design.md](../design.md). Numbers match the decision log there.", "", "| ADR | Title | Status |", "|---|---|---|"]
    for d in decisions:
        name = f"{d['id']:04d}-{slug(d['title'])}.md"
        (OUT / name).write_text(render(d))
        index.append(f"| {d['id']:04d} | [{d['title']}]({name}) | {d.get('status', 'accepted')} |")
    (OUT / "README.md").write_text("\n".join(index) + "\n")
    print(f"wrote {len(decisions)} ADRs and docs/adr/README.md")
    return 0


BY_ID: dict[int, dict] = {}

if __name__ == "__main__":
    sys.exit(main())
