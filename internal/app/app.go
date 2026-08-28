package app

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"fiberpulse.dev/agent/internal/confidence"
	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/localapi"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/network"
	"fiberpulse.dev/agent/internal/reporting"
	"fiberpulse.dev/agent/internal/scheduler"
	"fiberpulse.dev/agent/internal/storage"
)

const consentPolicyVersion = "privacy-v1"

type Config struct {
	Version      string
	DatabasePath string
	Provider     measurement.Provider
	ProbeURL     string
	DNSName      string
	Logger       *slog.Logger
}

type App struct {
	config     Config
	store      *storage.Store
	inspector  network.Inspector
	health     health.Checker
	local      *localapi.Server
	scheduler  scheduler.Scheduler
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	state      string
	testState  string
	paused     bool
	nextRun    time.Time
	lastHealth health.Sample
	lastError  string
	testMu     sync.Mutex
	wg         sync.WaitGroup
	closing    bool
	closeOnce  sync.Once
	closeErr   error
}

type Snapshot struct {
	Version           string               `json:"version"`
	State             string               `json:"state"`
	TestState         string               `json:"test_state"`
	Paused            bool                 `json:"paused"`
	NextAutomaticTest time.Time            `json:"next_automatic_test,omitempty"`
	Provider          measurement.Metadata `json:"provider"`
	MLabConsent       storage.Consent      `json:"mlab_consent"`
	SharingConsent    storage.Consent      `json:"sharing_consent"`
	LastHealth        health.Sample        `json:"last_health"`
	Measurements      []measurement.Result `json:"measurements"`
	ShareQueueCount   int                  `json:"share_queue_count"`
	LastError         string               `json:"last_error,omitempty"`
}

func New(config Config) (*App, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Provider == nil {
		return nil, errors.New("measurement provider is required")
	}
	store, err := storage.Open(config.DatabasePath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	inspector := network.SystemInspector{}
	a := &App{config: config, store: store, inspector: inspector, ctx: ctx, cancel: cancel, state: "starting", testState: "idle"}
	a.health = health.Checker{Inspector: inspector, DNSName: config.DNSName, ProbeURL: config.ProbeURL}
	a.scheduler = scheduler.Scheduler{Store: store}
	a.local = localapi.New(a)
	return a, nil
}

func (a *App) Start() (string, error) {
	mlab, _ := a.store.CurrentConsent(a.ctx, "mlab")
	state, next, paused, err := a.store.Scheduler(a.ctx)
	if err != nil {
		return "", err
	}
	if next.IsZero() || next.Before(time.Now().UTC()) {
		next = time.Now().UTC().Add(scheduler.RecoveryDelay(randomUnit()))
		state = "recovered"
		_ = a.store.SetScheduler(a.ctx, state, next, paused)
	}
	a.mu.Lock()
	a.paused = paused
	a.nextRun = next
	if mlab.Granted {
		a.state = "monitoring"
	} else {
		a.state = "consent_required"
	}
	a.mu.Unlock()
	if err := a.local.Start(); err != nil {
		return "", err
	}
	a.runAsync(a.healthLoop)
	a.runAsync(a.schedulerLoop)
	a.runAsync(a.retentionLoop)
	a.config.Logger.Info("FiberPulse started", "dashboard", a.local.BaseURL(), "provider", a.config.Provider.Metadata().Name)
	return a.local.BootstrapURL(), nil
}

func (a *App) Wait() { <-a.ctx.Done() }

func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.testMu.Lock()
		a.mu.Lock()
		a.closing = true
		a.state = "stopping"
		next := a.nextRun
		paused := a.paused
		a.mu.Unlock()
		a.testMu.Unlock()
		a.cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		serverErr := a.local.Close(ctx)
		done := make(chan struct{})
		go func() {
			a.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			_ = a.store.SetScheduler(ctx, "stopped", next, paused)
			a.closeErr = errors.Join(serverErr, a.store.Close())
			a.mu.Lock()
			a.state = "stopped"
			a.mu.Unlock()
		case <-ctx.Done():
			a.closeErr = errors.Join(serverErr, errors.New("graceful shutdown exceeded 10 seconds"))
		}
	})
	return a.closeErr
}

func (a *App) runAsync(fn func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		fn()
	}()
}

func (a *App) BootstrapURL() string { return a.local.BootstrapURL() }

