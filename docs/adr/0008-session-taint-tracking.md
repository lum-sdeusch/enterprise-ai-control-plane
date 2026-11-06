# ADR 0008: Session taint tracking

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q8
- Related: [ADR 0027](0027-conversation-id-from-baggage-subject-and-agent-fallback-stat.md)

## Context

The attacks that matter are only visible across calls: read a customer record in call three,
fetch a URL with two kilobytes of query string in call seven. Each call looks fine alone.

## Decision

The adapter keeps three boolean labels per conversation (touched_pii, touched_restricted,
touched_untrusted_content) in Valkey and passes them to the PDP on every call. Tools declare
a capability class; the headline rule denies exfiltration-capable tools after restricted
data was read.

## Alternatives considered

- **Stateless per-call checks only.** Misses the chaining attacks the demo is built to show.
- **Full information-flow control.** Far beyond a POC; noted as future work.

## Consequences

- Session identity had to be defined because the 2026-07-28 MCP spec removed protocol sessions (ADR 0027).
- Taint is written on response phases and read on request phases, including when a result was blocked.
