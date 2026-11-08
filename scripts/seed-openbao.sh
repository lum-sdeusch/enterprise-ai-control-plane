#!/usr/bin/env bash
# Seed OpenBao with the demo secrets and hand the generated OAuth client secrets to Keycloak.
#
#   scripts/seed-openbao.sh secrets    # write eacp/* into OpenBao (needs OpenBao unsealed)
#   scripts/seed-openbao.sh keycloak   # set the eacp-gateway, eacp-adapter and demo-runner client secrets
#   scripts/seed-openbao.sh            # both
#   scripts/seed-openbao.sh runner-secret   # print the demo-runner client secret (demo/scenarios)
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
KC_URL="${EACP_KEYCLOAK_URL:-https://auth.127.0.0.1.sslip.io:8081}"
# Keycloak's certificate is signed by the laptop CA that scripts/up.sh exports.
KC_CA="${EACP_CA_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.eacp/ca.pem}"
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

fail() { echo "seed-openbao: $*" >&2; exit 1; }

# exists PATH: 0 when PATH holds a value, 1 when OpenBao says there is none. Any other failure
# (permission, network, a missing pod) aborts, so put_once never mistakes it for "absent" and
# rotates a generated secret.
exists() {
  local out rc=0
  out="$(printf '%s\n' "$TOKEN" | bao_in_pod "bao kv get -mount=$MOUNT $1 2>&1 >/dev/null" 2>&1)" || rc=$?
  ((rc == 0)) && return 0
  [[ "$out" == *"No value found at $MOUNT/data/$1"* ]] && return 1
  fail "reading $1 from OpenBao failed (exit $rc): $out"
}

# field PATH KEY: prints the value. Callers assign it to a variable on its own line (so set -e
# sees a failure) and check it is not empty.
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

# copy_once PATH USERNAME KEY: a username/password pair whose password is KEY of the ClickHouse
# users path. The adapter and the collector read their own copies; keep them equal to the store.
copy_once() {
  if exists "$1"; then log "kept  $1"; return; fi
  local pass
  pass="$(field eacp/telemetry/clickhouse "$3")"
  [[ -n "$pass" ]] || fail "eacp/telemetry/clickhouse has no $3"
  put "$1" "$(jq -n --arg u "$2" --arg p "$pass" '{username: $u, password: $p}')"
}

seed_clickhouse() {
  put_once eacp/telemetry/clickhouse "$(jq -n --arg a "$(rand)" --arg r "$(rand)" --arg l "$(rand)" \
    '{"audit-writer-password": $a, "audit-reader-password": $r, "logs-writer-password": $l}')"
  copy_once eacp/audit/writer eacp_audit_writer audit-writer-password
  copy_once eacp/telemetry/logs-writer logs_writer logs-writer-password
}

seed_secrets() {
  login
  seed_anthropic
  seed_clickhouse
  put_once eacp/gateway/oauth-client "$(jq -n --arg s "$(rand)" '{"client-id": "eacp-gateway", "client-secret": $s}')"
  put_once eacp/adapter/oauth-client "$(jq -n --arg s "$(rand)" '{"client-id": "eacp-adapter", "client-secret": $s}')"
  # The scenario runner's password-grant client (laptop only; demo/scenarios/run.py).
  put_once eacp/demo/runner-client "$(jq -n --arg s "$(rand)" '{"client-id": "demo-runner", "client-secret": $s}')"
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
  printf '%s' "$pass" | curl -sf --cacert "$KC_CA" "$KC_URL/realms/master/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=admin-cli --data-urlencode "username=$user" \
    --data-urlencode "password@-" | jq -r .access_token
}

kc_set_client_secret() { # kc_set_client_secret ADMIN_TOKEN CLIENT_ID SECRET
  local auth id
  [[ -n "$2" && -n "$3" ]] || fail "kc_set_client_secret needs a client id and a secret"
  auth="Authorization: Bearer $1"
  # clientId is a search filter, not an exact match: keep only the client whose id is exactly $2.
  id="$(curl -sfG --cacert "$KC_CA" -H @<(printf '%s' "$auth") --data-urlencode "clientId=$2" \
    "$KC_URL/admin/realms/$KC_REALM/clients" |
    jq -r --arg c "$2" '[.[] | select(.clientId == $c)] | if length == 1 then .[0].id else empty end')"
  [[ -n "$id" ]] || fail "client $2 not found (exactly once) in realm $KC_REALM"
  curl -sf --cacert "$KC_CA" -H @<(printf '%s' "$auth") "$KC_URL/admin/realms/$KC_REALM/clients/$id" |
    jq --arg s "$3" '.secret = $s' |
    curl -sf --cacert "$KC_CA" -X PUT -H @<(printf '%s' "$auth") -H 'Content-Type: application/json' \
      --data-binary @- "$KC_URL/admin/realms/$KC_REALM/clients/$id"
  log "set Keycloak client secret for $2"
}

seed_keycloak() {
  login
  local at path
  at="$(kc_admin_token)"
  [[ -n "$at" && "$at" != null ]] || fail "Keycloak admin login at $KC_URL failed"
  local client secret
  for path in eacp/gateway/oauth-client eacp/adapter/oauth-client eacp/demo/runner-client; do
    client="$(field "$path" client-id)"
    secret="$(field "$path" client-secret)"
    [[ -n "$client" && -n "$secret" ]] || fail "$path is missing client-id or client-secret"
    kc_set_client_secret "$at" "$client" "$secret"
  done
}

main() {
  command -v jq >/dev/null || { echo "missing: jq"; exit 1; }
  case "${1:-all}" in
    secrets) seed_secrets ;;
    keycloak) seed_keycloak ;;
    all) seed_secrets; seed_keycloak ;;
    runner-secret) login; field eacp/demo/runner-client client-secret ;;
    *) echo "usage: $0 [secrets|keycloak|all|runner-secret]"; exit 2 ;;
  esac
}

main "$@"
