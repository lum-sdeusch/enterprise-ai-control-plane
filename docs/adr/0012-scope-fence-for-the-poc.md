# ADR 0012: Scope fence for the POC

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q12

## Context

The components not in the original list each either close a real enterprise gap or double
the surface area.

## Decision

In scope: secrets management, MCP server hardening and supply chain, human-in-the-loop
approval, red-teaming in CI, cost controls, GitOps. Out of scope as documented extensions:
RAG with permission-aware retrieval, Agent-to-Agent gateway, semantic cache.

## Alternatives considered

- **Include RAG.** Permission-aware retrieval is a project of its own.
- **Include A2A.** Doubles the protocol surface for no demo value.

## Consequences

- docs/design.md section 10 lists the extensions so readers know they were considered.
