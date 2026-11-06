"""Runs the requested detectors over every text and assembles one response."""

from __future__ import annotations

import logging
import time

from .config import Settings
from .detectors import injection as injection_mod
from .detectors import judge as judge_mod
from .detectors import pii as pii_mod
from .detectors import secrets as secrets_mod
from .detectors import sensitivity as sensitivity_mod
from .models import AnalyzeRequest, AnalyzeResponse, Findings, Injection
from .redact import redact

log = logging.getLogger("eacp.guardrails")


class RequiredDetectorUnavailable(RuntimeError):
    """A detector that policy relies on could not answer; the caller must fail closed."""


class Analyzer:
    def __init__(self, settings: Settings) -> None:
        self.s = settings
        self.pii, self.pii_version = pii_mod.build(settings.pii_engine, settings.pii_language, settings.pii_score_threshold)
        self.guard = (
            injection_mod.QwenGuard(settings.guard_base_url, settings.guard_model, settings.guard_timeout_ms, settings.guard_max_chars)
            if settings.guard_enabled
            else None
        )
        self.judge = (
            judge_mod.GraniteGuardian(settings.judge_base_url, settings.judge_model, settings.judge_timeout_ms)
            if settings.judge_enabled and settings.judge_base_url
            else None
        )
        self.sensitivity = (
            sensitivity_mod.Lev(settings.sensitivity_url, settings.sensitivity_timeout_ms, settings.sensitivity_min_confidence)
            if settings.sensitivity_enabled and settings.sensitivity_url
            else None
        )

    def versions(self) -> dict[str, str]:
        v = {"pii": self.pii_version, "secrets": secrets_mod.VERSION}
        if self.guard:
            v["injection"] = self.guard.version
        if self.judge:
            v["judge"] = self.judge.version
        if self.sensitivity:
            v["sensitivity"] = self.sensitivity.version
        return v

    def analyze(self, req: AnalyzeRequest) -> AnalyzeResponse:
        checks = set(req.checks)
        findings = Findings()
        latency: dict[str, float] = {}
        versions: dict[str, str] = {}

        if "secrets" in checks:
            t = time.perf_counter()
            for path, text in req.texts.items():
                findings.secrets.extend(secrets_mod.detect(path, text))
            latency["secrets"] = _ms(t)
            versions["secrets"] = secrets_mod.VERSION

        if "pii" in checks:
            t = time.perf_counter()
            for path, text in req.texts.items():
                findings.pii.extend(self.pii(path, text))
            latency["pii"] = _ms(t)
            versions["pii"] = self.pii_version

        if "injection" in checks and self.guard:
            t = time.perf_counter()
            worst: Injection | None = None
            for text in req.texts.values():
                if not text.strip():
                    continue
                try:
                    res = self.guard.classify(text, req.phase)
                except injection_mod.GuardUnavailable as exc:
                    if self.s.guard_required:
                        raise RequiredDetectorUnavailable(f"injection guard: {exc}") from exc
                    log.warning("injection guard unavailable, continuing without it: %s", exc)
                    res = None
                if res and (worst is None or res.score > worst.score):
                    worst = Injection(score=res.score, label=res.label, categories=sorted(set(worst.categories if worst else []) | set(res.categories)))
            findings.injection = worst or Injection(score=0.0, label="safe", categories=[])
            latency["injection"] = _ms(t)
            versions["injection"] = self.guard.version

        if "sensitivity" in checks and self.sensitivity:
            t = time.perf_counter()
            joined = "\n\n".join(req.texts.values())
            findings.sensitivity = self.sensitivity.classify(joined) if joined.strip() else None
            latency["sensitivity"] = _ms(t)
            versions["sensitivity"] = self.sensitivity.version

        if "judge" in checks and self.judge:
            t = time.perf_counter()
            joined = "\n\n".join(req.texts.values())
            findings.judge = self.judge.judge(joined, req.phase) if joined.strip() else None
            latency["judge"] = _ms(t)
            versions["judge"] = self.judge.version

        # Redacted copies: every PII and secret finding masked, per text.
        redacted = {}
        for path, text in req.texts.items():
            spans = [f for f in findings.pii + findings.secrets if f.path == path]
            redacted[path] = redact(text, spans) if spans else text

        return AnalyzeResponse(findings=findings, redacted=redacted, model_versions=versions, latency_ms=latency)


def _ms(start: float) -> float:
    return round((time.perf_counter() - start) * 1000, 2)
