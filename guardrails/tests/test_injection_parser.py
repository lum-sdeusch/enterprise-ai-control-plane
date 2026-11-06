from eacp_guardrails.detectors.injection import parse


def test_unsafe_jailbreak():
    r = parse("Safety: Unsafe\nCategories: Jailbreak")
    assert r.label == "unsafe"
    assert r.score >= 0.9
    assert r.categories == ["Jailbreak"]


def test_safe_none():
    r = parse("Safety: Safe\nCategories: None")
    assert r.label == "safe"
    assert r.score < 0.1
    assert r.categories == []


def test_controversial_taints_but_does_not_block():
    r = parse("Safety: Controversial\nCategories: Politically Sensitive Topics")
    assert 0.5 <= r.score < 0.8


def test_unparsable_answer_is_unknown_mid_score():
    r = parse("I cannot help with that.")
    assert r.label == "unknown"
    assert r.score == 0.5
