# ADR 0037: Classification stays deterministic; Lev is an optional flagged sensitivity source

- Status: accepted
- Date: 2026-11-06
- Decided in: design round 3, question Q37

## Context

The model asked about was Jev (TypeSafe AI, September 2026), a proprietary hosted decision
model: API-only, 70 to 500 ms, no documented EU operation. It fails the open-source rule, the
200 ms budget and data residency. Lev (Interfaze) is an Apache-2.0 clone on Qwen3.5-4B that
mimics its API. Red Hat's October 2026 benchmark found decision models do not reliably beat
small pre-trained classifiers for guardrails and recommends them only where no pre-trained
classifier exists.

## Decision

Lev is the optional fourth classification source, behind a flag, cloud-gpu profile only,
used for one job: zero-shot sensitivity labelling of free text with a fixed-order choice over
the four labels, merged with max semantics. Never used for injection, PII or secrets. Jev's
hosted API is never on the path. Thresholds are set from a few hundred of the team's own
samples; the model version is recorded in every finding; English only unless validated.

## Alternatives considered

- **Jev hosted API.** Proprietary, too slow for the synchronous budget, and would send restricted text to a third party.
- **Laya or Kev.** Laya's license is unstated; Kev carries a contamination flag.
