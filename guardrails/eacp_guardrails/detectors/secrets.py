"""Secret detection with a curated subset of gitleaks-style rules (gitleaks itself is MIT and
"feature complete"; its rule set is the reference for these patterns). Secrets are the one
class of finding the policy never lets through in any direction, so precision matters more
than recall here: every rule below targets a vendor-specific prefix or a self-identifying
format, and the one generic rule carries a lower score."""

from __future__ import annotations

import re
from dataclasses import dataclass

from ..models import Finding

VERSION = "rules/2026.11"


@dataclass(frozen=True)
class Rule:
    type: str
    pattern: re.Pattern[str]
    score: float = 0.95
    group: int = 0  # capture group holding the secret itself


RULES: list[Rule] = [
    Rule("private_key", re.compile(r"-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY-----"), 0.99),
    Rule("aws_access_key_id", re.compile(r"\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ABIA|ACCA)[A-Z0-9]{16}\b"), 0.97),
    Rule("aws_secret_access_key", re.compile(r"(?i)aws[_-]?secret[_-]?access[_-]?key\s*[=:]\s*['\"]?([A-Za-z0-9/+=]{40})"), 0.95, 1),
    Rule("github_token", re.compile(r"\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b"), 0.98),
    Rule("github_fine_grained_token", re.compile(r"\bgithub_pat_[A-Za-z0-9_]{82}\b"), 0.98),
    Rule("anthropic_api_key", re.compile(r"\bsk-ant-(?:api|admin)?[A-Za-z0-9_-]{20,}\b"), 0.98),
    Rule("openai_api_key", re.compile(r"\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{32,}\b"), 0.9),
    Rule("slack_token", re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{10,}\b"), 0.97),
    Rule("google_api_key", re.compile(r"\bAIza[0-9A-Za-z_-]{35}\b"), 0.95),
    Rule("stripe_key", re.compile(r"\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{24,}\b"), 0.97),
    Rule("jwt", re.compile(r"\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b"), 0.9),
    Rule("generic_credential", re.compile(r"(?i)\b(?:api[_-]?key|secret|token|passwd|password)\b\s*[:=]\s*['\"]?([A-Za-z0-9_\-/+=]{16,})['\"]?"), 0.6, 1),
]


def detect(path: str, text: str) -> list[Finding]:
    found: list[Finding] = []
    taken: list[tuple[int, int]] = []
    for rule in RULES:
        for m in rule.pattern.finditer(text):
            start, end = m.span(rule.group)
            if any(s <= start < e or s < end <= e for s, e in taken):
                continue
            taken.append((start, end))
            found.append(Finding(path=path, type=rule.type, start=start, end=end, score=rule.score))
    found.sort(key=lambda f: f.start)
    return found
