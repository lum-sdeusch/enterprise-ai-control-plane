# ADR 0009: Redact at the collector, keep a separate append-only audit log, make content capture a switch

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q9

## Context

Prompts and tool results in spans make the trace store the most sensitive datastore in the
company. Debugging traces are sampled and purged in days; compliance records must be
unsampled and kept.

## Decision

The OpenTelemetry Collector's redaction processor masks PII and secrets before any exporter.
The adapter writes an unsampled audit record per decision phase to an insert-only store,
separate from traces. Content capture into spans is a per-environment switch, on in the POC.

## Alternatives considered

- **Capture everything and lock the trace store down.** Access control is not minimisation; the data would still be there.
- **Metadata only.** Loses the ability to show what the guardrails saw during a demo or an investigation.

## Consequences

- The adapter is the only content-capture point (ADR 0021).
- OPA decision logs are masked by a system.log policy so arguments never reach the log store.
