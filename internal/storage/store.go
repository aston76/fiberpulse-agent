package storage

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/scheduler"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	db   *sql.DB
	path string
}

type Consent struct {
	Scope         string    `json:"scope"`
	Granted       bool      `json:"granted"`
	PolicyVersion string    `json:"policy_version"`
	Language      string    `json:"language"`
	Source        string    `json:"source"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, path: path}
	if err := store.quickCheck(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) quickCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&result); err != nil {
		return fmt.Errorf("sqlite quick_check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("sqlite quick_check failed: %s", result)
	}
	return nil
}

func (s *Store) IntegrityCheck(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity_check failed: %s", result)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	entries, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, name := range entries {
		body, err := migrationFS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) Backup(ctx context.Context, destination string) error {
	if destination == "" {
		return errors.New("backup destination is required")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	quoted := strings.ReplaceAll(filepath.ToSlash(destination), "'", "''")
	_, err := s.db.ExecContext(ctx, "VACUUM INTO '"+quoted+"'")
	return err
}

func (s *Store) SaveResult(ctx context.Context, r measurement.Result) error {
	before, err := json.Marshal(r.NetworkBefore)
	if err != nil {
		return err
	}
	after, err := json.Marshal(r.NetworkAfter)
	if err != nil {
		return err
	}
	reasons, err := json.Marshal(r.ConfidenceReasons)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO performance_tests(
		id,provider,protocol_version,client_version,schema_version,methodology_version,confidence_version,
		started_at,completed_at,server_fqdn,download_bps,upload_bps,min_rtt_us,bytes_down,bytes_up,
		download_duration_us,upload_duration_us,status,error_code,error_detail,network_before_json,
		network_after_json,confidence_score,confidence_level,confidence_reasons_json,public_eligible
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Provider, r.ProtocolVersion, r.ClientVersion, r.SchemaVersion, r.MethodologyVersion, r.ConfidenceVersion,
		formatTime(r.StartedAt), formatTime(r.CompletedAt), r.ServerFQDN, r.DownloadBPS, r.UploadBPS, r.MinRTTUS,
		r.BytesDown, r.BytesUp, r.DownloadDurationUS, r.UploadDurationUS, r.Status, r.ErrorCode, r.ErrorDetail,
		string(before), string(after), r.ConfidenceScore, r.ConfidenceLevel, string(reasons), boolInt(r.PublicEligible))
	return err
}

func (s *Store) ListResults(ctx context.Context, limit int) ([]measurement.Result, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,provider,protocol_version,client_version,schema_version,
		methodology_version,confidence_version,started_at,completed_at,server_fqdn,download_bps,upload_bps,
		min_rtt_us,bytes_down,bytes_up,download_duration_us,upload_duration_us,status,error_code,error_detail,
		network_before_json,network_after_json,confidence_score,confidence_level,confidence_reasons_json,public_eligible
		FROM performance_tests ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]measurement.Result, 0)
	for rows.Next() {
		var r measurement.Result
		var started, completed, before, after, reasons string
		var eligible int
		if err := rows.Scan(&r.ID, &r.Provider, &r.ProtocolVersion, &r.ClientVersion, &r.SchemaVersion,
			&r.MethodologyVersion, &r.ConfidenceVersion, &started, &completed, &r.ServerFQDN, &r.DownloadBPS,
			&r.UploadBPS, &r.MinRTTUS, &r.BytesDown, &r.BytesUp, &r.DownloadDurationUS, &r.UploadDurationUS,
			&r.Status, &r.ErrorCode, &r.ErrorDetail, &before, &after, &r.ConfidenceScore, &r.ConfidenceLevel, &reasons, &eligible); err != nil {
			return nil, err
		}
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		_ = json.Unmarshal([]byte(before), &r.NetworkBefore)
		_ = json.Unmarshal([]byte(after), &r.NetworkAfter)
		_ = json.Unmarshal([]byte(reasons), &r.ConfidenceReasons)
		r.PublicEligible = eligible != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CountAttempts(ctx context.Context, since time.Time, kind *scheduler.Kind) (int, error) {
	query := "SELECT COUNT(*) FROM test_attempts WHERE reserved_at >= ?"
	args := []any{formatTime(since)}
	if kind != nil {
		query += " AND kind = ?"
		args = append(args, string(*kind))
	}
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Store) LastAttempt(ctx context.Context, kind scheduler.Kind) (time.Time, error) {
	var raw sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(reserved_at) FROM test_attempts WHERE kind = ?", string(kind)).Scan(&raw); err != nil {
		return time.Time{}, err
	}
	if !raw.Valid {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, raw.String)
}

func (s *Store) ReserveAttempt(ctx context.Context, kind scheduler.Kind, at time.Time) error {
	_, err := s.db.ExecContext(ctx, "INSERT INTO test_attempts(id,kind,reserved_at) VALUES(?,?,?)", uuid.NewString(), string(kind), formatTime(at))
	return err
}

func (s *Store) SetConsent(ctx context.Context, c Consent) error {
	if c.Scope != "mlab" && c.Scope != "fiberpulse" {
		return errors.New("invalid consent scope")
	}
	if c.PolicyVersion == "" {
		return errors.New("policy version is required")
	}
	if c.OccurredAt.IsZero() {
		c.OccurredAt = time.Now().UTC()
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if c.Source == "" {
		c.Source = "local_dashboard"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO consent_events(id,scope,granted,policy_version,language,source,occurred_at) VALUES(?,?,?,?,?,?,?)`,
		uuid.NewString(), c.Scope, boolInt(c.Granted), c.PolicyVersion, c.Language, c.Source, formatTime(c.OccurredAt))
	if err == nil && c.Scope == "fiberpulse" && !c.Granted {
		_, err = s.db.ExecContext(ctx, "DELETE FROM share_queue")
	}
	return err
}

