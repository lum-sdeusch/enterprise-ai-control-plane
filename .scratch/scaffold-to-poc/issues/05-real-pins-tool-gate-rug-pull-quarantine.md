# 05: Real pins, a working tool gate, and the rug-pull quarantine (scenario 1)

**What to build:** All five demo manifests carry real image digests and real listing hashes
computed from the running servers, so the tool gate stops rejecting placeholders and starts
doing its job: signature verification, image scan, description scan and pin check on every
changed manifest in a pull request. Then the rug-pull: the poisoned demo server is redeployed
with its poisoned description, its tools vanish from listings, calls to it are denied with the
quarantine code, an audit row records the event, and merging a manifest with the new hash (or
restoring the benign image) lifts the quarantine.

**Blocked by:** 04 Adapter in the pathway

**Status:** ready-for-agent

- [ ] The pin script, run against each live server, writes real hashes into all five manifests, and the Go adapter accepts those pins without quarantining anything on first listing.
- [ ] The demo images are built and pushed by CI with real digests in the manifests; the gate's signature check passes for images this repository builds and records the publisher identity question for third-party images.
- [ ] The tool gate runs green on a pull request that changes one manifest and red on a pull request that reintroduces a placeholder or a stale pin.
- [ ] Redeploying the poisoned server with the poisoned description removes its tools from the aggregate listing within one listing cycle and calls return `server_quarantined`.
- [ ] The quarantine audit row and an alert (log line or metric) exist.
- [ ] Restoring the pinned state lifts the quarantine without restarting anything.
