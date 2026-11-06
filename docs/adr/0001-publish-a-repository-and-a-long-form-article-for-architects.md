# ADR 0001: Publish a repository and a long-form article for architects

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q1

## Context

The deliverable shapes every later choice: a conference talk tolerates hand-waving, a sales
asset tolerates a single cloud, a repository that engineers clone tolerates neither. The
primary reader was chosen as the platform or security architect who must defend the design
internally; engineers who run it are the secondary reader.

## Decision

Publish a GitHub repository under Apache-2.0 plus a long-form article. The README is the
article's skeleton so the two cannot drift. Control mappings and the threat model live in
the repository, not only in the article.

## Alternatives considered

- **Talk or whitepaper only.** Cannot be verified by the reader; a reference architecture that does not boot is a diagram.
- **Sales asset.** Would bias component choices toward a vendor's stack and away from the open-source rule.

## Consequences

- Every claim in the article must be reproducible from the repository.
- Rejected alternatives are documented with their license or feature-gate reason, since that is what architects are asked.
