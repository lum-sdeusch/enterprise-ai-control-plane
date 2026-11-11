# ADR 0023: Keycloak, with preview delegation as the primary actor-chain path

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q23

## Context

Keycloak 26.8 (Apache-2.0, CNCF incubating) has standard token exchange and Kubernetes
service-account federation generally available; delegation with actor tokens and the act
claim is preview, the Identity Assertion grant behind Enterprise-Managed Authorization is
experimental, AuthZEN is experimental. Zitadel is ahead on delegation but AGPL. Authentik
gates agent accounts. Ory Hydra has no token exchange.

## Decision

Keycloak. The preview delegation flow is the primary path because it is the standard
RFC 8693 actor pattern a reference architecture should teach; a two-token fallback sits
behind a flag. In-cluster agents use Kubernetes service-account federation. SPIRE is the
multi-cluster extension. Enterprise-Managed Authorization is an optional flag, not the main
demo.

## Alternatives considered

- **Zitadel.** AGPL-3.0 since v3, excluded by the license rule for code the platform integrates with.
- **Authentik.** Agent accounts are enterprise-only.

## Consequences

- The realm is generated from manifests (ADR 0024, 0048).
- Tokens that user-facing clients obtain directly (LibreChat, Claude Code's public MCP client, the scenario runner) carry the user and the client in azp but no actor claim. An interactive agent's manifest lists those clients under identity.actsThrough, and the control plane resolves the agent from azp. Tokens from the delegation flow name the agent in the actor claim and need no entry. One client belongs to one agent.
- An actor claim is authoritative. A token that names an actor without a manifest has no agent, even when it was issued to a client some agent acts through; the client's agent never stands in for an unknown actor.

## To verify at build time

- Keycloak 26.8 feature flag names for the delegation preview and identity-assertion-jwt.
- The service-account token exchange parameters used by demo/agent-nightly-triage.
