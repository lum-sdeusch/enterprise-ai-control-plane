# ADR 0007: Deterministic policy decision point separated from probabilistic guardrails, one PDP for both pathways

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q7

## Context

OPA evaluates claims, JSON, regexes and text in microseconds, reproducibly, with unit tests.
PII detection and LLM judges take 50 ms to seconds and are not reproducible. Invoking
classifiers from inside a policy produces a slow, untestable policy with hidden network
calls.

## Decision

A guardrail stage runs first and emits structured findings. The PDP then decides from
claims, manifests, findings and taint, producing one decision document with allow, deny,
redact, require-approval and route outcomes. One PDP and one policy repository serve both
enforcement points, the MCP path and the LLM path.

## Alternatives considered

- **Classifiers inside OPA via http.send.** Non-deterministic policies cannot be unit tested or reproduced from the audit log.
- **Two PDPs.** Policy drift between the prompt path and the tool path is exactly the failure mode enterprises have today.

## Consequences

- policies/README.md defines the input and output contracts; thresholds are policy data, so changing one is a reviewed change.
- The guardrail service never decides; its outputs are data.
