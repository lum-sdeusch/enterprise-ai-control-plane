#!/usr/bin/env bash
# Bring up the control plane on a kind cluster (laptop) or an existing cluster (cloud-gpu).
#
#   PROFILE=laptop scripts/up.sh                       # what `make up` runs
#   EACP_REVISION=my-branch scripts/up.sh              # sync every in-repo source from a branch
#   EACP_REPO=https://github.com/me/fork.git scripts/up.sh
#   scripts/up.sh policy                               # laptop: reload the policy bundle only
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
# The laptop CA that signs Keycloak's certificate (platform/keycloak/tls.yaml), exported here for
# curl, the scenario runner and Claude Code (NODE_EXTRA_CA_CERTS).
export EACP_CA_FILE="${EACP_CA_FILE:-$ROOT/.eacp/ca.pem}"

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
COREDNS_REWRITES=(
  "rewrite name exact auth.127.0.0.1.sslip.io keycloak-nodeport.keycloak.svc.cluster.local answer auto"
  "rewrite name exact gw.127.0.0.1.sslip.io agentgateway-proxy.agentgateway-system.svc.cluster.local answer auto"
)

coredns_rewrites() {
  step "CoreDNS: *.127.0.0.1.sslip.io inside the cluster"
  local corefile missing=() rule
  corefile="$(kubectl get configmap coredns -n kube-system -o jsonpath='{.data.Corefile}')"
  for rule in "${COREDNS_REWRITES[@]}"; do
    grep -qF "$rule" <<<"$corefile" || missing+=("$rule")
  done
  if ((${#missing[@]} == 0)); then echo "    already set"; return; fi
  # The rules go through the environment: BSD awk (macOS) rejects a newline in an -v value.
  corefile="$(ADD="$(printf '    %s\n' "${missing[@]}")" \
    awk '{print} /^ *ready$/ {print ENVIRON["ADD"]}' <<<"$corefile")"
  kubectl patch configmap coredns -n kube-system --type merge \
    -p "$(jq -n --arg c "$corefile" '{data: {Corefile: $c}}')" >/dev/null
  kubectl rollout restart deployment/coredns -n kube-system >/dev/null
  printf '    added: %s\n' "${missing[@]}"
}

# Images built on the laptop and loaded into kind (scripts/local-images.sh): the demo servers as
# "<tool> <image>@<digest>" lines, and the adapter and guardrail service as
# "adapter <image>@<digest>" and "guardrails <image>@<digest>".
LOCAL_IMAGES=""
PLATFORM_IMAGES=""
# A hash of the policy sources; a new value makes the eacp-system chart roll the adapter pods,
# because OPA reads the mounted bundle only at start.
POLICY_CHECKSUM=""

demo_images() {
  step "demo MCP server images (built locally, loaded into kind)"
  LOCAL_IMAGES="$(scripts/local-images.sh tools)"
}

platform_images() {
  step "adapter and guardrail service images (built locally, loaded into kind)"
  PLATFORM_IMAGES="$(scripts/local-images.sh platform)"
}

# The laptop gets its policy without a registry: build the bundle from this checkout and put the
# tarball in ConfigMap eacp-system/eacp-policy-bundle, which the adapter's OPA sidecar mounts
# (platform/charts/eacp-system, policy.source: configmap). Rerun after changing a policy or a
# manifest: `make policy-load`.
policy_bundle() {
  step "policy bundle (built from this checkout, loaded as a ConfigMap)"
  local rev
  rev="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
  git diff --quiet HEAD -- policies tools agents teams 2>/dev/null || rev="$rev-dirty"
  scripts/build-bundle.sh "$rev" >/dev/null
  POLICY_CHECKSUM="$(find policies/rego policies/eacp -type f | LC_ALL=C sort | xargs cat | openssl dgst -sha256 | awk '{print substr($NF, 1, 16)}')"
  kubectl get namespace eacp-system >/dev/null 2>&1 || kubectl create namespace eacp-system >/dev/null
  kubectl create configmap eacp-policy-bundle -n eacp-system \
    --from-file=eacp-bundle.tar.gz=dist/eacp-bundle.tar.gz --dry-run=client -o yaml |
    kubectl apply -f - >/dev/null
  kubectl label configmap eacp-policy-bundle -n eacp-system --overwrite \
    app.kubernetes.io/part-of=eacp eacp.dev/policy-revision="$rev" >/dev/null
  echo "    revision $rev, checksum $POLICY_CHECKSUM"
}

install_argocd() {
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

# Kustomize patch that hands the locally built demo images to every tool Application.
local_image_patch() {
  local map
  map="$(jq -Rnc '[inputs | select(length > 0) | split(" ") | {(.[0]): .[1]}] | add // {}' <<<"$LOCAL_IMAGES")"
  cat <<EOF
        - target: { kind: ApplicationSet, name: eacp-tools }
          patch: |
            - { op: add, path: /spec/template/spec/sources/0/helm/valuesObject/platform/localImages, value: $map }
EOF
}

# Kustomize patch that hands the locally built adapter and guardrail images, and the policy
# checksum, to the eacp-system chart.
platform_patch() {
  local values
  values="$(jq -Rnc --arg checksum "$POLICY_CHECKSUM" \
    '[inputs | select(length > 0) | split(" ") | {(.[0]): .[1]}] | add // {} | {images: ., policy: {bundleChecksum: $checksum}}' <<<"$PLATFORM_IMAGES")"
  cat <<EOF
        - target: { kind: Application, name: eacp-system }
          patch: |
            - { op: add, path: /spec/source/helm/valuesObject, value: $values }
EOF
}

# Every patch the root application needs; prints nothing when it needs none.
root_patches() {
  local retarget=0
  [[ "$EACP_REPO" == "$DEFAULT_REPO" && "$EACP_REVISION" == main ]] || retarget=1
  ((retarget)) || [[ -n "$LOCAL_IMAGES$PLATFORM_IMAGES$POLICY_CHECKSUM" ]] || return 0
  printf '    kustomize:\n      patches:\n'
  ((retarget == 0)) || retarget_patches
  [[ -z "$LOCAL_IMAGES" ]] || local_image_patch
  [[ -z "$PLATFORM_IMAGES$POLICY_CHECKSUM" ]] || platform_patch
}

root_app() {
  local file="platform/argocd/root-${PROFILE}.yaml"
  step "root application ($file, $EACP_REPO @ $EACP_REVISION)"
  local patches
  patches="$(mktemp)"
  root_patches >"$patches"
  if [[ ! -s "$patches" ]]; then
    kubectl apply -f "$file" >/dev/null
  else
    sed -e "s#repoURL: $DEFAULT_REPO#repoURL: $EACP_REPO#" \
        -e "s#targetRevision: main#targetRevision: $EACP_REVISION#" \
        -e "\#^    path: platform/argocd/profiles/#r $patches" "$file" |
      kubectl apply -f - >/dev/null
  fi
  rm -f "$patches"
  echo "    applied"
  record_bringup
}

wait_for() { # wait_for DESCRIPTION COMMAND...: retry COMMAND every 5s until it succeeds
  local what="$1" deadline=$((SECONDS + WAIT_TIMEOUT))
  shift
  until "$@" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { echo "    timed out waiting for $what"; return 1; }
    resync_root
    sleep 5
  done
}

# The controller stops a sync after 15 minutes (it happens when the laptop sleeps mid bring-up).
# Argo CD neither retries that sync nor auto-syncs the same commit again, so the later waves would
# stay unapplied; while up.sh waits, it starts the root sync again.
resync_root() {
  local message
  message="$(kubectl get application eacp-root -n argocd \
    -o jsonpath='{.status.operationState.phase} {.status.operationState.message}' 2>/dev/null)" || return 0
  [[ "$message" == "Failed Operation terminated, triggered by controller sync timeout" ]] || return 0
  kubectl patch application eacp-root -n argocd --type merge -p \
    '{"operation":{"initiatedBy":{"username":"make-up"},"sync":{"prune":true,"syncOptions":["CreateNamespace=true"]}}}' \
    >/dev/null && echo "    restarted the root sync after the controller's sync timeout"
}

openbao_ready() { kubectl wait --for=condition=Ready pod/openbao-0 -n openbao --timeout=5s; }
realm_imported() { kubectl wait --for=condition=Done keycloakrealmimport/eacp -n keycloak --timeout=5s; }
ca_issued() { kubectl get secret eacp-local-ca -n keycloak -o jsonpath='{.data.ca\.crt}' | grep -q .; }

export_ca() {
  wait_for "the laptop CA (cert-manager)" ca_issued
  mkdir -p "$(dirname "$EACP_CA_FILE")"
  kubectl get secret eacp-local-ca -n keycloak -o jsonpath='{.data.ca\.crt}' | base64 -d >"$EACP_CA_FILE"
  echo "    laptop CA written to $EACP_CA_FILE"
}

seed() {
  [[ "$PROFILE" == laptop ]] || return 0
  step "OpenBao: demo secrets (scripts/seed-openbao.sh)"
  wait_for "OpenBao to unseal" openbao_ready
  scripts/seed-openbao.sh secrets
  step "Keycloak: realm import and client secrets"
  wait_for "the eacp realm import" realm_imported
  export_ca
  wait_for "Keycloak on host port 8081" curl -sf --cacert "$EACP_CA_FILE" https://auth.127.0.0.1.sslip.io:8081/realms/eacp
  scripts/seed-openbao.sh keycloak
}

# Applications in the foundation and platform-store waves, from the profile in this checkout.
# Filled once by load_store_apps; an empty list means the render failed, not that nothing is due.
STORE_APPS=()

load_store_apps() {
  local rendered
  rendered="$(kubectl kustomize "platform/argocd/profiles/$PROFILE")" ||
    { echo "    cannot render platform/argocd/profiles/$PROFILE"; exit 1; }
  # shellcheck disable=SC2207 # Application names have no spaces.
  STORE_APPS=($(awk 'function flush() { if (kind == "Application" && wave <= -1) print name; kind = ""; name = ""; wave = 0 }
         /^---/ {flush(); next}
         /^kind: / {kind = $2}
         /^  name: / {name = $2}
         /argocd.argoproj.io\/sync-wave:/ {gsub(/"/, "", $2); wave = $2 + 0}
         END {flush()}' <<<"$rendered"))
  ((${#STORE_APPS[@]} > 0)) || { echo "    no Applications in waves <= -1 for profile $PROFILE"; exit 1; }
}

apps_settled() {
  local status app
  status="$(kubectl get applications -n argocd -o json |
    jq -r '.items[] | "\(.metadata.name) \(.status.sync.status // "-") \(.status.health.status // "-")"')" || return 1
  for app in "${STORE_APPS[@]}"; do
    grep -qx "$app Synced Healthy" <<<"$status" || return 1
  done
}

wait_platform() {
  load_store_apps
  step "waiting for the ${#STORE_APPS[@]} foundation and platform-store Applications (up to ${WAIT_TIMEOUT}s)"
  if ! wait_for "the platform" apps_settled; then
    kubectl get applications -n argocd
    exit 1
  fi
  local app
  for app in "${STORE_APPS[@]}"; do printf '    %-22s Synced Healthy\n' "$app"; done
}

summary() {
  cat <<EOF

Gateway     http://gw.127.0.0.1.sslip.io:8080      (MCP at /mcp, LLM at /v1, approvals at /approvals)
Keycloak    https://auth.127.0.0.1.sslip.io:8081  (realm eacp; demo users alice, bob, carol, erin with password changeme-<name>)
            signed by the laptop CA in $EACP_CA_FILE; for Claude Code: export NODE_EXTRA_CA_CERTS=$EACP_CA_FILE
LibreChat   http://chat.127.0.0.1.sslip.io:3080
Argo CD     http://127.0.0.1:8082                 (user admin, password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d)
Langfuse    http://127.0.0.1:8083
Jaeger      http://127.0.0.1:8084
Mailpit     http://127.0.0.1:8085

Next: make demo
EOF
}

# `make policy-load`: rebuild the bundle from this checkout, replace the ConfigMap and restart the
# adapter pods so OPA loads it. The pods are deleted rather than the Deployment patched, so Argo
# CD sees no drift to heal.
policy_load() {
  need kubectl; need opa; need openssl
  policy_bundle
  kubectl delete pod -n eacp-system -l app.kubernetes.io/name=eacp-adapter --wait=false >/dev/null 2>&1 || true
  kubectl rollout status deployment/eacp-adapter -n eacp-system --timeout=180s
}

main() {
  if [[ "${1:-}" == policy ]]; then policy_load; return; fi
  need kubectl; need helm; need jq; need openssl; need curl
  if [[ "$PROFILE" == laptop ]]; then
    need docker; need kind
    need opa
    kind_cluster
    coredns_rewrites
    demo_images
    platform_images
    policy_bundle
  fi
  install_argocd
  if [[ "$PROFILE" == laptop ]]; then openbao_seal_key; fi
  root_app
  seed
  wait_platform
  summary
}

main "$@"
