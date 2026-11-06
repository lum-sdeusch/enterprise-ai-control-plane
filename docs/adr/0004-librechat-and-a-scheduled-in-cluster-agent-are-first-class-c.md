# ADR 0004: LibreChat and a scheduled in-cluster agent are first-class clients; Claude Code is the tested IDE path

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q4
- Related: [ADR 0033](0033-librechat-pydantic-ai-and-claude-code-as-the-clients.md), [ADR 0042](0042-the-in-cluster-pydantic-ai-agent-is-a-scheduled-autonomous-a.md)

## Context

Each client class needs its own identity flow: a chat UI needs OIDC login and token
passthrough, an in-cluster agent needs workload identity, an IDE agent needs OAuth 2.1 with
PKCE and client registration from the gateway. IDE agents are the most common MCP consumer
in enterprises and the hardest to govern.

## Decision

LibreChat (MIT) and a scheduled Pydantic AI agent are fully working in the POC. Claude Code
is the one IDE client tested and documented.

## Alternatives considered

- **IDE-first.** Would leave the autonomous-agent identity path untested, which is the hardest part of the identity design.

## Consequences

- LibreChat caveats are recorded: no elicitation rendering, MCP SDK at spec 2025-11-25, no Client ID Metadata Documents; the gateway's stateful mode bridges the spec gap.
