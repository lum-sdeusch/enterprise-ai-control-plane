# 06: web-fetch through kmcp and the taint exfiltration block (scenario 3)

**What to build:** The reference fetch server, a stdio program, runs under kmcp behind the
transport adapter and is reachable through the gateway like any HTTP server. With the customer
database onboarded as well, the scenario plays end to end: alice reads a customer record, the
conversation is tainted as restricted, and her next fetch call is denied with the taint code
even though that call looks harmless on its own. The team-layer host allow-list for fetch is
enforced too.

**Blocked by:** 05 Real pins, tool gate, rug-pull quarantine

**Status:** ready-for-agent

- [ ] The kmcp MCPServer for the fetch server becomes Ready, the chart's Service selects its pods (label confirmed or chart values corrected), and the gateway lists its tool.
- [ ] The fetch image is pre-built with its package installed so the pod needs no package download at start; the egress policy is tightened accordingly.
- [ ] The customer database server is onboarded with its connection string delivered from OpenBao and answers a read-only query.
- [ ] After a customer-database read, the conversation's taint shows `touched_restricted` in the state store and in the audit row.
- [ ] A subsequent fetch call in the same conversation is denied with `exfil_after_restricted`; the same call in a fresh conversation succeeds.
- [ ] A fetch to a host outside the support team's allow-list is denied with the team-layer code.
