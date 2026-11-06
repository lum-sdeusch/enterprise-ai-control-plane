#!/usr/bin/env bash
# Build the OPA bundle with the git revision stamped into its manifest.
# Usage: scripts/build-bundle.sh [revision]
# Output: dist/eacp-bundle.tar.gz
#
# Signing: in CI the bundle is signed with `opa sign` using a key from the secrets manager, or
# pushed to the OCI registry and signed with cosign; either way OPA verifies the signature on
# load (bundles.*.signing in platform/eacp-system/opa-config.yaml). Not wired here yet.
set -euo pipefail

REV="${1:-$(git rev-parse --short HEAD 2>/dev/null || echo dev)}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OPA="${OPA:-opa}"

cd "$ROOT"
mkdir -p dist
"$OPA" build \
  --bundle policies \
  --ignore tests \
  --ignore README.md \
  --revision "$REV" \
  --output dist/eacp-bundle.tar.gz

echo "built dist/eacp-bundle.tar.gz at revision $REV"
tar -tzf "$ROOT/dist/eacp-bundle.tar.gz" | sed 's/^/  /'
