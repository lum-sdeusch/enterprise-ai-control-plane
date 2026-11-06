# ADR 0015: Onboarding is one pull request, measured by an acceptance test, with a security gate

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q15

## Context

"As easy as a deployment and a configuration" needs a measurable definition or it is a
slogan.

## Decision

Merging one pull request containing one manifest results, within ten minutes and with zero
manual steps, in a deployed and sandboxed container, a registered and health-checked server,
tools visible only to entitled groups, policies active and traces flowing. A mandatory CI
gate runs before merge.

## Alternatives considered

- **No gate.** Would make the demo an enterprise demo in name only.

## Consequences

- scripts/tool-gate.sh and the tool-gate job in CI implement the gate (ADR 0025).
