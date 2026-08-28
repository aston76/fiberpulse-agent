CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value_json TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS secure_settings (
    key TEXT PRIMARY KEY,
    value_blob BLOB NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS consent_events (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK(scope IN ('mlab','fiberpulse')),
    granted INTEGER NOT NULL CHECK(granted IN (0,1)),
    policy_version TEXT NOT NULL,
    language TEXT NOT NULL,
    source TEXT NOT NULL,
    occurred_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS consent_events_scope_time ON consent_events(scope, occurred_at DESC);

CREATE TABLE IF NOT EXISTS network_profiles (
    id TEXT PRIMARY KEY,
    fingerprint_hmac TEXT NOT NULL UNIQUE,
    connection_type TEXT NOT NULL,
    encrypted_profile BLOB,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS interface_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    captured_at TEXT NOT NULL,
    interface_id TEXT NOT NULL DEFAULT '',
    route_id TEXT NOT NULL DEFAULT '',
    connection_type TEXT NOT NULL,
    wifi_quality INTEGER NOT NULL DEFAULT 0,
    metered INTEGER NOT NULL DEFAULT 0,
    roaming INTEGER NOT NULL DEFAULT 0,
    vpn_suspected INTEGER NOT NULL DEFAULT 0,
    proxy_suspected INTEGER NOT NULL DEFAULT 0,
    online INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS health_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    captured_at TEXT NOT NULL,
    connectivity_state TEXT NOT NULL,
    gateway_ok INTEGER NOT NULL DEFAULT 0,
    dns_ok INTEGER NOT NULL DEFAULT 0,
    probe_ok INTEGER NOT NULL DEFAULT 0,
    probe_rtt_us INTEGER NOT NULL DEFAULT 0,
    category TEXT NOT NULL DEFAULT 'unknown',
    detail_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS health_samples_time ON health_samples(captured_at DESC);

CREATE TABLE IF NOT EXISTS health_rollups (
    bucket_start TEXT NOT NULL,
    granularity TEXT NOT NULL CHECK(granularity IN ('5m','1h')),
    sample_count INTEGER NOT NULL,
    degraded_count INTEGER NOT NULL,
    median_probe_rtt_us INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(bucket_start, granularity)
);

CREATE TABLE IF NOT EXISTS test_attempts (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK(kind IN ('automatic','manual')),
    reserved_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'reserved',
    result_id TEXT
);
CREATE INDEX IF NOT EXISTS test_attempts_time ON test_attempts(reserved_at DESC);

CREATE TABLE IF NOT EXISTS performance_tests (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    protocol_version TEXT NOT NULL,
    client_version TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    methodology_version TEXT NOT NULL,
    confidence_version TEXT NOT NULL,
    started_at TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    server_fqdn TEXT NOT NULL DEFAULT '',
    download_bps INTEGER NOT NULL DEFAULT 0,
    upload_bps INTEGER NOT NULL DEFAULT 0,
    min_rtt_us INTEGER NOT NULL DEFAULT 0,
    bytes_down INTEGER NOT NULL DEFAULT 0,
    bytes_up INTEGER NOT NULL DEFAULT 0,
    download_duration_us INTEGER NOT NULL DEFAULT 0,
    upload_duration_us INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_detail TEXT NOT NULL DEFAULT '',
    network_before_json TEXT NOT NULL,
    network_after_json TEXT NOT NULL,
    confidence_score INTEGER NOT NULL DEFAULT 0,
    confidence_level TEXT NOT NULL DEFAULT 'low',
    confidence_reasons_json TEXT NOT NULL DEFAULT '[]',
    public_eligible INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS performance_tests_time ON performance_tests(started_at DESC);

CREATE TABLE IF NOT EXISTS incidents (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    state TEXT NOT NULL,
    suspected_at TEXT NOT NULL,
    active_at TEXT,
    recovering_at TEXT,
    resolved_at TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS incident_measurements (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    measurement_id TEXT NOT NULL REFERENCES performance_tests(id) ON DELETE CASCADE,
    PRIMARY KEY(incident_id, measurement_id)
);

CREATE TABLE IF NOT EXISTS scheduler_state (
    singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
    state TEXT NOT NULL,
    next_run_at TEXT,
    paused INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS share_queue (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    next_attempt_at TEXT NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS share_queue_due ON share_queue(next_attempt_at, created_at);

CREATE TABLE IF NOT EXISTS reports (
    id TEXT PRIMARY KEY,
    format TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    path TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    level TEXT NOT NULL,
    code TEXT NOT NULL,
    detail_json TEXT NOT NULL DEFAULT '{}'
);

INSERT OR IGNORE INTO scheduler_state(singleton, state, paused, updated_at)
VALUES(1, 'waiting', 0, strftime('%Y-%m-%dT%H:%M:%fZ','now'));

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES(1, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
