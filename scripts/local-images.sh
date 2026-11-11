#!/usr/bin/env bash
# Build the images this repository ships and load them into the kind cluster, pinned by digest.
# The laptop profile runs these local builds: CI builds and scans the same images but publishes
# and signs nothing (ADR 0046), and the tool manifests of the demo servers still carry the
# placeholder digest until ticket 05.
#
#   scripts/local-images.sh tools      demo MCP servers; prints "<tool> <image>@<digest>" lines,
#                                      which scripts/up.sh hands to the tools ApplicationSet as
#                                      platform.localImages
#   scripts/local-images.sh platform   the adapter and the guardrail service; prints
#                                      "adapter <image>@<digest>" and "guardrails <image>@<digest>",
#                                      which scripts/up.sh hands to the eacp-system chart as
#                                      images.adapter and images.guardrails
#
# Local images are named eacp.local/<name>. They are outside the Kyverno verify-images scope
# (ghcr.io/lum-sdeusch/enterprise-ai-control-plane/*) because nothing signed them; the
# require-digests policy still applies in team namespaces, and every reference printed here is a
# digest reference. Signed, published images are ticket 14.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER="${EACP_KIND_CLUSTER:-eacp}"
VERSION="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo dev)"

# <tool manifest name>:<directory under demo/servers>. Later tickets add theirs.
DEMO_TOOLS=(tickets:tickets)

log() { printf '    %s\n' "$*" >&2; }

# The digest the node's containerd records for a loaded image, and a digest reference for it:
# kind load registers only the tag, and a pod that names the image by digest would otherwise
# try to pull it from a registry.
pin_on_node() {
  local node="$1" ref="$2" digest
  digest="$(docker exec "$node" ctr -n k8s.io images ls "name==$ref" | awk 'NR == 2 {print $3}')"
  [[ "$digest" == sha256:* ]] || { log "no digest for $ref on $node"; return 1; }
  docker exec "$node" ctr -n k8s.io images tag --force "$ref" "${ref%:*}@$digest" >/dev/null
  printf '%s' "$digest"
}

# load_pinned LABEL REF: load a built image into every node and print "LABEL <image>@<digest>".
load_pinned() {
  local label="$1" ref="$2" digest node
  kind load docker-image "$ref" --name "$CLUSTER" >/dev/null
  for node in $(kind get nodes --name "$CLUSTER"); do
    digest="$(pin_on_node "$node" "$ref")"
  done
  log "$label: ${ref%:*}@$digest"
  printf '%s %s@%s\n' "$label" "${ref%:*}" "$digest"
}

# No provenance attestation on any build: it carries a timestamp, so every rebuild would get a
# new digest and restart the workload for nothing.
build_tool() {
  local tool="$1" server="$2" ref="eacp.local/demo-$2:dev"
  docker build -q --provenance=false --build-arg "SERVER=$server" -t "$ref" -f "$ROOT/demo/servers/Dockerfile" "$ROOT/demo/servers" >/dev/null
  load_pinned "$tool" "$ref"
}

build_platform() {
  local name="$1" context="$2" ref="eacp.local/eacp-$1:dev"
  docker build -q --provenance=false --build-arg "VERSION=$VERSION" -t "$ref" "$ROOT/$context" >/dev/null
  load_pinned "$name" "$ref"
}

case "${1:-tools}" in
  tools)
    for entry in "${DEMO_TOOLS[@]}"; do build_tool "${entry%%:*}" "${entry##*:}"; done
    ;;
  platform)
    build_platform adapter adapter
    build_platform guardrails guardrails
    ;;
  *)
    echo "usage: scripts/local-images.sh [tools|platform]" >&2
    exit 2
    ;;
esac
