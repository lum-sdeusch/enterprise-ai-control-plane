# ADR 0040: Anthropic only, model-level failover, in-house as last resort

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q40

## Context

A second commercial provider means a second key for every reader who clones the repository.

## Decision

Anthropic is the only external provider. Failover goes from a primary Anthropic model to a
secondary one, then to the in-house model for non-restricted traffic.
