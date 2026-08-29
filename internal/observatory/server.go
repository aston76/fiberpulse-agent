package observatory

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"fiberpulse.dev/agent/internal/sharing"
	"github.com/google/uuid"
)

//go:embed web/*
var webFiles embed.FS

type Config struct {
	Store                   *Store
	Logger                  *slog.Logger
	TrustCloudflareLocation bool
}

type Server struct {
	store                     *Store
	logger                    *slog.Logger
	trustCF                   bool
	registrationLimiter       *rateLimiter
	globalRegistrationLimiter *rateLimiter
}

func NewServer(config Config) (*Server, error) {
	if config.Store == nil {
		return nil, errors.New("observatory store is required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Server{
		store: config.Store, logger: config.Logger, trustCF: config.TrustCloudflareLocation,
		registrationLimiter: newRateLimiter(20, time.Hour), globalRegistrationLimiter: newRateLimiter(10_000, time.Hour),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/installations", s.handleRegistration)
	mux.HandleFunc("POST /api/v1/measurements", s.handleMeasurement)
	mux.HandleFunc("GET /api/v1/public/measurements", s.handleSearch)
	mux.HandleFunc("GET /api/v1/public/summary", s.handleSummary)
	mux.HandleFunc("GET /api/v1/public/facets", s.handleFacets)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	assets, _ := fs.Sub(webFiles, "web")
	mux.Handle("GET /", http.FileServer(http.FS(assets)))
	return s.securityHeaders(s.limitBody(mux))
}

func (s *Server) handleRegistration(w http.ResponseWriter, r *http.Request) {
	limiter, remote := s.registrationLimiter, r.RemoteAddr
	if s.trustCF {
		if connectingIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); net.ParseIP(connectingIP) != nil {
			remote = net.JoinHostPort(connectingIP, "0")
		} else {
			// When Cloudflare removes visitor IP headers, edge rate limiting is
			// authoritative and this bounded global guard is only a final fuse.
			limiter, remote = s.globalRegistrationLimiter, "cloudflare-edge"
		}
	}
	if !limiter.Allow(remote, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "registration_rate_limited")
		return
	}
	var body struct {
		InstallationID string `json:"installation_id"`
		PublicKey      string `json:"public_key"`
	}
	if err := decodeStrict(r.Body, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_registration")
		return
	}
	public, err := base64.StdEncoding.DecodeString(body.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || body.InstallationID != sharing.InstallationID(ed25519.PublicKey(public)) {
		writeError(w, http.StatusBadRequest, "invalid_registration")
		return
	}
	if err := s.store.Register(r.Context(), body.InstallationID, ed25519.PublicKey(public), time.Now().UTC()); err != nil {
		s.logger.Error("installation registration failed", "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"installation_id": body.InstallationID})
}

func (s *Server) handleMeasurement(w http.ResponseWriter, r *http.Request) {
	installationID := r.Header.Get("X-FiberPulse-Installation")
	timestampText := r.Header.Get("X-FiberPulse-Timestamp")
	nonce := r.Header.Get("X-FiberPulse-Nonce")
	sequenceText := r.Header.Get("X-FiberPulse-Sequence")
	signature := r.Header.Get("X-FiberPulse-Signature")
	if len(installationID) != 32 || len(nonce) > 80 || signature == "" {
		writeError(w, http.StatusUnauthorized, "invalid_signature")
		return
	}
	timestamp, err := time.Parse(time.RFC3339, timestampText)
	if err != nil || time.Since(timestamp).Abs() > 5*time.Minute {
		writeError(w, http.StatusUnauthorized, "stale_signature")
		return
	}
	if _, err := uuid.Parse(nonce); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_nonce")
		return
	}
	sequence, err := strconv.ParseUint(sequenceText, 10, 64)
	if err != nil || sequence == 0 {
		writeError(w, http.StatusUnauthorized, "invalid_sequence")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload")
		return
	}
	public, _, err := s.store.Installation(r.Context(), installationID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusUnauthorized, "unknown_installation")
		return
	}
	if err != nil {
		s.logger.Error("installation lookup failed", "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	if !sharing.Verify(public, signature, r.Method, r.URL.Path, timestampText, nonce, sequence, body) {
		writeError(w, http.StatusUnauthorized, "invalid_signature")
		return
	}
	var event sharing.MeasurementEvent
	if err := decodeStrict(strings.NewReader(string(body)), &event); err != nil || validateEvent(event, time.Now().UTC()) != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_measurement")
		return
	}
	location, err := s.location(r, event)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "location_unavailable")
		return
	}
	inserted, err := s.store.Accept(r.Context(), installationID, sequence, event, location, time.Now().UTC())
	if err != nil {
		if errors.Is(err, ErrInstallationRateLimited) {
			writeError(w, http.StatusTooManyRequests, "installation_rate_limited")
			return
		}
		if err.Error() == "sequence replay" {
			writeError(w, http.StatusConflict, "sequence_replay")
			return
		}
		s.logger.Error("anonymous measurement storage failed", "event_id", event.EventID, "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	status := http.StatusCreated
	if !inserted {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"accepted": true, "duplicate": !inserted})
}

