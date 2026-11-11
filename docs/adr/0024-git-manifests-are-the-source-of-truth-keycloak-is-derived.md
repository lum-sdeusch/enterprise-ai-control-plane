# ADR 0024: Git manifests are the source of truth; Keycloak is derived

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q24

## Context

Owner, team, risk tier, allowed tools, model classes and output classification must live
somewhere reviewable. Keycloak client attributes are editable by hand; custom resources
would need an operator.

## Decision

Tool, agent and team manifests in Git, validated against JSON schemas, compiled by CI into
the OPA data bundle, the Keycloak realm import, team namespaces and reference grants.
Keycloak is never edited by hand.

## Alternatives considered

- **Keycloak attributes as source of truth.** No review, no history, no tests.
- **Custom resources with an operator.** Several hundred lines of controller for what a compile step does.

## Consequences

- scripts/compile-manifests.py is the only writer of policies/eacp/data.json, platform/keycloak/realm-import.yaml and platform/generated.
