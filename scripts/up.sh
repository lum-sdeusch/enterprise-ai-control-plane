#!/usr/bin/env bash
# Bring up the control plane on a kind cluster (laptop) or an existing cluster (cloud-gpu).
#
#   PROFILE=laptop scripts/up.sh                       # what `make up` runs
#   EACP_REVISION=my-branch scripts/up.sh              # sync every in-repo source from a branch
#   EACP_REPO=https://github.com/me/fork.git scripts/up.sh
#
# Safe to run again: every step checks what exists before it creates anything. It returns once
# every Application in the foundation and platform-store waves (sync wave -1 and lower) is Synced
# and Healthy. Wave 0 and later (the eacp charts, models, tools, demo) keep syncing afterwards.
set -euo pipefail

PROFILE="${PROFILE:-laptop}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export PATH="$ROOT/bin:$PATH"

DEFAULT_REPO=https://github.com/lum-sdeusch/enterprise-ai-control-plane.git
EACP_REPO="${EACP_REPO:-$DEFAULT_REPO}"
EACP_REVISION="${EACP_REVISION:-main}"
ARGOCD_CHART_VERSION=10.15.0   # Argo CD v3.6.1
WAIT_TIMEOUT="${EACP_WAIT_TIMEOUT:-2400}"   # seconds for the platform to settle

need() { command -v "$1" >/dev/null || { echo "missing: $1 (see the quick start in README.md)"; exit 1; }; }
step() { printf '\n==> %s\n' "$*"; }

kind_cluster() {
  step "kind cluster"
  if kind get clusters 2>/dev/null | grep -qx eacp; then
    echo "    cluster eacp exists"
  else
    kind create cluster --config scripts/kind-config.yaml
  fi
  kubectl config use-context kind-eacp >/dev/null
}

# Resolve the laptop hostnames inside the cluster too, so a pod reaches Keycloak and the gateway
# at the same URLs (and Keycloak issuer) as the browser.
coredns_rewrites() {
  step "CoreDNS: *.127.0.0.1.sslip.io inside the cluster"
  local corefile
  corefile="$(kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}')"
  if grep -q 'auth.127.0.0.1.sslip.io' <<<"$corefile"; then echo "    already set"; return; fi
  corefile="$(awk '{print} /^ *ready$/ {
    print "    rewrite name exact auth.127.0.0.1.sslip.io keycloak-nodeport.keycloak.svc.cluster.local answer auto"
    print "    rewrite name exact gw.127.0.0.1.sslip.io agentgateway-proxy.agentgateway-system.svc.cluster.local answer auto"
  }' <<<"$corefile")"
  kubectl create configmap coredns -n kube-system --from-literal=Corefile="$corefile" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  kubectl rollout restart deployment/coredns -n kube-system >/dev/null
  echo "    auth.127.0.0.1.sslip.io -> keycloak-nodeport, gw.127.0.0.1.sslip.io -> agentgateway-proxy"
}

argocd() {
  step "Argo CD (chart $ARGOCD_CHART_VERSION)"
  helm upgrade --install argocd argo-cd --repo https://argoproj.github.io/argo-helm \
    --version "$ARGOCD_CHART_VERSION" --namespace argocd --create-namespace \
    -f platform/argocd/install-values.yaml --wait --timeout 10m >/dev/null
  echo "    installed"
}

# OpenBao auto-unseals from this key. Created once and never replaced: a new key would make the
# existing storage unreadable. Laptop demo only (ADR 0030; ticket 14 moves this to a KMS).
openbao_seal_key() {
  step "OpenBao static seal key"
  kubectl get namespace openbao >/dev/null 2>&1 || kubectl create namespace openbao >/dev/null
  if kubectl get secret openbao-static-seal -n openbao >/dev/null 2>&1; then
    echo "    exists"
  else
    kubectl create secret generic openbao-static-seal -n openbao \
      --from-literal=key="$(openssl rand -hex 32)" >/dev/null
    echo "    created"
  fi
}

# What was installed and how, for whoever debugs this cluster later.
record_bringup() {
  local kmcp_version
  kmcp_version="$(grep -A1 '^    chart: kmcp$' platform/argocd/apps/00-foundation.yaml |
    awk '/targetRevision/ {print $2}')"
  kubectl create configmap eacp-bringup -n argocd \
    --from-literal=profile="$PROFILE" \
    --from-literal=repo="$EACP_REPO" \
    --from-literal=revision="$EACP_REVISION" \
    --from-literal=argocd-chart="$ARGOCD_CHART_VERSION" \
    --from-literal=kmcp-install="helm chart oci://ghcr.io/kagent-dev/kmcp/helm/{kmcp-crds,kmcp}:$kmcp_version via Argo CD (not the kmcp CLI)" \
    --from-literal=last-run="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  echo "    kmcp: published Helm chart $kmcp_version, recorded in configmap argocd/eacp-bringup"
}

