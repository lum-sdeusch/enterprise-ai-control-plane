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
