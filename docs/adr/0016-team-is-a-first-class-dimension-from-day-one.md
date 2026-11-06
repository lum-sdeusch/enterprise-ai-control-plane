# ADR 0016: Team is a first-class dimension from day one

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q16

## Context

Retrofitting tenancy is expensive; adding it to a POC is cheap.

## Decision

Identity groups map to a team; the team determines tool entitlements, budgets, approver
groups, model classes and trace visibility. Each team's MCP servers run in their own
namespace.

## Alternatives considered

- **Single-tenant POC.** Every later feature would need a tenancy retrofit.

## Consequences

- teams/<team>/manifest.yaml exists alongside tools and agents and generates namespaces, reference grants and the Keycloak groups.
