# Argo CD layout

`scripts/up.sh` installs Argo CD and applies `root-<profile>.yaml`. The root application points
at `profiles/<profile>/`, a kustomization that includes every Application in `apps/` and patches
the few that differ per profile (chart values file, models directory, Keycloak hostname).

Sync waves order the bring-up:

| wave | what |
|---|---|
| -3 | Gateway API CRDs, kmcp CRDs |
| -2 | cert-manager, Kyverno, External Secrets Operator, OpenBao, CloudNativePG, Keycloak operator, agentgateway CRDs, kmcp |
| -1 | agentgateway and the Gateway (`platform/gateway`), the openbao ClusterSecretStore, Keycloak with the realm import, telemetry stores (ClickHouse, PostgreSQL, Valkey, SeaweedFS) |
| 0 | namespaces and ReferenceGrants generated from team manifests, eacp-gateway and eacp-system charts, Kyverno policies, models, OTel collector, Jaeger, Langfuse |
| 1 | tools (ApplicationSet from `tools/*/manifest.yaml`), demo |

Every chart and git source is pinned to a release that exists (checked against the registries
on 2026-11-07). OCI chart registries are registered in `install-values.yaml`, which `scripts/up.sh`
passes to the Argo CD chart along with the health checks app-of-apps waves need.

Applications whose source is this repository carry the label `eacp.dev/source` (`repo` for a
single source, `repo-values` when the repository is the second, values-only source). `scripts/up.sh`
uses it to retarget them at another branch or fork (`EACP_REVISION`, `EACP_REPO`).