func (s *Store) CurrentConsent(ctx context.Context, scope string) (Consent, error) {
	var c Consent
	var granted int
	var occurred string
	err := s.db.QueryRowContext(ctx, `SELECT scope,granted,policy_version,language,source,occurred_at FROM consent_events WHERE scope=? ORDER BY occurred_at DESC LIMIT 1`, scope).
		Scan(&c.Scope, &granted, &c.PolicyVersion, &c.Language, &c.Source, &occurred)
	if errors.Is(err, sql.ErrNoRows) {
		return Consent{Scope: scope}, nil
	}
	if err != nil {
		return Consent{}, err
	}
	c.Granted = granted != 0
	c.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
	return c, nil
}

func (s *Store) SetSetting(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at`, key, string(encoded), formatTime(time.Now().UTC()))
	return err
}

func (s *Store) GetSetting(ctx context.Context, key string, target any) (bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, "SELECT value_json FROM settings WHERE key=?", key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(raw), target)
}

func (s *Store) SetScheduler(ctx context.Context, state string, next time.Time, paused bool) error {
	var nextValue any
	if !next.IsZero() {
		nextValue = formatTime(next)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE scheduler_state SET state=?,next_run_at=?,paused=?,updated_at=? WHERE singleton=1`, state, nextValue, boolInt(paused), formatTime(time.Now().UTC()))
	return err
}

func (s *Store) Scheduler(ctx context.Context) (state string, next time.Time, paused bool, err error) {
	var nextRaw sql.NullString
	var pausedInt int
	err = s.db.QueryRowContext(ctx, "SELECT state,next_run_at,paused FROM scheduler_state WHERE singleton=1").Scan(&state, &nextRaw, &pausedInt)
	if nextRaw.Valid {
		next, _ = time.Parse(time.RFC3339Nano, nextRaw.String)
	}
	paused = pausedInt != 0
	return
}

func (s *Store) QueueShare(ctx context.Context, id, eventType string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO share_queue(id,event_type,payload_json,created_at,next_attempt_at) VALUES(?,?,?,?,?)`, id, eventType, string(encoded), formatTime(now), formatTime(now))
	return err
}

func (s *Store) SaveHealth(ctx context.Context, sample health.Sample) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO health_samples(captured_at,connectivity_state,gateway_ok,dns_ok,probe_ok,probe_rtt_us,category,detail_code) VALUES(?,?,?,?,?,?,?,?)`,
		formatTime(sample.At), sample.State, boolInt(sample.Network.Online), boolInt(sample.DNSOK), boolInt(sample.ProbeOK), sample.ProbeRTTUS, sample.Category, sample.DetailCode)
	return err
}

