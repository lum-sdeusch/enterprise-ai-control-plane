# ADR 0035: Argo CD, a monorepo, one namespace per team

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q35

## Context

Argo CD and Flux are both Apache-2.0 and CNCF graduated. Argo CD's UI makes the "merge a PR
and watch it deploy" moment visible on stage.

## Decision

Argo CD with an ApplicationSet that creates one application per tool manifest. One
monorepo with platform, policies, tools, agents, teams, adapter, guardrails, demo and docs.
One Kubernetes namespace per team for MCP servers plus platform namespaces.

## Alternatives considered

- **Flux.** Equivalent capability, no UI for the demo.

## Consequences

- CODEOWNERS maps the layering onto review rights.
