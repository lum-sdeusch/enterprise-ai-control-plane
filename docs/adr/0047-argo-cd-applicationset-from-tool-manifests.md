# ADR 0047: Argo CD ApplicationSet from tool manifests

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q47

## Context

The "merge a PR and watch it appear" moment should need no manual Application.

## Decision

A git files generator reads every tools/*/manifest.yaml and renders charts/mcp-tool with
the profile's platform values and the manifest, into the team's namespace.
