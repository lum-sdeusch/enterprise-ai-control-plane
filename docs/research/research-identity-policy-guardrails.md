# OSS Identity, Policy and Guardrail Components for an Enterprise AI Reference Architecture

Research date: 2026-11-06. All versions/licenses verified from primary sources (GitHub release pages, LICENSE files, official docs, IETF/OIDF pages, Hugging Face model cards) unless marked **UNVERIFIED**. GitHub's REST API was blocked by the session proxy, so release dates and commit dates come from the GitHub web pages (year inferred from asset timestamps where the page omits it).

---

## PART A — Identity providers / SSO for users and agents

### A.1 Comparison table

| Product | Latest release (verified) | License | Governance | RFC 8693 token exchange | `act` / delegation | RFC 7591 DCR | RFC 8628 device flow | Fine-grained authz | Agent / workload identity |
|---|---|---|---|---|---|---|---|---|---|
| **Keycloak** | 26.8.2 (2026-11-05) | Apache-2.0 | CNCF Incubating (accepted 2023-04-10) | **GA (V2) since 26.2**, internal→internal only | `act` on impersonation tokens; delegation (`actor_token`, `may_act`/`act`) **preview** in 26.8 | Supported | Supported | Authorization Services (UMA 2.0 GA), FGAP V2 GA (26.2), AuthZEN PDP **experimental** (26.7) | Federated client auth: OIDC + K8s SA tokens GA (26.6), **SPIFFE JWT-SVID preview**; ID-JAG receiver **experimental** |
| **Zitadel** | v4.20.1 (2026-11-03) | **AGPL-3.0** (since v3, 2025-03-31; API protos/SDKs Apache-2.0/MIT) | Company (Zitadel) | Yes, incl. `actor_token` | Yes — issues `act` claim (treats delegation as impersonation) | Not checked | Not checked | Roles/grants; no policy engine | Service users w/ private-key JWT; official "AI agent" pattern guide |
| **authentik** | 2026.8.5 (2026-10-21) | MIT core; `authentik/enterprise/` is **source-available** (dev/test only w/o subscription) | Company (Authentik Security) | Not checked | — | — | — | Basic RBAC | "Agent accounts" is an **Enterprise** feature |
| **Ory Hydra** | 26.2.0 (2026-03-20, per CHANGELOG) | Apache-2.0 | Company (Ory) | **No native RFC 8693** (JWT bearer RFC 7523 only; third-party add-on exists) | — | Yes (OIDC DCR) | Yes (device-auth migration in 25.4.0) | via Keto | — |
| **Ory Kratos / Keto / Oathkeeper** | not shown on repo pages | Apache-2.0 each | Company (Ory) | n/a | n/a | n/a | n/a | Keto = Zanzibar ReBAC | — |
| **Dex** | not captured | Apache-2.0 | CNCF Sandbox (2020-06-25) | Yes (OIDC connector only; upstream access tokens need `getUserInfo`) | No | — | **UNVERIFIED** | None | — |
| **Casdoor** | not captured | Apache-2.0 | Community (Casbin org) | Not checked | — | — | — | Casbin (ACL/RBAC/ABAC) | Markets itself "Agent-first IAM" with an MCP gateway + A2A |
| **Logto** | not captured | **MPL-2.0** | Company (Silverhand) | Not checked | — | — | — | RBAC / org RBAC | README lists MCP + agent support |
| **SPIFFE/SPIRE** | SPIRE v1.16.0 (2026-11-05) | Apache-2.0 | CNCF **Graduated** (2022-08-23) | n/a | n/a | n/a | n/a | n/a | X.509/JWT-SVID, federation; experimental WIT-SVID (WIMSE) in 1.16 |

### A.2 Keycloak detail

Sources: specifications page https://www.keycloak.org/securing-apps/specifications ; release notes https://www.keycloak.org/docs/latest/release_notes/index.html ; GitHub releases https://github.com/keycloak/keycloak/releases ; CNCF https://www.cncf.io/projects/keycloak/ ; repo https://github.com/keycloak/keycloak (Apache-2.0, 37.7k stars).

