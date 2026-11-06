# ADR 0006: Every token carries the user and an actor chain; scheduled agents have a service identity with a named owner

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q6
- Related: [ADR 0023](0023-keycloak-with-preview-delegation-as-the-primary-actor-chain.md)

## Context

"Agent identity" can mean an agent acting purely for a user, an agent as its own principal,
or both. Policies need both: who the human is and which software acted. Autonomous agents
with no human in the loop break the delegation-only model.

## Decision

Tokens carry the user subject plus an actor chain (user, then agent, then gateway) through
RFC 8693 token exchange with an actor token. Scheduled agents authenticate with a workload
identity and their manifest names a human owner, so there is always someone accountable.

## Alternatives considered

- **Delegation only.** Cannot represent scheduled agents at all.
- **Agent-only identity.** Loses the human for audit and entitlement.

## Consequences

- The policy input has subject.sub, subject.groups, subject.team, subject.clearance and subject.act.
- Keycloak's delegation flow is preview in 26.8, so a two-header fallback exists behind a flag.
