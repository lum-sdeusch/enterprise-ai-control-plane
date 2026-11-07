# Enterprise AI Control Plane (eacp)

A reference architecture and runnable proof of concept for **secure, compliant, controlled
enterprise AI with MCP tools**, built only from open-source components.

One gateway for LLM and MCP traffic. One policy engine for every prompt and every tool call.
One identity chain from the human to the tool. One pull request to onboard a new tool.

> Status: scaffold. The design is agreed and recorded in [docs/design.md](docs/design.md) and
> the [ADRs](docs/adr/README.md). Policies, schemas, the Helm chart, the adapter and the guardrail
> service exist as verified skeletons. Nothing has been deployed to a cluster yet. See
> [What is verified](#what-is-verified-and-what-is-not).

---

## Why this exists

Enterprises adopting agents hit the same four walls: every tool integration is a bespoke
security review, nobody can prove which human authorised which tool call, prompts with customer
data leak to whichever model the client picked, and the audit trail lives in five vendor
consoles. Commercial "AI gateways" answer parts of this. This repository answers all of it with
components you can read, fork and run, and documents every rejected alternative with the license
or feature-gate that excluded it.

## Architecture

```mermaid
flowchart LR
  subgraph Clients
    UI[LibreChat]
    IDE[Claude Code]
    CRON[Scheduled Pydantic AI agent]
  end
  KC[Keycloak]
  subgraph Gateway[agentgateway]
    LLMR[LLM routes]
    MCPR[MCP routes / virtual MCP]
  end
  subgraph Enforcement
    AD[eacp-adapter]
    OPA[OPA sidecar]
    GR[eacp-guardrails]
    VK[Valkey taint + approvals]
  end
  subgraph Tools[Team namespaces]
    T1[kmcp MCPServer pods]
  end
  ANTH[Anthropic API]
  LOCAL[vLLM / Ollama in-house model]
  subgraph Telemetry
    COL[OTel Collector + redaction]
    LF[Langfuse]
    JG[Jaeger]
    CH[(ClickHouse audit DB)]
  end
  UI & IDE & CRON -- OIDC / OAuth 2.1 --> KC
  UI & IDE & CRON --> MCPR
  UI & CRON --> LLMR
  MCPR -- ExtMCP hook --> AD
  LLMR -- callout + prompt guard --> AD
  AD --> OPA
  AD --> GR
  AD --> VK
  MCPR -- token exchange --> T1
  LLMR -- route label --> ANTH
  LLMR -- route label --> LOCAL
  AD -- spans with redacted content --> COL
  Gateway --> COL
  COL --> LF
  COL --> JG
  AD -- audit events --> CH
  OPA -- decision logs --> CH
```

| Role | Component | License |
|---|---|---|
| LLM gateway and MCP gateway | [agentgateway](https://agentgateway.dev) (Agentic AI Foundation, Linux Foundation) | Apache-2.0 |
| MCP server runtime for stdio servers | [kmcp](https://kagent.dev/docs/kmcp) `MCPServer` | Apache-2.0 |
| Policy decision point | [Open Policy Agent](https://www.openpolicyagent.org) | Apache-2.0 |
| Policy enforcement orchestrator | `eacp-adapter` (this repo, Go) | Apache-2.0 |
| Guardrails: PII, secrets, injection, sensitivity | `eacp-guardrails` (this repo, Python) with Presidio, GLiNER2-PII, gitleaks rules, Qwen3Guard-Gen, Granite Guardian, Lev | MIT / Apache-2.0 |
| Identity | Keycloak | Apache-2.0 |
| LLM observability and infrastructure tracing | Langfuse, Jaeger, OpenTelemetry Collector | MIT core, Apache-2.0 |
| Audit store, session state | ClickHouse, Valkey | Apache-2.0, BSD-3 |
| Secrets | OpenBao, External Secrets Operator | MPL-2.0, Apache-2.0 |
| Admission, GitOps, supply chain | Kyverno, Argo CD, cosign, Trivy, mcp-scanner, Conftest | Apache-2.0 |
| Clients | LibreChat, Pydantic AI, Claude Code | MIT, MIT, n/a |

The full bill of materials, with every rejected alternative and the reason, is in
[docs/design.md](docs/design.md#2-component-bill-of-materials).

## What the demo proves

Six scenarios, each one a policy test fixture, an end-to-end test, and a stage demo:

1. **Rug-pull quarantine.** A server changes a tool description after onboarding. The adapter
   hashes every `tools/list`, finds the pinned hash no longer matches, and quarantines the server.
2. **Indirect prompt injection in a tool result.** A fetched web page carries instructions. The
   guardrail classifier flags it; policy blocks the result and taints the conversation.
3. **Session-taint exfiltration block.** The agent read a customer record. Its next call to an
   exfiltration-capable tool is denied, even though that call looks harmless on its own.
4. **Classification-aware model routing.** A prompt touching restricted data is routed to the
   in-house model by the gateway's callout to the adapter. The trace proves where it went.
5. **Human approval on a high-risk tool.** Sending email requires one human approver from the
   tool owner's group. Deny with link, approve, retry succeeds, both events audited.
6. **Budget exhaustion.** A team's daily token budget runs out; the gateway denies with a clear message.

## Quick start (laptop profile)

Target: 16 GB RAM, 4 cores, no GPU. You bring Docker, `kind`, `kubectl`, Go 1.26, Python 3.12
and an Anthropic API key for the external model. Everything else runs in the cluster.

```bash
make tools        # builds opa, helm, buf and the protobuf generators into ./bin via `go install`
python3.12 -m venv .venv && source .venv/bin/activate   # Homebrew and Debian Pythons refuse a system-wide pip install
pip install -e "guardrails[test]" jsonschema pyyaml
make test         # schemas, policy tests, chart render, adapter build and tests, guardrail tests, ADRs
make up PROFILE=laptop
make demo
```

`make up` is a skeleton today. The order it will follow, and the components it installs, are in
[scripts/up.sh](scripts/up.sh).

## Onboarding a tool in one pull request

A tool is one file: `tools/<name>/manifest.yaml`, validated by
[schemas/tool-manifest.schema.json](schemas/tool-manifest.schema.json).

```yaml
apiVersion: eacp.dev/v1alpha1
kind: ToolManifest
metadata:
  name: tickets
  team: support
  owner: support-platform@example.com
spec:
  image: ghcr.io/example/tickets-mcp@sha256:...
  transport: http
  tools:
    - name: create_ticket
      riskTier: medium
      capability: write
      outputClassification: internal
  pins:
    tools: sha256:...
```

On merge, CI has already scanned the image, verified its signature, scanned the tool
descriptions, pinned their hash and run the policy tests. Argo CD renders the
[`mcp-tool` chart](charts/mcp-tool) into a kmcp `MCPServer` or Deployment, a Service, an
`AgentgatewayBackend`, an `HTTPRoute`, the authentication and guardrail policies, a
`NetworkPolicy` egress allow-list and an `ExternalSecret`. Kyverno admits it only if the image is
signed and the pod is restricted. The tool appears in `tools/list` only for entitled groups.

## Repository map

| Path | What it is |
|---|---|
| `docs/design.md` | The agreed design, bill of materials, flows, decision log |
| `docs/adr/` | One ADR per decision, generated from `docs/adr/decisions.yaml` |
| `docs/threat-model.md`, `docs/control-mapping.md` | OWASP mapping and SOC 2 / ISO 42001 / EU AI Act mapping |
| `schemas/` | JSON schemas for tool, agent and team manifests |
| `tools/`, `agents/`, `teams/` | The manifests. The only files most contributors touch |
| `policies/` | OPA bundle: global, team, tool and agent layers, compiled data, tests and fixtures |
| `charts/mcp-tool` | Helm chart that expands a tool manifest into Kubernetes objects |
| `platform/` | Argo CD applications, gateway configuration, adapter and guardrail deployments, telemetry, Kyverno, Keycloak |
| `adapter/` | Go. agentgateway ExtMCP hook, LLM prompt guard and routing callout, OPA, taint, approvals, audit |
| `guardrails/` | Python. PII, secrets, injection, sensitivity and judge detectors behind one HTTP API |
| `demo/` | Demo MCP servers, LibreChat config, the scheduled agent, scenario runner |
| `promptfoo/` | Nightly red-team configuration |
| `scripts/` | Manifest compilation, validation, bundle build, ADR generation, cluster bring-up |

## Policy model in one paragraph

Deterministic decisions live in OPA; probabilistic checks live in the guardrail service and only
produce findings. For every tool call the adapter runs guardrails on the arguments, loads the
conversation's taint labels, asks OPA for one decision document (`allow`, `reasons`,
`redactions`, `require_approval`, `route_label`, `taint_updates`), and fails closed if any piece
is unavailable. Policies are layered: global rules owned by security, then team, tool and agent
rules owned by their teams, enforced through `CODEOWNERS`. Every decision record carries the
policy bundle revision and the trace id.

## What is verified and what is not

Verified in this repository today: manifest schemas validate the demo manifests; the OPA policies
pass their unit tests against the six fixtures; the Helm chart renders for every demo manifest;
the Go adapter compiles and its unit tests pass; the Python guardrail service's tests pass. The
component facts in the design were verified against primary sources on 2026-11-06, with URLs in
`docs/research/`.

Not yet done: nothing has run on a cluster. The build-time spikes listed in
[docs/design.md section 11](docs/design.md#11-open-items-to-confirm-or-spike-before-building)
are open. Fields marked `TODO(verify)` in `platform/` and `charts/` name the exact CRD details to
confirm against a running agentgateway and kmcp.

## License

Apache-2.0. The vendored `adapter/proto/extmcp/ext_mcp.proto` is from the agentgateway project,
also Apache-2.0; its license text is kept next to it.
