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

**Status:** ready-for-agent

- [ ] All applications in the foundation and platform-store waves show Synced and Healthy in Argo CD after one `make up`.
- [ ] No chart or manifest source in the Argo CD applications still carries a verification marker; the Gateway API release matches what agentgateway 1.7 expects; kmcp is installed from its published chart or by its CLI, and the bring-up script records which.
- [ ] Keycloak serves the `eacp` realm with the generated groups, clients and demo users; a demo user can log in.
- [ ] A seeding script writes the demo secrets into OpenBao and the ClusterSecretStore resolves them; the ExternalSecrets in the platform charts become Ready.
- [ ] The documented host ports reach the gateway, Keycloak and Argo CD from the laptop.
- [ ] `make down` then `make up` works.
