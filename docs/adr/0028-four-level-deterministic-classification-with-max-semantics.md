# ADR 0028: Four-level deterministic classification with max semantics

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q28

## Context

Routing and taint need one label per conversation turn. Sources disagree, so a merge rule is
needed, and the merge must be explainable to an auditor.

## Decision

Labels public, internal, confidential, restricted. Sources: the tool manifest's output
classification, guardrail findings (PII makes a turn confidential), conversation taint, and
an explicit data_label claim. The maximum wins. Clearance is a separate ceiling claim on the
caller, not a source.

## Alternatives considered

- **An ML classifier in the POC.** Not reproducible; kept optional behind a flag (ADR 0037).

## Consequences

- The label travels as a span attribute and in the audit record, so routing can be proven from the trace.
