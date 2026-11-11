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

# The bundle format loads only files named data.json or data.yaml. A data file under another
# name is dropped without a word, and the unit tests, which load the directory, would not
# notice. So ask the built bundle itself: is the data there, and do the tests pass against it?
"$OPA" eval --fail --format raw --bundle dist/eacp-bundle.tar.gz \
  'count(data.eacp.tools) > 0; count(data.eacp.agents) > 0; data.eacp.config.models.inhouse != ""; data.eacp.config.thresholds.injection_block > 0' >/dev/null \
  || { echo "bundle is missing policy data (data.eacp.tools / agents / config)" >&2; exit 1; }
# Unpack the bundle beside a copy of the tests so the tests run against exactly what ships.
CHECK="$(mktemp -d)"
trap 'rm -rf "$CHECK"' EXIT
tar -xzf dist/eacp-bundle.tar.gz -C "$CHECK" 2>/dev/null
cp -r policies/tests "$CHECK/tests"
"$OPA" test "$CHECK" >/dev/null \
  || { echo "policy unit tests fail against the built bundle" >&2; "$OPA" test "$CHECK" | tail -20 >&2; exit 1; }

echo "built dist/eacp-bundle.tar.gz at revision $REV (data present, tests pass against the bundle)"
tar -tzf "$ROOT/dist/eacp-bundle.tar.gz" | sed 's/^/  /'
