# ADR 0017: agentgateway as the MCP gateway, kmcp as the stdio server runtime

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q17

## Context

Three candidates satisfied "policy in the pathway of every tool call": agentgateway
(Apache-2.0, Rust, Agentic AI Foundation under the Linux Foundation, spec 2026-07-28, OAuth
resource-server role, upstream token exchange, gRPC pre/post hook that mutates or denies,
OpenTelemetry), ToolHive (Apache-2.0, Go, Cedar embedded, operator, Sigstore, but 0.x,
undocumented webhook payload, enterprise-gated group mapping and audit, and Cedar
contradicts one-PDP), and IBM ContextForge (Apache-2.0, Python, OPA and Cedar plugins, no
operator, fail-open revocation). The originally proposed registry had only a
registration-time webhook.

## Decision

agentgateway as the MCP data plane. kmcp's MCPServer CRD runs stdio servers behind the
transport adapter sidecar because agentgateway's own CRD cannot run stdio servers.
Hardening comes from Kubernetes primitives: Kyverno, pod security, network policy, cosign.

## Alternatives considered

- **ToolHive alone.** Second policy language at the server edge, enterprise-gated features, 0.x release cadence.
- **IBM ContextForge.** No operator, hand-built stdio sidecar, fail-open behaviours to audit.
- **agentic-community/mcp-gateway-registry.** No runtime policy hook, no protected resource metadata, no client registration.

## Consequences

- The adapter implements agentgateway's ExtMCP protocol; the proto is vendored under adapter/proto.
- ToolHive is documented as the alternative for teams that want argument-level Cedar at the edge.

## Verified at build time

- The label kmcp puts on pods it creates, so the chart's Service selects them (charts/mcp-tool values: platform.kmcp.podLabelKey).
  **Answer (ticket 03, kmcp v0.4.0 source):** app.kubernetes.io/name and app.kubernetes.io/instance, both set to the MCPServer name (pkg/controller/transportadapter/transportadapter_translator.go), so the default podLabelKey app.kubernetes.io/name is right. Read from the source, not yet exercised on a cluster; the first stdio server runs in ticket 06.
- Whether the virtual MCP backend's label selector spans namespaces.
  **Answer (ticket 03, agentgateway 1.7.0 on kind):** No. Without selector.namespaces only the backend's own namespace is searched; the aggregate in platform/charts/eacp-gateway selects namespaces carrying eacp.dev/team. Selected Services need appProtocol agentgateway.dev/mcp on the port. The target (and so the tool prefix) is named "<service>-<port name>" unless the Service carries agentgateway.dev/mcp-target-name, and the path comes from agentgateway.dev/mcp-path; the mcp-tool chart sets both, giving tickets_list_tickets on /mcp.
