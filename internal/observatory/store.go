package observatory

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fiberpulse.dev/agent/internal/sharing"
	_ "github.com/mattn/go-sqlite3"
)

type Location struct {
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	Region      string `json:"region,omitempty"`
	City        string `json:"city,omitempty"`
}

type PublicMeasurement struct {
	ID                     string    `json:"-"`
	Timestamp              time.Time `json:"timestamp"`
	CountryCode            string    `json:"country_code"`
	CountryName            string    `json:"country_name"`
	Region                 string    `json:"region,omitempty"`
	City                   string    `json:"city,omitempty"`
	ISP                    string    `json:"isp,omitempty"`
	OfferName              string    `json:"offer_name,omitempty"`
	SubscriptionType       string    `json:"subscription_type,omitempty"`
	AdvertisedDownloadMbps int       `json:"advertised_download_mbps,omitempty"`
	AdvertisedUploadMbps   int       `json:"advertised_upload_mbps,omitempty"`
	DownloadMbps           float64   `json:"download_mbps"`
	UploadMbps             float64   `json:"upload_mbps"`
	LatencyMS              float64   `json:"latency_ms"`
	ConnectionType         string    `json:"connection_type"`
	ConfidenceScore        int       `json:"confidence_score"`
	ConfidenceLevel        string    `json:"confidence_level"`
	MeasurementProvider    string    `json:"measurement_provider"`
	CatalogOffer           bool      `json:"catalog_offer"`
}

type SearchParams struct {
	Query    string
	Country  string
	Provider string
	Page     int
	Limit    int
}

type SearchResult struct {
	Items   []PublicMeasurement `json:"items"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	Limit   int                 `json:"limit"`
	Pages   int                 `json:"pages"`
	Summary Summary             `json:"summary"`
}

type Summary struct {
	Measurements int     `json:"measurements"`
	Countries    int     `json:"countries"`
	Providers    int     `json:"providers"`
	AvgDownload  float64 `json:"avg_download_mbps"`
	AvgUpload    float64 `json:"avg_upload_mbps"`
}

type CountryFacet struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ProviderFacet struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Facets struct {
	Countries []CountryFacet  `json:"countries"`
	Providers []ProviderFacet `json:"providers"`
}

type Store struct {
	db *sql.DB
}

var ErrInstallationRateLimited = errors.New("installation measurement rate limited")

func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("observatory database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create observatory data directory: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(path) + "?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS installations (
  id TEXT PRIMARY KEY,
  public_key BLOB NOT NULL UNIQUE,
  last_sequence INTEGER NOT NULL DEFAULT 0,
  event_window_at TEXT NOT NULL DEFAULT '',
  event_window_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS measurements (
  id TEXT PRIMARY KEY,
  timestamp_bucket TEXT NOT NULL,
  received_at TEXT NOT NULL,
  country_code TEXT NOT NULL,
  country_name TEXT NOT NULL,
  region TEXT NOT NULL DEFAULT '',
  city TEXT NOT NULL DEFAULT '',
  isp TEXT NOT NULL DEFAULT '',
  offer_name TEXT NOT NULL DEFAULT '',
  subscription_type TEXT NOT NULL DEFAULT '',
  advertised_download_mbps INTEGER NOT NULL DEFAULT 0,
  advertised_upload_mbps INTEGER NOT NULL DEFAULT 0,
  download_bps INTEGER NOT NULL,
  upload_bps INTEGER NOT NULL,
  min_rtt_us INTEGER NOT NULL,
  connection_type TEXT NOT NULL,
  confidence_score INTEGER NOT NULL,
  confidence_level TEXT NOT NULL,
  measurement_provider TEXT NOT NULL,
  catalog_offer INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS measurements_public_time ON measurements(timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS measurements_country_time ON measurements(country_code,timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS measurements_isp_time ON measurements(isp,timestamp_bucket DESC);
CREATE INDEX IF NOT EXISTS measurements_city_time ON measurements(city,timestamp_bucket DESC);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) Register(ctx context.Context, id string, public ed25519.PublicKey, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO installations(id,public_key,created_at,last_seen_at) VALUES(?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET last_seen_at=excluded.last_seen_at`, id, []byte(public), formatTime(now), formatTime(now))
	return err
}