func (a *App) Snapshot(ctx context.Context) (any, error) {
	mlab, err := a.store.CurrentConsent(ctx, "mlab")
	if err != nil {
		return nil, err
	}
	sharing, err := a.store.CurrentConsent(ctx, "fiberpulse")
	if err != nil {
		return nil, err
	}
	results, err := a.store.ListResults(ctx, 100)
	if err != nil {
		return nil, err
	}
	queue, _ := a.store.ShareQueueCount(ctx)
	a.mu.RLock()
	defer a.mu.RUnlock()
	return Snapshot{Version: a.config.Version, State: a.state, TestState: a.testState, Paused: a.paused, NextAutomaticTest: a.nextRun, Provider: a.config.Provider.Metadata(), MLabConsent: mlab, SharingConsent: sharing, LastHealth: a.lastHealth, Measurements: results, ShareQueueCount: queue, LastError: a.lastError}, nil
}

func (a *App) Action(ctx context.Context, name string, raw json.RawMessage) error {
	switch name {
	case "test":
		return a.StartTest(ctx, scheduler.Manual)
	case "pause":
		var body struct {
			Paused bool `json:"paused"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		a.mu.Lock()
		a.paused = body.Paused
		if body.Paused {
			a.state = "paused"
		} else {
			a.state = "monitoring"
		}
		next := a.nextRun
		a.mu.Unlock()
		return a.store.SetScheduler(ctx, "waiting", next, body.Paused)
	case "consent":
		var body struct {
			Scope    string `json:"scope"`
			Granted  bool   `json:"granted"`
			Language string `json:"language"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if err := a.store.SetConsent(ctx, storage.Consent{Scope: body.Scope, Granted: body.Granted, PolicyVersion: consentPolicyVersion, Language: body.Language, Source: "local_dashboard"}); err != nil {
			return err
		}
		a.mu.Lock()
		if body.Scope == "mlab" {
			if body.Granted {
				a.state = "monitoring"
			} else {
				a.state = "consent_required"
			}
		}
		a.mu.Unlock()
		return nil
	case "quit":
		a.cancel()
		return nil
	default:
		return fmt.Errorf("unknown action %q", name)
	}
}

func (a *App) Export(ctx context.Context, format string) ([]byte, string, error) {
	results, err := a.store.ListResults(ctx, 1000)
	if err != nil {
		return nil, "", err
	}
	end := time.Now().UTC()
	start := end.AddDate(-1, -1, 0)
	switch format {
	case "csv":
		body, err := reporting.CSV(results)
		return body, "text/csv; charset=utf-8", err
	case "pdf":
		body, err := reporting.PDF(results, start, end)
		return body, "application/pdf", err
	default:
		return nil, "", errors.New("unsupported export format")
	}
}

func (a *App) StartTest(ctx context.Context, kind scheduler.Kind) error {
	a.testMu.Lock()
	defer a.testMu.Unlock()
	a.mu.RLock()
	busy := a.testState != "idle" || a.closing
	networkContext := a.lastHealth.Network
	a.mu.RUnlock()
	if busy {
		return errors.New("a measurement is already running")
	}
	consent, err := a.store.CurrentConsent(ctx, "mlab")
	if err != nil {
		return err
	}
	preflight, err := a.config.Provider.Preflight(ctx, networkContext, consent.Granted)
	if err != nil {
		return err
	}
	if !preflight.Eligible {
		return measurement.ErrNetworkIneligible
	}
	if err := a.scheduler.Reserve(ctx, kind); err != nil {
		return err
	}
	a.mu.Lock()
	a.testState = "reserved"
	a.wg.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.wg.Done()
		a.runTest(kind, preflight.Network)
	}()
	return nil
}

