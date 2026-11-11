# ADR 0019: The adapter is the single enforcement orchestrator, with OPA as its sidecar

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q19

## Context

Chaining external authorisation to OPA for the request phase and a separate guardrail
service for the response phase would leave OPA blind to tool results, so post-tool policy
would live in a second service anyway. One adapter makes fail-closed, budgets, taint and
audit one coherent thing.

## Decision

One adapter implements agentgateway's ExtMCP hook, the LLM prompt-guard webhook and the
routing callout. For each phase it runs guardrails, reads and writes taint, queries the OPA
sidecar over localhost, handles approvals, writes the audit record and fails closed. Go for
the adapter, Python for the guardrail service. Budget revised to 2,500 to 3,000 lines.

## Alternatives considered

- **ext_authz chain.** Cannot see tool results; splits the fail-closed logic across services.

## Consequences

- adapter/internal/pipeline is the one place to read to understand enforcement order.
- Verified against agentgateway 1.7.0 (ticket 04, test/pathway): the hook learns everything from metadata_context, filled by CEL expressions on the processor. The contract is documented in adapter/internal/extmcp and set identically in charts/mcp-tool, platform/charts/eacp-gateway and the pathway test.
- The adapter's only identity source is the `jwt` metadata entry, the claims of the token the gateway verified. The Authorization header does not reach the hook, and the adapter no longer decodes bearer tokens at all: on a route that lacks a JWT policy it would otherwise trust a token nobody verified. Such a route gets a denial.
- A denied tools/call reaches the client as a tool result with isError true whose text is the denial; the gateway drops structured error data for that method. The text is therefore a JSON object carrying eacp_code, every reason code and the conversation id. Denied listings are JSON-RPC errors with the same data.
- The pathway test found three defects no unit test had - an adapter that crashed at start on an OpenTelemetry schema conflict, a policy bundle that shipped without its data, and denial codes that never reached the client. It now runs in CI on every pull request.
