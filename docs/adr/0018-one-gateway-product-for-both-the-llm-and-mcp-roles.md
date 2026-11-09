# ADR 0018: One gateway product for both the LLM and MCP roles

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q18

## Context

agentgateway routes LLM traffic too: providers including Anthropic and custom
OpenAI-compatible backends, failover priority groups, virtual models with weighted,
failover, conditional and callout selection, token budgets keyed by JWT claims, prompt
guards, GenAI-convention spans. Agent Router (formerly Envoy AI Gateway, same foundation)
was the separate candidate. LiteLLM gates JWT auth and audit logs, Kong has no free tier
after 3.9, Portkey's repository went idle after its acquisition, TensorZero is archived,
Helicone's gateway has a GPL license file contradicting its README.

## Decision

agentgateway fills both boxes. Agent Router is documented as the drop-in for the LLM box.

## Alternatives considered

- **Agent Router for the LLM box.** Two CRD families, two policy models, two tracing pipelines for no capability gain.

## Consequences

- Classification routing uses the virtual model callout, which is native to agentgateway.

## Verified at build time

- AgentgatewayModel provider and model fields for Anthropic.
  **Answer (ticket 03, agentgateway 1.7.0 CRD schema and server-side dry run):** provider Anthropic exists; there is no model field (the API server rejects it). The name clients request is match.model, which defaults to metadata.name. Ticket 07 sets the upstream model id and runs the first request.
- Whether a callout may target a virtual model; otherwise the adapter returns concrete model names.
  **Answer (ticket 03, agentgateway 1.7.0 CRD schema):** It may not. callout.transformation.model must select a concrete model, matched against match.model of models on the same listener, so the adapter returns concrete names and the external failover chain has to be arranged without a virtual model behind the callout (ticket 07). callout.headers accepts the :path pseudo-header for the adapter's /v1/route.
- promptGuard target kind when models are AgentgatewayModel resources.
  **Answer (ticket 03, agentgateway 1.7.0 server-side dry run):** An AgentgatewayPolicy cannot target AgentgatewayModel (allowed: Gateway, HTTPRoute, GRPCRoute, ListenerSet, Service, AgentgatewayBackend, InferencePool). The prompt guard goes in spec.policies.promptGuard on each concrete model; virtual models cannot carry policies. Until ticket 07 the LLM resources are behind llm.enabled in the eacp-gateway chart.
