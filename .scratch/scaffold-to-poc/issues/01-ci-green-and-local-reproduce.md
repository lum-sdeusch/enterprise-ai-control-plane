# 01: CI green and reproducible locally

**What to build:** A contributor who clones the repository on a clean laptop runs the toolchain
install and the full local check and sees everything pass, and the per-PR workflow on GitHub is
green for the same checks. The vulnerability gate on the guardrail image is satisfied by a
cleaner base image or an explicit, justified ignore list, never by lowering the severity. The
nightly red-team workflow is either disabled until a cluster exists or given its four secrets.

**Blocked by:** None (can start immediately)

**Status:** done (PR 1)

- [x] The per-PR workflow is green on `main` for manifests, policies, chart, adapter, guardrails, ADRs, images and tool gate.
- [x] The guardrail image passes the image scan at HIGH and CRITICAL with unfixed findings ignored; any ignore entry carries a one-line justification.
- [x] The nightly red-team workflow does not produce failing runs: it is disabled or its secrets exist.
- [x] `make tools` and `make test` pass on a clean machine with only Docker, Go, Python and git installed; anything else they need is written into the quick start.
- [x] The quick start in the README matches what actually happens.

**Notes:** Both images scanned clean at HIGH and CRITICAL after bumping go-jose to v4.1.6 and
x/net to v0.62.0, so no ignore list exists. The nightly schedule is commented out until tickets
10 and 13 provide an environment and its secrets. `make tools` and `make test` were verified in a
clean container with only Go 1.26, Python 3.12 and git; the quick start gained a venv step. The
tool gate has only run its early exit (no manifest changes); ticket 05 exercises it for real,
including whether mcp-scanner exits non-zero on findings.
Keyless cosign signing runs only on pushes to `main`, so it is first exercised by the merge of PR 1.

**Update 2026-11-11:** the workflow no longer pushes or signs images; it builds and scans them on
the runner only. Signing moves to a key-based path outside GitHub Actions (tickets 04 and 14).
