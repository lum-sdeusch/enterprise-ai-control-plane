# ADR 0042: The in-cluster Pydantic AI agent is a scheduled, autonomous agent

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q42

## Context

LibreChat covers interactive use. The autonomous-identity path from ADR 0006 is otherwise
untested.

## Decision

A nightly ticket-triage CronJob with a service identity, a named owner, in-house models
only, no exfiltration-capable or destructive tools, and a confidential ceiling.
