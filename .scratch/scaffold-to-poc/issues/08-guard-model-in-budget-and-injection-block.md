# 08: Guard model within the latency budget and the injection block (scenario 2)

**What to build:** The injection classifier runs on the laptop profile inside the agreed budget,
and the scenario plays end to end: a documentation page served inside the cluster carries
instructions aimed at the model; the fetch result is screened, blocked with the injection code,
the conversation is tainted as untrusted, and a destructive ticket action attempted afterwards
requires a human approval even though the manifest did not demand one. If the Qwen3Guard GGUF
cannot be served on CPU within budget, the smaller DeBERTa prompt-injection classifier from the
October 2026 Red Hat benchmark replaces it for the laptop profile, license permitting.

**Blocked by:** 06 web-fetch via kmcp and taint exfil block

**Status:** ready-for-agent

- [ ] A guard model is served on the laptop profile and the guardrail service reports its version in every finding.
- [ ] Measured p95 latency of the injection check on four CPU cores is under 200 ms for a two-kilobyte text, or the alternative classifier is adopted with its license recorded in ADR 0029.
- [ ] An in-cluster documentation server hosts the demo pages, including one injected page; the fetch host allow-list points at it.
- [ ] Fetching the injected page returns a tool error with `injection_in_tool_result`; the conversation shows `touched_untrusted_content`.
- [ ] A `close_ticket` call in that conversation returns an approval link, while the same call in a fresh conversation does not.
- [ ] A stopped guard model makes the guardrail service answer 503 and the adapter fail closed, and the audit row shows it.