func (s *Server) location(r *http.Request, event sharing.MeasurementEvent) (Location, error) {
	code := strings.ToUpper(strings.TrimSpace(event.PlanCountryCode))
	region, city := "", ""
	if s.trustCF {
		code = strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
		region = cleanLocation(r.Header.Get("CF-Region"))
		city = cleanLocation(r.Header.Get("CF-IPCity"))
	}
	if len(code) != 2 || code == "XX" || code == "T1" {
		return Location{}, errors.New("coarse country is unavailable")
	}
	for _, char := range code {
		if char < 'A' || char > 'Z' {
			return Location{}, errors.New("invalid country code")
		}
	}
	name := code
	if strings.EqualFold(code, event.PlanCountryCode) && validPublicText(event.PlanCountryName, 80) {
		name = strings.TrimSpace(event.PlanCountryName)
	}
	return Location{CountryCode: code, CountryName: name, Region: region, City: city}, nil
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	params := SearchParams{Query: r.URL.Query().Get("q"), Country: r.URL.Query().Get("country"), Provider: r.URL.Query().Get("provider"), Page: page, Limit: limit}
	if len(params.Query) > 120 || len(params.Provider) > 120 || len(params.Country) > 2 {
		writeError(w, http.StatusBadRequest, "invalid_search")
		return
	}
	result, err := s.store.Search(r.Context(), params)
	if err != nil {
		s.logger.Error("public measurement search failed", "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.store.Summary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleFacets(w http.ResponseWriter, r *http.Request) {
	facets, err := s.store.Facets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_error")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, facets)
}

func validateEvent(event sharing.MeasurementEvent, now time.Time) error {
	if _, err := uuid.Parse(event.EventID); err != nil {
		return err
	}
	if event.TimestampBucket.IsZero() || event.TimestampBucket.After(now.Add(time.Hour)) || event.TimestampBucket.Before(now.Add(-35*24*time.Hour)) {
		return errors.New("event timestamp outside retention window")
	}
	if event.TimestampBucket.Minute()%15 != 0 || event.TimestampBucket.Second() != 0 || event.TimestampBucket.Nanosecond() != 0 {
		return errors.New("event timestamp is not a 15 minute bucket")
	}
	if event.MeasurementProvider != "mlab_ndt7" || !event.PublicEligible || event.DownloadBPS <= 0 || event.DownloadBPS > 20e12 || event.UploadBPS < 0 || event.UploadBPS > 20e12 || event.MinRTTUS < 0 || event.MinRTTUS > int64(10*time.Second/time.Microsecond) {
		return errors.New("measurement values are not eligible")
	}
	if event.ConfidenceScore < 0 || event.ConfidenceScore > 100 || !oneOf(event.ConnectionType, "ethernet", "wifi", "cellular", "other", "unknown") {
		return errors.New("invalid confidence or connection type")
	}
	for value, limit := range map[string]int{
		event.ProtocolVersion: 40, event.AgentVersion: 40, event.SchemaVersion: 40, event.MethodologyVersion: 40,
		event.ConfidenceVersion: 40, event.ServerFQDN: 253, event.PlanCountryName: 80, event.ISP: 80,
		event.OfferName: 120, event.SubscriptionType: 80, event.ConfidenceLevel: 30,
	} {
		if !validPublicText(value, limit) {
			return errors.New("invalid public text")
		}
	}
	if event.AdvertisedDownloadMbps < 0 || event.AdvertisedDownloadMbps > 100000 || event.AdvertisedUploadMbps < 0 || event.AdvertisedUploadMbps > 100000 {
		return errors.New("invalid advertised speed")
	}
	return nil
}

func cleanLocation(value string) string {
	value = strings.TrimSpace(value)
	if !validPublicText(value, 80) {
		return ""
	}
	return value
}

func validPublicText(value string, maximum int) bool {
	if len(value) > maximum {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func decodeStrict(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func (s *Server) limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

type rateLimiter struct {
	mu      sync.Mutex
	secret  [32]byte
	limit   int
	window  time.Duration
	entries map[string]rateEntry
}

type rateEntry struct {
	started time.Time
	count   int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	limiter := &rateLimiter{limit: limit, window: window, entries: make(map[string]rateEntry)}
	_, _ = rand.Read(limiter.secret[:])
	return limiter
}

func (l *rateLimiter) Allow(remote string, now time.Time) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	hash := sha256.Sum256(append(l.secret[:], []byte(host)...))
	key := hex.EncodeToString(hash[:12])
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.entries[key]
	if entry.started.IsZero() && len(l.entries) >= 10_000 {
		for candidate, current := range l.entries {
			if now.Sub(current.started) >= l.window {
				delete(l.entries, candidate)
			}
		}
		if len(l.entries) >= 10_000 {
			return false
		}
	}
	if entry.started.IsZero() || now.Sub(entry.started) >= l.window {
		l.entries[key] = rateEntry{started: now, count: 1}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func Run(ctx context.Context, address string, handler http.Handler, logger *slog.Logger) error {
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("observatory server: %w", err)
	}
}
