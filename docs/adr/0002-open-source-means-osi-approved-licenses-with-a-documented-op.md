# ADR 0002: Open source means OSI-approved licenses with a documented open-core boundary

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 1, question Q2

## Context

"All open-source components" was doing a lot of work. Several popular candidates are
open-core with the needed features gated (LiteLLM's JWT auth and audit logs), source
available rather than open (Arize Phoenix under Elastic License 2.0, HashiCorp Vault under
BUSL), custom-licensed (Open WebUI's branding clause, Lobe Chat's community license), or
copyleft (Zitadel on AGPL-3.0, Redis 8 on AGPL/RSAL/SSPL).

## Decision

OSI-approved licenses only, plus open-core projects when every feature actually used sits
in the OSI-licensed tier and the boundary is written down per component. AGPL is allowed
for standalone services that are never linked into code. Source-available licenses are
excluded. The control plane is open source; the models are mixed: Anthropic through the
gateway plus at least one self-hosted open-weight model.

## Alternatives considered

- **Allow Elastic License 2.0 and BUSL.** They are not open source by the OSI definition and the claim in the title would be false.
- **Open-weight models only.** Would make the data-residency routing demo moot and the assistant worse than what employees already use.

## Consequences

- LibreChat's MongoDB dependency (SSPL) is the one known exception and is tracked as an open item with FerretDB as the spike.
- Component versions and gating were verified on 2026-11-06 and recorded in docs/research.
