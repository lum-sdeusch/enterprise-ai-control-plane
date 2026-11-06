# Threat model

Status: seed. Written from the agreed design; to be reviewed against the current published OWASP
lists before publication. Category identifiers below follow the OWASP Top 10 for LLM Applications
2025 and the OWASP Top 10 for Agentic Applications; verify the exact titles and ids against the
published documents and adjust.

## Assets

| Asset | Where it lives | Why an attacker wants it |
|---|---|---|
| Customer and employee data reachable through tools | Tool backends (`customer-db`, `tickets`) | Exfiltration, extortion |
| Prompts and completions | Transit through the gateway; redacted copies in Langfuse | Contain the above |
| Tool credentials | OpenBao, delivered by External Secrets Operator | Direct access to backends |
| Identity tokens | Keycloak, client sessions, gateway token exchange | Impersonation, privilege escalation |
| The policy bundle | OCI registry, OPA sidecar | Disabling controls silently |
| The audit log | ClickHouse `eacp_audit` | Covering tracks |
| Model API keys and budgets | Gateway secrets | Cost abuse |

## Trust boundaries

1. **Client to gateway.** Clients are untrusted. Everything is authenticated with a Keycloak-issued
   token whose audience is the gateway resource. Token passthrough is forbidden.
2. **Gateway to adapter.** Trusted, in-cluster, h2c. The adapter trusts the claims the gateway
   verified; it never re-trusts a raw client header unless the caller's identity is LibreChat's.
3. **Adapter to tool.** The tool is semi-trusted: its image is signed and scanned, but its output
   is data, never instructions. Tool results pass through the response-phase guardrails.
4. **Tool to the outside world.** Egress is an allow-list per tool. The default is deny.
5. **Gateway to model providers.** The external provider only ever sees prompts that routing
   allowed to leave; restricted and confidential prompts stay with the in-house model.

## Attacker model

- An employee with a valid login who tries to use the agent to reach data outside their
  entitlement, or to move data out through a tool.
- A third-party MCP server author who changes a tool's description after it was approved, or
  ships a malicious image.
- A web page, document or ticket that contains instructions aimed at the agent.
- An attacker with a stolen client token, bounded by audience, lifetime and the actor chain.
- A compromised in-cluster workload trying to reach tools or secrets directly.

## Threats, controls, and how the demo proves each one

| # | Threat | OWASP LLM 2025 | OWASP Agentic | Primary control | Secondary controls | Demo scenario |
|---|---|---|---|---|---|---|
| T1 | Tool poisoning and rug-pull: tool description changes after review | LLM03 Supply Chain | ASI04 Agentic Supply Chain | Pinned hash of tools, prompts and resources; adapter quarantines on mismatch | mcp-scanner in CI; cosign and Trivy on the image; Kyverno admits signed images only | 1 |
| T2 | Indirect prompt injection via tool result | LLM01 Prompt Injection | ASI01 Agent Goal Hijack | Response-phase injection classifier; policy blocks above threshold | Untrusted-content taint; destructive tools require approval after untrusted content | 2 |
| T3 | Data exfiltration through a tool after reading sensitive data | LLM02 Sensitive Information Disclosure | ASI02 Tool Misuse | Session taint: no exfil-capable tool after `touched_restricted` | PII redaction in arguments of exfil-capable tools; egress allow-list | 3 |
| T4 | Sensitive prompt sent to an external model | LLM02 | ASI03 Identity & Privilege Abuse (data boundary) | Classification routing: callout decides the target model; fallback is in-house | Prompt guard masks PII before external models; access logs record the chosen target | 4 |
| T5 | Excessive agency: high-impact action without a human | LLM06 Excessive Agency | ASI02 Tool Misuse, ASI09 Human-Agent Trust Exploitation | One human approver from the tool owner's group; deny-with-link then retry | Risk tier and capability class in the manifest; audit of both events | 5 |
| T6 | Unbounded consumption and cost abuse | LLM10 Unbounded Consumption | ASI08 Cascading Failures | Token budgets per user and team keyed by JWT claims in the gateway | Cost attribution in Langfuse; per-model limits | 6 |
| T7 | Privilege escalation through the gateway (confused deputy) | LLM06 | ASI03 Identity & Privilege Abuse | Audience-bound tokens; token exchange per backend; actor chain in every token | Keycloak client per agent; Kubernetes service-account federation for workloads | covered by scenario 3 and 5 identity checks |
| T8 | Unauthorised tool discovery and use | LLM06 | ASI03 | `tools/list` filtered to group entitlement intersected with the agent manifest | Clearance ceiling versus tool output classification | policy tests |
| T9 | Compromised MCP server image or lateral movement | LLM03 | ASI04, ASI05 Unexpected Code Execution | Signed images, restricted pod security, default-deny egress with allow-list | Namespace per team; no secrets in the model's context | CI gate |
| T10 | Control plane outage used to bypass policy | n/a | ASI08 | Fail closed in every hook; audit write required | Health policies; alerts | adapter unit tests |

## Residual risks and explicit non-goals

- Full information-flow control across an agent run is not attempted; session taint is coarse.
- Classification of free text beyond PII is a flagged, optional model (Lev) and is not a control.
- Standard `NetworkPolicy` cannot express hostnames. FQDN egress rules require Cilium; without it
  the manifest's `fqdn` entries are documentation, not enforcement, and CI warns about it.
- The guardrail classifiers are probabilistic. Their thresholds are tuned per deployment and the
  model versions are recorded in every finding so decisions can be reproduced.
- Elicitation-based approval is behind a flag because the chosen chat UI cannot render it.
