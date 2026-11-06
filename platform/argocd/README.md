# Argo CD layout

`scripts/up.sh` installs Argo CD and applies `root-<profile>.yaml`. The root application points
at `profiles/<profile>/`, a kustomization that includes every Application in `apps/` and patches
the few that differ per profile (chart values file, models directory, Keycloak hostname).

Sync waves order the bring-up:

| wave | what |
|---|---|
| -3 | namespaces generated from team manifests, Gateway API CRDs |
| -2 | cert-manager, Kyverno, External Secrets Operator, OpenBao, CloudNativePG, Keycloak operator, agentgateway CRDs, kmcp |
| -1 | agentgateway, Keycloak, telemetry stores |
| 0 | eacp-gateway and eacp-system charts, Kyverno policies, models, OTel collector, Jaeger, Langfuse |
| 1 | tools (ApplicationSet from `tools/*/manifest.yaml`), demo |

Chart versions carry `# TODO(verify)` where they were not checked on 2026-11-06. Pin them before
the first real sync.
