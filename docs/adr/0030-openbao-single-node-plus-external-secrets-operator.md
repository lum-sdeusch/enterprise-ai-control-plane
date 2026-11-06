# ADR 0030: OpenBao single node plus External Secrets Operator

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q30

## Context

Smallest footprint was the selection criterion. OpenBao (MPL-2.0, Linux Foundation) runs as
one pod with integrated Raft storage; Infisical needs Postgres, Redis, a backend and a
frontend. SOPS has no runtime component but is not a secrets manager.

## Decision

OpenBao, single node with integrated Raft, delivered into namespaces by External Secrets
Operator (Apache-2.0, CNCF). The manifest chooses per tool between a shared service
credential and per-user delegation; one demo tool uses delegation.

## Alternatives considered

- **Infisical.** Four components where one will do.
- **SOPS with age.** No dynamic secrets, no rotation, no audit.

## Consequences

- Every ExternalSecret references the openbao ClusterSecretStore; scripts/up.sh seeds the demo paths.
