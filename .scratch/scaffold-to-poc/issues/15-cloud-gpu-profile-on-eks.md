# 15: cloud-gpu profile on EKS

**What to build:** The same six scenarios pass on an EKS cluster with GPU nodes: vLLM serves the
in-house model, the Qwen3Guard classifier and the Granite Guardian judge; the judge is consulted
for high-risk tools; TLS terminates at the gateway with cert-manager certificates on real
hostnames; and the optional Lev sensitivity labeller can be switched on. Latency and cost are
measured and written down for both profiles.

**Blocked by:** 13 Six scenarios automated in CI

**Status:** ready-for-agent

- [ ] The cloud-gpu root application syncs on EKS with the GPU operator and the three vLLM deployments Ready.
- [ ] The gateway, Keycloak and LibreChat are reachable over HTTPS on the profile's hostnames with certificates from cert-manager.
- [ ] The end-to-end scenarios pass against the EKS deployment.
- [ ] The judge runs for the email tool and its verdict appears in the findings and the audit row; a judge outage degrades to alert-only for non-high-risk tools and fails closed for high-risk ones.
- [ ] Lev is deployed behind its flag, labels a free-text prompt, and the label participates in routing; its thresholds are set from a labelled sample and recorded.
- [ ] p95 latencies for the tool and LLM paths, and the GPU node cost per hour, are written into the design document.
