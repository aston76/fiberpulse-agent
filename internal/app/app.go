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

	"fiberpulse.dev/agent/internal/baseline"
	"fiberpulse.dev/agent/internal/confidence"
	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/localapi"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/network"
	"fiberpulse.dev/agent/internal/reporting"
	"fiberpulse.dev/agent/internal/scheduler"
	"fiberpulse.dev/agent/internal/storage"
	"github.com/google/uuid"
)

const consentPolicyVersion = "privacy-v1"
const incidentRuntimeSetting = "incident_runtime_v1"

type Config struct {
	Version                 string
	DatabasePath            string
	Provider                measurement.Provider
	ProbeURL                string
	DNSName                 string
	SharingTransportEnabled bool
	Logger                  *slog.Logger
}

type App struct {
	config      Config
	store       *storage.Store
	inspector   network.Inspector
	health      health.Checker
	local       *localapi.Server
	scheduler   scheduler.Scheduler
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	state       string
	testMachine measurement.TestMachine
	paused      bool
	nextRun     time.Time
	lastHealth  health.Sample
	lastError   string
	incidentMu  sync.Mutex
	incident    incidents.Machine
	current     incidents.Record
	testMu      sync.Mutex
	wg          sync.WaitGroup
	closing     bool
	closeOnce   sync.Once
	closeErr    error
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
	SharingAvailable  bool                 `json:"sharing_available"`
	Baseline          baseline.Result      `json:"baseline"`
	Incidents         []incidents.Record   `json:"incidents"`
	Reports           []reporting.Record   `json:"reports"`
	LastError         string               `json:"last_error,omitempty"`
}

type persistedIncidentRuntime struct {
	Machine incidents.Snapshot `json:"machine"`
	Current incidents.Record   `json:"current"`
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
	a := &App{config: config, store: store, inspector: inspector, ctx: ctx, cancel: cancel, state: "starting", testMachine: measurement.TestMachine{State: measurement.TestIdle}}
	a.health = health.Checker{Inspector: inspector, DNSName: config.DNSName, ProbeURL: config.ProbeURL}
	a.scheduler = scheduler.Scheduler{Store: store}
	a.local = localapi.New(a)
	if err := store.RecoverInterruptedReports(context.Background(), time.Now().UTC()); err != nil {
		cancel()
		_ = store.Close()
		return nil, fmt.Errorf("recover interrupted reports: %w", err)
	}
	if err := a.restoreIncidentRuntime(context.Background()); err != nil {
		cancel()
		_ = store.Close()
		return nil, err
	}
	return a, nil
}

