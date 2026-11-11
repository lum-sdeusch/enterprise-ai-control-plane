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
- Policy data files are named data.json (eacp/data.json, eacp/config/data.json). An OPA bundle loads no other file name, while `opa test` on the directory loads any; under the old names the bundle shipped empty and every decision would have failed in the cluster. scripts/build-bundle.sh now asks the built bundle for its data and runs the unit tests against the unpacked bundle.
- The laptop profile gets its bundle without a registry: scripts/up.sh builds it from the checkout into ConfigMap eacp-policy-bundle, and the OPA sidecar loads the tarball from the mount through the bundle plugin, so the revision still reaches data.system.bundles. OPA does not reload a changed file, so a policy change rolls the adapter pods (make policy-load). The OCI registry remains the production source.
- Test fixtures name the manifest's pin symbolically, so re-pinning a tool does not touch the tests.
