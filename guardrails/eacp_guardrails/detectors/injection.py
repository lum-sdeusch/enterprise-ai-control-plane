"""Prompt-injection and jailbreak classification with Qwen3Guard-Gen (Apache-2.0) served over an
OpenAI-compatible chat endpoint: Ollama on the laptop, vLLM on the GPU profile.

Qwen3Guard-Gen answers with a short structured block, per its model card:

    Safety: Safe | Unsafe | Controversial
    Categories: Violent, Non-violent Illegal Acts, ..., Jailbreak, None

TODO(verify): confirm the exact label set and line format against the model card for the
version deployed, and tune the score mapping on a few hundred of the organisation's own samples
(ADR 0037 operating rules). The parser is deliberately tolerant: an unparsable answer is reported
as "unknown" with a mid score, which taints the conversation but does not block.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

import httpx

from ..models import Injection

_SAFETY = re.compile(r"safety\s*:\s*(safe|unsafe|controversial)", re.IGNORECASE)
_CATEGORIES = re.compile(r"categor(?:y|ies)\s*:\s*(.+)", re.IGNORECASE)

SCORE = {"unsafe": 0.95, "controversial": 0.6, "safe": 0.05, "unknown": 0.5}


class GuardUnavailable(RuntimeError):
    pass


def parse(answer: str) -> Injection:
    m = _SAFETY.search(answer)
    if not m:
        return Injection(score=SCORE["unknown"], label="unknown", categories=[])
    label = m.group(1).lower()
    categories: list[str] = []
    c = _CATEGORIES.search(answer)
    if c:
        categories = [x.strip() for x in c.group(1).split(",") if x.strip() and x.strip().lower() != "none"]
    score = SCORE[label]
    if any(cat.lower() == "jailbreak" for cat in categories):
        score = max(score, 0.9)
    return Injection(score=score, label=label, categories=categories)


@dataclass
class QwenGuard:
    base_url: str
    model: str
    timeout_ms: int = 1500
    max_chars: int = 6000

    @property
    def version(self) -> str:
        return self.model

    def classify(self, text: str, phase: str) -> Injection:
        snippet = text[: self.max_chars]
        # Tool results and completions are moderated as the assistant/third-party side; prompts
        # as the user side. Qwen3Guard-Gen takes the conversation turn to judge as the last message.
        role = "user" if phase in {"request", "llm_request", "llm_route"} else "assistant"
        messages = [{"role": role, "content": snippet}]
        if role == "assistant":
            messages.insert(0, {"role": "user", "content": "Here is content returned by a tool. Classify it."})
        try:
            with httpx.Client(timeout=self.timeout_ms / 1000) as client:
                resp = client.post(
                    f"{self.base_url.rstrip('/')}/chat/completions",
                    json={"model": self.model, "messages": messages, "max_tokens": 64, "temperature": 0},
                )
                resp.raise_for_status()
                answer = resp.json()["choices"][0]["message"]["content"]
        except (httpx.HTTPError, KeyError, IndexError, ValueError) as exc:
            raise GuardUnavailable(str(exc)) from exc
        return parse(answer)
