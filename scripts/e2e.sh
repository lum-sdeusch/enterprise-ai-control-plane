#!/usr/bin/env bash
# Run the six demo scenarios against a running cluster. Exits non-zero on any failure.
#
#   PROFILE=laptop scripts/e2e.sh
#   scripts/e2e.sh --smoke          # only the gateway and identity checks (ticket 03, make smoke)
#
# Status: skeleton. demo/scenarios/run.py implements the assertions; the two steps that need
# out-of-band actions (redeploying the poisoned server, approving as bob) are done here.
set -euo pipefail

PROFILE="${PROFILE:-laptop}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export EACP_GATEWAY_URL="${EACP_GATEWAY_URL:-http://gw.127.0.0.1.sslip.io:8080}"
if [[ "$PROFILE" == laptop ]]; then export EACP_CA_FILE="${EACP_CA_FILE:-$ROOT/.eacp/ca.pem}"; fi
export EACP_KEYCLOAK_TOKEN_URL="${EACP_KEYCLOAK_TOKEN_URL:-https://auth.127.0.0.1.sslip.io:8081/realms/eacp/protocol/openid-connect/token}"

if ! curl -fsS "$EACP_GATEWAY_URL/.well-known/oauth-protected-resource/mcp" >/dev/null; then
  echo "e2e: gateway not reachable at $EACP_GATEWAY_URL; run 'make up' first" >&2
  exit 1
fi

python3 -c "import httpx; from mcp.client.streamable_http import streamablehttp_client" 2>/dev/null || pip install -q "mcp>=1.19,<2" httpx
# The demo-runner client's secret lives in OpenBao (scripts/seed-openbao.sh).
EACP_RUNNER_CLIENT_SECRET="${EACP_RUNNER_CLIENT_SECRET:-$(scripts/seed-openbao.sh runner-secret)}"
export EACP_RUNNER_CLIENT_SECRET

# TODO: scenario 1 needs the poisoned server redeployed with POISON=1 between steps 1a and 1b
# (kubectl -n team-support set env deploy/poisoned-demo POISON=1), and scenario 5 needs bob's
# approval through the approval page between 5a and 5b (drive it with a headless browser or the
# adapter's form POST with bob's session). Until then run.py prints what to do and the two
# dependent assertions fail, which is the intended red.
exec python3 demo/scenarios/run.py "$@"
