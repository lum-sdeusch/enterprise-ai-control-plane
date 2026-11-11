# ClickHouse schema and users for the control plane. The image's entrypoint sources this file
# once, on the first start with an empty data directory, while a temporary local server runs.
# It is a shell script, not a .sql file, because the passwords come from the environment and the
# entrypoint does not expand variables inside .sql files.
#
# platform/telemetry/kustomization.yaml turns it into the clickhouse-init ConfigMap; the pathway
# test mounts the same file (test/pathway/run.sh --clickhouse) and writes real audit rows to it.

: "${AUDIT_WRITER_PASSWORD:?}" "${AUDIT_READER_PASSWORD:?}" "${LOGS_WRITER_PASSWORD:?}"

# shellcheck disable=SC2154 # clickhouseclient is the entrypoint's client command, when sourced.
if [ -n "${clickhouseclient+set}" ]; then eacp_client=("${clickhouseclient[@]}"); else eacp_client=(clickhouse-client --multiquery); fi

"${eacp_client[@]}" <<SQL
CREATE DATABASE IF NOT EXISTS eacp_audit;

-- One row per policy decision phase. Unsampled. Columns mirror the control mapping in
-- docs/control-mapping.md so an auditor can read this table without the code.
CREATE TABLE IF NOT EXISTS eacp_audit.events
(
    ts               DateTime64(3) CODEC(Delta, ZSTD),
    event_id         UUID,
    trace_id         String,
    span_id          String,
    conversation_id  String,
    phase            LowCardinality(String),
    subject          String,
    team             LowCardinality(String),
    groups           Array(String),
    agent            LowCardinality(String),
    actor_chain      String,
    server           LowCardinality(String),
    tool             String,
    model_requested  LowCardinality(String),
    model_selected   LowCardinality(String),
    route_label      LowCardinality(String),
    decision         LowCardinality(String),   -- allow | deny | approval_required | approved | quarantine | fail_closed
    reasons          Array(String),
    redactions       UInt16,
    findings         String,                    -- JSON summary: counts and labels, never content
    taint_before     String,
    taint_after      String,
    pin_status       LowCardinality(String),
    policy_revision  String,
    approver         String,
    latency_ms       UInt32,
    adapter_version  LowCardinality(String)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (team, ts, subject)
TTL toDateTime(ts) + INTERVAL 400 DAY;

-- The adapter may only add rows; nobody may change or remove them through these users.
CREATE USER IF NOT EXISTS eacp_audit_writer IDENTIFIED WITH sha256_password BY '${AUDIT_WRITER_PASSWORD}';
GRANT INSERT ON eacp_audit.events TO eacp_audit_writer;

CREATE USER IF NOT EXISTS eacp_audit_reader IDENTIFIED WITH sha256_password BY '${AUDIT_READER_PASSWORD}';
GRANT SELECT ON eacp_audit.* TO eacp_audit_reader;

CREATE DATABASE IF NOT EXISTS eacp_logs;
CREATE USER IF NOT EXISTS logs_writer IDENTIFIED WITH sha256_password BY '${LOGS_WRITER_PASSWORD}';
GRANT CREATE TABLE, INSERT ON eacp_logs.* TO logs_writer;
SQL
