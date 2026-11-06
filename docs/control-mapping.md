# Control mapping

Status: seed. Maps the architecture's controls to SOC 2 Trust Services Criteria, ISO/IEC 42001
Annex A, and EU AI Act Article 12 record-keeping. Criterion identifiers are quoted from memory of
the frameworks and must be checked against the licensed texts by a compliance reviewer before
publication. The point of this table is traceability: every control names the component and the
evidence it produces.

## Evidence produced by the platform

| Evidence | Produced by | Store | Retention |
|---|---|---|---|
| Policy decision records, one per tool call and LLM call phase | eacp-adapter, OPA decision logs | ClickHouse `eacp_audit` (insert-only role) | Policy-defined, exported daily in production |
| Traces with redacted content | adapter and gateway via OTel Collector | Langfuse, Jaeger | Days to weeks |
| Approval events with approver identity | eacp-adapter | ClickHouse `eacp_audit` | As decisions |
| Tool onboarding evidence: scan results, signature verification, pinned hashes | CI | Git history, CI artefacts | Life of the repo |
| Identity events | Keycloak event log | Keycloak database | Keycloak policy |
| Policy change history with reviewer | Git plus CODEOWNERS | Git | Life of the repo |

## SOC 2 Trust Services Criteria

| Criterion (verify id) | Control in this architecture | Evidence |
|---|---|---|
| CC6.1 Logical access security | Every call authenticated by Keycloak-issued, audience-bound tokens; tools filtered by group entitlement and agent manifest | Decision records, Keycloak events |
| CC6.2 Access provisioning | Group membership drives entitlement; agents are clients derived from manifests in Git | Realm import generated from manifests |
| CC6.3 Least privilege | Per-tool token exchange; capability classes; clearance ceilings; egress allow-lists | Rendered policies, NetworkPolicies |
| CC6.6 External threats | Prompt and tool-result guardrails; rug-pull quarantine; signed images | Decision records with findings |
| CC6.8 Unauthorised software | Kyverno admits only cosign-verified images; Trivy and mcp-scanner gate in CI | Admission logs, CI artefacts |
| CC7.2 Monitoring | Traces, access logs, decision records, alerts on quarantine and fail-closed events | Telemetry stores |
| CC7.3 Incident evaluation | Trace id in every decision record links decision to full trace | Audit and trace stores |
| CC8.1 Change management | Policies, manifests and platform are code; CODEOWNERS enforces reviewers; bundles carry the git revision | Git history, bundle revision in records |
| CC9.2 Vendor risk | Component bill of materials with licenses and rejected alternatives | docs/design.md, ADRs |

## ISO/IEC 42001 Annex A (verify control ids)

| Control area | Control in this architecture | Evidence |
|---|---|---|
| A.6 AI system life cycle: requirements, design, verification | Design document, ADRs, threat model, policy unit tests per threat | docs/, policies/tests |
| A.7 Data for AI systems: classification, provenance | Four-level classification with max semantics; classification routing; tool output classification in manifests | Route label in traces and records |
| A.8 Information for interested parties | Audit records readable per team; approval UI explains why a call paused | ClickHouse, approval page |
| A.9 Use of AI systems: responsible use, human oversight | One-human approval tier; scheduled agents carry a named owner | Approval events, agent manifests |
| A.10 Third-party and supplier relationships | Signed, scanned, pinned third-party MCP servers; license rule for every component | CI gate, docs/design.md |

## EU AI Act Article 12 record-keeping (verify against the final text)

Article 12 requires high-risk AI systems to allow automatic recording of events over their
lifetime, sufficient to identify risk situations and support post-market monitoring. The
platform's decision record is the mechanism. Each record carries:

| Field | Purpose under Article 12 |
|---|---|
| Timestamp, duration, phase | Period of each use |
| Subject, actor chain, team, agent | Identification of the natural persons involved and the software acting for them |
| Model requested and model selected, route label | Which system processed the input, and why |
| Tool, server, pinned-hash status | Reference to the tool version in use |
| Findings summary, taint before and after | Input data characteristics that influenced the decision |
| Decision, reasons, policy bundle revision | The rule that produced the outcome, reproducibly |
| Approval event with approver | Human oversight evidence |
| Trace id | Link to the full technical trace |

Records are unsampled and written to an insert-only store. Their retention and export are
operational decisions documented in docs/design.md section 7.

## Mappings deliberately not claimed

HIPAA, PCI DSS and GDPR Article 30 are not mapped here. GDPR considerations that do apply,
such as minimisation through redaction before storage and the handling of PII in classifiers,
are described in docs/design.md and ADR 0009.
