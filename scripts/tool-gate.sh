#!/usr/bin/env bash
# Tool onboarding gate (ADR 0015, 0025, 0026). For every tool manifest changed relative to a base
# ref, this script enforces what the schema cannot: real digests and pins, a verifiable image
# signature, a clean vulnerability scan, and clean tool descriptions.
#
#   scripts/tool-gate.sh origin/main
#
# Requires: python3 with pyyaml, cosign, trivy (or grype), mcp-scanner, docker. Each check can
# be skipped with EACP_GATE_SKIP="cosign,trivy,mcpscan" for local dry runs; CI never skips.
set -euo pipefail

BASE="${1:-origin/main}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
SKIP=",${EACP_GATE_SKIP:-},"
ZERO="0000000000000000000000000000000000000000000000000000000000000000"

changed=$(git diff --name-only "$BASE"...HEAD -- 'tools/*/manifest.yaml' 2>/dev/null || git diff --name-only "$BASE" -- 'tools/*/manifest.yaml')
if [[ -z "$changed" ]]; then
  echo "tool-gate: no tool manifests changed"
  exit 0
fi

fail=0
for manifest in $changed; do
  [[ -f "$manifest" ]] || continue
  name=$(basename "$(dirname "$manifest")")
  echo "== $name"
  image=$(python3 -c "import yaml,sys; print(yaml.safe_load(open(sys.argv[1]))['spec']['image'])" "$manifest")
  pins=$(python3 -c "import yaml,sys; d=yaml.safe_load(open(sys.argv[1]))['spec']['pins']; print(' '.join(d.values()))" "$manifest")
  transport=$(python3 -c "import yaml,sys; print(yaml.safe_load(open(sys.argv[1]))['spec']['transport'])" "$manifest")

  if [[ "$image" == *"$ZERO" ]]; then
    echo "   FAIL image digest is the bootstrap placeholder"; fail=1
  fi
  for p in $pins; do
    if [[ "$p" == *"$ZERO" ]]; then
      echo "   FAIL pin is the bootstrap placeholder; run scripts/pin-tools.py"; fail=1
    fi
  done
  [[ $fail -eq 1 ]] && continue

  if [[ "$SKIP" != *",cosign,"* ]]; then
    # Images built by this repository are verified keyless against the CI workflow identity;
    # third-party images need their publisher's identity or key here.
    if [[ "$image" == ghcr.io/lum-sdeusch/enterprise-ai-control-plane/* ]]; then
      cosign verify "$image" \
        --certificate-identity-regexp '^https://github.com/lum-sdeusch/enterprise-ai-control-plane/\.github/workflows/pr\.yaml@refs/heads/main$' \
        --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null \
        && echo "   ok   cosign signature" || { echo "   FAIL cosign signature"; fail=1; }
    else
      echo "   warn third-party image: add its signing identity to scripts/tool-gate.sh (ADR 0025)"
    fi
  fi

  if [[ "$SKIP" != *",trivy,"* ]]; then
    trivy image --quiet --exit-code 1 --severity CRITICAL,HIGH --ignore-unfixed "$image" \
      && echo "   ok   trivy" || { echo "   FAIL trivy found CRITICAL/HIGH vulnerabilities"; fail=1; }
  fi

  if [[ "$SKIP" != *",mcpscan,"* ]]; then
    if [[ "$transport" == "http" ]]; then
      port=$(python3 -c "import yaml,sys; print(yaml.safe_load(open(sys.argv[1]))['spec']['http']['port'])" "$manifest")
      path=$(python3 -c "import yaml,sys; print(yaml.safe_load(open(sys.argv[1]))['spec']['http'].get('path','/mcp'))" "$manifest")
      cid=$(docker run -d --rm -p "127.0.0.1::${port}" "$image")
      trap 'docker stop "$cid" >/dev/null 2>&1 || true' EXIT
      hostport=$(docker port "$cid" "$port" | head -1)
      sleep 3
      # Cisco mcp-scanner (Apache-2.0): tool poisoning and prompt injection in descriptions.
      mcp-scanner --server-url "http://${hostport}${path}" --analyzers yara,llm --format summary \
        && echo "   ok   mcp-scanner" || { echo "   FAIL mcp-scanner flagged the server"; fail=1; }
      python3 scripts/pin-tools.py "tools/$name" --url "http://${hostport}${path}" --check \
        && echo "   ok   pins match the live server" || { echo "   FAIL pins differ from the live server"; fail=1; }
      docker stop "$cid" >/dev/null 2>&1 || true
      trap - EXIT
    else
      # TODO(verify): mcp-scanner invocation for stdio servers (run through kmcp's adapter or a
      # local supergateway and scan the resulting HTTP endpoint).
      echo "   warn stdio server: scan through a local Streamable HTTP wrapper before merging"
    fi
  fi
done

if [[ $fail -ne 0 ]]; then
  echo "tool-gate: FAILED"
  exit 1
fi
echo "tool-gate: ok"
