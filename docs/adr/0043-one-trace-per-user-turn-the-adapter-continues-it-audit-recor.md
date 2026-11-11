# ADR 0043: One trace per user turn; the adapter continues it; audit records carry the trace id

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q43

## Context

agentgateway's tool hook receives the allow-listed incoming headers on the request phase, so
trace context and baggage reach the adapter. The response phase receives only a CEL-computed
metadata block, and tool errors skip it entirely.

## Decision

The client roots the trace. agentgateway adds its spans. The adapter continues the trace from
the request headers and, on the response phase, from the metadata block, adding child spans
for guardrails, policy, taint and audit. Langfuse groups by conversation id. Every audit
record stores the trace id. Everything is sampled in the POC.

## Consequences

- Answered in ticket 04 against agentgateway 1.7.0: the processor's metadata map (CRD remote.metadata) is evaluated on both phases. What the request phase returns in McpRequestResult.metadata is readable afterwards as mcpGuardrails.<key>, so expressions such as mcpGuardrails.eacp_tool hand the tool name, server and resolved conversation id to the response phase. No correlation fallback is needed, and none is implemented: a response phase without the stashed tool name is denied.
- The traceparent the gateway exposes in CEL (request.headers) already carries the gateway's own span id, and with tracing enabled the gateway also sends a traceparent on the gRPC call to the hook. The adapter prefers the latter, so its spans sit under the gateway's policy-call span. test/pathway asserts one trace per call with the adapter's request and response spans under a gateway span.
- A tool's own error result (isError true) still passes through the response phase; only protocol-level errors skip it.
