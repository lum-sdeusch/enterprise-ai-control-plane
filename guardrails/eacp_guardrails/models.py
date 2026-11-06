"""Wire models. They mirror adapter/internal/guardrails/client.go field for field."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field

ALL_CHECKS = ["pii", "secrets", "injection", "sensitivity", "judge"]


class Finding(BaseModel):
    path: str
    type: str
    start: int
    end: int
    score: float = 1.0


class Injection(BaseModel):
    score: float
    label: str  # safe | controversial | unsafe | unknown
    categories: list[str] = Field(default_factory=list)


class Sensitivity(BaseModel):
    label: str  # public | internal | confidential | restricted
    confidence: float


class Judge(BaseModel):
    verdict: str  # yes | no | unknown
    risk: str
    score: float


class Findings(BaseModel):
    pii: list[Finding] = Field(default_factory=list)
    secrets: list[Finding] = Field(default_factory=list)
    injection: Injection | None = None
    sensitivity: Sensitivity | None = None
    judge: Judge | None = None


class AnalyzeRequest(BaseModel):
    phase: str = "request"
    texts: dict[str, str] = Field(default_factory=dict)
    checks: list[str] = Field(default_factory=lambda: ["pii", "secrets", "injection"])
    context: dict[str, Any] = Field(default_factory=dict)


class AnalyzeResponse(BaseModel):
    findings: Findings
    redacted: dict[str, str]
    model_versions: dict[str, str]
    latency_ms: dict[str, float]
