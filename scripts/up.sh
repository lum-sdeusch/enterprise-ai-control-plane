#!/usr/bin/env bash
# Bring up the control plane on a kind cluster (laptop) or an existing cluster (cloud-gpu).
#
#   PROFILE=laptop scripts/up.sh
#
# Status: skeleton. The order and the components are final; the steps marked TODO need the
# build-time verifications listed in docs/design.md section 11 before they run unattended.
set -euo pipefail

PROFILE="${PROFILE:-laptop}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export PATH="$ROOT/bin:$PATH"

need() { command -v "$1" >/dev/null || { echo "missing: $1"; exit 1; }; }
need kubectl; need helm; need docker

step() { printf '\n==> %s\n' "$*"; }

if [[ "$PROFILE" == "laptop" ]]; then
  need kind
  step "kind cluster"
  kind get clusters | grep -qx eacp || kind create cluster --config scripts/kind-config.yaml
  kubectl config use-context kind-eacp
fi

step "Argo CD"
kubectl get ns argocd >/dev/null 2>&1 || kubectl create ns argocd
helm repo add argo https://argoproj.github.io/argo-helm >/dev/null 2>&1 || true
helm upgrade --install argocd argo/argo-cd --namespace argocd --version 8.5.0 \
  --set configs.params."server\.insecure"=true \
  --set server.service.type=NodePort --set server.service.nodePortHttp=30082 \
  --wait --timeout 10m   # TODO(verify) chart version

step "secrets manager seed (laptop only: dev-mode OpenBao with demo values)"
if [[ "$PROFILE" == "laptop" ]]; then
  # TODO: after the openbao Application syncs, initialise/unseal with a static key for the demo
  # and write: eacp/gateway/anthropic (Authorization=Bearer $ANTHROPIC_API_KEY),
  # eacp/tools/customer-db (DSN), eacp/gateway/oauth-client, eacp/adapter/oauth-client,
  # eacp/audit/writer, eacp/telemetry/clickhouse, eacp/models/hf-token. See scripts/seed-openbao.sh.
  : "${ANTHROPIC_API_KEY:?set ANTHROPIC_API_KEY for the external model}"
fi

step "root application for profile $PROFILE"
kubectl apply -f "platform/argocd/root-${PROFILE}.yaml"

step "waiting for the platform to sync (this pulls the model weights on first run)"
# TODO: poll `kubectl get applications -n argocd` until every app is Synced/Healthy, then
# patch the agentgateway Service to NodePort 30080 and Keycloak to 30081 on the laptop profile.

cat <<EOF

Gateway     http://gw.127.0.0.1.sslip.io:8080      (MCP at /mcp, LLM at /v1, approvals at /approvals)
Keycloak    http://auth.127.0.0.1.sslip.io:8081   (realm eacp; alice/bob with temporary passwords)
LibreChat   http://chat.127.0.0.1.sslip.io:3080
Argo CD     http://127.0.0.1:8082
Langfuse    http://127.0.0.1:8083
Jaeger      http://127.0.0.1:8084
Mailpit     http://127.0.0.1:8085

Next: make demo
EOF
