# Architecture decision records

Generated from [decisions.yaml](decisions.yaml) by `scripts/gen-adrs.py`; edit the YAML, not these files.

Each record is one decision from the design rounds recorded in [docs/design.md](../design.md). Numbers match the decision log there.

| ADR | Title | Status |
|---|---|---|
| 0001 | [Publish a repository and a long-form article for architects](0001-publish-a-repository-and-a-long-form-article-for-architects.md) | accepted |
| 0002 | [Open source means OSI-approved licenses with a documented open-core boundary](0002-open-source-means-osi-approved-licenses-with-a-documented-op.md) | accepted |
| 0003 | [Kubernetes only, cloud-agnostic, with a kind laptop profile and an EKS GPU profile](0003-kubernetes-only-cloud-agnostic-with-a-kind-laptop-profile-an.md) | accepted |
| 0004 | [LibreChat and a scheduled in-cluster agent are first-class clients; Claude Code is the tested IDE path](0004-librechat-and-a-scheduled-in-cluster-agent-are-first-class-c.md) | accepted |
| 0005 | [OWASP threat taxonomies and SOC 2, ISO 42001 and EU AI Act Article 12 as mappings](0005-owasp-threat-taxonomies-and-soc-2-iso-42001-and-eu-ai-act-ar.md) | accepted |
| 0006 | [Every token carries the user and an actor chain; scheduled agents have a service identity with a named owner](0006-every-token-carries-the-user-and-an-actor-chain-scheduled-ag.md) | accepted |
| 0007 | [Deterministic policy decision point separated from probabilistic guardrails, one PDP for both pathways](0007-deterministic-policy-decision-point-separated-from-probabili.md) | accepted |
| 0008 | [Session taint tracking](0008-session-taint-tracking.md) | accepted |
| 0009 | [Redact at the collector, keep a separate append-only audit log, make content capture a switch](0009-redact-at-the-collector-keep-a-separate-append-only-audit-lo.md) | accepted |
| 0010 | [Fail closed everywhere, with latency budgets per check class](0010-fail-closed-everywhere-with-latency-budgets-per-check-class.md) | accepted |
| 0011 | [Anthropic plus one in-house model, with classification-aware routing](0011-anthropic-plus-one-in-house-model-with-classification-aware.md) | accepted |
| 0012 | [Scope fence for the POC](0012-scope-fence-for-the-poc.md) | accepted |
| 0013 | [A small published adapter layer; two boxes in the diagram, one product may fill both](0013-a-small-published-adapter-layer-two-boxes-in-the-diagram-one.md) | accepted |
| 0014 | [Named products are candidates; OPA is the catch-all policy engine](0014-named-products-are-candidates-opa-is-the-catch-all-policy-en.md) | accepted |
| 0015 | [Onboarding is one pull request, measured by an acceptance test, with a security gate](0015-onboarding-is-one-pull-request-measured-by-an-acceptance-tes.md) | accepted |
| 0016 | [Team is a first-class dimension from day one](0016-team-is-a-first-class-dimension-from-day-one.md) | accepted |
| 0017 | [agentgateway as the MCP gateway, kmcp as the stdio server runtime](0017-agentgateway-as-the-mcp-gateway-kmcp-as-the-stdio-server-run.md) | accepted |
| 0018 | [One gateway product for both the LLM and MCP roles](0018-one-gateway-product-for-both-the-llm-and-mcp-roles.md) | accepted |
| 0019 | [The adapter is the single enforcement orchestrator, with OPA as its sidecar](0019-the-adapter-is-the-single-enforcement-orchestrator-with-opa.md) | accepted |
| 0020 | [Langfuse plus Jaeger through collector fan-out](0020-langfuse-plus-jaeger-through-collector-fan-out.md) | accepted |
| 0021 | [Content is captured only in the adapter; client instrumentation stays metadata-only](0021-content-is-captured-only-in-the-adapter-client-instrumentati.md) | accepted |
| 0022 | [Audit records in a separate ClickHouse database with an insert-only role](0022-audit-records-in-a-separate-clickhouse-database-with-an-inse.md) | accepted |
| 0023 | [Keycloak, with preview delegation as the primary actor-chain path](0023-keycloak-with-preview-delegation-as-the-primary-actor-chain.md) | accepted |
| 0024 | [Git manifests are the source of truth; Keycloak is derived](0024-git-manifests-are-the-source-of-truth-keycloak-is-derived.md) | accepted |
| 0025 | [Helm chart expansion, Kyverno admission, Trivy pinned and verified, mcp-scanner](0025-helm-chart-expansion-kyverno-admission-trivy-pinned-and-veri.md) | accepted |
| 0026 | [Pin tools, prompts and resources; quarantine on mismatch](0026-pin-tools-prompts-and-resources-quarantine-on-mismatch.md) | accepted |
| 0027 | [Conversation id from baggage, subject-and-agent fallback, state in Valkey](0027-conversation-id-from-baggage-subject-and-agent-fallback-stat.md) | accepted |
| 0028 | [Four-level deterministic classification with max semantics](0028-four-level-deterministic-classification-with-max-semantics.md) | accepted |
| 0029 | [Two profiles, laptop on CPU with Ollama and cloud-gpu with vLLM](0029-two-profiles-laptop-on-cpu-with-ollama-and-cloud-gpu-with-vl.md) | accepted |
| 0030 | [OpenBao single node plus External Secrets Operator](0030-openbao-single-node-plus-external-secrets-operator.md) | accepted |
| 0031 | [One human approver from the tool's approver group; deny-with-link then retry](0031-one-human-approver-from-the-tool-s-approver-group-deny-with.md) | accepted |
| 0032 | [Tool listings are the intersection of entitlement and the agent manifest; names are prefixed](0032-tool-listings-are-the-intersection-of-entitlement-and-the-ag.md) | accepted |
| 0033 | [LibreChat, Pydantic AI and Claude Code as the clients](0033-librechat-pydantic-ai-and-claude-code-as-the-clients.md) | accepted |
| 0034 | [Customer-support domain with five threats on stage plus a budget demo](0034-customer-support-domain-with-five-threats-on-stage-plus-a-bu.md) | accepted |
| 0035 | [Argo CD, a monorepo, one namespace per team](0035-argo-cd-a-monorepo-one-namespace-per-team.md) | accepted |
| 0036 | [Repository enterprise-ai-control-plane with the eacp attribute prefix](0036-repository-enterprise-ai-control-plane-with-the-eacp-attribu.md) | accepted |
| 0037 | [Classification stays deterministic; Lev is an optional flagged sensitivity source](0037-classification-stays-deterministic-lev-is-an-optional-flagge.md) | accepted |
| 0038 | [The approver comes from the manifest's approver group; self-approval is a per-tool flag, off by default](0038-the-approver-comes-from-the-manifest-s-approver-group-self-a.md) | accepted |
| 0039 | [EKS for the cloud-gpu profile](0039-eks-for-the-cloud-gpu-profile.md) | accepted |
| 0040 | [Anthropic only, model-level failover, in-house as last resort](0040-anthropic-only-model-level-failover-in-house-as-last-resort.md) | accepted |
| 0041 | [Identity on the LLM path, JWT first and trusted header second](0041-identity-on-the-llm-path-jwt-first-and-trusted-header-second.md) | accepted |
| 0042 | [The in-cluster Pydantic AI agent is a scheduled, autonomous agent](0042-the-in-cluster-pydantic-ai-agent-is-a-scheduled-autonomous-a.md) | accepted |
| 0043 | [One trace per user turn; the adapter continues it; audit records carry the trace id](0043-one-trace-per-user-turn-the-adapter-continues-it-audit-recor.md) | accepted |
| 0044 | [Layered policy repository, one decision document, fixtures per threat, signed bundles](0044-layered-policy-repository-one-decision-document-fixtures-per.md) | accepted |
| 0045 | [Token budgets enforced, currency reported](0045-token-budgets-enforced-currency-reported.md) | accepted |
| 0046 | [GitHub Actions with a per-PR gate and a nightly red-team](0046-github-actions-with-a-per-pr-gate-and-a-nightly-red-team.md) | accepted |
| 0047 | [Argo CD ApplicationSet from tool manifests](0047-argo-cd-applicationset-from-tool-manifests.md) | accepted |
| 0048 | [One Keycloak realm; groups are teams; two gateway audiences; realm import from manifests](0048-one-keycloak-realm-groups-are-teams-two-gateway-audiences-re.md) | accepted |
| 0049 | [Publish under the organisation's GitHub, cross-post the article, generate ADRs, record a short video](0049-publish-under-the-organisation-s-github-cross-post-the-artic.md) | accepted |
