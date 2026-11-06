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

- [ ] The chart has a value that omits the guardrail processor policy, used only for this slice and off by default.
- [ ] Protected resource metadata is served for the per-server path and for the aggregate path, and Claude Code completes the OAuth flow against Keycloak, with dynamic client registration proxied or a pre-registered client, and the chosen approach documented.
- [ ] A token with the wrong audience is rejected; a token with the `eacp-mcp` audience is accepted.
- [ ] The precedence between the gateway-level JWT policy and the per-route policies is understood and the committed arrangement works for both MCP and LLM audiences.
- [ ] The aggregate endpoint lists the tickets tools with the server prefix; the virtual backend's selector finds servers in team namespaces, or the alternative is committed.
- [ ] The backend target's protocol value and path handling for Streamable HTTP servers are confirmed and the chart updated.
- [ ] ADRs 0017 and 0018 have their verification items answered.
