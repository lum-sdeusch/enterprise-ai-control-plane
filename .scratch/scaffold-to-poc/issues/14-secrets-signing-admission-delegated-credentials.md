# 14: Secrets seeding, bundle signing, admission verification, delegated credentials

**What to build:** The supply chain and secrets story is demonstrably enforced rather than
configured: an unsigned image cannot start in a team namespace, OPA refuses a policy bundle that
CI did not sign, every demo secret reaches its pod from OpenBao, and one tool receives a
per-user credential through token exchange so its backend sees the real user instead of a
shared account.

**Blocked by:** 02 Foundation bring-up on kind; 05 Real pins, tool gate, rug-pull quarantine

**Status:** ready-for-agent

- [ ] A pod with an unsigned image, or an image referenced by tag, is rejected by Kyverno in a team namespace with a readable message; the signed demo images are admitted.
- [ ] CI signs the policy bundle and OPA is configured to verify it; a tampered bundle is refused and the previous bundle stays active.
- [ ] The OpenBao seeding script is idempotent and every ExternalSecret in the platform and the demo is Ready after a fresh bring-up.
- [ ] One tool is switched to delegated credentials: the gateway exchanges the caller's token for the backend's audience and the backend logs the real user; the Secret key name the gateway expects is recorded in the chart.
- [ ] ADRs 0025 and 0030 have their verification items answered.
