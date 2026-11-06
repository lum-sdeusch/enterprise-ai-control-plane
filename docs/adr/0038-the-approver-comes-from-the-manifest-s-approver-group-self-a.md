# ADR 0038: The approver comes from the manifest's approver group; self-approval is a per-tool flag, off by default

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q38
- Related: [ADR 0031](0031-one-human-approver-from-the-tool-s-approver-group-deny-with.md)

## Context

One human, but which one: the requester consenting, or a designated approver. User consent
is what MCP clients already do; the gateway-side approval adds value when the approver is
designated.

## Decision

The approver group defaults to the tool's owning team's approver group. The requester may
approve only if the tool manifest sets allowSelf. The audit record names the approver.
