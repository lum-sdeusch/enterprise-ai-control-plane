# 16: Publication

**What to build:** A reader who knows nothing about this project can read the article, watch
the video, clone the tagged release, run the laptop demo and reach the same results. The design
document and the ADRs say what the spikes found, including where they contradicted the original
decision. The compliance mappings have been checked by someone qualified to check them. The
repository is ready to move to the organisation.

**Blocked by:** 13 Six scenarios automated in CI; 15 cloud-gpu profile on EKS

**Status:** ready-for-agent

- [ ] The design document's open items are closed or carried forward with a reason, and every ADR with a verification section has its answers and, where needed, a status change.
- [ ] The threat model and control mapping have been reviewed against the published OWASP lists and the licensed framework texts, and the review is recorded.
- [ ] The README has grown into the article, cross-posted as agreed, with the architecture diagram, the six scenarios and the rejected-alternatives table.
- [ ] A video under ten minutes shows the six scenarios on the laptop profile.
- [ ] Release `v0.1.0` is tagged, with signed images and a signed bundle for that tag.
- [ ] A rehearsed move to the organisation exists: the owner rewrite, CODEOWNERS with real teams, and the signing identity update, as one documented procedure.
