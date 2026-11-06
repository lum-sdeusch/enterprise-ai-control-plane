# ADR 0032: Tool listings are the intersection of entitlement and the agent manifest; names are prefixed

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q32

## Context

Too many tools degrade models and expose surface. Tool name collisions across servers must
be impossible.

## Decision

tools/list is filtered by the adapter to group entitlement intersected with the agent
manifest's allowed tools, the caller's clearance and the agent's classification ceiling.
agentgateway prefixes tool names with the server name. Semantic tool search is an
extension.

## Consequences

- data.eacp.entitlement.visible_tools is the one policy query for listings.
