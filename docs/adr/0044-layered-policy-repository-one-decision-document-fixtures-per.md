# ADR 0044: Layered policy repository, one decision document, fixtures per threat, signed bundles

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q44

## Context

Who may change which rule is as important as the rule.

## Decision

Global guardrails owned by security, then team, tool and agent layers owned by their teams,
enforced by CODEOWNERS. Layers can only add denials. Rego v1 with one decision document.
One fixture per demo threat. Manifests validated with their JSON schemas in CI. Bundles
carry the git revision, which appears in every audit record through data.system.bundles.

## Consequences

- Forty policy unit tests run in CI against the six fixtures and their variants.
