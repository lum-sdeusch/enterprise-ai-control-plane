# ADR 0026: Pin tools, prompts and resources; quarantine on mismatch

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q26

## Context

No candidate implements rug-pull detection. A server that changes a tool description after
review is the supply-chain attack specific to MCP.

## Decision

CI pins the SHA-256 of each listing's canonical JSON into the manifest. The adapter hashes
every live listing; on mismatch it quarantines the server, denies all calls, empties the
listing, alerts and audits until a new manifest is merged. A matching listing lifts the
quarantine.

## Alternatives considered

- **Warn only.** A warning nobody reads is not a control.

## Consequences

- The Go adapter and the Python pin script share a canonical form and a test vector so they can never disagree.
