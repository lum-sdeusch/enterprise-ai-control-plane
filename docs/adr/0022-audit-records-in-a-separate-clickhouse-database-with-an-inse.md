# ADR 0022: Audit records in a separate ClickHouse database with an insert-only role

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q22

## Context

OPA decision logs and the adapter's audit events need an append-only home with their own
credentials. ClickHouse is already deployed for Langfuse; OpenSearch, Loki and object
storage with object lock were the alternatives.

## Decision

A separate ClickHouse database, eacp_audit, with an insert-only writer and a read-only
reader. Production exports daily to object-locked storage controlled by a different team.

## Alternatives considered

- **OpenSearch.** A new component for no gain in the POC.
- **Object storage with object lock.** MinIO's community edition was gutted and the Apache-2.0 alternatives are young.

## Consequences

- The adapter writes the record synchronously; with EACP_AUDIT_REQUIRED a failed write denies the call.
