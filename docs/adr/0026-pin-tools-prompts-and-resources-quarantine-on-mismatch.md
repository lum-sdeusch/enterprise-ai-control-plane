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
- The pin is the hash of the items exactly as the server sends them, taken by raw JSON-RPC rather than through an MCP SDK, which reshapes items into its own models. agentgateway 1.7 passes listing items through unchanged, so that is what the adapter sees; on the aggregate endpoint it first removes the "<server>_" prefix from names, so one pin holds on every route (ticket 04).
- A server framework writes each tool's schema and metadata, so another framework version can change the hash. The demo servers pin their framework version exactly, and test/pathway checks the manifest pins against the running server on every run.
- A server that is listed behind the aggregate but has no manifest is hidden and uncallable, not quarantined.