# Kustomize patches for the root application that point every in-repo source (Applications
# labelled eacp.dev/source, and the tools ApplicationSet) at EACP_REPO and EACP_REVISION.
retarget_patches() {
  local r="$EACP_REPO" v="$EACP_REVISION"
  cat <<EOF
    kustomize:
      patches:
        - target: { kind: Application, labelSelector: eacp.dev/source=repo }
          patch: |
            - { op: replace, path: /spec/source/repoURL, value: "$r" }
            - { op: replace, path: /spec/source/targetRevision, value: "$v" }
        - target: { kind: Application, labelSelector: eacp.dev/source=repo-values }
          patch: |
            - { op: replace, path: /spec/sources/1/repoURL, value: "$r" }
            - { op: replace, path: /spec/sources/1/targetRevision, value: "$v" }
        - target: { kind: ApplicationSet, name: eacp-tools }
          patch: |
            - { op: replace, path: /spec/generators/0/git/repoURL, value: "$r" }
            - { op: replace, path: /spec/generators/0/git/revision, value: "$v" }
            - { op: replace, path: /spec/template/spec/sources/0/repoURL, value: "$r" }
            - { op: replace, path: /spec/template/spec/sources/0/targetRevision, value: "$v" }
            - { op: replace, path: /spec/template/spec/sources/1/repoURL, value: "$r" }
            - { op: replace, path: /spec/template/spec/sources/1/targetRevision, value: "$v" }
EOF
}

root_app() {
  local file="platform/argocd/root-${PROFILE}.yaml"
  step "root application ($file, $EACP_REPO @ $EACP_REVISION)"
  if [[ "$EACP_REPO" == "$DEFAULT_REPO" && "$EACP_REVISION" == main ]]; then
    kubectl apply -f "$file" >/dev/null
  else
    local patches
    patches="$(mktemp)"
    retarget_patches >"$patches"
    sed -e "s#repoURL: $DEFAULT_REPO#repoURL: $EACP_REPO#" \
        -e "s#targetRevision: main#targetRevision: $EACP_REVISION#" \
        -e "\#^    path: platform/argocd/profiles/#r $patches" "$file" |
      kubectl apply -f - >/dev/null
    rm -f "$patches"
  fi
  echo "    applied"
  record_bringup
}

wait_for() { # wait_for DESCRIPTION COMMAND...: retry COMMAND every 5s until it succeeds
  local what="$1" deadline=$((SECONDS + WAIT_TIMEOUT))
  shift
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { echo "    timed out waiting for $what"; return 1; }
    sleep 5
  done
}

openbao_ready() { kubectl wait --for=condition=Ready pod/openbao-0 -n openbao --timeout=5s; }
realm_imported() { kubectl wait --for=condition=Done keycloakrealmimport/eacp -n keycloak --timeout=5s; }

seed() {
  [[ "$PROFILE" == laptop ]] || return 0
  step "OpenBao: demo secrets (scripts/seed-openbao.sh)"
  wait_for "OpenBao to unseal" openbao_ready
  scripts/seed-openbao.sh secrets
  step "Keycloak: realm import and client secrets"
  wait_for "the eacp realm import" realm_imported
  wait_for "Keycloak on host port 8081" curl -sf http://127.0.0.1:8081/realms/eacp
  scripts/seed-openbao.sh keycloak
}

# Applications in the foundation and platform-store waves, from the profile in this checkout.
store_apps() {
  kubectl kustomize "platform/argocd/profiles/$PROFILE" |
    awk 'function flush() { if (kind == "Application" && wave <= -1) print name; kind = ""; name = ""; wave = 0 }
         /^---/ {flush(); next}
         /^kind: / {kind = $2}
         /^  name: / {name = $2}
         /argocd.argoproj.io\/sync-wave:/ {gsub(/"/, "", $2); wave = $2 + 0}
         END {flush()}'
}

apps_settled() {
  local status app bad=0
  status="$(kubectl get applications -n argocd -o json |
    jq -r '.items[] | "\(.metadata.name) \(.status.sync.status // "-") \(.status.health.status // "-")"')"
  for app in $(store_apps); do
    grep -qx "$app Synced Healthy" <<<"$status" || bad=1
  done
  return $bad
}

wait_platform() {
  step "waiting for the foundation and platform-store Applications (up to ${WAIT_TIMEOUT}s)"
  if ! wait_for "the platform" apps_settled; then
    kubectl get applications -n argocd
    exit 1
  fi
  local app
  for app in $(store_apps); do printf '    %-22s Synced Healthy\n' "$app"; done
}

summary() {
  cat <<EOF

Gateway     http://gw.127.0.0.1.sslip.io:8080      (MCP at /mcp, LLM at /v1, approvals at /approvals)
Keycloak    http://auth.127.0.0.1.sslip.io:8081   (realm eacp; alice, bob, carol: password changeme-<name>, temporary)
LibreChat   http://chat.127.0.0.1.sslip.io:3080
Argo CD     http://127.0.0.1:8082                 (user admin, password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)
Langfuse    http://127.0.0.1:8083
Jaeger      http://127.0.0.1:8084
Mailpit     http://127.0.0.1:8085

Next: make demo
EOF
}

main() {
  need kubectl; need helm; need jq; need openssl; need curl
  if [[ "$PROFILE" == laptop ]]; then
    need docker; need kind
    kind_cluster
    coredns_rewrites
  fi
  argocd
  if [[ "$PROFILE" == laptop ]]; then openbao_seal_key; fi
  root_app
  seed
  wait_platform
  summary
}

main "$@"
