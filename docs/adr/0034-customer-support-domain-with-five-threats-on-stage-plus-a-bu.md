# ADR 0034: Customer-support domain with five threats on stage plus a budget demo

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q34

## Context

The demo needs restricted data, an exfiltration channel, a high-risk action and a
third-party server, all self-hosted so it runs offline except for the external model.

## Decision

Tools: a customer database over PostgreSQL, tickets, web fetch, email send backed by
Mailpit, and a deliberately poisonable server. Scenarios: rug-pull quarantine, injection in
a tool result, session-taint exfiltration block, classification routing, human approval on
email, and team budget exhaustion.

## Consequences

- Each scenario is a policy fixture, a pipeline unit test where applicable, and an end-to-end assertion in demo/scenarios/run.py.
