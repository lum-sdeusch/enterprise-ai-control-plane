# ADR 0031: One human approver from the tool's approver group; deny-with-link then retry

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q31

## Context

MCP has no approval primitive. Elicitation plus the Tasks extension's input-required state is
the spec-native pattern, but the Tasks extension is beta and LibreChat cannot render
elicitation. The one human is a designated approver, not the requester.

## Decision

The adapter denies with a structured error carrying an approval link; the approver, a member
of the tool manifest's approver group, signs in to the adapter's page through Keycloak and
approves; the agent retries the exact same call and the adapter matches it by request hash.
Self-approval is off unless the manifest allows it. The spec-native flow is behind a flag.

## Alternatives considered

- **Two-person rule.** Explicitly not wanted; one human, one approval.
- **Tasks extension only.** Would fail on stage with the chosen chat UI.

## Consequences

- Both the request and the decision are audited with the approver's identity.
- The deny-with-link reaches the client as the text of a tool result (see ADR 0019): a JSON object whose message names the approver group and the approval URL and whose data repeats them as approval_url and approver_group.
- The approval page connects to Keycloak on first use, not at start-up, so the policy hook never waits on the identity provider.
- A destructive tool after untrusted content requires approval dynamically, as the prompt-injection backstop.
