# ADR 0033: LibreChat, Pydantic AI and Claude Code as the clients

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q33
- Related: [ADR 0004](0004-librechat-and-a-scheduled-in-cluster-agent-are-first-class-c.md)

## Context

LibreChat was the only chat UI meeting OSI license, OIDC login, remote MCP over OAuth,
OpenTelemetry and a Helm chart. Pydantic AI (MIT) has an MCP client with OAuth and native
GenAI-convention instrumentation. Open WebUI and Lobe Chat have non-OSI licenses; Hugging
Face chat-ui lacks OpenTelemetry.

## Decision

LibreChat for employees, Pydantic AI for the scheduled agent, Claude Code as the tested IDE
client.

## Consequences

- LibreChat's MongoDB dependency is the one open license exception (FerretDB spike).
