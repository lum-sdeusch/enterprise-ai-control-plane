# ADR 0041: Identity on the LLM path, JWT first and trusted header second

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q41

## Context

LibreChat calls upstream model endpoints with one configured service key, so per-user
budgets and audit on prompts need identity from somewhere.

## Decision

The gateway resolves the effective user and team from a JWT when the client sends one, as
in-cluster agents do, and otherwise from a user id header trusted only when the caller is
LibreChat's own identity. Budgets key off the resolved identity.

## To verify at build time

- Whether LibreChat's placeholder headers can forward a per-user token.
- Precedence between gateway-level and route-level JWT policies in agentgateway.
