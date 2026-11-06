# 12: Observability stores, redaction and content capture

**What to build:** One tool call and one prompt can be followed from the audit row to the full
trace to the redacted content, by an auditor who knows only the trace id. Langfuse runs on
Valkey and SeaweedFS with Keycloak sign-on and shows the GenAI spans grouped by conversation;
Jaeger shows every span; the collector's redaction provably removes seeded PII and secrets
before storage; a Grafana dashboard shows request, deny and quarantine rates.

**Blocked by:** 04 Adapter in the pathway

**Status:** ready-for-agent

- [ ] Langfuse is up against Valkey and SeaweedFS and a demo user signs in through Keycloak; the result is written into the design's open items.
- [ ] A conversation with one prompt and two tool calls appears in Langfuse as one session with the adapter's content spans, and in Jaeger as one trace with gateway and adapter spans.
- [ ] A seeded email address and a seeded access key in a tool result cannot be found in any stored span in Langfuse or Jaeger, while the redaction markers can.
- [ ] A SQL query on the audit table by trace id returns the decision rows, and the trace id opens the matching trace in Jaeger.
- [ ] Turning content capture off removes content attributes from new spans without any other change in behaviour.
- [ ] Prometheus scrapes the gateway and the adapter and a Grafana dashboard shows calls, denials, approvals and quarantines per team.
