# ADR 0013: A small published adapter layer; two boxes in the diagram, one product may fill both

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q13

## Context

Best-of-breed per box means integration code. A reference architecture should be
product-agnostic at the box level while being runnable with concrete products.

## Decision

A small adapter layer, published and tested in the repository. The AI gateway and the MCP
gateway stay two boxes in the architecture; one product may implement both.

## Alternatives considered

- **Zero custom code.** No candidate implemented rug-pull detection, session taint or the approval flow.

## Consequences

- The budget was revised in round 2 to 2,500 to 3,000 lines across the Go adapter, the Python guardrail service and the approval page (ADR 0019).
