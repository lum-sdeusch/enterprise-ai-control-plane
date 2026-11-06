from eacp_guardrails.detectors import secrets


def types(text: str) -> list[str]:
    return [f.type for f in secrets.detect("t", text)]


def test_vendor_keys_are_detected():
    assert types("key AKIAIOSFODNN7EXAMPLE here") == ["aws_access_key_id"]
    assert types("ghp_" + "a" * 36) == ["github_token"]
    assert types("token sk-ant-api03-" + "x" * 40) == ["anthropic_api_key"]
    assert types("xoxb-123456789012-abcdefghijkl") == ["slack_token"]
    assert types("AIza" + "b" * 35) == ["google_api_key"]
    assert types("sk_live_" + "c" * 24) == ["stripe_key"]


def test_private_key_and_jwt():
    assert types("-----BEGIN RSA PRIVATE KEY-----\nMIIE...") == ["private_key"]
    jwt = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbGljZSJ9.c2lnbmF0dXJlc2lnbmF0dXJl"
    assert types(jwt) == ["jwt"]


def test_generic_credential_has_lower_score():
    found = secrets.detect("t", "password = hunter2hunter2hunter2")
    assert found[0].type == "generic_credential"
    assert found[0].score < 0.9


def test_benign_text_has_no_findings():
    assert types("The quarterly numbers look fine; ticket 4412 is closed.") == []


def test_overlapping_matches_are_reported_once():
    text = "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    found = secrets.detect("t", text)
    assert [f.type for f in found] == ["aws_secret_access_key"]
    assert text[found[0].start : found[0].end] == "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
