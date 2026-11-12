#!/usr/bin/env bash
# Run the six demo scenarios against a running cluster. Exits non-zero on any failure.
#
#   PROFILE=laptop scripts/e2e.sh
#   scripts/e2e.sh --smoke          # only the gateway and identity checks (ticket 03, make smoke)
#   scripts/e2e.sh --pathway        # only the adapter-in-the-pathway checks (ticket 04, make pathway-cluster)
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

# `make up` returns once sync waves -1 and lower are healthy; the gateway routes, the adapter and
# the tools are wave 0 and may still be syncing. Wait for the Applications a check needs.
wait_apps() { # wait_apps APPLICATION...
  command -v kubectl >/dev/null && kubectl get namespace argocd >/dev/null 2>&1 || return 0
  local deadline=$((SECONDS + ${EACP_E2E_WAIT:-900})) pending="" app status announced=0
  while :; do
    pending=""
    for app in "$@"; do
      status="$(kubectl get application "$app" -n argocd \
        -o jsonpath='{.status.sync.status}/{.status.health.status}' 2>/dev/null || true)"
      [[ "$status" == Synced/Healthy ]] || pending+=" $app(${status:-missing})"
    done
    [[ -n "$pending" ]] || return 0
    if ((SECONDS >= deadline)); then
      echo "e2e: Applications not Synced and Healthy:$pending; run 'make diagnose'" >&2
      return 1
    fi
    ((announced)) || { echo "e2e: waiting for Argo CD Applications:$pending"; announced=1; }
    sleep 5
  done
}
if [[ " $* " == *" --smoke "* ]]; then wait_apps eacp-gateway eacp-system tool-tickets; fi
if [[ " $* " == *" --pathway "* ]]; then
  wait_apps eacp-gateway eacp-system tool-tickets telemetry-stores otel-collector jaeger
fi

if ! curl -fsS "$EACP_GATEWAY_URL/.well-known/oauth-protected-resource/mcp" >/dev/null; then
  echo "e2e: gateway not reachable at $EACP_GATEWAY_URL; run 'make up' first" >&2
  exit 1
fi

python3 -c "import httpx; from mcp.client.streamable_http import streamablehttp_client" 2>/dev/null || pip install -q "mcp>=1.19,<2" httpx
# The demo-runner client's secret lives in OpenBao (scripts/seed-openbao.sh).
EACP_RUNNER_CLIENT_SECRET="${EACP_RUNNER_CLIENT_SECRET:-$(scripts/seed-openbao.sh runner-secret)}"
export EACP_RUNNER_CLIENT_SECRET

# The pathway checks read audit rows as the read-only audit user.
if [[ " $* " == *" --pathway "* ]]; then
  EACP_AUDIT_READER_PASSWORD="${EACP_AUDIT_READER_PASSWORD:-$(scripts/seed-openbao.sh audit-reader-password)}"
  export EACP_AUDIT_READER_PASSWORD
fi

# TODO: scenario 1 needs the poisoned server redeployed with POISON=1 between steps 1a and 1b
# (kubectl -n team-support set env deploy/poisoned-demo POISON=1), and scenario 5 needs bob's
# approval through the approval page between 5a and 5b (drive it with a headless browser or the
# adapter's form POST with bob's session). Until then run.py prints what to do and the two
# dependent assertions fail, which is the intended red.
exec python3 demo/scenarios/run.py "$@"
