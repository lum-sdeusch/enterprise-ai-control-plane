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

**Status:** in-progress (built and verified without a cluster; cluster verification pending)

- [x] The gateway's guardrail policy is configured so the request-phase hook receives the verified identity (through metadata context or the forwarded Authorization header) plus `traceparent` and `baggage`; the exact configuration is committed to the platform chart.
- [x] The response-phase hook receives the stashed tool name and conversation id, or the adapter's correlation fallback is implemented, tested and its concurrency caveat written into ADR 0043.
- [ ] `tools/list` for alice shows only the tools the policy's visible set allows; a user outside the required group sees none of that server's tools.
- [ ] A call denied by policy reaches the client as a tool error whose data carries the `eacp_code` and conversation id.
- [ ] Every request and response phase writes one row to the audit table with subject, agent, decision, reasons, policy revision and trace id.
- [ ] One tool call produces a single trace in Jaeger containing the gateway spans and the adapter's request and response spans.
- [ ] A deliberately stopped OPA sidecar makes every call fail closed, and the audit row records `fail_closed`.

## Progress (2026-11-11)

Boxes 1 and 2 are closed. Boxes 3 to 7 hold in `test/pathway` (the real agentgateway 1.7.0,
adapter, OPA, guardrail service, tickets server and ClickHouse on one machine, 48 checks) and stay
open until `make pathway-cluster` passes on the laptop cluster, which no one has run yet.

What the hook contract turned out to be (details in ADR 0019 and ADR 0043):

- Identity arrives only as the `jwt` metadata entry (the verified claims). The Authorization
  header is never forwarded to the hook, so that option in box 1 does not exist; the adapter's
  bearer-token fallback was removed because it trusted unverified tokens on a route without a
  JWT policy.
- Request-phase metadata is readable on the response phase as `mcpGuardrails.<key>`. The
  correlation fallback in box 2 is not needed and was not built.
- A denied `tools/call` reaches the client as a tool result with `isError: true` whose text is the
  denial; the gateway drops structured error data for that method. Box 4's "data carries" is met
  by making that text a JSON object with `eacp_code` and `conversation_id`.

Defects found on the way and fixed: the policy bundle shipped without data (files must be named
`data.json`), the adapter crashed at start on an OpenTelemetry schema conflict, the ClickHouse
init SQL never expanded its password variables, and the approval page blocked start-up on
Keycloak.

To close the ticket, on the laptop:

```bash
make down && EACP_REVISION=feat/04-adapter-in-the-pathway make up PROFILE=laptop
make smoke
make pathway-cluster
```

Unverified until then: `scripts/up.sh` (local platform images, the policy ConfigMap, the Argo CD
values patch), the CRD fields `allowedRequestHeaders` and `metadata` as a live controller applies
them, the ReferenceGrants for the adapter backend, Kyverno admitting the `eacp.local` images in
`eacp-system`, and pausing the OPA sidecar on the kind node in `demo/scenarios/pathway.py`.
