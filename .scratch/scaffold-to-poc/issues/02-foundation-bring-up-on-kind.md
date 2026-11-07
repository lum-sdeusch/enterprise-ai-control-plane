# 02: Foundation bring-up on kind

**What to build:** `make up PROFILE=laptop` on a 16 GB laptop creates the kind cluster, installs
Argo CD, applies the laptop root application, and every foundation and platform-store
application reaches Synced and Healthy without manual steps: Gateway API CRDs, cert-manager,
Kyverno, External Secrets Operator, OpenBao, CloudNativePG, the Keycloak operator with the
generated realm imported, agentgateway CRDs and the gateway itself, kmcp, the telemetry stores.
Every chart version that was marked for verification is pinned to a version that actually
exists. OpenBao is initialised and seeded with the demo secrets by a script. Running the
bring-up twice is harmless.

**Blocked by:** None (can start immediately)

**Status:** done (PR 2)

- [x] All applications in the foundation and platform-store waves show Synced and Healthy in Argo CD after one `make up`.
- [x] No chart or manifest source in the Argo CD applications still carries a verification marker; the Gateway API release matches what agentgateway 1.7 expects; kmcp is installed from its published chart or by its CLI, and the bring-up script records which.
- [x] Keycloak serves the `eacp` realm with the generated groups, clients and demo users; a demo user can log in.
- [x] A seeding script writes the demo secrets into OpenBao and the ClusterSecretStore resolves them; the ExternalSecrets in the platform charts become Ready.
- [x] The documented host ports reach the gateway, Keycloak and Argo CD from the laptop.
- [x] `make down` then `make up` works.

## Notes from the bring-up (2026-11-07)

Verified on kind 0.33.0 (Kubernetes 1.37.0) under Docker Desktop on an Apple-silicon Mac with
7.7 GiB for the node: `make down && make up` reached all 16 foundation and platform-store
Applications Synced and Healthy in 421 s, a rerun changed nothing, and the host ports answered.

Found on the way and left for the tickets that own them:

- The realm import declares `clientScopes`, which replaces Keycloak's built-in scopes: the realm
  has no `profile`, `email`, `roles`, `web-origins` or `basic`. LibreChat and Langfuse request
  `profile` and `email` and get `invalid_scope` (tickets 10, 12); tokens carry `sub` but no
  `preferred_username`. Fix in `scripts/compile-manifests.py`.
- `temporary: true` on imported credentials is ignored by Keycloak, so the demo users are not forced
  to change their password (the scenario runner's password grant relies on that today).
- The eacp-gateway chart does not validate against agentgateway 1.7: `AgentgatewayModel.spec.model`,
  `jwtAuthentication.providers[].mcp` and the prompt-guard `targetRefs` (tickets 03, 07). The
  Gateway itself moved to `platform/gateway` so it no longer waits on that chart.
- Kyverno `eacp-verify-images` blocks the unsigned adapter image in eacp-system (tickets 04, 14).
- The Langfuse chart (2.2.3) fails to render with `platform/telemetry/langfuse-values.yaml`, and the
  ClickHouse init `.sql` files use `${VAR}`, which the image's entrypoint does not substitute
  (ticket 12).
