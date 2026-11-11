#!/usr/bin/env bash
# The pathway test: the real agentgateway in front of the real adapter, OPA, the guardrail
# service and the tickets demo server, on one machine, without Kubernetes. It proves the
# contract between the gateway's ExtMCP hook and the adapter that everything else rests on:
# which identity, trace context and stashed metadata reach each phase, how listings are filtered
# and hashed, how a denial reaches the client, that every phase is audited, and that a dead
# policy engine or adapter closes the gate.
#
#   test/pathway/run.sh            run the checks; exit non-zero if any fails
#   test/pathway/run.sh --probe    replace the adapter with eacp-hookprobe and print, per hook
#                                  call, exactly what the gateway sent
#   test/pathway/run.sh --keep     run the checks, then leave everything up until Ctrl-C
#   test/pathway/run.sh --clickhouse   also start ClickHouse (Docker) with the cluster's schema
#                                  and users, have the adapter write its audit rows there with
#                                  the insert-only user, and read them back with the reader
#
# Needs: go, opa, python3 with test/pathway/requirements.txt, and agentgateway 1.7 either as a
# binary (EACP_AGENTGATEWAY=/path/to/agentgateway) or through Docker (the default; the image is
# EACP_AGENTGATEWAY_IMAGE). Everything listens on 127.0.0.1; logs land in dist/pathway/.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
MODE=check
CLICKHOUSE=0
for arg in "$@"; do
  case "$arg" in
    --probe|--keep) MODE="$arg" ;;
    --clickhouse) CLICKHOUSE=1 ;;
    *) echo "unknown option: $arg" >&2; exit 2 ;;
  esac
done
CLICKHOUSE_IMAGE="${EACP_CLICKHOUSE_IMAGE:-clickhouse/clickhouse-server:25.8}"
PY="${EACP_PYTHON:-python3}"
OPA="${OPA:-opa}"
AGW_IMAGE="${EACP_AGENTGATEWAY_IMAGE:-cr.agentgateway.dev/agentgateway:v1.7.0}"
W="$ROOT/dist/pathway"

GATEWAY_PORT=13000 ADMIN_PORT=13001 READY_PORT=13002
TICKETS_PORT=18001 NOTES_PORT=18002 GUARDRAILS_PORT=18000 OPA_PORT=18181
HOOK_PORT=19001 ADAPTER_HTTP_PORT=18080 OTLP_PORT=14317
ISSUER="https://auth.pathway.test/realms/eacp"

PIDS=()
log() { printf '==> %s\n' "$*"; }
start() { # start NAME COMMAND...: run in the background, log to $W/NAME.log
  local name="$1"; shift
  "$@" >>"$W/$name.log" 2>&1 &
  PIDS+=("$!")
  echo "$!" >"$W/$name.pid"
}
stop_one() { [[ -f "$W/$1.pid" ]] && kill "$(cat "$W/$1.pid")" 2>/dev/null || true; }
wait_port() { # wait_port NAME PORT
  for _ in $(seq 1 100); do (echo >"/dev/tcp/127.0.0.1/$2") 2>/dev/null && return 0; sleep 0.2; done
  echo "$1 did not start on port $2; see $W/$1.log" >&2
  tail -20 "$W/$1.log" >&2 || true
  return 1
}
cleanup() {
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  [[ -n "${EACP_AGENTGATEWAY:-}" ]] || docker rm -f eacp-pathway-gateway >/dev/null 2>&1 || true
  ((CLICKHOUSE == 0)) || docker rm -f eacp-pathway-clickhouse >/dev/null 2>&1 || true
}
trap cleanup EXIT

start_opa() { start opa "$OPA" run --server --addr "127.0.0.1:$OPA_PORT" --config-file "$W/opa.yaml" --log-level error; wait_port opa "$OPA_PORT"; }
start_gateway() {
  if [[ -n "${EACP_AGENTGATEWAY:-}" ]]; then
    start gateway env SESSION_KEY="$SESSION_KEY" "$EACP_AGENTGATEWAY" -f "$W/gateway.yaml"
  else
    docker rm -f eacp-pathway-gateway >/dev/null 2>&1 || true
    docker run -d --name eacp-pathway-gateway --network host -e SESSION_KEY="$SESSION_KEY" \
      -v "$W:$W:ro" "$AGW_IMAGE" -f "$W/gateway.yaml" >/dev/null
    (docker logs -f eacp-pathway-gateway >>"$W/gateway.log" 2>&1 &)
  fi
  wait_port gateway "$GATEWAY_PORT"
}
start_hook() {
  if [[ "$MODE" == "--probe" ]]; then
    start hook "$W/eacp-hookprobe" -addr "127.0.0.1:$HOOK_PORT"
  else
    local audit=()
    if ((CLICKHOUSE)); then
      audit=(EACP_CLICKHOUSE_URL="http://127.0.0.1:8123" EACP_CLICKHOUSE_USER=eacp_audit_writer EACP_CLICKHOUSE_PASSWORD="$AUDIT_WRITER_PASSWORD")
    fi
    start hook env "${audit[@]}" \
      EACP_GRPC_ADDR="127.0.0.1:$HOOK_PORT" EACP_HTTP_ADDR="127.0.0.1:$ADAPTER_HTTP_PORT" \
      EACP_PUBLIC_URL="http://127.0.0.1:$GATEWAY_PORT" EACP_OPA_URL="http://127.0.0.1:$OPA_PORT" \
      EACP_GUARDRAILS_URL="http://127.0.0.1:$GUARDRAILS_PORT" EACP_AUDIT_REQUIRED=true \
      EACP_CONTENT_CAPTURE=false EACP_LOG_LEVEL=debug \
      OTEL_EXPORTER_OTLP_ENDPOINT="127.0.0.1:$OTLP_PORT" "$W/eacp-adapter"
  fi
  wait_port hook "$HOOK_PORT"
}