func (a *App) runTest(kind scheduler.Kind, before measurement.NetworkContext) {
	a.mu.Lock()
	a.testState = "running"
	a.lastError = ""
	a.mu.Unlock()
	result, runErr := a.config.Provider.Run(a.ctx, before, func(p measurement.Progress) { a.mu.Lock(); a.testState = p.Phase; a.mu.Unlock() })
	after, _ := a.inspector.Snapshot()
	result.NetworkAfter = after
	score := confidence.Calculate(confidence.Input{Complete: result.Status == measurement.StatusComplete, Cancelled: result.Status == measurement.StatusCancelled, ImpossibleValue: result.DownloadBPS < 0 || result.UploadBPS < 0 || result.MinRTTUS < 0, InterfaceChanged: before.InterfaceID != "" && after.InterfaceID != "" && before.InterfaceID != after.InterfaceID, RouteChanged: before.RouteID != "" && after.RouteID != "" && before.RouteID != after.RouteID, Metered: before.Metered, ConnectionType: before.ConnectionType, WiFiQuality: before.WiFiQuality, VPNSuspected: before.VPNDetected, ProxySuspected: before.ProxyDetected})
	result.ConfidenceScore = score.Score
	result.ConfidenceLevel = score.Level
	result.ConfidenceReasons = score.Reasons
	result.PublicEligible = score.PublicEligible
	if err := a.store.SaveResult(context.Background(), result); err != nil {
		runErr = errors.Join(runErr, err)
	}
	sharing, _ := a.store.CurrentConsent(context.Background(), "fiberpulse")
	if sharing.Granted {
		_ = a.store.QueueShare(context.Background(), result.ID, "measurement", sharedMeasurement(result))
	}
	a.mu.Lock()
	a.testState = "idle"
	if runErr != nil {
		a.lastError = runErr.Error()
	}
	a.nextRun = time.Now().UTC().Add(scheduler.NextInterval(2, randomUnit()))
	next := a.nextRun
	paused := a.paused
	a.mu.Unlock()
	_ = a.store.SetScheduler(context.Background(), "waiting", next, paused)
	a.config.Logger.Info("measurement completed", "kind", kind, "status", result.Status, "error", runErr)
}

func (a *App) healthLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	a.captureHealth()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.captureHealth()
		}
	}
}
func (a *App) captureHealth() {
	sample := a.health.Check(a.ctx)
	a.mu.Lock()
	a.lastHealth = sample
	a.mu.Unlock()
	_ = a.store.SaveHealth(context.Background(), sample)
}

func (a *App) schedulerLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			due := !a.paused && !a.nextRun.IsZero() && !time.Now().UTC().Before(a.nextRun) && a.testState == "idle"
			a.mu.RUnlock()
			if due {
				if err := a.StartTest(a.ctx, scheduler.Automatic); err != nil {
					a.mu.Lock()
					a.lastError = err.Error()
					a.nextRun = time.Now().UTC().Add(time.Hour)
					next := a.nextRun
					paused := a.paused
					a.mu.Unlock()
					_ = a.store.SetScheduler(context.Background(), "blocked", next, paused)
				}
			}
		}
	}
}
func (a *App) retentionLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case now := <-ticker.C:
			_ = a.store.PurgeExpired(context.Background(), now.UTC())
		}
	}
}

func randomUnit() float64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return .5
	}
	return float64(binary.BigEndian.Uint64(b[:])>>11) / float64(uint64(1)<<53)
}

func sharedMeasurement(r measurement.Result) map[string]any {
	bucket := r.StartedAt.UTC().Truncate(15 * time.Minute)
	quality := "unknown"
	if r.NetworkBefore.ConnectionType == measurement.ConnectionWiFi {
		switch {
		case r.NetworkBefore.WiFiQuality >= 80:
			quality = "excellent"
		case r.NetworkBefore.WiFiQuality >= 60:
			quality = "good"
		case r.NetworkBefore.WiFiQuality >= 40:
			quality = "fair"
		case r.NetworkBefore.WiFiQuality > 0:
			quality = "poor"
		}
	}
	return map[string]any{"event_id": r.ID, "timestamp_bucket": bucket.Format(time.RFC3339), "provider": r.Provider, "protocol_version": r.ProtocolVersion, "agent_version": r.ClientVersion, "schema_version": measurement.SchemaVersion, "methodology_version": measurement.MethodologyVersion, "confidence_version": measurement.ConfidenceVersion, "status": r.Status, "server_fqdn": r.ServerFQDN, "download_bps": r.DownloadBPS, "upload_bps": r.UploadBPS, "min_rtt_us": r.MinRTTUS, "bytes_down": r.BytesDown, "bytes_up": r.BytesUp, "connection_type": r.NetworkBefore.ConnectionType, "wifi_quality_bucket": quality, "metered": r.NetworkBefore.Metered, "vpn_suspected": r.NetworkBefore.VPNDetected, "proxy_suspected": r.NetworkBefore.ProxyDetected, "confidence_score": r.ConfidenceScore, "confidence_level": r.ConfidenceLevel, "confidence_reasons": r.ConfidenceReasons}
}
