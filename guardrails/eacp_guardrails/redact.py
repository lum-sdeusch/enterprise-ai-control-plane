"""Replace finding spans with [REDACTED:<TYPE>] markers, longest-first, without touching
anything outside the spans."""

from __future__ import annotations

from .models import Finding


def redact(text: str, findings: list[Finding]) -> str:
    spans = sorted(((f.start, f.end, f.type) for f in findings if f.end > f.start), key=lambda s: (s[0], -s[1]))
    out: list[str] = []
    cursor = 0
    for start, end, kind in spans:
        if start < cursor:  # overlapping with a previous span: already covered
            continue
        out.append(text[cursor:start])
        out.append(f"[REDACTED:{kind}]")
        cursor = end
    out.append(text[cursor:])
    return "".join(out)
