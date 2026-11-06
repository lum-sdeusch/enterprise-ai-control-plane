# 01: CI green and reproducible locally

**What to build:** A contributor who clones the repository on a clean laptop runs the toolchain
install and the full local check and sees everything pass, and the per-PR workflow on GitHub is
green for the same checks. The vulnerability gate on the guardrail image is satisfied by a
cleaner base image or an explicit, justified ignore list, never by lowering the severity. The
nightly red-team workflow is either disabled until a cluster exists or given its four secrets.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] The per-PR workflow is green on `main` for manifests, policies, chart, adapter, guardrails, ADRs, images and tool gate.
- [ ] The guardrail image passes the image scan at HIGH and CRITICAL with unfixed findings ignored; any ignore entry carries a one-line justification.
- [ ] The nightly red-team workflow does not produce failing runs: it is disabled or its secrets exist.
- [ ] `make tools` and `make test` pass on a clean machine with only Docker, Go, Python and git installed; anything else they need is written into the quick start.
- [ ] The quick start in the README matches what actually happens.
