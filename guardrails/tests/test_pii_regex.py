from eacp_guardrails.detectors.pii import detect_regex
from eacp_guardrails.redact import redact


def types(text: str) -> list[str]:
    return [f.type for f in detect_regex("t", text)]


def test_email_and_phone():
    assert types("write to jane.doe@example.org or call +1 415-555-0134") == ["EMAIL_ADDRESS", "PHONE_NUMBER"]


def test_luhn_validation_for_cards():
    assert types("card 4111 1111 1111 1111") == ["CREDIT_CARD"]  # valid Luhn
    assert types("card 4111 1111 1111 1112") == []  # invalid checksum


def test_iban_mod97_validation():
    assert types("IBAN DE89 3704 0044 0532 0130 00") == ["IBAN_CODE"]
    assert types("IBAN DE89 3704 0044 0532 0130 01") == []


def test_ssn():
    assert types("ssn 123-45-6789") == ["US_SSN"]
    assert types("ssn 000-45-6789") == []


def test_numeric_ids_are_not_phone_numbers():
    assert types("ticket 4412 account 9981234567890 amount 1200.50") == []


def test_redaction_preserves_surroundings():
    text = "note=jane.doe@example.org&acct=4412"
    out = redact(text, detect_regex("t", text))
    assert out == "note=[REDACTED:EMAIL_ADDRESS]&acct=4412"
