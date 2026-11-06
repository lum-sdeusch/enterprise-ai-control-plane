# 11: Scheduled agent identity path

**What to build:** The nightly triage CronJob runs with no human in the loop and still has a
verifiable identity: its projected Kubernetes service-account token is exchanged at Keycloak for
the agent's own access token, it lists and reads tickets through the gateway, it is routed to
the in-house model only, and the agent-layer policy stops it from closing tickets. Every audit
row names the agent and its human owner.

**Blocked by:** 07 LLM path: routing, prompt guard, failover

**Status:** ready-for-agent

- [ ] Keycloak accepts the projected service-account token in a token exchange for the agent client; the parameters and feature flags are recorded in ADR 0023.
- [ ] A manually triggered run of the CronJob completes and creates the summary ticket.
- [ ] Audit rows for the run show agent `nightly-triage`, the service subject and the owner attribute, with no user in the chain.
- [ ] The run's prompts are served by the in-house model regardless of content.
- [ ] A `close_ticket` call from the agent is denied with the scheduled-agent capability code.
- [ ] The agent's traces appear in Langfuse grouped under its conversation id.