rm -rf "$W"
mkdir -p "$W"

log "build: adapter, hook probe, trace sink, policy bundle"
(cd adapter && go build -o "$W/eacp-adapter" ./cmd/eacp-adapter && go build -o "$W/eacp-hookprobe" ./cmd/eacp-hookprobe && go build -o "$W/eacp-otlpsink" ./cmd/eacp-otlpsink)
OPA="$OPA" scripts/build-bundle.sh pathway >/dev/null
cp dist/eacp-bundle.tar.gz "$W/eacp-bundle.tar.gz"
# The same bundle source the laptop cluster uses: a tarball on disk, loaded by OPA's bundle
# plugin so data.system.bundles carries the revision.
cat >"$W/opa.yaml" <<EOF
bundles:
  eacp:
    resource: file://$W/eacp-bundle.tar.gz
EOF

log "keys and gateway configuration"
"$PY" test/pathway/mint.py keys "$W/keys"
SESSION_KEY="$(openssl rand -hex 32)"
sed -e "s#__GATEWAY_PORT__#$GATEWAY_PORT#g; s#__ADMIN_PORT__#$ADMIN_PORT#g; s#__READY_PORT__#$READY_PORT#g" \
    -e "s#__TICKETS_PORT__#$TICKETS_PORT#g; s#__NOTES_PORT__#$NOTES_PORT#g; s#__HOOK_PORT__#$HOOK_PORT#g" \
    -e "s#__OTLP_PORT__#$OTLP_PORT#g; s#__ISSUER__#$ISSUER#g; s#__JWKS__#$W/keys/jwks.json#g" \
    test/pathway/gateway.yaml.tmpl >"$W/gateway.yaml"

log "start: trace sink, tickets, notes, guardrails, OPA, hook, gateway"
"$W/eacp-otlpsink" -addr "127.0.0.1:$OTLP_PORT" >"$W/spans.jsonl" 2>"$W/otlpsink.log" &
PIDS+=("$!")
start tickets env PORT="$TICKETS_PORT" "$PY" demo/servers/tickets/server.py
start notes env PORT="$NOTES_PORT" "$PY" test/pathway/notes_server.py
start guardrails env EACP_PII_ENGINE=regex EACP_GUARD_BASE_URL= EACP_GUARD_MODEL= PYTHONPATH="$ROOT/guardrails" \
  "$PY" -m uvicorn eacp_guardrails.app:app --host 127.0.0.1 --port "$GUARDRAILS_PORT" --log-level warning
wait_port tickets "$TICKETS_PORT"; wait_port notes "$NOTES_PORT"; wait_port guardrails "$GUARDRAILS_PORT"
wait_port otlpsink "$OTLP_PORT"
if ((CLICKHOUSE)); then
  log "start: ClickHouse with platform/telemetry/clickhouse-init.sh"
  export AUDIT_WRITER_PASSWORD="$(openssl rand -hex 12)" AUDIT_READER_PASSWORD="$(openssl rand -hex 12)"
  docker rm -f eacp-pathway-clickhouse >/dev/null 2>&1 || true
  docker run -d --name eacp-pathway-clickhouse --network host --user 101:101 \
    -e AUDIT_WRITER_PASSWORD -e AUDIT_READER_PASSWORD -e LOGS_WRITER_PASSWORD="$(openssl rand -hex 12)" \
    -v "$ROOT/platform/telemetry/clickhouse-init.sh:/docker-entrypoint-initdb.d/init.sh:ro" "$CLICKHOUSE_IMAGE" >/dev/null
  for _ in $(seq 1 90); do
    curl -sf -u "eacp_audit_reader:$AUDIT_READER_PASSWORD" "http://127.0.0.1:8123/?query=EXISTS%20eacp_audit.events" 2>/dev/null | grep -q 1 && break
    sleep 1
  done
  export EACP_PATHWAY_CLICKHOUSE="http://127.0.0.1:8123"
fi
start_opa
start_hook
start_gateway

export EACP_PATHWAY_DIR="$W" EACP_PATHWAY_GATEWAY="http://127.0.0.1:$GATEWAY_PORT"
export EACP_PATHWAY_TICKETS="http://127.0.0.1:$TICKETS_PORT/mcp"

if [[ "$MODE" == "--probe" ]]; then
  log "probe: driving one listing and one call through each route"
  "$PY" test/pathway/check.py drive
  log "what the gateway sent to the hook:"
  "$PY" test/pathway/check.py show-probe
  exit 0
fi

rc=0
log "checks: the pathway"
"$PY" test/pathway/check.py pathway || rc=1

if ((CLICKHOUSE)); then
  log "checks: the audit store"
  "$PY" test/pathway/check.py audit-store || rc=1
fi

log "checks: policy engine down"
stop_one opa; sleep 0.5
"$PY" test/pathway/check.py opa-down || rc=1
start_opa

log "checks: adapter down"
stop_one hook; sleep 0.5
"$PY" test/pathway/check.py adapter-down || rc=1
start_hook

log "checks: recovered"
"$PY" test/pathway/check.py recovered || rc=1

if [[ "$MODE" == "--keep" ]]; then
  log "left running: gateway http://127.0.0.1:$GATEWAY_PORT/mcp, logs in $W (Ctrl-C to stop)"
  "$PY" test/pathway/mint.py token "$W/keys" --sub alice >"$W/alice.token"
  log "a token for alice is in $W/alice.token"
  wait
fi

[[ $rc -eq 0 ]] && log "pathway test passed" || log "pathway test FAILED; logs in $W"
exit $rc
