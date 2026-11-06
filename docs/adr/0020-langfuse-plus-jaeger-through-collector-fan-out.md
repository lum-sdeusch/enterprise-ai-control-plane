# ADR 0020: Langfuse plus Jaeger through collector fan-out

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q20

## Context

Langfuse is MIT core (owned by ClickHouse), ingests OTLP, renders tool spans, sessions and
cost, and has OIDC single sign-on and organisation-level roles in the free tier; project
roles, retention policies and server-side masking are enterprise. Opik is Apache-2.0 but has
no user management. Phoenix is Elastic-licensed. SigNoz gates OIDC. Jaeger renders every
span but knows nothing about LLMs.

## Decision

The OpenTelemetry Collector exports GenAI spans to Langfuse and all spans to Jaeger. One
Langfuse organisation per team stands in for project roles. Retention is a ClickHouse TTL.
Jaeger uses Badger in the POC and OpenSearch in production notes.

## Alternatives considered

- **Opik behind an OIDC proxy.** No per-team visibility without user management.
- **Arize Phoenix.** Elastic License 2.0 is not OSI-approved.

## Consequences

- Langfuse needs S3-compatible storage and a Redis-compatible cache; SeaweedFS and Valkey are wired and must be verified (open item).
