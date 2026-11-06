"""Settings from the environment. Profiles (platform/models/<profile>/guardrails-profile.yaml)
set the model endpoints; everything else has a safe default."""

from __future__ import annotations

import os
from dataclasses import dataclass, field


def _bool(key: str, default: bool) -> bool:
    v = os.environ.get(key)
    if v is None or v == "":
        return default
    return v.strip().lower() in {"1", "true", "yes", "on"}


def _float(key: str, default: float) -> float:
    try:
        return float(os.environ.get(key, default))
    except ValueError:
        return default


def _int(key: str, default: int) -> int:
    try:
        return int(os.environ.get(key, default))
    except ValueError:
        return default


@dataclass
class Settings:
    profile: str = field(default_factory=lambda: os.environ.get("EACP_PROFILE", "dev"))

    # PII
    pii_engine: str = field(default_factory=lambda: os.environ.get("EACP_PII_ENGINE", "regex"))  # regex | presidio
    pii_score_threshold: float = field(default_factory=lambda: _float("EACP_PII_SCORE_THRESHOLD", 0.5))
    pii_language: str = field(default_factory=lambda: os.environ.get("EACP_PII_LANGUAGE", "en"))

    # Injection / jailbreak guard model (OpenAI-compatible chat endpoint).
    guard_base_url: str = field(default_factory=lambda: os.environ.get("EACP_GUARD_BASE_URL", ""))
    guard_model: str = field(default_factory=lambda: os.environ.get("EACP_GUARD_MODEL", ""))
    guard_timeout_ms: int = field(default_factory=lambda: _int("EACP_GUARD_TIMEOUT_MS", 1500))
    guard_required: bool = field(default_factory=lambda: _bool("EACP_GUARD_REQUIRED", True))
    guard_max_chars: int = field(default_factory=lambda: _int("EACP_GUARD_MAX_CHARS", 6000))

    # Judge (Granite Guardian) for high-risk tools; optional.
    judge_enabled: bool = field(default_factory=lambda: _bool("EACP_JUDGE_ENABLED", False))
    judge_base_url: str = field(default_factory=lambda: os.environ.get("EACP_JUDGE_BASE_URL", ""))
    judge_model: str = field(default_factory=lambda: os.environ.get("EACP_JUDGE_MODEL", ""))
    judge_timeout_ms: int = field(default_factory=lambda: _int("EACP_JUDGE_TIMEOUT_MS", 4000))

    # Sensitivity labeller (Lev); optional, cloud-gpu only (ADR 0037).
    sensitivity_enabled: bool = field(default_factory=lambda: _bool("EACP_SENSITIVITY_ENABLED", False))
    sensitivity_url: str = field(default_factory=lambda: os.environ.get("EACP_SENSITIVITY_URL", ""))
    sensitivity_timeout_ms: int = field(default_factory=lambda: _int("EACP_SENSITIVITY_TIMEOUT_MS", 2000))
    sensitivity_min_confidence: float = field(default_factory=lambda: _float("EACP_SENSITIVITY_MIN_CONFIDENCE", 0.6))

    @property
    def guard_enabled(self) -> bool:
        return bool(self.guard_base_url and self.guard_model)


def load() -> Settings:
    return Settings()
