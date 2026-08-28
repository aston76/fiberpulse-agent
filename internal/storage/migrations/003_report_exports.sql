CREATE TABLE IF NOT EXISTS report_exports (
    id TEXT PRIMARY KEY,
    format TEXT NOT NULL CHECK(format IN ('pdf','csv')),
    state TEXT NOT NULL CHECK(state IN ('drafting','ready','exporting','exported','failed','deleted')),
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    byte_count INTEGER NOT NULL DEFAULT 0 CHECK(byte_count >= 0),
    error_code TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS report_exports_created ON report_exports(created_at DESC);

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES(3, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
