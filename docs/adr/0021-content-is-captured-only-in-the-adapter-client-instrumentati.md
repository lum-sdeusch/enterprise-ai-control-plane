# ADR 0021: Content is captured only in the adapter; client instrumentation stays metadata-only

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 2, question Q21

## Context

agentgateway puts tool arguments and results in access logs, not spans. The adapter sees
every argument, result, prompt and completion, so it can be the one capture point with one
redaction pass.

## Decision

The adapter emits opt-in GenAI content attributes after redaction when content capture is
on. Client SDKs are instrumented for structure and timing only, which is also what the
OpenTelemetry GenAI conventions recommend by default.

## Alternatives considered

- **Client-side capture.** N configurations and N redaction paths.

## Consequences

- gen_ai.tool.call.arguments, gen_ai.tool.call.result, gen_ai.input.messages and gen_ai.output.messages are set by the adapter only.