func (s *Store) Installation(ctx context.Context, id string) (ed25519.PublicKey, uint64, error) {
	var public []byte
	var sequence uint64
	err := s.db.QueryRowContext(ctx, "SELECT public_key,last_sequence FROM installations WHERE id=?", id).Scan(&public, &sequence)
	if err != nil {
		return nil, 0, err
	}
	if len(public) != ed25519.PublicKeySize {
		return nil, 0, errors.New("stored installation key is invalid")
	}
	return ed25519.PublicKey(public), sequence, nil
}

func (s *Store) Accept(ctx context.Context, installationID string, sequence uint64, event sharing.MeasurementEvent, location Location, now time.Time) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var last uint64
	var windowText string
	var windowCount int
	if err := tx.QueryRowContext(ctx, "SELECT last_sequence,event_window_at,event_window_count FROM installations WHERE id=?", installationID).Scan(&last, &windowText, &windowCount); err != nil {
		return false, err
	}
	var eventExists int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM measurements WHERE id=?", event.EventID).Scan(&eventExists); err != nil {
		return false, err
	}
	windowAt, _ := time.Parse(time.RFC3339Nano, windowText)
	if windowAt.IsZero() || now.Sub(windowAt) >= 24*time.Hour || now.Before(windowAt) {
		windowAt, windowCount = now, 0
	}
	if sequence <= last {
		if eventExists == 1 {
			return false, nil
		}
		return false, errors.New("sequence replay")
	}
	if eventExists == 1 {
		if _, err := tx.ExecContext(ctx, "UPDATE installations SET last_sequence=?,last_seen_at=? WHERE id=?", sequence, formatTime(now), installationID); err != nil {
			return false, err
		}
		return false, tx.Commit()
	}
	if windowCount >= 100 {
		return false, ErrInstallationRateLimited
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO measurements(
		id,timestamp_bucket,received_at,country_code,country_name,region,city,isp,offer_name,subscription_type,
		advertised_download_mbps,advertised_upload_mbps,download_bps,upload_bps,min_rtt_us,connection_type,confidence_score,
		confidence_level,measurement_provider,catalog_offer
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, event.EventID, formatTime(event.TimestampBucket), formatTime(now),
		location.CountryCode, location.CountryName, location.Region, location.City, event.ISP, event.OfferName, event.SubscriptionType,
		event.AdvertisedDownloadMbps, event.AdvertisedUploadMbps, event.DownloadBPS, event.UploadBPS, event.MinRTTUS,
		event.ConnectionType, event.ConfidenceScore, event.ConfidenceLevel, event.MeasurementProvider, boolInt(event.CatalogOffer))
	if err != nil {
		return false, err
	}
	inserted, _ := result.RowsAffected()
	if _, err := tx.ExecContext(ctx, "UPDATE installations SET last_sequence=?,last_seen_at=?,event_window_at=?,event_window_count=? WHERE id=?", sequence, formatTime(now), formatTime(windowAt), windowCount+1, installationID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return inserted == 1, nil
}

func (s *Store) PurgeInactiveInstallations(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM installations WHERE last_seen_at < ?", formatTime(before))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) Search(ctx context.Context, params SearchParams) (SearchResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Limit < 1 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	if q := strings.TrimSpace(params.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		where = append(where, "(lower(country_name) LIKE ? OR lower(region) LIKE ? OR lower(city) LIKE ? OR lower(isp) LIKE ? OR lower(offer_name) LIKE ? OR lower(subscription_type) LIKE ?)")
		for range 6 {
			args = append(args, like)
		}
	}
	if country := strings.ToUpper(strings.TrimSpace(params.Country)); country != "" {
		where = append(where, "country_code=?")
		args = append(args, country)
	}
	if provider := strings.TrimSpace(params.Provider); provider != "" {
		where = append(where, "isp=?")
		args = append(args, provider)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM measurements WHERE "+clause, args...).Scan(&total); err != nil {
		return SearchResult{}, err
	}
	query := `SELECT id,timestamp_bucket,country_code,country_name,region,city,isp,offer_name,subscription_type,
		advertised_download_mbps,advertised_upload_mbps,download_bps,upload_bps,min_rtt_us,connection_type,confidence_score,
		confidence_level,measurement_provider,catalog_offer FROM measurements WHERE ` + clause + ` ORDER BY timestamp_bucket DESC,id DESC LIMIT ? OFFSET ?`
	queryArgs := append(append([]any{}, args...), params.Limit, (params.Page-1)*params.Limit)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()
	items := make([]PublicMeasurement, 0, params.Limit)
	for rows.Next() {
		var item PublicMeasurement
		var timestamp string
		var down, up, rtt int64
		var catalog int
		if err := rows.Scan(&item.ID, &timestamp, &item.CountryCode, &item.CountryName, &item.Region, &item.City, &item.ISP,
			&item.OfferName, &item.SubscriptionType, &item.AdvertisedDownloadMbps, &item.AdvertisedUploadMbps, &down, &up, &rtt,
			&item.ConnectionType, &item.ConfidenceScore, &item.ConfidenceLevel, &item.MeasurementProvider, &catalog); err != nil {
			return SearchResult{}, err
		}
		item.Timestamp, _ = time.Parse(time.RFC3339Nano, timestamp)
		item.DownloadMbps = float64(down) / 1e6
		item.UploadMbps = float64(up) / 1e6
		item.LatencyMS = float64(rtt) / 1e3
		item.CatalogOffer = catalog != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SearchResult{}, err
	}
	pages := 0
	if total > 0 {
		pages = (total + params.Limit - 1) / params.Limit
	}
	summary, err := s.Summary(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	return SearchResult{Items: items, Total: total, Page: params.Page, Limit: params.Limit, Pages: pages, Summary: summary}, nil
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	var summary Summary
	var avgDown, avgUp sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(DISTINCT country_code),COUNT(DISTINCT CASE WHEN isp<>'' THEN isp END),AVG(download_bps)/1000000.0,AVG(upload_bps)/1000000.0 FROM measurements`).
		Scan(&summary.Measurements, &summary.Countries, &summary.Providers, &avgDown, &avgUp)
	if err != nil {
		return Summary{}, err
	}
	if avgDown.Valid {
		summary.AvgDownload = avgDown.Float64
	}
	if avgUp.Valid {
		summary.AvgUpload = avgUp.Float64
	}
	return summary, nil
}

func (s *Store) Facets(ctx context.Context) (Facets, error) {
	facets := Facets{Countries: []CountryFacet{}, Providers: []ProviderFacet{}}
	rows, err := s.db.QueryContext(ctx, `SELECT country_code,country_name,COUNT(*) FROM measurements GROUP BY country_code,country_name ORDER BY country_name`)
	if err != nil {
		return Facets{}, err
	}
	for rows.Next() {
		var facet CountryFacet
		if err := rows.Scan(&facet.Code, &facet.Name, &facet.Count); err != nil {
			rows.Close()
			return Facets{}, err
		}
		facets.Countries = append(facets.Countries, facet)
	}
	if err := rows.Close(); err != nil {
		return Facets{}, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT isp,COUNT(*) FROM measurements WHERE isp<>'' GROUP BY isp ORDER BY lower(isp)`)
	if err != nil {
		return Facets{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var facet ProviderFacet
		if err := rows.Scan(&facet.Name, &facet.Count); err != nil {
			return Facets{}, err
		}
		facets.Providers = append(facets.Providers, facet)
	}
	return facets, rows.Err()
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
