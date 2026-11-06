# ADR 0048: One Keycloak realm; groups are teams; two gateway audiences; realm import from manifests

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q48

## Context

A realm per team would multiply clients and trust configuration for no isolation gain in a
single-enterprise deployment.

## Decision

One realm. Groups are teams and approver groups. Clients: LibreChat, the adapter's approval
page, Langfuse, the gateway's token-exchange identity, the demo runner, the red-team runner,
and one client per agent with a service account. Two bearer-only resource clients carry the
MCP and LLM audiences. Claims mapped: groups, team, clearance, data_label. The realm is
imported by the Keycloak operator from a generated resource.
