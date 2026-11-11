# ADR 0046: GitHub Actions with a per-PR gate and a nightly red-team

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q46

## Context

Red-teaming is slow and spends model tokens.

## Decision

Per pull request: manifest validation, policy tests, chart render, adapter and guardrail
tests, image build with keyless signing, Trivy, the tool gate, and an end-to-end run on kind
for the six scenarios. Nightly: promptfoo against the LLM route and the MCP endpoint.

## Consequences

- The end-to-end job is dispatch-only until scripts/e2e.sh is complete; it must never be green by accident.
- Amended 2026-11-11: the workflow no longer pushes images to a registry or signs them. It builds each image on the runner, scans it and discards it. Publishing and signing move out of GitHub Actions to a key-based cosign path, which tickets 04 and 14 own.
- Until that path exists, nothing produces the signatures the Kyverno image policy and the tool gate's signature check expect for images this repository builds.
