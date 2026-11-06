# ADR 0027: Conversation id from baggage, subject-and-agent fallback, state in Valkey

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q27

## Context

The 2026-07-28 MCP spec removed protocol sessions, so session identity is the platform's to
define. Redis 8 is no longer OSI-licensed.

## Decision

The client sets a conversation id, carried as W3C baggage or an explicit header; the
fallback is the pair of user subject and agent client id with a 30-minute sliding window.
Taint, pins, quarantine flags, route decisions and pending approvals live in Valkey (BSD-3).

## Alternatives considered

- **Custom header only.** Does not propagate through OpenTelemetry-instrumented clients.

## Consequences

- The adapter's in-memory store exists for tests and single-replica demos only.
