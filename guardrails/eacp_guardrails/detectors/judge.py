"""Optional LLM judge with Granite Guardian 4.1 (Apache-2.0) on vLLM, for tools tagged high
risk only (latency budget, ADR 0010). Granite Guardian takes the risk name through its chat
template; vLLM exposes that as chat_template_kwargs.

TODO(verify): guardian_config keys and the yes/no output format for the deployed version.
"""

from __future__ import annotations

from dataclasses import dataclass

import httpx

from ..models import Judge


@dataclass
class GraniteGuardian:
    base_url: str
    model: str
    timeout_ms: int = 4000
    risk: str = "jailbreak"

    @property
    def version(self) -> str:
        return self.model

    def judge(self, text: str, phase: str) -> Judge:
        role = "user" if phase in {"request", "llm_request"} else "assistant"
        body = {
            "model": self.model,
            "messages": [{"role": role, "content": text[:6000]}],
            "max_tokens": 8,
            "temperature": 0,
            "chat_template_kwargs": {"guardian_config": {"risk_name": self.risk}},
        }
        try:
            with httpx.Client(timeout=self.timeout_ms / 1000) as client:
                resp = client.post(f"{self.base_url.rstrip('/')}/chat/completions", json=body)
                resp.raise_for_status()
                answer = resp.json()["choices"][0]["message"]["content"].strip().lower()
        except (httpx.HTTPError, KeyError, IndexError, ValueError):
            return Judge(verdict="unknown", risk=self.risk, score=0.5)
        if answer.startswith("yes"):
            return Judge(verdict="yes", risk=self.risk, score=0.9)
        if answer.startswith("no"):
            return Judge(verdict="no", risk=self.risk, score=0.1)
        return Judge(verdict="unknown", risk=self.risk, score=0.5)