func (s *Store) ShareQueueCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM share_queue").Scan(&count)
	return count, err
}

type incidentExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func validateIncident(incident incidents.Record) error {
	if incident.ID == "" || incident.Category == "" || incident.SuspectedAt.IsZero() {
		return errors.New("incident id, category and suspected time are required")
	}
	switch incident.State {
	case incidents.Suspected, incidents.Active, incidents.Recovering, incidents.Resolved, incidents.Dismissed:
	default:
		return errors.New("invalid persisted incident state")
	}
	return nil
}

func saveIncident(ctx context.Context, executor incidentExecutor, incident incidents.Record) error {
	if err := validateIncident(incident); err != nil {
		return err
	}
	if incident.UpdatedAt.IsZero() {
		incident.UpdatedAt = time.Now().UTC()
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO incidents(
		id,category,state,suspected_at,active_at,recovering_at,resolved_at,updated_at
	) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
		category=excluded.category,state=excluded.state,active_at=excluded.active_at,
		recovering_at=excluded.recovering_at,resolved_at=excluded.resolved_at,updated_at=excluded.updated_at`,
		incident.ID, incident.Category, string(incident.State), formatTime(incident.SuspectedAt), nullableTime(incident.ActiveAt),
		nullableTime(incident.RecoveringAt), nullableTime(incident.ResolvedAt), formatTime(incident.UpdatedAt))
	return err
}

func (s *Store) SaveIncident(ctx context.Context, incident incidents.Record) error {
	return saveIncident(ctx, s.db, incident)
}

func (s *Store) DeleteIncident(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM incidents WHERE id=?", id)
	return err
}

func (s *Store) ListIncidents(ctx context.Context, limit int) ([]incidents.Record, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,category,state,suspected_at,active_at,recovering_at,resolved_at,updated_at
		FROM incidents ORDER BY suspected_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]incidents.Record, 0)
	for rows.Next() {
		var incident incidents.Record
		var state, suspected, updated string
		var active, recovering, resolved sql.NullString
		if err := rows.Scan(&incident.ID, &incident.Category, &state, &suspected, &active, &recovering, &resolved, &updated); err != nil {
			return nil, err
		}
		incident.State = incidents.State(state)
		incident.SuspectedAt, _ = time.Parse(time.RFC3339Nano, suspected)
		incident.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		incident.ActiveAt = parseNullableTime(active)
		incident.RecoveringAt = parseNullableTime(recovering)
		incident.ResolvedAt = parseNullableTime(resolved)
		result = append(result, incident)
	}
	return result, rows.Err()
}

// PersistIncidentRuntime commits the visible incident record and its private
// hysteresis snapshot together, so a crash cannot manufacture or forget
// evidence between two independent SQLite writes.
func (s *Store) PersistIncidentRuntime(ctx context.Context, incident *incidents.Record, deleteID, settingKey string, runtime any) error {
	if settingKey == "" {
		return errors.New("incident runtime setting key is required")
	}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if deleteID != "" {
		if _, err := tx.ExecContext(ctx, "DELETE FROM incidents WHERE id=?", deleteID); err != nil {
			return err
		}
	}
	if incident != nil {
		if err := saveIncident(ctx, tx, *incident); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value_json,updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=excluded.updated_at`,
		settingKey, string(encoded), formatTime(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) PurgeExpired(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := []struct {
		q      string
		cutoff time.Time
	}{
		{"DELETE FROM health_samples WHERE captured_at < ?", now.Add(-7 * 24 * time.Hour)},
		{"DELETE FROM performance_tests WHERE started_at < ?", now.AddDate(-1, -1, 0)},
		{"DELETE FROM share_queue WHERE created_at < ?", now.Add(-30 * 24 * time.Hour)},
		{"DELETE FROM app_events WHERE occurred_at < ?", now.Add(-30 * 24 * time.Hour)},
	}
	for _, item := range queries {
		if _, err := tx.ExecContext(ctx, item.q, formatTime(item.cutoff)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}
func parseNullableTime(value sql.NullString) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	parsed, _ := time.Parse(time.RFC3339Nano, value.String)
	return parsed
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
