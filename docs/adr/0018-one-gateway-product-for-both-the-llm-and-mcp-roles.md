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

## To verify at build time

- AgentgatewayModel provider and model fields for Anthropic.
- Whether a callout may target a virtual model; otherwise the adapter returns concrete model names.
- promptGuard target kind when models are AgentgatewayModel resources.
