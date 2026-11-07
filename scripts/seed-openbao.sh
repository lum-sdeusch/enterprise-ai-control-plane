#!/usr/bin/env bash
# Seed OpenBao with the demo secrets and hand the generated OAuth client secrets to Keycloak.
#
#   scripts/seed-openbao.sh secrets    # write eacp/* into OpenBao (needs OpenBao unsealed)
#   scripts/seed-openbao.sh keycloak   # set the eacp-gateway and eacp-adapter client secrets
#   scripts/seed-openbao.sh            # both
#
# OpenBao initialises and unseals itself (static seal plus self-initialisation, see the openbao
# Application in platform/argocd/apps/00-foundation.yaml), so there is no root token. This script
# requests a ten-minute token for the openbao-seeder ServiceAccount, logs in with it through the
# Kubernetes auth method, and gets an OpenBao token that can only write eacp/*.
#
# Idempotent: generated values are created once and kept; a second run changes nothing unless
# ANTHROPIC_API_KEY is set, which always overwrites eacp/gateway/anthropic. Secret values travel
# on stdin, never on a command line.
set -euo pipefail

NS=openbao
POD=openbao-0
MOUNT=secret
KC_URL="${EACP_KEYCLOAK_URL:-http://127.0.0.1:8081}"
KC_REALM=eacp
TOKEN=""

log() { printf '    %s\n' "$*"; }
rand() { openssl rand -hex 24; }

# Runs a bao command in the OpenBao pod. stdin: the OpenBao token on line 1, then the payload.
bao_in_pod() {
  kubectl exec -i -n "$NS" "$POD" -- sh -c \
    'read -r BAO_TOKEN; export BAO_TOKEN BAO_ADDR=http://127.0.0.1:8200; '"$1"
}

login() {
  local deadline=$((SECONDS + ${EACP_SEED_WAIT:-900}))
  until kubectl get serviceaccount openbao-seeder -n "$NS" >/dev/null 2>&1; do
    ((SECONDS < deadline)) || { echo "ServiceAccount $NS/openbao-seeder never appeared (secret-store app)"; exit 1; }
    sleep 5
  done
  TOKEN="$(kubectl create token openbao-seeder -n "$NS" --duration=10m |
    kubectl exec -i -n "$NS" "$POD" -- sh -c \
      'BAO_ADDR=http://127.0.0.1:8200 bao write -field=token auth/kubernetes/login role=seeder jwt=-')"
  [[ -n "$TOKEN" ]] || { echo "OpenBao login as openbao-seeder failed"; exit 1; }
}

exists() { printf '%s\n' "$TOKEN" | bao_in_pod "bao kv get -mount=$MOUNT $1 >/dev/null 2>&1" 2>/dev/null; }

field() { printf '%s\n' "$TOKEN" | bao_in_pod "bao kv get -mount=$MOUNT -field=$2 $1"; }

put() { # put PATH JSON
  { printf '%s\n' "$TOKEN"; printf '%s' "$2"; } | bao_in_pod "bao kv put -mount=$MOUNT $1 - >/dev/null"
  log "wrote $1"
}

put_once() { # put_once PATH JSON: write only if PATH does not exist yet
  if exists "$1"; then log "kept  $1"; else put "$1" "$2"; fi
}

seed_anthropic() {
  local path=eacp/gateway/anthropic
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    put "$path" "$(jq -n --arg v "Bearer $ANTHROPIC_API_KEY" '{Authorization: $v}')"
  elif exists "$path"; then
    log "kept  $path (ANTHROPIC_API_KEY not set)"
  else
    put "$path" '{"Authorization":"Bearer unset-export-ANTHROPIC_API_KEY-and-rerun-seed"}'
    log "WARNING: ANTHROPIC_API_KEY is not set; the external model will answer 401 until you"
    log "         export it and run scripts/seed-openbao.sh secrets again."
  fi
}

seed_clickhouse() {
  put_once eacp/telemetry/clickhouse "$(jq -n --arg a "$(rand)" --arg r "$(rand)" --arg l "$(rand)" \
    '{"audit-writer-password": $a, "audit-reader-password": $r, "logs-writer-password": $l}')"
  # The adapter and the collector read their own copies; keep them equal to the store's users.
  put_once eacp/audit/writer "$(jq -n --arg p "$(field eacp/telemetry/clickhouse audit-writer-password)" \
    '{username: "eacp_audit_writer", password: $p}')"
  put_once eacp/telemetry/logs-writer "$(jq -n --arg p "$(field eacp/telemetry/clickhouse logs-writer-password)" \
    '{username: "logs_writer", password: $p}')"
}

seed_secrets() {
  login
  seed_anthropic
  seed_clickhouse
  put_once eacp/gateway/oauth-client "$(jq -n --arg s "$(rand)" '{"client-id": "eacp-gateway", "client-secret": $s}')"
  put_once eacp/adapter/oauth-client "$(jq -n --arg s "$(rand)" '{"client-id": "eacp-adapter", "client-secret": $s}')"
  # Matches the read-only role demo/k8s/customers-db.yaml creates.
  put_once eacp/tools/customer-db \
    '{"DSN":"postgresql://readonly:readonly-demo@customers-rw.demo.svc.cluster.local:5432/customers"}'
  if [[ -n "${HF_TOKEN:-}" ]]; then put eacp/models/hf-token "$(jq -n --arg t "$HF_TOKEN" '{token: $t}')"; fi
}

# --- Keycloak ---------------------------------------------------------------------------------

kc_admin_token() {
  local user pass
  user="$(kubectl get secret keycloak-initial-admin -n keycloak -o jsonpath='{.data.username}' | base64 -d)"
  pass="$(kubectl get secret keycloak-initial-admin -n keycloak -o jsonpath='{.data.password}' | base64 -d)"
  printf '%s' "$pass" | curl -sf "$KC_URL/realms/master/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=admin-cli --data-urlencode "username=$user" \
    --data-urlencode "password@-" | jq -r .access_token
}

kc_set_client_secret() { # kc_set_client_secret ADMIN_TOKEN CLIENT_ID SECRET
  local auth id
  auth="Authorization: Bearer $1"
  id="$(curl -sf -H @<(printf '%s' "$auth") "$KC_URL/admin/realms/$KC_REALM/clients?clientId=$2" | jq -r '.[0].id')"
  [[ -n "$id" && "$id" != null ]] || { echo "client $2 not found in realm $KC_REALM"; exit 1; }
  curl -sf -H @<(printf '%s' "$auth") "$KC_URL/admin/realms/$KC_REALM/clients/$id" |
    jq --arg s "$3" '.secret = $s' |
    curl -sf -X PUT -H @<(printf '%s' "$auth") -H 'Content-Type: application/json' \
      --data-binary @- "$KC_URL/admin/realms/$KC_REALM/clients/$id"
  log "set Keycloak client secret for $2"
}

seed_keycloak() {
  login
  local at path
  at="$(kc_admin_token)"
  [[ -n "$at" && "$at" != null ]] || { echo "Keycloak admin login at $KC_URL failed"; exit 1; }
  for path in eacp/gateway/oauth-client eacp/adapter/oauth-client; do
    kc_set_client_secret "$at" "$(field "$path" client-id)" "$(field "$path" client-secret)"
  done
}

main() {
  command -v jq >/dev/null || { echo "missing: jq"; exit 1; }
  case "${1:-all}" in
    secrets) seed_secrets ;;
    keycloak) seed_keycloak ;;
    all) seed_secrets; seed_keycloak ;;
    *) echo "usage: $0 [secrets|keycloak|all]"; exit 2 ;;
  esac
}

main "$@"
