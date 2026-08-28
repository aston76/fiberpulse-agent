UPDATE performance_tests
SET public_eligible = 0,
    confidence_reasons_json = CASE
        WHEN TRIM(COALESCE(confidence_reasons_json, '')) IN ('', 'null', '[]')
            THEN '["provider.not_public"]'
        ELSE confidence_reasons_json
    END
WHERE provider <> 'mlab_ndt7';

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES(2, strftime('%Y-%m-%dT%H:%M:%fZ','now'));
