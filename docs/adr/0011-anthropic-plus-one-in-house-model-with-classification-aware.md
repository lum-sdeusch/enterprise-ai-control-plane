# ADR 0011: Anthropic plus one in-house model, with classification-aware routing

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q11

## Context

A request that touched restricted data must not leave for an external provider. That is
the single most convincing compliance demo available and it reuses the guardrail findings.

## Decision

Both an external provider (Anthropic) and a self-hosted open-weight model. The gateway's
virtual model calls the adapter, which returns the model class the conversation's label
allows; restricted and confidential go in-house, the rest may go external.

## Alternatives considered

- **External only.** No credible answer to data residency.

## Consequences

- agentgateway's callout selector made this native rather than a workaround (ADR 0018).
- The in-house model doubles as the fallback for every virtual model, so a routing outage never pushes data outward.
