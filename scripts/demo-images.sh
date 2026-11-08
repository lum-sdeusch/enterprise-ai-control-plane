#!/usr/bin/env bash
# Build the demo MCP servers this repository ships and load them into the kind cluster, pinned
# by digest. Until CI publishes the demo images (ticket 05) their manifests carry the placeholder
# digest, so the laptop runs these local builds instead (scripts/up.sh passes them to the tools
# ApplicationSet as platform.localImages).
#
#   scripts/demo-images.sh            # prints one "<tool> <image>@<digest>" line per server
#
# Local images are named eacp.local/demo-<server>. They are outside the Kyverno verify-images
# scope (ghcr.io/lum-sdeusch/enterprise-ai-control-plane/*) because nothing signed them; the
# require-digests policy still applies.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER="${EACP_KIND_CLUSTER:-eacp}"

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

build_and_load() {
  local tool="$1" server="$2" ref digest node
  ref="eacp.local/demo-$server:dev"
  # No provenance attestation: it carries a timestamp, so every rebuild would get a new digest.
  docker build -q --provenance=false --build-arg "SERVER=$server" -t "$ref" -f "$ROOT/demo/servers/Dockerfile" "$ROOT/demo/servers" >/dev/null
  kind load docker-image "$ref" --name "$CLUSTER" >/dev/null
  for node in $(kind get nodes --name "$CLUSTER"); do
    digest="$(pin_on_node "$node" "$ref")"
  done
  log "$tool: ${ref%:*}@$digest"
  printf '%s %s@%s\n' "$tool" "${ref%:*}" "$digest"
}

main() {
  local entry
  for entry in "${DEMO_TOOLS[@]}"; do
    build_and_load "${entry%%:*}" "${entry##*:}"
  done
}

main "$@"
