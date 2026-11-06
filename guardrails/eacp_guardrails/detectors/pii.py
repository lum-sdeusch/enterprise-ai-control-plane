"""PII detection. Two engines behind one function:

- regex: dependency-free fallback with validation where a checksum exists (Luhn for cards,
  mod-97 for IBAN). Good enough for the laptop profile and for tests.
- presidio: Microsoft Presidio (MIT) with its spaCy engine, plus GLiNER2-PII when installed.
  Imported lazily so the service starts without the ML stack.
"""

from __future__ import annotations

import re
from typing import Callable

from ..models import Finding

REGEX_VERSION = "regex/1"

_EMAIL = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
# Phone numbers: require either an international prefix or separators, and 10 to 15 digits in
# total, so plain numeric ids and amounts are not flagged.
_PHONE = re.compile(
    r"(?<![\w.])(?<!\d[\s.-])"  # not preceded by another digit group (card-shaped numbers)
    r"(?:\+\d{1,3}[\s.-]?)?(?:\(\d{2,4}\)[\s.-]?|\d{2,4}[\s.-])\d{3,4}[\s.-]?\d{3,4}"
    r"(?![\w.])(?![\s.-]\d)"  # nor followed by one
)
_CARD = re.compile(r"(?<!\d)(?:\d[ -]?){13,19}(?!\d)")
_IBAN = re.compile(r"\b[A-Z]{2}\d{2}(?:[ ]?[A-Z0-9]{4}){2,7}[ ]?[A-Z0-9]{1,4}\b")
_SSN = re.compile(r"\b(?!000|666|9\d{2})\d{3}-(?!00)\d{2}-(?!0000)\d{4}\b")


def _luhn_ok(digits: str) -> bool:
    total = 0
    for i, ch in enumerate(reversed(digits)):
        d = int(ch)
        if i % 2 == 1:
            d *= 2
            if d > 9:
                d -= 9
        total += d
    return total % 10 == 0


def _iban_ok(iban: str) -> bool:
    s = iban.replace(" ", "").upper()
    if not 15 <= len(s) <= 34:
        return False
    rearranged = s[4:] + s[:4]
    numeric = "".join(str(ord(c) - 55) if c.isalpha() else c for c in rearranged)
    return int(numeric) % 97 == 1


def detect_regex(path: str, text: str) -> list[Finding]:
    found: list[Finding] = []
    for m in _EMAIL.finditer(text):
        found.append(Finding(path=path, type="EMAIL_ADDRESS", start=m.start(), end=m.end(), score=0.9))
    for m in _SSN.finditer(text):
        found.append(Finding(path=path, type="US_SSN", start=m.start(), end=m.end(), score=0.85))
    for m in _IBAN.finditer(text):
        if _iban_ok(m.group()):
            found.append(Finding(path=path, type="IBAN_CODE", start=m.start(), end=m.end(), score=0.9))
    for m in _CARD.finditer(text):
        digits = re.sub(r"[ -]", "", m.group())
        if 13 <= len(digits) <= 19 and _luhn_ok(digits) and not _overlaps(found, m.start(), m.end()):
            found.append(Finding(path=path, type="CREDIT_CARD", start=m.start(), end=m.end(), score=0.85))
    for m in _PHONE.finditer(text):
        digits = re.sub(r"\D", "", m.group())
        if 10 <= len(digits) <= 15 and not _overlaps(found, m.start(), m.end()):
            found.append(Finding(path=path, type="PHONE_NUMBER", start=m.start(), end=m.end(), score=0.6))
    found.sort(key=lambda f: f.start)
    return found


def _overlaps(found: list[Finding], start: int, end: int) -> bool:
    return any(f.start < end and start < f.end for f in found)


class PresidioEngine:
    """Lazy Presidio wrapper. Construction fails with ImportError if the extra is missing."""

    VERSION = "presidio"

    def __init__(self, language: str = "en", score_threshold: float = 0.5) -> None:
        from presidio_analyzer import AnalyzerEngine  # type: ignore[import-not-found]

        self._engine = AnalyzerEngine()
        self._language = language
        self._threshold = score_threshold
        try:  # GLiNER2-PII recogniser (Apache-2.0) when the extra is installed.
            from presidio_analyzer.predefined_recognizers import GLiNERRecognizer  # type: ignore[import-not-found]

            self._engine.registry.add_recognizer(
                GLiNERRecognizer(model_name="urchade/gliner_multi_pii-v1", entity_mapping={"person": "PERSON", "organization": "ORGANIZATION"})
            )
            self.VERSION = "presidio+gliner"
        except Exception:  # noqa: BLE001 - optional component
            pass

    def detect(self, path: str, text: str) -> list[Finding]:
        results = self._engine.analyze(text=text, language=self._language, score_threshold=self._threshold)
        return [Finding(path=path, type=r.entity_type, start=r.start, end=r.end, score=float(r.score)) for r in results]


Detector = Callable[[str, str], list[Finding]]


def build(engine: str, language: str, threshold: float) -> tuple[Detector, str]:
    if engine == "presidio":
        p = PresidioEngine(language=language, score_threshold=threshold)
        return p.detect, p.VERSION
    return detect_regex, REGEX_VERSION
