# 09: One-human approval end to end (scenario 5)

**What to build:** alice asks the agent to email a customer. The call is denied with a link.
bob, a member of the approver group, opens the link, signs in through Keycloak, sees who asked
for what without seeing the message contents, and approves. alice's agent retries the exact
same call and the mail lands in Mailpit. alice trying to approve her own request is refused, a
call with different arguments needs its own approval, and both the request and the decision are
in the audit table with bob's identity.

**Blocked by:** 04 Adapter in the pathway

**Status:** ready-for-agent

- [ ] The email server is onboarded and delivers to Mailpit when called directly inside the cluster.
- [ ] alice's send call returns a tool error with `approval_required`, an approval link and the approver group.
- [ ] The approval page requires a Keycloak login; bob sees the request summary (argument names and sizes, not values) and approves.
- [ ] alice's retry with identical arguments succeeds and the message appears in Mailpit; a retry with changed arguments returns a new approval request.
- [ ] alice opening her own approval link is refused; a user outside the approver group is refused.
- [ ] The audit table holds the `approval_required` row, the `approved` row naming bob, and the final `approved` call row, all sharing the conversation id.
- [ ] The approval expires after the configured window and an expired approval does not satisfy a retry.
