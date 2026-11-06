# ADR 0005: OWASP threat taxonomies and SOC 2, ISO 42001 and EU AI Act Article 12 as mappings

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q5

## Context

"Compliance" without named frameworks is marketing. The threats the demo defeats must be
named from a taxonomy readers already use.

## Decision

Threat taxonomy: OWASP Top 10 for LLM Applications and OWASP Top 10 for Agentic
Applications. Control mappings: SOC 2 control families, ISO/IEC 42001, EU AI Act Article 12
record keeping. HIPAA and PCI DSS are extension notes. Five threats are demonstrated on
stage; the rest are a table.

## Alternatives considered

- **Map to every framework a prospect might name.** Each mapping is a reviewed document; unreviewed mappings are a liability.

## Consequences

- docs/threat-model.md and docs/control-mapping.md are seeds that a compliance reviewer must check against the licensed texts before publication.
