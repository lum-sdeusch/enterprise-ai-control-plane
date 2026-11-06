# ADR 0010: Fail closed everywhere, with latency budgets per check class

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q10

## Context

Fail-open is what every operations team wants and every auditor rejects. Without budgets,
an LLM judge on every call adds seconds and gets ripped out within a week.

## Decision

Every hook fails closed: PDP, guardrails, state store or audit store unavailable means deny,
and the gateway processors are configured FailClosed. Budgets: deterministic checks plus PDP
under 20 ms, ML classifiers under 200 ms synchronous, LLM judge only for tools tagged high
risk, otherwise asynchronous and alert-only.

## Alternatives considered

- **Fail open with an alert.** Turns an outage into an unaudited bypass.

## Consequences

- The adapter's unit tests cover every dependency failure.
- ContextForge's fail-open token revocation on database outage was one reason it was not chosen (ADR 0017).
