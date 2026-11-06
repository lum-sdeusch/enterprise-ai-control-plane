# 07: LLM path: routing callout, prompt guard, failover (scenario 4)

**What to build:** A client asks the gateway for the `eacp-auto` model. The gateway calls the
adapter, which answers with the model class the conversation's classification allows; the
prompt guard masks or rejects before the model sees anything; the completion comes back
guarded. A public prompt is answered by the external model, a prompt in a conversation that
touched restricted data is answered in-house, a prompt containing a secret is rejected, and PII
bound for the external model is masked. When the primary external model fails, the secondary
takes over, and when the callout fails, everything stays in-house.

**Blocked by:** 03 First tool through the gateway with OAuth; 04 Adapter in the pathway

**Status:** ready-for-agent

- [ ] The Anthropic models are defined on the gateway with the confirmed fields; the in-house model answers through Ollama; the external virtual model fails over to the secondary when the primary is evicted (demonstrated with an invalid key or a forced error).
- [ ] The callout contract is confirmed: the adapter's answer selects the target, whether by virtual or concrete name, and the decision is recorded in ADR 0018.
- [ ] The prompt guard attaches to the confirmed target kind; request and response phases both reach the adapter with the identity headers.
- [ ] A public prompt is served by an Anthropic model and the access log and audit row name it; the same prompt in a restricted-tainted conversation is served in-house.
- [ ] A prompt with a seeded access key is rejected with `secret_in_prompt`; a prompt with an email bound for the external model arrives masked, shown by the external model echoing the redaction marker.
- [ ] Stopping the adapter makes the callout fall back in-house rather than failing the request, and the audit row records `fail_closed`.
