# ADR 0014: Named products are candidates; OPA is the catch-all policy engine

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q14

## Context

The originally named registry had no runtime policy hook. Cedar is typed and formally
validated but has no open-source AuthZEN decision point and less ecosystem; Rego handles
regex, JSON and text natively and comes with decision logs, bundles and an Envoy
integration.

## Decision

Treat every named product as a candidate until research settles it. OPA is the PDP. Cedar is
documented as the choice for teams that want typed tool-authorisation policies at the server
edge.

## Alternatives considered

- **Cedar as the central PDP.** Would need a self-built decision service and gives up decision logging and bundles.
- **OpenFGA.** Relationship graphs are not the problem here; attribute and content rules are.

## Consequences

- Policies are Rego v1 with a single entry point and layered packages (ADR 0044).
