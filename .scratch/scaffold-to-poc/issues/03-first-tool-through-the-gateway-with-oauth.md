# 03: First tool through the gateway with OAuth, no adapter yet

**What to build:** The tickets server is onboarded by merging its manifest; the ApplicationSet
renders it into its team namespace; a demo user signs in through Keycloak and lists and calls
its tools from Claude Code and from the scenario runner, both through the per-server route and
through the single aggregate MCP endpoint. The policy hook is switched off for this slice (a
chart value that omits the guardrail processor) so the gateway and identity path can be proven
on their own. The verification markers on the gateway configuration are resolved and the
answers written into the affected ADRs.

**Blocked by:** 02 Foundation bring-up on kind

**Status:** ready-for-agent

- [x] The chart has a value that omits the guardrail processor policy, used only for this slice and off by default.
- [x] Protected resource metadata is served for the per-server path and for the aggregate path, and Claude Code completes the OAuth flow against Keycloak, with dynamic client registration proxied or a pre-registered client, and the chosen approach documented.
- [x] A token with the wrong audience is rejected; a token with the `eacp-mcp` audience is accepted.
- [x] The precedence between the gateway-level JWT policy and the per-route policies is understood and the committed arrangement works for both MCP and LLM audiences.
- [x] The aggregate endpoint lists the tickets tools with the server prefix; the virtual backend's selector finds servers in team namespaces, or the alternative is committed.
- [x] The backend target's protocol value and path handling for Streamable HTTP servers are confirmed and the chart updated.
- [x] ADRs 0017 and 0018 have their verification items answered.

## Notes (from the build)

- **MCP OAuth on agentgateway 1.7.** The config sits in `traffic.jwtAuthentication.mcp`, beside `providers`, which must then hold exactly one provider in Strict mode. The protected-resource metadata names Keycloak as the authorization server (`resourceMetadata.authorizationServers`), not the gateway. Claude Code checks the RFC 9207 `iss` and rejects the gateway's authorization-server facade, because Keycloak stamps its own issuer.
- **Client registration.** Claude Code uses the pre-registered public client `eacp-mcp-public` (PKCE, loopback callback on port 33418): `claude mcp add --transport http --client-id eacp-mcp-public --callback-port 33418 ...`. Keycloak refuses anonymous dynamic registration, so registration is not proxied.
- **Keycloak over TLS.** Claude Code refuses an http token endpoint unless the host is literally localhost, 127.0.0.1 or ::1. The laptop issuer is therefore `https://auth.127.0.0.1.sslip.io:8081`, with a certificate from a cert-manager laptop CA. `make up` exports the CA to `.eacp/ca.pem`. Clients trust it through `NODE_EXTRA_CA_CERTS`, `--cacert` or `EACP_CA_FILE`. Tickets 10 and 12 (LibreChat and Langfuse discovering Keycloak by issuer URL) need `ca.crt` from Secret `keycloak/eacp-local-ca`.
- **Realm import.** Declaring `clientScopes` drops Keycloak's built-in scopes, so the generator declares `basic`, `profile`, `email` and `web-origins`. Imported users get no roles, so demo users get `default-roles-eacp`; without it `offline_access` fails. The operator skips realms that already exist: to re-import on a running cluster, delete the realm and the KeycloakRealmImport, then wait up to 5 minutes for the gateway's JWKS cache.
- **Precedence.** Traffic policies merge per field and the more precise attachment wins. The gateway-level `llm-jwt` accepts `eacp-llm` and `eacp-mcp` on routes without their own policy; the MCP routes' route-level policy accepts only `eacp-mcp`. Checked on a temporary probe route, since the LLM route arrives in ticket 07.
- **Aggregate.** The virtual backend's selector needs `namespaces` (default: the backend's own namespace). The target name defaults to `<svc>-<port name>`; the Service annotation `agentgateway.dev/mcp-target-name` makes the prefix `tickets_`.
- **Bring-up.** The tools ApplicationSet runs in sync wave 0 with retry, because wave 0 holds Applications other tickets own and wave 1 never ran. The root Applications retry their syncs, but Argo CD does not retry a sync stopped by the controller's 15-minute timeout (seen when the laptop slept mid bring-up); rerun `make up` to restart it. Demo server images are built locally and pinned by digest in kind (`scripts/demo-images.sh`).
- **Runner.** Pinned to the `mcp` Python SDK below 2.0, whose client API changed.