- **Version**: 26.8.2 released 2026-11-05 (26.8.0 on 2026-10-01; 26.7.x line still patched).
- **CNCF**: Incubating, accepted 2023-04-10; no later level change listed.
- **License**: Apache-2.0.
- **Standard Token Exchange (RFC 8693)**: "Supported" on the spec page but with the comment "Token exchange V2 only supports the internal to internal use-case". The token-exchange guide (https://www.keycloak.org/securing-apps/token-exchange) says V2 is "the fully supported token exchange implementation that is enabled by default", GA since 26.2.0 ("we added support for the Standard token exchange!"). Limits: subject token must be an access token; requested type access/ID/refresh; no `resource` parameter; public clients cannot call it; subject impersonation "Not implemented yet"; internal→external goes through Identity Brokering APIs; external→internal via V2 + JWT Authorization Grant.
- **`act` claim / delegation**: 26.8 release notes — "Token exchange delegation support status is now preview"; delegation uses `actor_token` and `may_act`/`act` claims, requires `parameterized-scopes` feature and FGAP V2, and user consent that "is never stored permanently". Tokens from impersonation sessions include the actor (`act`) claim (cannot be disabled). Positioned explicitly as "Token exchange delegation for AI agents and automation with consent and FGAP authorization" (preview).
- **Dynamic client registration**: RFC 7591 and OIDC Dynamic Client Registration are "Supported" (spec page). OAuth Client ID Metadata Document (CIMD, draft-ietf-oauth-client-id-metadata-document) is **experimental** (`cimd` feature flag), used for VS Code and Claude Code integration.
- **RFC 9728 Protected Resource Metadata**: Keycloak does **not** serve it and does not list it on the spec page. The MCP guide (https://www.keycloak.org/securing-apps/mcp-authz-server) states "The standard is for an MCP server and not for an authorization server like Keycloak" — the MCP server (resource server) must publish `/.well-known/oauth-protected-resource`. Keycloak provides RFC 8414/OIDC Discovery (Supported) and RFC 9207 `iss` (Supported). RFC 8707 `resource` indicators are **experimental** (`resource-indicators` feature; without it "Keycloak does not recognize the `resource` parameter"). Keycloak claims support for MCP 2025-03-26; MCP 2025-06-18 / 2025-11-25 / 2026-07-28 are marked Experimental. Documented clients: VS Code, Claude Code, Claude Desktop, MCP Inspector; ChatGPT's CIMD is not supported.
- **Authorization Services**: UMA 2.0 Grant and Federated Authorization "Supported"; FGAP V2 "supported" since 26.2; **AuthZEN Authorization API 1.0 PDP is experimental** (`--features=authzen`; endpoints `/realms/{realm}/authzen/access/v1/evaluation(s)` and `/.well-known/authzen-configuration`; subjects limited to `user`/`client`; "do not use in production") — https://www.keycloak.org/securing-apps/authzen-authorization .
- **Device flow**: RFC 8628 "Supported".
- **Identity Assertion Authorization Grant / Cross-App Access (ID-JAG)**: based on draft-ietf-oauth-identity-assertion-authz-grant-01; feature flag `identity-assertion-jwt`; **experimental**; only the *receiving* AS side is implemented, issuer side "not yet implemented" (https://www.keycloak.org/securing-apps/identity-assertion-jwt-authorization-grant). Spec page lists "OAuth Identity and Authorization Chaining Across Domains (draft)" as Supported and RFC 7523 JWT Authorization Grant as Supported (GA in 26.6).
- **Workload identity / SPIFFE**: "Federated client authentication" (26.4 preview → 26.6) lets OIDC clients authenticate with tokens from an external IdP: OIDC and Kubernetes service-account tokens are supported; **SPIFFE JWT-SVID stays preview** until draft-ietf-oauth-spiffe-client-auth is finalized (https://www.keycloak.org/2026/01/federated-client-authentication). SPIFFE provider must issue JWT-SVIDs and expose a SPIFFE Bundle Endpoint; `sub` carries `spiffe://trust-domain/client-id`.
- Other relevant: DPoP (RFC 9449), mTLS (RFC 8705), PAR, JAR, FAPI 1/2 certified, OAuth 2.1 draft supported, OID4VCI preview, SCIM tech preview.

### A.3 Zitadel detail

- **License**: AGPL-3.0 since v3.0 (2025-03-31) — https://zitadel.com/blog/apache-to-agpl ; FAQ https://zitadel.com/license-faq . Repo https://github.com/zitadel/zitadel shows "AGPL-3.0 license" with LICENSING.md "Apache 2.0 and MIT exceptions for specific directories" (API protos remain Apache-2.0, so generated clients carry no AGPL obligation). Earlier versions keep Apache-2.0. Commercial license available.
- **Latest**: v4.20.1 (2026-11-03) — https://github.com/zitadel/zitadel/releases .
- **Token exchange** (https://zitadel.com/docs/guides/integrate/token-exchange): RFC 8693; subject token = access/ID/JWT/user-id; `actor_token` (access or ID token) supported; requested type opaque/JWT access or ID token; issued token contains **`act` claim** with actor `iss`/`sub`, persists through refresh; Zitadel "doesn't distinguish between" delegation and impersonation; requires instance "Allow Impersonation" + impersonator roles; `resource` param unsupported; audience can only be reduced.
- **Agent/service user model** (https://help.zitadel.com/identity-patterns-for-ai-securing-agents-with-zitadel): each agent = Service User (machine user) authenticating with private-key JWT (preferred) or client credentials; PATs supported but discouraged for production agents; Auth Code+PKCE for user-facing assistants; token exchange to mint narrow per-MCP-server tokens.

### A.4 authentik detail

- Root LICENSE (https://github.com/goauthentik/authentik/blob/main/LICENSE): MIT, except `website/` (CC BY-SA 4.0) and `authentik/enterprise/` (separate license). `authentik/enterprise/LICENSE`: copying/modifying permitted "for development and testing purposes" only; production use requires a valid Enterprise subscription; redistribution forbidden → **source-available, not OSS**.
- Enterprise-only features (https://docs.goauthentik.io/enterprise/enterprise-features): **Agent accounts**, scheduled offboarding, Google Workspace & Entra ID user/group sync, external OAuth/SAML *sources*, Shared Signals Framework provider, OAuth auth for SCIM, WS-Federation provider, privileged access management, password history, client-cert (mTLS) auth, RADIUS EAP-TLS, account lockdown, enhanced audit logging, event maps, CSV export, object lifecycle mgmt, device/fleet connectors. (RAC and the mobile authenticator are not on that page; older docs listed RAC as enterprise — **UNVERIFIED** for 2026.)
- Latest: 2026.8.5 (2026-10-21).

### A.5 Ory

- Hydra (Apache-2.0, 17.6k stars): CHANGELOG top versioned entry 26.2.0 (2026-03-20); **no "token exchange"/"RFC 8693" entries at all**; device-flow code present (25.4.0). Maintainers confirmed in https://github.com/ory/hydra/discussions/3359 that the Hydra 2.0 release-note claim of RFC 8693 support was incorrect; what exists is RFC 7523 JWT-bearer. Docs URL `/docs/hydra/guides/token-exchange` returns 404 (ory.com). Third-party `hydra-rfc8693` npm package exists (not Ory-endorsed).
- Kratos (Apache-2.0, 13.9k), Keto (Apache-2.0, 5.4k, Zanzibar-style ReBAC), Oathkeeper (Apache-2.0, 3.6k; no maintenance/deprecation notice on the repo page, but no release visible either — activity level **UNVERIFIED**).

### A.6 Dex, Casdoor, Logto

- **Dex**: Apache-2.0, 11.2k stars, CNCF Sandbox since 2020-06-25. RFC 8693 token exchange implemented (https://dexidp.io/docs/guides/token-exchange/) but only via the OIDC connector; exchanging opaque upstream access tokens requires `getUserInfo: true`; exchanging client must be *public*. SAML connector flagged unmaintained/possibly vulnerable. Device flow support **UNVERIFIED** from fetched pages.
- **Casdoor**: Apache-2.0, ~14.8k stars; OAuth2/OIDC/SAML/CAS/LDAP/SCIM/WebAuthn/MFA; Casbin for ACL/RBAC/ABAC; README now describes itself as "Agent-first IAM" with an "LLM MCP & agent gateway" and A2A support.
- **Logto**: **MPL-2.0**, ~14.8k stars; OIDC/OAuth 2.1/SAML, org RBAC, M2M, "Support for Model Context Protocol and agent-based AI architectures"; paid/cloud-only feature split not identified on the repo page.

### A.7 Agent / workload identity standards — what is real vs draft

**Shipping / standardized**
- SPIFFE/SPIRE: CNCF Graduated (2022-08-23); SPIRE v1.16.0 (2026-11-05) adds experimental **WIT-SVID** (WIMSE workload identity token) support, KMIP KeyManager, federation improvements. Repo Apache-2.0.
- RFC 9728 OAuth 2.0 Protected Resource Metadata (2025-04) — required by the MCP spec: MCP servers **MUST** implement RFC 9728; clients **MUST** use it for AS discovery; AS **MUST** be OAuth 2.1; CIMD **SHOULD**; RFC 7591 DCR **MAY** and is deprecated; RFC 8707 `resource` **MUST** be sent by clients and tokens **MUST** be audience-validated (MCP spec 2026-07-28, https://modelcontextprotocol.io/specification/latest/basic/authorization).
- RFC 9700 OAuth Security BCP (2025-01), RFC 10017 OAuth for browser apps (2026-08), RFC 10027 cross-device flows BCP (2026-08).
- OpenID AuthZEN Authorization API 1.0 — **Final** (approved January 2026).

**IETF WIMSE WG (all still Internet-Drafts, none RFC)** — https://datatracker.ietf.org/wg/wimse/documents/
- draft-ietf-wimse-workload-identity-practices-08 — in IESG Last Call (ends 2026-11-13), Informational.
- draft-ietf-wimse-http-signature-08 — publication requested, Proposed Standard.
- draft-ietf-wimse-arch-08, -identifier-04, -mutual-tls-03, -workload-creds-03 (WIT + X.509 WIC), -wpt-02 — WG documents.
- **draft-ietf-wimse-aims-01 "AI Identity Management System"** (2026-11-02) — WG document; WIMSE identifier as primary agent identifier; credential exchange via OAuth token exchange / transaction tokens / workload identity federation.

**IETF OAuth WG** — https://datatracker.ietf.org/wg/oauth/documents/
- draft-ietf-oauth-identity-assertion-authz-grant-06 (2026-10-30, "Cross-App Access"/ID-JAG, Parecki/McGuinness/Campbell) — WG document, not yet in IESG.
- draft-ietf-oauth-transaction-tokens-11 — WG consensus, waiting for write-up.
- draft-ietf-oauth-spiffe-client-auth-03 (2026-11-02) — WG document (what Keycloak's SPIFFE federated client auth tracks).
- draft-ietf-oauth-client-id-metadata-document-03 — WG document (what MCP's CIMD points to).
- draft-ietf-oauth-deferred-token-response-01 (2026-10-31) — WG document (user-to-agent deferred flows).
- draft-ietf-oauth-attestation-based-client-auth-12 — waiting for write-up.
- Agent-specific **individual** drafts (not adopted): draft-oauth-ai-agents-on-behalf-of-user-02 (expired), draft-klrc-aiagent-auth (replaced), draft-araut-oauth-transaction-tokens-for-agents-03, draft-hamr-oauth-agent-delegation-03, draft-chen-oauth-agent-* (use-cases-05, intent-token, revocation), draft-mcguinness-oauth-ai-agent-instance-00, draft-mishra-oauth-agent-grants-02, draft-niyikiza-oauth-attenuating-agent-tokens-02, draft-rosenberg-oauth-aauth-02, draft-mcguinness-oauth-rfc9728bis-02, and ~10 more. None has WG status.

**OpenID Foundation**
- AI Identity Management Community Group (AIIM CG) whitepaper "Identity Management for Agentic AI" (2025-10-07, arXiv 2510.25819) — non-normative. No 2026 AIIM spec found.
- AuthZEN WG drafts: COAZ Framework 1.0, **COAZ-MCP Binding 1.0** (mapping MCP operations to AuthZEN requests), Access Request & Approval profile, Obligations profile — all working-group drafts (https://openid.net/wg/authzen/specifications/).

**Conclusion**: for production today use OAuth 2.1 + RFC 8693 (with `act`) + RFC 9728/8414 + SPIFFE for workloads; treat ID-JAG, WIMSE credentials, AIMS and all "agent OAuth" drafts as roadmap items behind feature flags.

---

## PART B — Authorization / policy engines

### B.1 Comparison table

| Engine | Latest (verified) | License | CNCF | Model | AuthZEN | Envoy ext_authz | Decision logs | Policy distribution | WASM / embedding | MCP/agent usage |
|---|---|---|---|---|---|---|---|---|---|---|
| **OPA** | v1.22.2 (2026-11-06) | Apache-2.0 | **Graduated** (2021) | Rego, general-purpose | Community plugin (kanywst/opa-authzen-plugin, Apache-2.0, 4 stars); OPA listed on AuthZEN interop | opa-envoy-plugin v1.22.0-envoy (2026-10-29), Apache-2.0, gRPC ext_authz, Istio/Gloo | Built-in: remote upload, masking, drop rules, console | Bundles over HTTP (ETag, long-poll), **OCI registries**, delta bundles, signed bundles | `opa build -t wasm` (npm SDK + community SDKs; no `http.send`) | Topaz (Aserto) builds on OPA; agentgateway uses CEL not OPA |
| **Cedar** | v4.14.1 (2026-11-02) | Apache-2.0 | **Sandbox** (2025-10-08) | Typed, schema-validated, analyzable (symcc, Lean spec) | AWS reference impl for Verified Permissions only; no OSS Cedar AuthZEN PDP found | None native | None native (library) | None native (cedar-agent/OPAL) | cedar-wasm crate; cedar-go v1.9 (no validator/partial-eval); cedar-java 4.2.3 (FFI) | **ToolHive** (cedar-go v1.9.0 in go.mod, default-deny per tool/prompt/resource); cedar-for-agents (MCP tool → Cedar schema generator, Apache-2.0) |
| **OpenFGA** | v1.22.1 (2026-10-28) | Apache-2.0 | **Incubating** (2025-10-28) | Zanzibar ReBAC + conditions | **Experimental** (`--experimentals=authzen`): evaluation(s), subject/resource/action search, discovery; no pagination/contextual tuples | None native | — | Model in store | SDKs; server only | Docs recommend AuthZEN "when integrating an AuthZEN-compatible product like an API/MCP Gateway" |
| **Cerbos PDP** | v0.57.0 (2026-10-23) | Apache-2.0 (open-core; Hub/Synapse commercial) | — | YAML resource/principal policies | **Yes, native since v0.48 (2025-12-09)**; on interop list | — | audit log (not verified here) | git/bundle | Go | Markets agent/MCP use |
| **OPAL** | 0.9.9 stable (2026-08-27); 0.10.0-rc.5 (2026-11-04) | Apache-2.0 | — | Admin/sync layer, not a PDP | n/a | n/a | n/a | Pushes policy+data to OPA and Cedar (cedar-agent) via WebSocket pub/sub | n/a | — |
| **cedar-agent** (permitio) | no releases; last commit **2025-10-16** | Apache-2.0 | — | HTTP PDP for Cedar with schema store | No | No | No | via OPAL | Rust | Low activity |
| **Topaz** (Aserto) | not captured | Apache-2.0 | — | OPA + Zanzibar directory | On interop list | — | — | — | — | — |

### B.2 OPA detail
- Repo https://github.com/open-policy-agent/opa (Apache-2.0, 12.4k stars, "graduated project in the CNCF"); releases https://github.com/open-policy-agent/opa/releases (v1.22.2 2026-11-06, v1.22.0 2026-10-29).
- Governance: Aug 2025 blog — OPA creators and Styra staff joined **Apple**; "no changes to the project governance or licensing"; EOPA, OPA Control Plane, SDKs and Regal proposed for the CNCF OPA org (https://www.openpolicyagent.org/blog).
- opa-envoy-plugin: https://github.com/open-policy-agent/opa-envoy-plugin — Apache-2.0, implements Envoy External Authorization gRPC API (`envoy_ext_authz_grpc`, default `:9191`, dry-run, Prometheus histogram), Istio/Gloo examples; releases track OPA (v1.22.0-envoy 2026-10-29).
- Decision logs (https://www.openpolicyagent.org/docs/management-decision-logs): events with input/result/bundle revision/decision_id; gzip batch upload with backoff; `system.log.mask` masking (erase/remove/upsert) and `drop` rules; console sink; rate limiting.
- Bundles (https://www.openpolicyagent.org/docs/management-bundles): HTTP polling with ETag + long polling, OCI registry pull, delta (JSON-patch data) bundles, JWT-signed bundles, multi-bundle roots.
- Performance (https://www.openpolicyagent.org/docs/policy-performance): "linear fragment" evaluates in near constant time via rule indexing; early exit; partial evaluation (`opa build -O 1/2`); RBAC example ≈45 µs/op in `opa bench`; ~20x memory inflation for JSON data (8 MB → ~160 MB); single-threaded evaluation (server parallelizes across requests).
- WASM (https://www.openpolicyagent.org/docs/wasm): `opa build -t wasm -e <entrypoint>`; npm-opa-wasm official, ~8 community SDKs; built-ins like `http.send` unsupported in Wasm; ABI 1.0–1.3.
- AuthZEN: not in upstream OPA. OPA ecosystem entry "opa-authzen" → https://github.com/kanywst/opa-authzen-plugin (Apache-2.0, native OPA plugin serving `/access/v1/evaluation(s)`, search endpoints opt-in, `/.well-known/authzen-configuration`). Aserto's interop reference also fronts "plain-old OPA". Also Hexa `policy-opa` (last published Dec 2024).

### B.3 Cedar detail
- https://github.com/cedar-policy/cedar — Apache-2.0, 1.8k stars; validator against declared schema; `cedar-policy-symcc` (SMT-based verification); formal Lean spec (cedar-spec); releases v4.14.1 (2026-11-02).
- CNCF: Sandbox, accepted 2025-10-08 (https://www.cncf.io/projects/cedar/).
- Bindings: Rust core; `cedar-wasm` crate (JS/TS); **cedar-go** (Apache-2.0, 248 stars; includes authorizer, schema parsing, but *no* schema validation (only `x/exp/schema`), no partial evaluation, no formatter); **cedar-java** (Apache-2.0, 4.2.3, Rust FFI). No official Python binding (cedar-for-agents ships a Python wrapper for the MCP schema generator only).
- **cedar-for-agents** (https://github.com/cedar-policy/cedar-for-agents, Apache-2.0, 58 stars): `mcp-tools-sdk`, `cedar-policy-mcp-schema-generator` (+wasm, +python), `cedar-analysis-mcp-server`.
- **cedar-agent** (https://github.com/permitio/cedar-agent): Apache-2.0, HTTP PDP with policy/data/schema stores; **last commit 2025-10-16**, no releases, 204 stars → maintenance risk.
- AuthZEN: AWS blog shows an AuthZEN PEP → Amazon Verified Permissions reference (not self-hostable OSS). No OSS Cedar AuthZEN PDP found.
- **ToolHive** (https://github.com/stacklok/toolhive, Apache-2.0, 2.3k stars): Cedar authorizer in-process via `github.com/cedar-policy/cedar-go v1.9.0`; entities `Client::"<sub>"`, actions `call_tool`/`get_prompt`/`read_resource`/`get_skill`, resources `Tool`/`Prompt`/`Resource`/`Skill`, JWT claims become `claim_*` attributes; list responses filtered per item; default deny; also an `httpv1` external PDP option (https://docs.stacklok.com/toolhive/reference/authz-policy-reference).

### B.4 OpenFGA detail
- https://github.com/openfga/openfga — Apache-2.0; CNCF accepted 2022-09-14, **Incubating 2025-10-28**; v1.22.1 (2026-10-28); Postgres/MySQL/SQLite(beta); HTTP+gRPC; Check/BatchCheck/ListObjects/ListUsers.
- AuthZEN (https://openfga.dev/docs/interacting/authzen): experimental flag; maps evaluation→Check, evaluations→BatchCheck, subject search→ListUsers, resource search→ListObjects, action search; "It's not possible to implement OpenFGA only with AuthZEN endpoints"; no pagination, no contextual tuples; `Openfga-Authorization-Model-Id` header extension.
- Suitability: strong for *relationship* questions (which users/agents may use which tool/resource/collection, sharing graphs); weak for attribute/content conditions on tool arguments.

### B.5 OPAL
- https://github.com/permitio/opal — Apache-2.0, 5.5k stars; server publishes policy/data change topics over WebSocket pub/sub; clients fetch from sources and push into **OPA** or **Cedar (cedar-agent)**; 0.9.9 stable (2026-08-27), 0.10.0-rc.x in progress (Nov 2026).

### B.6 AuthZEN status and implementations
- Authorization API 1.0: **Final** (OIDF vote 81–1–25; spec dated 2026-01-11) — https://openid.net/authorization-api-1-0-final-specification-approved/ , https://openid.net/specs/authorization-api-1_0.html . Working-group drafts: COAZ framework, COAZ-MCP binding, Access Request & Approval, Obligations.
- Interop PDPs listed (https://authzen-interop.net/docs/intro): Aserto/Topaz, Axiomatics, Amazon Verified Permissions, Cerbos, EmpowerID, Hexa, IndyKite, Kogito/Apache KIE, **Open Policy Agent**, **OpenFGA**, Permit, Ping Authorize, PlainID, Rock Solid Knowledge, SGNL, Thales/AuthZForce, WSO2, 3Edges. PEPs: Envoy, Kong, Tyk, AWS API GW, Layer7, WSO2, Zuplo; IdPs incl. **Keycloak**, Curity, Duende, Gluu/Janssen, Auth0.
- OSS PDPs with AuthZEN: Cerbos (native, GA v0.48), OpenFGA (experimental), Keycloak (experimental), OPA (community plugin), Topaz, Hexa, Apache KIE.

### B.7 OSS "AI/agent policy engine" / "agent firewall" projects

| Project | License | Status (verified) | Notes |
|---|---|---|---|
| **Meta LlamaFirewall** (PurpleLlama/LlamaFirewall) | **MIT** | PurpleLlama repo active (commits Oct 2026); LlamaFirewall-specific commit Aug 2026 | Scanners: PromptGuard 2 (needs gated Llama weights), AlignmentCheck (needs Together API key), CodeShield (semgrep), regex |
| **NVIDIA NeMo Guardrails** | Apache-2.0 | v0.24.1 (2026-09-16); 7.4k stars | Colang 1/2; input/dialog/retrieval/execution/output rails; tool-call validation rails (0.23); streaming fails closed (0.24); `/v1/chat/completions` server |
| **Invariant Guardrails/Gateway** | Apache-2.0 | last commit **2026-01-12**; 475 stars; Invariant acquired by Snyk (2025-06-24) | Rule language over agent traces; MCP gateway (stdio/SSE/streamable HTTP) |
| **Snyk Agent Scan** (ex mcp-scan) | Apache-2.0 | 0.6.11 (2026-11-04), active | Scanner for MCP servers/skills (tool poisoning, shadowing); **requires SNYK_TOKEN**; not a runtime firewall |
| **Cisco mcp-scanner** | Apache-2.0 | active (commits 2026-11-06) | YARA + LLM-judge + optional Cisco AI Defense API; scanner, not PEP |
| **agentgateway** | Apache-2.0, Linux Foundation project | ~6.0k stars | MCP/A2A/LLM proxy; JWT/API-key/OAuth; **CEL** RBAC policies; regex/OpenAI moderation/Bedrock/Model Armor/webhook guardrails; supports ID-JAG demo w/ Keycloak |
| **ToolHive** | Apache-2.0 | active | Cedar per-tool authz (see B.3) |
| **Lasso mcp-gateway** | MIT | 397 stars, 40 commits, dates not visible — **activity UNVERIFIED** | Presidio + regex secret plugins; Lasso cloud API optional |
| **protectai/llm-guard** | MIT | **ARCHIVED 2026-07-09**; HF models "no longer maintained" | Do not adopt |
| **OpenShield** | Apache-2.0 | **ARCHIVED 2026-02-03**; "not ready for production" | Do not adopt |
| **guardrails-ai** | Apache-2.0 | commits Aug 2026; hub hosted inference discontinued 2026-08-25, validators moving to PyPI | Validator framework; per-validator licenses not stated |
| **mozilla-ai/any-guardrail** | Apache-2.0 | active (June 2026 sweep added 15 models) | Unified wrapper over guard models |
| Pangea, Cisco AI Defense core, Lasso cloud | proprietary | — | out of scope |

---

## PART C — Guardrail / content-safety components (OSS)

### C.1 PII detection

| Component | License | Latest / activity | Entities & languages | Notes |
|---|---|---|---|---|
| **Presidio** (now `data-privacy-stack/presidio`, docs presidio.dataprivacystack.org; microsoft/presidio README says "Presidio has moved to a new home!") | MIT | 2.2.366 (2026-10-27); 11.7k stars; frequent releases | 14 global entities (CREDIT_CARD, EMAIL, IBAN, IP, PERSON, LOCATION, PHONE, URL, UUID, CRYPTO…) + **77 country-specific** across 18 countries (US 13, DE 9, ZA 10, UK 6, IN 6, IT 5, KR 5…) + 9 medical entities (transformers NER). Detection = regex + checksum + context + NER | NLP engines: spaCy, Stanza, transformers, and a `gliner` extra (gliner ≥0.2.29 in 2.2.366). Analyzer/Anonymizer/Image-Redactor/Structured. Performance: 2.2.365 replaced quadratic duplicate/conflict code; no official throughput figures found (**UNVERIFIED**). |
| **GLiNER-PII** `urchade/gliner_multi_pii-v1` | Apache-2.0 | GLiNER lib 4.2k stars, Apache-2.0 | ~50 PII labels, 6 languages | zero-shot label set |
| **GLiNER2-PII** `fastino/gliner2-privacy-filter-PII-multi` | Apache-2.0 | May 2026 (arXiv 2605.09973) | 205M params; 42 types / 7 groups; EN/FR/ES/DE/IT/PT/NL | SPY F1 0.477 vs nvidia/gliner-PII 0.400, urchade 0.398, openai/privacy-filter 0.380 (vendor numbers); precision ≈0.35 |
| **OpenAI Privacy Filter** `openai/privacy-filter` | Apache-2.0 | April 2026 | 8 categories incl. **secrets**; 1.5B total / 50M active MoE token classifier; 128k context; ONNX/Transformers.js | English-first |
| nvidia/gliner-PII | NVIDIA Open Model License (non-OSI) | Oct 2025 | — | license caveat |
| knowledgator/gliner-stream-pii-v1.0 | not checked | — | streaming, Qwen3-0.6B base | — |

### C.2 Prompt-injection / jailbreak / content-safety guard models

| Model | Size | License | Task | Serving | Gated |
|---|---|---|---|---|---|
| **Llama Guard 4** (meta-llama/Llama-Guard-4-12B) | 12B, multimodal | **Llama 4 Community License (not OSI)** | 14 MLCommons hazard categories (S1–S14) | vLLM (`vllm serve`), SGLang, Transformers | Yes |
| **Llama Prompt Guard 2** 86M / 22M | 86M (mDeBERTa) / 22M (DeBERTa-xsmall) | **Llama 4 Community License** | prompt injection + jailbreak, 512-token window; 97.5% recall @1% FPR (Meta) ; 92 ms / 19 ms on A100 | Transformers/ONNX | Yes |
| **Granite Guardian 3.3-8b** | 8B | Apache-2.0 | harm, social bias, **jailbreak**, RAG groundedness, **function-call hallucination**; thinking mode | vLLM/SGLang/llama.cpp/Ollama quant | No |
| **Granite Guardian 4.1-8b** | 8B | Apache-2.0 (April 2026) | same + bring-your-own-criteria; English only | vLLM/SGLang | No |
| Granite Guardian HAP-38M | 38M | Apache-2.0 | hate/abuse/profanity, low latency | CPU | No |
| **Qwen3Guard-Gen** 0.6B/4B/8B (+ -Stream) | 0.6–8B | Apache-2.0 | Safe/Controversial/Unsafe, 9 categories incl. **Jailbreak** and PII; 119 languages | vLLM ≥0.9, SGLang, Ollama quant | No |
| **gpt-oss-safeguard** 20b/120b | 21B (3.6B active) / 117B | Apache-2.0 | policy-following safety reasoner (bring your own policy), harmony format | vLLM/Ollama/LM Studio | No |
| **ShieldGemma 2** (google/shieldgemma-2-4b-it) | 4B | Gemma Terms (not OSI) | **image-only** safety (sexual/dangerous/violence) — no text mode | — | Yes |
| Nemotron-Content-Safety-Reasoning-4B | 4B | NVIDIA Open Model License + Gemma terms | content safety | NIM | — |
| GLiGuard (Fastino) | 300M | Apache-2.0 | safety/toxicity/jailbreak/refusal (May 2026) | encoder | No |
| DynaGuard | 1.7–8B | Apache-2.0 | dialogue-policy compliance | — | No |
| WildGuard (AI2) | 7B | Apache-2.0 | harm/refusal | — | No |
| SGuard-v1 (Samsung SDS) | 2B×2 | Apache-2.0 | content + jailbreak filters | — | No |
| Kanana Safeguard (Kakao) | 2.1–8B | Apache-2.0 | harm/legal/prompt-attack | — | No |
| protectai/deberta-v3-base-prompt-injection-v2 | 184M | Apache-2.0 | prompt injection (2024; org archived llm-guard, models "no longer maintained") | — | No |
| PIGuard (ex-InjecGuard) | DeBERTa-v3 | MIT | prompt injection | — | No |
| Horizon-Labs/prompt-injection-guard-base | 308M | Apache-2.0 (2026-09) | multilingual injection | — | No |
| Patronus wolf-defender-prompt-injection-small | small | Apache-2.0 | injection (BENIGN/INJECTION), ONNX | — | No |
| Sentinel v2 (Qualifire) | 0.6B | Elastic License (non-OSI) | injection/jailbreak | — | — |

Source for the 2026 sweep: https://github.com/mozilla-ai/any-guardrail/issues/179 (June–July 2026), plus HF model cards cited inline.

### C.3 Orchestrators (see also B.7)
- NeMo Guardrails — Apache-2.0, v0.24.1 (2026-09-16), tool-call validation rails, OpenAI-compatible server; depends on NemoGuard NIM models (NVIDIA licensed) for its best built-ins but works with any LLM.
- guardrails-ai — Apache-2.0; Hub validators moving to PyPI, hosted inference ends 2026-08-25.
- protectai/llm-guard — MIT but **archived 2026-07-09**.
- LlamaFirewall — MIT (folder LICENSE verified), scanners depend on gated Llama models + Together API.
- Invariant Guardrails — Apache-2.0, stale since Jan 2026; mcp-scan → Snyk Agent Scan (Apache-2.0, needs Snyk token).
- mozilla-ai/any-guardrail — Apache-2.0, uniform interface over ~30 guard models.

### C.4 Secret detection in text
| Tool | License | Text/stdin mode | Status |
|---|---|---|---|
| **gitleaks** | MIT | `gitleaks stdin`, `gitleaks dir` | 30.7k stars; README says **"feature complete"**, future releases security-only, maintainer moving to "Betterleaks" |
| **detect-secrets** (Yelp) | Apache-2.0 | Python API (`SecretsCollection.scan_file`), CLI `--string` | last code commit Jan 2025 (Apr 2026 workflow only) — low activity |
| **trufflehog** | **AGPL-3.0** (since v3) | `trufflehog stdin`, `filesystem`; 700–800 detectors w/ verification | active; AGPL caveat for embedding |
| openai/privacy-filter | Apache-2.0 | "secrets" category in a token classifier | model-based complement |

### C.5 OSS red-teaming / eval tools for CI

| Tool | License | Latest/activity | OpenAI-compatible gateway target | MCP tools |
|---|---|---|---|---|
| **promptfoo** | MIT | 26.9k stars | `openai:chat:<model>` + `config.apiBaseUrl`/`OPENAI_BASE_URL`; HTTP provider | **Yes**: `mcp` provider calls tools directly (stdio/url, bearer/basic/api-key/OAuth); red-team `mcp` plugin (8 attack classes incl. tool-metadata injection, excessive calling, privilege escalation) |
| **NVIDIA garak** | Apache-2.0 | 9.9k stars | `OpenAICompatible` generator (`uri` default `http://localhost:8000/v1/`), `rest.RestGenerator`, nim, huggingface | No MCP generator found |
| **PyRIT** (now microsoft/PyRIT; Azure/PyRIT archived 2026-03-27) | MIT | 4.8k stars | `OpenAITarget`s work with "OpenAI, or other compatible endpoints (Ollama, Groq…)"; HTTP/WebSocket/Playwright targets | No MCP mention |
| **DeepTeam** (+DeepEval) | Apache-2.0 | 3.1k stars | any model via `model_callback` / DeepEval custom LLM | agentic vulns (excessive agency, tool orchestration abuse, tool metadata poisoning); no MCP-native target |
| **UK AISI Inspect** | MIT | 3.2k stars | `openai-api/<provider>/<model>` with `<PROVIDER>_BASE_URL`; vLLM/Ollama/SGLang providers | **Yes**: `mcp_server_stdio/http/sse/sandbox`, `mcp_tools()` |

---

## Recommendations (rationale)

1. **IdP: Keycloak 26.8.x.** Only candidate that is Apache-2.0, CNCF-governed, has GA RFC 8693 (internal), GA RFC 7523/JWT grant, GA device flow and DCR, UMA/FGAP, GA Kubernetes workload federation, and already tracks every MCP-relevant draft (CIMD, resource indicators, ID-JAG, SPIFFE client auth, AuthZEN) behind flags. Zitadel is functionally strong (real `act` claims today) but AGPL-3.0 is a procurement blocker for many enterprises; Hydra lacks RFC 8693; authentik puts "Agent accounts" behind a paid, source-available license. Pair Keycloak with **SPIRE** for non-human workload identity (JWT-SVID → Keycloak federated client auth, preview).
2. **PDP for the MCP/LLM path: Cedar embedded in the gateway (ToolHive pattern) for per-tool-call decisions, with OPA as the general-purpose/central PDP (Envoy ext_authz, decision logs, bundles).** Cedar wins at the tool boundary: schema validation of tool argument types, default-deny, in-process microsecond latency via cedar-go, cedar-for-agents auto-generates schemas from MCP tool lists, formal analysis. OPA wins for infrastructure/API authz, audit (masked decision logs), OCI bundle distribution, Envoy integration and CNCF maturity. OpenFGA only if you need relationship graphs (who-shares-which-tool-with-whom); it has no native argument-level ABAC and AuthZEN is experimental. If you want a single AuthZEN-speaking PDP today, **Cerbos** (Apache-2.0, AuthZEN GA) is the pragmatic OSS choice; OPA's AuthZEN plugin is community-only.
3. **Minimal permissive guardrail pipeline** (all Apache-2.0/MIT, no Llama/Gemma terms): (a) **PII**: Presidio (MIT) with the GLiNER2-PII (Apache-2.0) or openai/privacy-filter (Apache-2.0) recognizer for free-text names/addresses; (b) **secrets**: gitleaks `stdin` rules (MIT) or privacy-filter's secrets class; (c) **injection/jailbreak**: Qwen3Guard-Gen-0.6B/4B (Apache-2.0, 119 languages, served by vLLM) as the fast classifier — Prompt Guard 2 is better-benchmarked but Llama-licensed; (d) **optional LLM judge**: Granite Guardian 4.1-8b (Apache-2.0; also covers RAG groundedness and function-call hallucination) or gpt-oss-safeguard-20b for bring-your-own-policy reasoning; (e) **orchestration**: NeMo Guardrails (Apache-2.0) or a thin custom pipeline in the gateway; (f) **CI red-team**: promptfoo (MIT) against the OpenAI-compatible gateway and MCP servers, Inspect (MIT) for MCP-tool evals.

## Facts I could not verify
- Keycloak: no official throughput numbers for AuthZEN/token-exchange; ID-JAG issuer side target release.
- Dex device-flow support; Zitadel/Casdoor/Logto DCR & device-flow specifics; Logto paid-feature split.
- Ory Oathkeeper maintenance level (no deprecation notice, but no release info captured).
- Presidio throughput/latency figures (none published).
- Lasso mcp-gateway commit dates; Topaz latest version; guardrails-ai per-validator licenses.
- Exact release dates of Llama Guard 4 / Prompt Guard 2 (cards only show the April 2025 license date).
- Whether `--custom_base_url` exists in garak's CLI (the `OpenAICompatible.uri` config key is documented; the CLI flag is not).
- OpenFGA AuthZEN interop results for 1.0 final.
