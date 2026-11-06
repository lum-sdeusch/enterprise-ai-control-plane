# 04: Adapter in the pathway with real request and response context

**What to build:** The adapter with its OPA sidecar and the guardrail service (regex PII only,
no guard model yet) run in the cluster and every tickets call passes through both hook phases.
The request phase receives the caller's verified identity and the trace context; the response
phase receives the tool name and conversation id that the request phase stashed, or the adapter
falls back to its own correlation with the caveat documented. A user sees only entitled tools in
listings, a denied call shows the control plane's error code in the client, every phase writes
an audit row, and the trace in Jaeger shows the gateway and adapter spans with the policy
attributes. This is the slice that decides whether taint and approvals work as designed.

**Blocked by:** 03 First tool through the gateway with OAuth

**Status:** ready-for-agent

- [ ] The gateway's guardrail policy is configured so the request-phase hook receives the verified identity (through metadata context or the forwarded Authorization header) plus `traceparent` and `baggage`; the exact configuration is committed to the platform chart.
- [ ] The response-phase hook receives the stashed tool name and conversation id, or the adapter's correlation fallback is implemented, tested and its concurrency caveat written into ADR 0043.
- [ ] `tools/list` for alice shows only the tools the policy's visible set allows; a user outside the required group sees none of that server's tools.
- [ ] A call denied by policy reaches the client as a tool error whose data carries the `eacp_code` and conversation id.
- [ ] Every request and response phase writes one row to the audit table with subject, agent, decision, reasons, policy revision and trace id.
- [ ] One tool call produces a single trace in Jaeger containing the gateway spans and the adapter's request and response spans.
- [ ] A deliberately stopped OPA sidecar makes every call fail closed, and the audit row records `fail_closed`.
