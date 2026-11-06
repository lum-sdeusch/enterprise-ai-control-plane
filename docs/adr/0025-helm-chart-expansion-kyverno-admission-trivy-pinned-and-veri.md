# ADR 0025: Helm chart expansion, Kyverno admission, Trivy pinned and verified, mcp-scanner

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q25

## Context

One manifest must expand into roughly seven Kubernetes objects. Kyverno graduated in CNCF in
March 2026 and verifies cosign signatures natively. Trivy had a supply-chain compromise in
March 2026 and is still Apache-2.0 and the most adopted scanner; Grype and Syft are the
alternatives. Cisco's mcp-scanner is Apache-2.0 and scans tool descriptions over Streamable
HTTP.

## Decision

A Helm chart with a JSON schema generated from the manifest schema renders the objects.
Kyverno enforces verified images, restricted pod security and digests, and generates
default-deny network policies per team namespace. Trivy, pinned by digest and
cosign-verified in CI, scans images; mcp-scanner scans tool descriptions.

## Alternatives considered

- **Kustomize components.** Weaker templating and no values schema.
- **Grype and Syft.** Documented alternative; Trivy kept for adoption and feature breadth.

## Consequences

- charts/mcp-tool/values.schema.json is generated; the chart cannot accept a manifest the validator rejects.
