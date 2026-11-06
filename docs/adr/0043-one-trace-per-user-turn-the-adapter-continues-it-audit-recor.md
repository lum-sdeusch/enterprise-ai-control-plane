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

## To verify at build time

- The AgentgatewayPolicy field that fills McpResponse.metadata_context from CEL, including the stashed tool name and conversation id.
