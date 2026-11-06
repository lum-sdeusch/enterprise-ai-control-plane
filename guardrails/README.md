# eacp-guardrails

The probabilistic half of the policy pathway (ADR 0007). One HTTP endpoint, `POST /v1/analyze`,
takes a map of texts and returns findings plus redacted copies. It never decides; OPA does.

| Detector | Default engine | Alternative | Licence |
|---|---|---|---|
| secrets | built-in regex rules (gitleaks-style subset) | | Apache-2.0 (this repo) |
| pii | regex fallback (email, phone, card with Luhn, IBAN with mod-97, SSN) | Presidio + GLiNER2-PII | MIT, Apache-2.0 |
| injection | Qwen3Guard-Gen through an OpenAI-compatible endpoint (Ollama on the laptop, vLLM on GPU) | | Apache-2.0 model |
| sensitivity | off | Lev (open Jev clone) through `/v1/systemone` | Apache-2.0 model |
| judge | off | Granite Guardian 4.1 through vLLM | Apache-2.0 model |

Every finding carries the path of the text it was found in, offsets, a type and a score. Every
response names the model versions used, so a decision can be reproduced (ADR 0037 operating
rules).

Failure semantics: a detector that is configured as required and cannot answer makes the whole
call fail with HTTP 503. The adapter then fails closed. A detector that is disabled is simply
absent from the findings.

```
POST /v1/analyze
{"phase": "request", "texts": {"arguments.url": "https://x?note=jane@example.org"},
 "checks": ["pii", "secrets", "injection"], "context": {"server": "web-fetch", "tool": "fetch"}}

200
{"findings": {"pii": [{"path": "arguments.url", "type": "EMAIL_ADDRESS", "start": 18, "end": 34, "score": 0.9}],
              "secrets": [], "injection": {"score": 0.05, "label": "safe", "categories": []}, "sensitivity": null},
 "redacted": {"arguments.url": "https://x?note=[REDACTED:EMAIL_ADDRESS]"},
 "model_versions": {"pii": "regex/1", "secrets": "rules/2026.11", "injection": "qwen3guard-gen-0.6b"},
 "latency_ms": {"pii": 0.4, "secrets": 0.2, "injection": 180.3}}
```

Run locally: `pip install -e ".[test]" && uvicorn eacp_guardrails.app:app --port 8000`.
