"""Optional zero-shot sensitivity labelling with Lev (Apache-2.0 open implementation of the Jev
decision-model API). Only used as the fourth classification source behind a flag, cloud-gpu
profile only (ADR 0037). Label order is fixed because reordering shifts the model's scores.

TODO(verify): the exact /v1/systemone request and answer schema of the deployed Lev version.
"""

from __future__ import annotations

from dataclasses import dataclass

import httpx

from ..models import Sensitivity

LABELS = ["public", "internal", "confidential", "restricted"]
QUESTION = (
    "Which data sensitivity classification applies to this text under a typical corporate "
    "policy? public: safe to publish; internal: employees only; confidential: named teams only; "
    "restricted: personal, financial, health or legally privileged data."
)


@dataclass
class Lev:
    url: str
    timeout_ms: int = 2000
    min_confidence: float = 0.6
    max_chars: int = 8000

    version = "lev/systemone"

    def classify(self, text: str) -> Sensitivity | None:
        payload = {
            "state": text[: self.max_chars],
            "questions": [{"id": "sensitivity", "type": "choice", "question": QUESTION, "options": LABELS}],
        }
        try:
            with httpx.Client(timeout=self.timeout_ms / 1000) as client:
                resp = client.post(self.url, json=payload)
                resp.raise_for_status()
                data = resp.json()
        except (httpx.HTTPError, ValueError):
            return None  # optional source: silence is the safe answer, the other sources still apply
        answer = (data.get("answers") or [{}])[0]
        label = str(answer.get("choice", "")).lower()
        confidence = float(answer.get("confidence", 0.0))
        if label not in LABELS or confidence < self.min_confidence:
            return None
        return Sensitivity(label=label, confidence=confidence)
