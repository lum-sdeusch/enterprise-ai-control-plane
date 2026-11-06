# 10: LibreChat with Keycloak, identity on the LLM path, budgets (scenario 6)

**What to build:** An employee opens LibreChat, signs in with Keycloak, chats with the `eacp-auto`
model and uses the aggregate MCP endpoint after an OAuth consent, all through the gateway. The
gateway attributes each prompt to the real user: either LibreChat forwards a per-user token, or
it uses its service credential and a user header the gateway trusts only from LibreChat's own
identity. carol, whose budget is small, runs out of tokens and sees a clear 429. The MongoDB
question is settled by trying FerretDB.

**Blocked by:** 07 LLM path: routing, prompt guard, failover

**Status:** ready-for-agent

- [ ] LibreChat deploys from its chart with Keycloak login and the configured custom endpoint; a demo user logs in and gets a completion through the gateway.
- [ ] LibreChat connects to the aggregate MCP endpoint through OAuth and lists the user's entitled tools.
- [ ] Per-user identity on the LLM path works by the confirmed mechanism, and the audit rows for LibreChat prompts show the real user, not the service identity; ADR 0041 records which mechanism.
- [ ] carol's prompts return 429 after her per-user token budget is exhausted; alice is unaffected.
- [ ] Per-team budgets are generated from the team manifests if the gateway's rate-limit key semantics allow different budgets per key value; otherwise the limitation is documented in ADR 0045.
- [ ] LibreChat on FerretDB works, or the MongoDB exception is written into ADR 0033 and the design's open items.
