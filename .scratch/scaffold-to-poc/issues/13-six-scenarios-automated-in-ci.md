# 13: The six scenarios automated in CI

**What to build:** `make e2e` runs all six demo scenarios against a kind cluster and passes, and
the same job runs on every pull request in GitHub Actions instead of being dispatch-only. The
two steps that needed a human in the earlier slices are automated: the poisoned redeploy
between the first scenario's steps, and bob's approval between the fifth scenario's steps.
Failures print the relevant audit rows so a red run explains itself.

**Blocked by:** 05 Real pins, tool gate, rug-pull quarantine; 06 web-fetch via kmcp and taint exfil block; 08 Guard model in budget and injection block; 09 One-human approval end to end; 10 LibreChat, LLM-path identity, budgets

**Status:** ready-for-agent

- [ ] The scenario runner's assertions for all six scenarios pass locally against `make up`.
- [ ] The poisoned redeploy and bob's approval are driven by the end-to-end script with no manual step.
- [ ] The end-to-end job runs on pull requests and on `main`, finishes within a documented time, and is green.
- [ ] A failing assertion prints the audit rows for the conversation under test.
- [ ] Flaky steps have bounded retries with logs, and a run cannot pass with a scenario skipped.