func (a *App) Start() (string, error) {
	mlab, _ := a.store.CurrentConsent(a.ctx, "mlab")
	_, next, paused, err := a.store.Scheduler(a.ctx)
	if err != nil {
		return "", err
	}
	if next.IsZero() || next.Before(time.Now().UTC()) {
		next = time.Now().UTC().Add(scheduler.RecoveryDelay(randomUnit()))
		_ = a.store.SetScheduler(a.ctx, "recovered", next, paused)
	}
	a.mu.Lock()
	a.paused = paused
	a.nextRun = next
	if !mlab.Granted {
		a.state = "consent_required"
	} else if paused {
		a.state = "paused"
	} else {
		a.state = "monitoring"
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

func (a *App) Wait()                 { <-a.ctx.Done() }
func (a *App) Done() <-chan struct{} { return a.ctx.Done() }

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
	recentIncidents, err := a.store.ListIncidents(ctx, 100)
	if err != nil {
		return nil, err
	}
	recentReports, err := a.store.ListReports(ctx, 100)
	if err != nil {
		return nil, err
	}
	personalBaseline := calculateBaseline(results)
	a.mu.RLock()
	defer a.mu.RUnlock()
	return Snapshot{Version: a.config.Version, State: a.state, TestState: string(a.testMachine.State), Paused: a.paused, NextAutomaticTest: a.nextRun, Provider: a.config.Provider.Metadata(), MLabConsent: mlab, SharingConsent: sharing, LastHealth: a.lastHealth, Measurements: results, ShareQueueCount: queue, SharingAvailable: a.config.SharingTransportEnabled, Baseline: personalBaseline, Incidents: recentIncidents, Reports: recentReports, LastError: a.lastError}, nil
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
		return a.SetPaused(ctx, body.Paused)
	case "consent":
		var body struct {
			Scope    string `json:"scope"`
			Granted  bool   `json:"granted"`
			Language string `json:"language"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		if body.Scope == "fiberpulse" && body.Granted && !a.config.SharingTransportEnabled {
			return errors.New("FiberPulse sharing is unavailable in this development build")
		}
		if err := a.store.SetConsent(ctx, storage.Consent{Scope: body.Scope, Granted: body.Granted, PolicyVersion: consentPolicyVersion, Language: body.Language, Source: "local_dashboard"}); err != nil {
			return err
		}
		a.mu.Lock()
		if body.Scope == "mlab" {
			if !body.Granted {
				a.state = "consent_required"
			} else if a.paused {
				a.state = "paused"
			} else {
				a.state = "monitoring"
			}
		}
		a.mu.Unlock()
		return nil
	case "quit":
		a.cancel()
		return nil
	case "incident-dismiss":
		var body struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		return a.dismissIncident(ctx, body.ID)
	case "report-delete":
		var body struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return err
		}
		return a.deleteReport(ctx, body.ID)
	default:
		return fmt.Errorf("unknown action %q", name)
	}
}

// SetPaused persists the scheduler decision before publishing it to the rest of
// the application. This keeps tray and dashboard actions on the same code path.
func (a *App) SetPaused(ctx context.Context, paused bool) error {
	a.mu.RLock()
	next := a.nextRun
	a.mu.RUnlock()
	consent, err := a.store.CurrentConsent(ctx, "mlab")
	if err != nil {
		return err
	}
	if err := a.store.SetScheduler(ctx, "waiting", next, paused); err != nil {
		return err
	}
	a.mu.Lock()
	a.paused = paused
	if !consent.Granted {
		a.state = "consent_required"
	} else if paused {
		a.state = "paused"
	} else {
		a.state = "monitoring"
	}
	a.mu.Unlock()
	return nil
}

// TogglePause is used by native tray implementations, which expose a single
// pause/resume command rather than a stateful switch.
func (a *App) TogglePause(ctx context.Context) error {
	a.mu.RLock()
	paused := !a.paused
	a.mu.RUnlock()
	return a.SetPaused(ctx, paused)
}

func (a *App) Export(ctx context.Context, format string) ([]byte, string, error) {
	if format != "csv" && format != "pdf" {
		return nil, "", errors.New("unsupported export format")
	}
	now := time.Now().UTC()
	report := reporting.Record{
		ID: uuid.NewString(), Format: format, State: reporting.Drafting,
		PeriodStart: now.AddDate(-1, -1, 0), PeriodEnd: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := a.store.SaveReport(ctx, report); err != nil {
		return nil, "", err
	}
	failFrom := func(from reporting.State, code string, cause error) ([]byte, string, error) {
		machine := reporting.Machine{State: from}
		if transitionErr := machine.Transition(reporting.Failed); transitionErr == nil {
			report.State = machine.State
			report.ErrorCode = code
			report.UpdatedAt = time.Now().UTC()
			if saveErr := a.store.SaveReport(context.Background(), report); saveErr != nil {
				cause = errors.Join(cause, saveErr)
			}
		}
		return nil, "", cause
	}
	results, err := a.store.ListResults(ctx, 10000)
	if err != nil {
		return failFrom(reporting.Drafting, "measurements.read_failed", err)
	}
	if len(results) == 0 {
		return failFrom(reporting.Drafting, "report.no_measurements", errors.New("no measurements are available for this report"))
	}
	var body []byte
	var contentType string
	switch format {
	case "csv":
		body, err = reporting.CSV(results)
		contentType = "text/csv; charset=utf-8"
	case "pdf":
		body, err = reporting.PDF(results, report.PeriodStart, report.PeriodEnd)
		contentType = "application/pdf"
	}
	if err != nil {
		return failFrom(reporting.Drafting, "report.generation_failed", err)
	}
	machine := reporting.Machine{State: report.State}
	if err := machine.Transition(reporting.Ready); err != nil {
		return failFrom(reporting.Drafting, "report.transition_failed", err)
	}
	if err := machine.Transition(reporting.Exporting); err != nil {
		return failFrom(reporting.Drafting, "report.transition_failed", err)
	}
	report.State = machine.State
	report.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveReport(ctx, report); err != nil {
		return failFrom(reporting.Drafting, "report.persistence_failed", err)
	}
	if err := machine.Transition(reporting.Exported); err != nil {
		return failFrom(reporting.Exporting, "report.transition_failed", err)
	}
	report.State = machine.State
	report.ByteCount = int64(len(body))
	report.UpdatedAt = time.Now().UTC()
	if err := a.store.SaveReport(ctx, report); err != nil {
		return failFrom(reporting.Exporting, "report.persistence_failed", err)
	}
	return body, contentType, nil
}

func (a *App) deleteReport(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("report id is required")
	}
	report, err := a.store.GetReport(ctx, id)
	if err != nil {
		return err
	}
	machine := reporting.Machine{State: report.State}
	if err := machine.Transition(reporting.Deleted); err != nil {
		return err
	}
	report.State = machine.State
	report.UpdatedAt = time.Now().UTC()
	return a.store.SaveReport(ctx, report)
}

func (a *App) transitionTest(next measurement.TestState) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.testMachine.Transition(next)
}

func (a *App) rejectTestStart(cause error) error {
	terminal := measurement.TestFailed
	if errors.Is(cause, context.Canceled) {
		terminal = measurement.TestCancelled
	}
	if err := a.transitionTest(terminal); err != nil {
		cause = errors.Join(cause, err)
	}
	if err := a.transitionTest(measurement.TestIdle); err != nil {
		cause = errors.Join(cause, err)
	}
	return cause
}

func (a *App) advanceTestProgress(phase string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch measurement.TestState(phase) {
	case measurement.TestDownload:
		if a.testMachine.State == measurement.TestDownload {
			return nil
		}
		return a.testMachine.Transition(measurement.TestDownload)
	case measurement.TestUpload:
		if a.testMachine.State == measurement.TestUpload {
			return nil
		}
		return a.testMachine.Transition(measurement.TestUpload)
	default:
		return fmt.Errorf("unsupported measurement progress phase %q", phase)
	}
}

func (a *App) StartTest(ctx context.Context, kind scheduler.Kind) error {
	a.testMu.Lock()
	defer a.testMu.Unlock()
	a.mu.RLock()
	busy := a.testMachine.State != measurement.TestIdle || a.closing
	networkContext := a.lastHealth.Network
	a.mu.RUnlock()
	if busy {
		return errors.New("a measurement is already running")
	}
	if err := a.transitionTest(measurement.TestPreflight); err != nil {
		return err
	}
	consent, err := a.store.CurrentConsent(ctx, "mlab")
	if err != nil {
		return a.rejectTestStart(err)
	}
	preflight, err := a.config.Provider.Preflight(ctx, networkContext, consent.Granted)
	if err != nil {
		return a.rejectTestStart(err)
	}
	if !preflight.Eligible {
		return a.rejectTestStart(measurement.ErrNetworkIneligible)
	}
	if err := a.scheduler.Reserve(ctx, kind); err != nil {
		return a.rejectTestStart(err)
	}
	if err := a.transitionTest(measurement.TestQuotaReserved); err != nil {
		return a.rejectTestStart(err)
	}
	a.mu.Lock()
	a.wg.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.wg.Done()
		a.runTest(kind, preflight.Network)
	}()
	return nil
}

func (a *App) runTest(kind scheduler.Kind, before measurement.NetworkContext) {
	lifecycleErr := a.transitionTest(measurement.TestLocate)
	var lifecycleMu sync.Mutex
	a.mu.Lock()
	a.lastError = ""
	a.mu.Unlock()
	result, runErr := a.config.Provider.Run(a.ctx, before, func(p measurement.Progress) {
		if err := a.advanceTestProgress(p.Phase); err != nil {
			lifecycleMu.Lock()
			lifecycleErr = errors.Join(lifecycleErr, err)
			lifecycleMu.Unlock()
		}
	})
	lifecycleMu.Lock()
	runErr = errors.Join(runErr, lifecycleErr)
	lifecycleMu.Unlock()
	after, _ := a.inspector.Snapshot()
	result.NetworkAfter = after
	score := confidence.Calculate(confidence.Input{Complete: result.Status == measurement.StatusComplete, Cancelled: result.Status == measurement.StatusCancelled, ImpossibleValue: result.DownloadBPS < 0 || result.UploadBPS < 0 || result.MinRTTUS < 0, NonPublicProvider: result.Provider != measurement.ProviderMLabNDT7, InterfaceChanged: before.InterfaceID != "" && after.InterfaceID != "" && before.InterfaceID != after.InterfaceID, RouteChanged: before.RouteID != "" && after.RouteID != "" && before.RouteID != after.RouteID, Metered: before.Metered, ConnectionType: before.ConnectionType, WiFiQuality: before.WiFiQuality, VPNSuspected: before.VPNDetected, ProxySuspected: before.ProxyDetected})
	result.ConfidenceScore = score.Score
	result.ConfidenceLevel = score.Level
	result.ConfidenceReasons = score.Reasons
	result.PublicEligible = score.PublicEligible
	if err := a.transitionTest(measurement.TestValidate); err != nil {
		runErr = errors.Join(runErr, err)
	}
	if err := a.transitionTest(measurement.TestPersist); err != nil {
		runErr = errors.Join(runErr, err)
	}
	saved := true
	if err := a.store.SaveResult(context.Background(), result); err != nil {
		saved = false
		runErr = errors.Join(runErr, err)
	}
	sharing, _ := a.store.CurrentConsent(context.Background(), "fiberpulse")
	queued := false
	if saved && a.config.SharingTransportEnabled && sharing.Granted && result.Provider == measurement.ProviderMLabNDT7 {
		if err := a.store.QueueShare(context.Background(), result.ID, "measurement", sharedMeasurement(result)); err != nil {
			runErr = errors.Join(runErr, err)
		} else {
			queued = true
			if err := a.transitionTest(measurement.TestShareQueued); err != nil {
				runErr = errors.Join(runErr, err)
			}
		}
	}
	terminal := measurement.TestFailed
	if result.Status == measurement.StatusCancelled || errors.Is(runErr, context.Canceled) {
		terminal = measurement.TestCancelled
	} else if saved && result.Status == measurement.StatusComplete && runErr == nil {
		terminal = measurement.TestComplete
	}
	if err := a.transitionTest(terminal); err != nil {
		runErr = errors.Join(runErr, err)
	}
	if err := a.transitionTest(measurement.TestIdle); err != nil {
		runErr = errors.Join(runErr, err)
	}
	a.mu.Lock()
	if runErr != nil {
		a.lastError = runErr.Error()
	}
	if kind == scheduler.Automatic {
		a.nextRun = time.Now().UTC().Add(scheduler.NextInterval(2, randomUnit()))
	}
	next := a.nextRun
	paused := a.paused
	a.mu.Unlock()
	_ = a.store.SetScheduler(context.Background(), "waiting", next, paused)
	a.config.Logger.Info("measurement completed", "kind", kind, "status", result.Status, "share_queued", queued, "error", runErr)
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
	if err := a.processHealthSample(context.Background(), sample); err != nil {
		a.mu.Lock()
		a.lastError = err.Error()
		a.mu.Unlock()
		a.config.Logger.Error("health sample persistence failed", "error", err)
	}
}

func (a *App) processHealthSample(ctx context.Context, sample health.Sample) error {
	if sample.At.IsZero() {
		sample.At = time.Now().UTC()
	}
	a.mu.Lock()
	a.lastHealth = sample
	a.mu.Unlock()
	if err := a.store.SaveHealth(ctx, sample); err != nil {
		return err
	}
	return a.observeIncident(ctx, sample)
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
			due := !a.paused && !a.nextRun.IsZero() && !time.Now().UTC().Before(a.nextRun) && a.testMachine.State == measurement.TestIdle
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

func calculateBaseline(results []measurement.Result) baseline.Result {
	samples := make([]baseline.Sample, 0, len(results))
	for _, result := range results {
		samples = append(samples, baseline.Sample{
			At:             result.StartedAt,
			DownloadBPS:    result.DownloadBPS,
			UploadBPS:      result.UploadBPS,
			MinRTTUS:       result.MinRTTUS,
			HighConfidence: result.Status == measurement.StatusComplete && result.ConfidenceLevel == "high",
		})
	}
	return baseline.Calculate(samples)
}

func (a *App) restoreIncidentRuntime(ctx context.Context) error {
	var persisted persistedIncidentRuntime
	found, err := a.store.GetSetting(ctx, incidentRuntimeSetting, &persisted)
	if err != nil {
		return fmt.Errorf("restore incident runtime: %w", err)
	}
	if !found {
		return nil
	}
	if err := a.incident.Restore(persisted.Machine); err != nil {
		return fmt.Errorf("restore incident machine: %w", err)
	}
	a.current = persisted.Current
	return nil
}

func (a *App) observeIncident(ctx context.Context, sample health.Sample) error {
	a.incidentMu.Lock()
	defer a.incidentMu.Unlock()
	previousMachine := a.incident.Snapshot()
	previousCurrent := a.current
	degraded := sample.State == "offline" || sample.State == "internet_degraded"
	category := sample.Category
	if category == "" {
		category = "unknown"
	}
	previous := a.incident.State
	state := a.incident.Observe(incidents.Observation{At: sample.At.UTC(), Degraded: degraded, Category: category})
	var incidentToSave *incidents.Record
	var incidentToDelete string
	if previous != state {
		switch state {
		case incidents.Suspected:
			a.current = incidents.Record{ID: uuid.NewString(), Category: a.incident.Category, State: state, SuspectedAt: a.incident.SuspectedAt, UpdatedAt: sample.At.UTC()}
		case incidents.Active:
			a.current.State = state
			if a.current.ActiveAt.IsZero() {
				a.current.ActiveAt = sample.At.UTC()
			}
			a.current.UpdatedAt = sample.At.UTC()
		case incidents.Recovering:
			a.current.State = state
			a.current.RecoveringAt = sample.At.UTC()
			a.current.UpdatedAt = sample.At.UTC()
		case incidents.Resolved:
			a.current.State = state
			a.current.ResolvedAt = sample.At.UTC()
			a.current.UpdatedAt = sample.At.UTC()
		case incidents.None:
			if previous == incidents.Suspected {
				incidentToDelete = a.current.ID
				a.current = incidents.Record{}
			}
		}
		if a.current.ID != "" {
			incidentToSave = &a.current
		}
	}
	runtime := persistedIncidentRuntime{Machine: a.incident.Snapshot(), Current: a.current}
	if err := a.store.PersistIncidentRuntime(ctx, incidentToSave, incidentToDelete, incidentRuntimeSetting, runtime); err != nil {
		if restoreErr := a.incident.Restore(previousMachine); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore incident state after persistence failure: %w", restoreErr))
		}
		a.current = previousCurrent
		return err
	}
	return nil
}

func (a *App) dismissIncident(ctx context.Context, id string) error {
	a.incidentMu.Lock()
	defer a.incidentMu.Unlock()
	if id == "" || a.current.ID != id || a.incident.State != incidents.Active {
		return errors.New("only the active incident can be dismissed")
	}
	dismissed := a.current
	dismissed.State = incidents.Dismissed
	dismissed.UpdatedAt = time.Now().UTC()
	reset := incidents.Machine{State: incidents.None}
	runtime := persistedIncidentRuntime{Machine: reset.Snapshot(), Current: incidents.Record{}}
	if err := a.store.PersistIncidentRuntime(ctx, &dismissed, "", incidentRuntimeSetting, runtime); err != nil {
		return err
	}
	a.incident = reset
	a.current = incidents.Record{}
	return nil
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
