#!/usr/bin/env bash
# Collect the state of the laptop cluster into one file, for when `make up`, `make smoke` or
# `make pathway-cluster` does not do what it should: `make diagnose`.
#
# Read-only. Prints status, events and log tails; never Secret contents or environment values.
# Output goes to the terminal and to .eacp/diagnose.txt.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
export PATH="$ROOT/bin:$PATH"
OUT="$ROOT/.eacp/diagnose.txt"
mkdir -p "$(dirname "$OUT")"
NODE="${EACP_KIND_NODE:-eacp-control-plane}"
CA="${EACP_CA_FILE:-$ROOT/.eacp/ca.pem}"

section() { printf '\n===== %s =====\n' "$*"; }
run() { printf '$ %s\n' "$*"; "$@" 2>&1 | tail -n "${LINES_MAX:-60}"; }
logs() { # logs NAMESPACE SELECTOR [CONTAINER]
  printf '$ logs %s %s %s\n' "$1" "$2" "${3:-}"
  kubectl logs -n "$1" -l "$2" ${3:+-c "$3"} --tail=40 --prefix 2>&1 | cut -c1-400
  kubectl logs -n "$1" -l "$2" ${3:+-c "$3"} --tail=15 --prefix --previous 2>/dev/null | cut -c1-400 | sed 's/^/(previous) /'
}
probe() { # probe NAME URL [curl options]: status code, server header and the first bytes
  local name="$1" url="$2"; shift 2
  printf '%-22s %s\n' "$name" "$(curl -sS -m 8 -o "$ROOT/.eacp/diagnose.body" -w '%{http_code}' "$@" "$url" 2>&1 | tail -1)  $(head -c 120 "$ROOT/.eacp/diagnose.body" 2>/dev/null | tr '\n' ' ')"
}

{
  section "checkout and tools"
  echo "branch $(git rev-parse --abbrev-ref HEAD) commit $(git rev-parse --short HEAD) $(git diff --quiet HEAD && echo clean || echo dirty)"
  echo "context $(kubectl config current-context 2>&1)"
  for t in kind kubectl helm opa go python3 docker jq; do printf '%-8s %s\n' "$t" "$(command -v "$t" || echo MISSING)"; done
  python3 -c "import httpx, mcp; print('python: httpx and mcp import')" 2>&1 | tail -1
  grep -q "^pathway-cluster:" Makefile && echo "Makefile has pathway-cluster" || echo "Makefile has NO pathway-cluster target (this checkout predates ticket 04)"

  section "host ports (8080 gateway, 8081 Keycloak, 8082 Argo CD, 8084 Jaeger)"
  probe "gateway :8080" http://gw.127.0.0.1.sslip.io:8080/.well-known/oauth-protected-resource/mcp
  probe "localhost:8080 /" http://localhost:8080/
  probe "keycloak :8081" https://auth.127.0.0.1.sslip.io:8081/realms/eacp --cacert "$CA"
  probe "argocd :8082" http://127.0.0.1:8082/
  probe "jaeger :8084" http://127.0.0.1:8084/api/services
  run docker ps --format '{{.Names}}  {{.Ports}}' --filter "name=$NODE"

  section "Argo CD applications"
  kubectl get applications -n argocd -o json 2>&1 | jq -r '
    .items[] | [.metadata.name, (.status.sync.status // "-"), (.status.health.status // "-"),
                (.status.sync.revision // "-" | tostring | .[0:10]), (.status.operationState.phase // "-")] | @tsv' 2>&1 | { command -v column >/dev/null && column -t || cat; }
  section "Argo CD applications that are not Synced and Healthy"
  kubectl get applications -n argocd -o json 2>/dev/null | jq -r '
    .items[] | select(.status.sync.status != "Synced" or .status.health.status != "Healthy") |
    "--- \(.metadata.name) (target \(.spec.source.targetRevision // "-"))",
    "operation: \((.status.operationState.message // "-") | .[0:600])",
    (.status.conditions // [] | .[] | "condition: \(.type): \(.message | .[0:600])"),
    (.status.resources // [] | .[] | select(.status != "Synced" or ((.health.status // "Healthy") != "Healthy")) |
      "resource: \(.kind) \(.namespace // "")/\(.name) sync=\(.status) health=\(.health.status // "-") \((.health.message // "") | .[0:200])")'

  section "pods that are not Running and Ready"
  kubectl get pods -A --no-headers 2>&1 | awk '$4 != "Completed" { split($3, r, "/"); if ($4 != "Running" || r[1] != r[2]) print }'
  for ns in eacp-system agentgateway-system team-support telemetry; do
    section "namespace $ns"
    run kubectl get pods -n "$ns" -o wide
    LINES_MAX=25 run kubectl get events -n "$ns" --sort-by=.lastTimestamp
  done

  section "locally built images on the kind node, and what the workloads ask for"
  run docker exec "$NODE" crictl images
  kubectl get deploy -n eacp-system -o json 2>&1 | jq -r '.items[] | .metadata.name as $d | .spec.template.spec.containers[] | "\($d)/\(.name)  \(.image)"'
  kubectl get deploy,statefulset -n team-support -o json 2>&1 | jq -r '.items[] | .metadata.name as $d | .spec.template.spec.containers[] | "\($d)/\(.name)  \(.image)"'

  section "policy bundle ConfigMap"
  run kubectl get configmap eacp-policy-bundle -n eacp-system --show-labels

  section "gateway objects"
  kubectl get agentgatewaypolicy -A -o json 2>&1 | jq -r '
    .items[] | "\(.metadata.namespace)/\(.metadata.name)",
    (.status.ancestors // [] | .[] | .conditions // [] | .[] | "    \(.type)=\(.status) \(.reason): \((.message // "") | .[0:300])")'
  run kubectl get httproute,referencegrant,agentgatewaybackend -A
  kubectl get httproute -A -o json 2>&1 | jq -r '
    .items[] | "\(.metadata.namespace)/\(.metadata.name)",
    (.status.parents // [] | .[] | .conditions // [] | .[] | select(.status != "True") | "    \(.type)=\(.status) \(.reason): \((.message // "") | .[0:300])")'

  section "secrets delivery and admission"
  kubectl get externalsecret -A --no-headers 2>&1 | awk '$NF != "True" && $(NF-1) != "True"'
  LINES_MAX=20 run kubectl get events -A --field-selector reason=PolicyViolation --sort-by=.lastTimestamp

  section "logs"
  logs eacp-system app.kubernetes.io/name=eacp-adapter adapter
  logs eacp-system app.kubernetes.io/name=eacp-adapter opa
  logs eacp-system app.kubernetes.io/name=eacp-guardrails
  printf '$ logs agentgateway-system (all pods)\n'
  for p in $(kubectl get pods -n agentgateway-system -o name 2>/dev/null); do
    kubectl logs -n agentgateway-system "$p" --tail=30 --prefix 2>&1 | cut -c1-400
  done
  printf '$ logs telemetry/clickhouse-0\n'
  kubectl logs -n telemetry clickhouse-0 --tail=20 2>&1 | cut -c1-300
} 2>&1 | tee "$OUT"

printf '\nwritten to %s\n' "$OUT"
