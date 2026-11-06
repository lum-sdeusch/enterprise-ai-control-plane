# ADR 0045: Token budgets enforced, currency reported

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q45

## Context

agentgateway enforces token budgets keyed by JWT claims; currency budgets would need custom
code.

## Decision

Tokens per user and per team per day are enforced in the gateway; Langfuse reports
currency. Exhaustion is a deny with a clear message and the sixth demo.

## To verify at build time

- Whether one rate-limit entry keyed by jwt.team can carry a different budget per team, or whether per-team policies are needed.
