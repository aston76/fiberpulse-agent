package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/accountsync"
	"fiberpulse.dev/agent/internal/complaint"
	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/observatory"
	"fiberpulse.dev/agent/internal/plan"
	"fiberpulse.dev/agent/internal/reporting"
	"fiberpulse.dev/agent/internal/scheduler"
	"fiberpulse.dev/agent/internal/sharing"
	"fiberpulse.dev/agent/internal/storage"
)

type inspectorFunc func() (measurement.NetworkContext, error)

func (f inspectorFunc) Snapshot() (measurement.NetworkContext, error) { return f() }

func newTestApp(t *testing.T) *App {
	t.Helper()
	a, err := New(Config{
		Version:      "test",
		DatabasePath: filepath.Join(t.TempDir(), "fiberpulse.db"),
		Provider:     &measurement.FakeProvider{Delay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	a.inspector = inspectorFunc(func() (measurement.NetworkContext, error) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		return a.lastHealth.Network, nil
	})
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("close app: %v", err)
		}
	})
	return a
}

func TestAccountSyncUploadsLocalAndImportsRemoteHistory(t *testing.T) {
	a := newTestApp(t)
	started := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	result := func(id string) measurement.Result {
		return measurement.Result{
			ID: id, Provider: measurement.ProviderMLabNDT7, ProtocolVersion: "ndt7", ClientVersion: "0.1.2",
			SchemaVersion: measurement.SchemaVersion, MethodologyVersion: measurement.MethodologyVersion, ConfidenceVersion: measurement.ConfidenceVersion,
			StartedAt: started, CompletedAt: started.Add(10 * time.Second), DownloadBPS: 400_000_000, UploadBPS: 100_000_000,
			MinRTTUS: 8_000, BytesDown: 50_000_000, BytesUp: 12_500_000, DownloadDurationUS: 5_000_000, UploadDurationUS: 5_000_000,
			Status: measurement.StatusComplete, NetworkBefore: measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet, Online: true, CapturedAt: started},
			NetworkAfter:    measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet, Online: true, CapturedAt: started.Add(10 * time.Second)},
			ConfidenceScore: 97, ConfidenceLevel: "high", PublicEligible: true,
		}
	}
	local := result("measurement_local_001")
	remote := result("measurement_remote_001")
	if err := a.store.SaveResult(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	uploadedLocal := false
	uploadRequests := 0
	storedResults := map[string]measurement.Result{remote.ID: remote}
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Header.Get("Authorization") != "Bearer account-token" {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method + " " + request.URL.Path {
		case http.MethodGet + " /v1/device/session":
			_, _ = response.Write([]byte(`{"email":"owner@example.com","plan":"pro","subscriptionStatus":"active","googleLinked":true}`))
		case http.MethodPost + " /v1/device/measurements":
			uploadRequests++
			var payload struct {
				Items []struct {
					ID     string             `json:"id"`
					Result measurement.Result `json:"result"`
				} `json:"items"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Error(err)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
			for _, item := range payload.Items {
				storedResults[item.ID] = item.Result
				if item.ID == local.ID && item.Result.ID == local.ID {
					uploadedLocal = true
				}
			}
			_, _ = response.Write([]byte(`{"accepted":1}`))
		case http.MethodGet + " /v1/device/measurements":
			items := make([]any, 0, len(storedResults))
			for id, stored := range storedResults {
				items = append(items, map[string]any{"id": id, "result": stored})
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"items": items})
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := accountsync.New(server.URL, server.Client().Transport)
	if err != nil {
		t.Fatal(err)
	}
	a.accountClient = client
	if err := a.store.SetSetting(context.Background(), accountConnectionSetting, persistedAccountConnection{AccessToken: "account-token"}); err != nil {
		t.Fatal(err)
	}
	if err := a.syncAccount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !uploadedLocal {
		t.Fatal("local measurement was not uploaded")
	}
	if err := a.syncAccount(context.Background()); err != nil {
		t.Fatal(err)
	}
	if uploadRequests != 1 {
		t.Fatalf("unchanged history caused %d upload requests", uploadRequests)
	}
	results, err := a.store.ListResults(context.Background(), 10)
	if err != nil || len(results) != 2 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	for _, synced := range results {
		if synced.ID == remote.ID && (synced.PublicEligible || !containsString(synced.ConfidenceReasons, "account.synced_copy")) {
			t.Fatalf("remote measurement was not imported privately: %+v", synced)
		}
	}
	var saved persistedAccountConnection
	if found, err := a.store.GetSetting(context.Background(), accountConnectionSetting, &saved); err != nil || !found || saved.Email != "owner@example.com" || saved.Plan != "pro" || !saved.GoogleLinked || saved.LastSyncAt.IsZero() {
		t.Fatalf("saved account=%+v found=%v err=%v", saved, found, err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestPauseStateNeverHidesMissingMLabConsent(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.SetPaused(ctx, true); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecycleConsentRequired || !a.paused {
		t.Fatalf("state=%q paused=%v", a.lifecycle.State, a.paused)
	}
	if err := a.Action(ctx, "consent", []byte(`{"scope":"mlab","granted":true,"language":"en"}`)); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecyclePaused {
		t.Fatalf("grant while paused produced state %q", a.lifecycle.State)
	}
	if err := a.SetPaused(ctx, false); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecycleMonitoring || a.paused {
		t.Fatalf("state=%q paused=%v", a.lifecycle.State, a.paused)
	}
}

func TestStartRestoresPersistedPausedState(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.SetScheduler(ctx, "waiting", time.Now().UTC().Add(time.Hour), true); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Start(); err != nil {
		t.Fatal(err)
	}
	if a.lifecycle.State != LifecyclePaused || !a.paused {
		t.Fatalf("state=%q paused=%v", a.lifecycle.State, a.paused)
	}
}

func TestStartRepairsStoppedSchedulerStateWithoutAdvancingFutureRun(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	next := time.Now().UTC().Add(2 * time.Hour).Round(time.Second)
	if err := a.store.SetScheduler(ctx, scheduler.Stopped, next, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Start(); err != nil {
		t.Fatal(err)
	}
	state, storedNext, paused, err := a.store.Scheduler(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state != scheduler.Waiting || paused || !storedNext.Equal(next) {
		t.Fatalf("state=%s next=%s paused=%v", state, storedNext, paused)
	}
}

func TestStartClampsLegacyLongAutomaticDelay(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	if err := a.store.SetScheduler(ctx, scheduler.Waiting, time.Now().UTC().Add(48*time.Hour), false); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	if _, err := a.Start(); err != nil {
		t.Fatal(err)
	}
	_, next, _, err := a.store.Scheduler(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if next.After(started.Add(9*time.Hour + 15*time.Minute)) {
		t.Fatalf("legacy delay was not clamped: %s", next.Sub(started))
	}
}

func TestStartDisablesAutomaticSchedulerWithoutConsent(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.Start(); err != nil {
		t.Fatal(err)
	}
	state, _, _, err := a.store.Scheduler(context.Background())
	if err != nil || state != scheduler.Disabled {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestPausedManualMeasurementDoesNotReenableAutomaticScheduler(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}
	if err := a.SetPaused(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := a.StartTest(ctx, scheduler.Manual); err != nil {
		t.Fatal(err)
	}
	a.wg.Wait()
	state, _, paused, err := a.store.Scheduler(ctx)
	if err != nil || state != scheduler.Disabled || !paused {
		t.Fatalf("state=%s paused=%v err=%v", state, paused, err)
	}
}

func TestAutomaticFailurePersistsMeteredBlock(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.schedule.State = scheduler.Due
	a.lastHealth.Network = measurement.NetworkContext{Online: true, Metered: true}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	a.handleAutomaticStartFailure(measurement.ErrNetworkIneligible, now)
	state, next, _, err := a.store.Scheduler(ctx)
	if err != nil || state != scheduler.BlockedMetered || !next.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("state=%s next=%s err=%v", state, next, err)
	}
}

type blockingPreflightProvider struct {
	measurement.FakeProvider
	entered chan struct{}
	release chan struct{}
}

func (p *blockingPreflightProvider) Preflight(ctx context.Context, network measurement.NetworkContext, consent bool) (measurement.PreflightResult, error) {
	close(p.entered)
	select {
	case <-ctx.Done():
		return measurement.PreflightResult{}, ctx.Err()
	case <-p.release:
		return p.FakeProvider.Preflight(ctx, network, consent)
	}
}

func TestPauseCannotRaceAutomaticReservation(t *testing.T) {
	a := newTestApp(t)
	provider := &blockingPreflightProvider{FakeProvider: measurement.FakeProvider{Delay: time.Millisecond}, entered: make(chan struct{}), release: make(chan struct{})}
	a.config.Provider = provider
	a.schedule.State = scheduler.Waiting
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}
	startResult := make(chan error, 1)
	go func() { startResult <- a.StartTest(ctx, scheduler.Automatic) }()
	<-provider.entered
	pauseResult := make(chan error, 1)
	go func() { pauseResult <- a.SetPaused(ctx, true) }()
	select {
	case err := <-pauseResult:
		t.Fatalf("pause bypassed in-progress reservation: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(provider.release)
	if err := <-startResult; err != nil {
		t.Fatal(err)
	}
	if err := <-pauseResult; err != nil {
		t.Fatal(err)
	}
	a.wg.Wait()
	state, _, paused, err := a.store.Scheduler(ctx)
	if err != nil || state != scheduler.Disabled || !paused {
		t.Fatalf("state=%s paused=%v err=%v", state, paused, err)
	}
}

func TestConsentRejectsUnknownScope(t *testing.T) {
	a := newTestApp(t)
	if err := a.Action(context.Background(), "consent", []byte(`{"scope":"invented","granted":true,"language":"en"}`)); err == nil {
		t.Fatal("unknown consent scope was stored")
	}
}

func TestPlanSelectionDrivesSnapshotVerdict(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.runTest(scheduler.Manual, measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet})

	snapshot := func() Snapshot {
		raw, err := a.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return raw.(Snapshot)
	}
	if got := snapshot().Plan; got != nil {
		t.Fatalf("plan without selection: %+v", got)
	}
	if err := a.Action(ctx, "plan", []byte(`{"offer_id":"invented"}`)); !errors.Is(err, plan.ErrUnknownOffer) {
		t.Fatalf("unknown offer accepted: %v", err)
	}

	// The fake provider measures 100 Mbps down: exactly the DITO 100 plan.
	if err := a.Action(ctx, "plan", []byte(`{"offer_id":"dito-wowfi-pro"}`)); err != nil {
		t.Fatal(err)
	}
	state := snapshot().Plan
	if state == nil || state.Verdict == nil {
		t.Fatalf("missing plan verdict: %+v", state)
	}
	if state.Verdict.Level != plan.LevelOnPar || state.Verdict.DownloadPct != 100 || state.Verdict.ComplaintWorthy {
		t.Fatalf("unexpected on-par verdict: %+v", state.Verdict)
	}

	// The same measurement against a 400 Mbps plan is complaint-worthy.
	if err := a.Action(ctx, "plan", []byte(`{"offer_id":"pldt-unli-1699"}`)); err != nil {
		t.Fatal(err)
	}
	state = snapshot().Plan
	if state == nil || state.Verdict == nil || state.Verdict.Level != plan.LevelWellBelow || !state.Verdict.ComplaintWorthy {
		t.Fatalf("unexpected well-below verdict: %+v", state)
	}

	if err := a.Action(ctx, "plan", []byte(`{"offer_id":""}`)); err != nil {
		t.Fatal(err)
	}
	if got := snapshot().Plan; got != nil {
		t.Fatalf("cleared plan still present: %+v", got)
	}
}

func TestCustomPlanSelectionAndVPNPreflight(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet, VPNDetected: true}
	if err := a.StartTest(ctx, scheduler.Manual); !errors.Is(err, measurement.ErrVPNDetected) {
		t.Fatalf("VPN test was not blocked: %v", err)
	}
	if a.testMachine.State != measurement.TestIdle {
		t.Fatalf("blocked VPN test left state %q", a.testMachine.State)
	}
	results, err := a.store.ListResults(ctx, 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("blocked VPN test stored results=%d err=%v", len(results), err)
	}

	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}
	a.runTest(scheduler.Manual, a.lastHealth.Network)
	if err := a.Action(ctx, "plan", []byte(`{"custom":{"isp":"Regional ISP","name":"Home 500","download_mbps":500,"upload_mbps":100}}`)); err != nil {
		t.Fatal(err)
	}
	raw, err := a.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state := raw.(Snapshot).Plan
	if state == nil || !state.Offer.Custom || state.Offer.ID != "custom" || state.Verdict == nil || state.Verdict.DownloadPct != 20 {
		t.Fatalf("unexpected custom plan state: %+v", state)
	}
}

func TestDevelopmentMeasurementIsLocalOnlyAndKeepsAutomaticSchedule(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	for _, scope := range []string{"mlab", "fiberpulse"} {
		if err := a.store.SetConsent(ctx, storage.Consent{Scope: scope, Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
			t.Fatal(err)
		}
	}
	next := time.Now().UTC().Add(2 * time.Hour).Round(time.Second)
	a.nextRun = next
	a.testMachine.State = measurement.TestQuotaReserved
	a.runTest(scheduler.Manual, measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet})
	results, err := a.store.ListResults(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	result := results[0]
	if result.PublicEligible || result.Provider != measurement.ProviderDevelopmentFake {
		t.Fatalf("development result became publishable: %+v", result)
	}
	if len(result.ConfidenceReasons) == 0 || result.ConfidenceReasons[0] != "provider.not_public" {
		t.Fatalf("missing local-only reason: %+v", result.ConfidenceReasons)
	}
	queued, err := a.store.ShareQueueCount(ctx)
	if err != nil || queued != 0 {
		t.Fatalf("development result queued=%d err=%v", queued, err)
	}
	if !a.nextRun.Equal(next) {
		t.Fatalf("manual test moved automatic schedule from %s to %s", next, a.nextRun)
	}
}

type invalidProgressProvider struct{ measurement.FakeProvider }

func (p *invalidProgressProvider) Run(ctx context.Context, network measurement.NetworkContext, progress func(measurement.Progress)) (measurement.Result, error) {
	result, err := p.FakeProvider.Run(ctx, network, progress)
	progress(measurement.Progress{Phase: "invented_phase"})
	return result, err
}

func TestMeasurementLifecycleRejectsProviderPhaseAndReturnsIdle(t *testing.T) {
	a := newTestApp(t)
	a.config.Provider = &invalidProgressProvider{FakeProvider: measurement.FakeProvider{Delay: time.Millisecond}}
	ctx := context.Background()
	if err := a.store.SetConsent(ctx, storage.Consent{Scope: "mlab", Granted: true, PolicyVersion: consentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}
	if err := a.StartTest(ctx, scheduler.Manual); err != nil {
		t.Fatal(err)
	}
	a.wg.Wait()
	if a.testMachine.State != measurement.TestIdle {
		t.Fatalf("measurement machine remained in %s", a.testMachine.State)
	}
	if !strings.Contains(a.lastError, "unsupported measurement progress phase") {
		t.Fatalf("invalid provider progress was hidden: %q", a.lastError)
	}
}

func TestRejectedPreflightReturnsIdleWithoutConsumingQuota(t *testing.T) {
	a := newTestApp(t)
	a.lastHealth.Network = measurement.NetworkContext{Online: true, ConnectionType: measurement.ConnectionEthernet}
	err := a.StartTest(context.Background(), scheduler.Manual)
	if !errors.Is(err, measurement.ErrConsentRequired) {
		t.Fatalf("preflight error=%v", err)
	}
	if a.testMachine.State != measurement.TestIdle {
		t.Fatalf("rejected preflight left state %s", a.testMachine.State)
	}
	count, countErr := a.store.CountAttempts(context.Background(), time.Time{}, nil)
	if countErr != nil || count != 0 {
		t.Fatalf("rejected preflight consumed quota: count=%d err=%v", count, countErr)
	}
}

func TestSharingCannotBeEnabledWithoutATransport(t *testing.T) {
	a := newTestApp(t)
	err := a.Action(context.Background(), "consent", []byte(`{"scope":"fiberpulse","granted":true,"language":"en"}`))
	if err == nil {
		t.Fatal("sharing consent was enabled without a transport")
	}
	consent, readErr := a.store.CurrentConsent(context.Background(), "fiberpulse")
	if readErr != nil || consent.Granted {
		t.Fatalf("consent=%+v err=%v", consent, readErr)
	}
}

func TestGrantedSharingRestoresSuspendedWithoutTransport(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetConsent(ctx, storage.Consent{Scope: "fiberpulse", Granted: true, PolicyVersion: sharingConsentPolicyVersion}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	a, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{Delay: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.shareState.State != sharing.Suspended {
		t.Fatalf("state=%s", a.shareState.State)
	}
	queued, err := a.queueMeasurementForSharing(ctx, measurement.Result{ID: "suspended", Provider: measurement.ProviderMLabNDT7})
	if err != nil || queued {
		t.Fatalf("suspended sharing queued=%v err=%v", queued, err)
	}
}

func TestSharingRevocationPurgesConcurrentQueue(t *testing.T) {
	ctx := context.Background()
	a, err := New(Config{Version: "test", DatabasePath: filepath.Join(t.TempDir(), "fiberpulse.db"), Provider: &measurement.FakeProvider{Delay: time.Millisecond}, SharingTransportEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.Action(ctx, "consent", []byte(`{"scope":"fiberpulse","granted":true,"language":"en"}`)); err != nil {
		t.Fatal(err)
	}
	if a.shareState.State != sharing.Enabled {
		t.Fatalf("state=%s", a.shareState.State)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = a.queueMeasurementForSharing(ctx, measurement.Result{ID: fmt.Sprintf("measurement-%d", i), Provider: measurement.ProviderMLabNDT7})
		}(i)
	}
	if err := a.Action(ctx, "consent", []byte(`{"scope":"fiberpulse","granted":false,"language":"en"}`)); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if a.shareState.State != sharing.Revoked {
		t.Fatalf("state=%s", a.shareState.State)
	}
	count, err := a.store.ShareQueueCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("queue count=%d err=%v", count, err)
	}
}

func TestAnonymousMeasurementTravelsFromAgentQueueToObservatory(t *testing.T) {
	ctx := context.Background()
	hubStore, err := observatory.OpenStore(filepath.Join(t.TempDir(), "observatory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer hubStore.Close()
	hub, err := observatory.NewServer(observatory.Config{Store: hubStore})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(hub.Handler())
	defer httpServer.Close()
	transport, err := sharing.NewHTTPTransport(httpServer.URL, httpServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(Config{Version: "test", DatabasePath: filepath.Join(t.TempDir(), "fiberpulse.db"), Provider: &measurement.FakeProvider{}, SharingTransport: transport})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.setPlanSelection(ctx, "converge-super-prime-2099"); err != nil {
		t.Fatal(err)
	}
	if err := a.Action(ctx, "consent", []byte(`{"scope":"fiberpulse","granted":true,"language":"en"}`)); err != nil {
		t.Fatal(err)
	}
	result := measurement.Result{
		ID: "11111111-1111-4111-8111-111111111111", Provider: measurement.ProviderMLabNDT7,
		ProtocolVersion: "ndt7", ClientVersion: "test", StartedAt: time.Now().UTC(), CompletedAt: time.Now().UTC(),
		DownloadBPS: 620_000_000, UploadBPS: 410_000_000, MinRTTUS: 8_000, Status: measurement.StatusComplete,
		NetworkBefore:   measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet},
		ConfidenceScore: 95, ConfidenceLevel: "high", PublicEligible: true,
	}
	queued, err := a.queueMeasurementForSharing(ctx, result)
	if err != nil || !queued {
		t.Fatalf("queued=%v err=%v", queued, err)
	}
	a.flushSharingQueue()
	count, err := a.store.ShareQueueCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("queue count=%d err=%v", count, err)
	}
	public, err := hubStore.Search(ctx, observatory.SearchParams{Query: "Converge", Page: 1, Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if public.Total != 1 || public.Items[0].CountryCode != "PH" || public.Items[0].ISP != "Converge ICT" || public.Items[0].OfferName != "Super FiberX Prime 2099" {
		t.Fatalf("public result=%+v", public)
	}
}

func TestCustomPlanFreeTextIsRedactedBeforeSharing(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	err := a.setCustomPlanSelection(ctx, plan.Offer{CountryCode: "PH", CountryName: "Philippines", ISP: "Account 123 Alain", Name: "Bill reference 456", DownloadMbps: 900, UploadMbps: 500})
	if err != nil {
		t.Fatal(err)
	}
	offer := a.sharePlan(ctx)
	if offer == nil || offer.ISP != "Unlisted provider" || offer.Name != "Custom plan" || offer.DownloadMbps != 900 || offer.UploadMbps != 500 || offer.CountryCode != "PH" {
		t.Fatalf("shared custom offer=%+v", offer)
	}
}

func TestLegacySharingConsentRequiresFreshObservatoryOptIn(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetConsent(ctx, storage.Consent{Scope: "fiberpulse", Granted: true, PolicyVersion: "privacy-v1"}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	a, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{}, SharingTransportEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.shareState.State != sharing.NotAsked {
		t.Fatalf("legacy consent restored as %s", a.shareState.State)
	}
	snapshotAny, err := a.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotAny.(Snapshot).SharingConsent.Granted {
		t.Fatal("legacy sharing consent was presented as current")
	}
}

func TestIncidentLifecyclePersistsAcrossRestartAndCanBeDismissed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	first, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{Delay: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	degraded := func(at time.Time) health.Sample {
		return health.Sample{At: at, State: "internet_degraded", Category: "dns", Network: measurement.NetworkContext{Online: true}}
	}
	if err := first.processHealthSample(ctx, degraded(base)); err != nil {
		t.Fatal(err)
	}
	if err := first.processHealthSample(ctx, degraded(base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if first.incident.State != incidents.Suspected {
		t.Fatalf("expected suspected before restart, got %s", first.incident.State)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{Delay: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if second.incident.State != incidents.Suspected {
		t.Fatalf("restart lost suspected state: %s", second.incident.State)
	}
	if err := second.processHealthSample(ctx, degraded(base.Add(2*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if second.incident.State != incidents.Active || second.current.ID == "" {
		t.Fatalf("incident did not become active: machine=%s record=%+v", second.incident.State, second.current)
	}
	id := second.current.ID
	if err := second.Action(ctx, "incident-dismiss", []byte(`{"id":"`+id+`"}`)); err != nil {
		t.Fatal(err)
	}
	if second.incident.State != incidents.None || second.current.ID != "" {
		t.Fatalf("dismiss did not reset detector: machine=%s record=%+v", second.incident.State, second.current)
	}
	stored, err := second.store.ListIncidents(ctx, 10)
	if err != nil || len(stored) != 1 || stored[0].State != incidents.Dismissed {
		t.Fatalf("dismissed incident=%+v err=%v", stored, err)
	}
}

func TestConnectivityHysteresisPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	first, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{Delay: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	usable := health.Sample{At: base, State: string(health.ConnectivityInternetUsable), Category: "healthy", Network: measurement.NetworkContext{Online: true}}
	offline := health.Sample{At: base.Add(time.Minute), State: string(health.ConnectivityOffline), Category: "internet_reachability"}
	if err := first.processHealthSample(ctx, usable); err != nil {
		t.Fatal(err)
	}
	if err := first.processHealthSample(ctx, offline); err != nil {
		t.Fatal(err)
	}
	if state := first.connectivity.State(); state != health.ConnectivityUnstable {
		t.Fatalf("state before restart=%s", state)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := New(Config{Version: "test", DatabasePath: path, Provider: &measurement.FakeProvider{Delay: time.Millisecond}})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if state := second.connectivity.State(); state != health.ConnectivityUnstable {
		t.Fatalf("state after restart=%s", state)
	}
	offline.At = base.Add(2 * time.Minute)
	if err := second.processHealthSample(ctx, offline); err != nil {
		t.Fatal(err)
	}
	if state := second.connectivity.State(); state != health.ConnectivityOffline {
		t.Fatalf("confirmed state=%s", state)
	}
}

func TestSnapshotIncludesPersonalBaseline(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	base := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		result := measurement.Result{
			ID:                 fmt.Sprintf("baseline-%d", i),
			Provider:           measurement.ProviderDevelopmentFake,
			ProtocolVersion:    "fake-v1",
			ClientVersion:      "test",
			SchemaVersion:      measurement.SchemaVersion,
			MethodologyVersion: measurement.MethodologyVersion,
			ConfidenceVersion:  measurement.ConfidenceVersion,
			StartedAt:          base.Add(time.Duration(i%3) * 24 * time.Hour).Add(time.Duration(i) * time.Minute),
			CompletedAt:        base.Add(time.Duration(i%3) * 24 * time.Hour).Add(time.Duration(i)*time.Minute + time.Second),
			DownloadBPS:        int64(100_000_000 + i*1_000_000),
			UploadBPS:          20_000_000,
			MinRTTUS:           12_000,
			Status:             measurement.StatusComplete,
			ConfidenceLevel:    "high",
			ConfidenceScore:    90,
		}
		if err := a.store.SaveResult(ctx, result); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := a.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := raw.(Snapshot)
	if snapshot.Baseline.Maturity != "provisional" || snapshot.Baseline.Count != 10 || snapshot.Baseline.Days != 3 || snapshot.Baseline.DownloadMAD == 0 {
		t.Fatalf("unexpected baseline: %+v", snapshot.Baseline)
	}
}

func TestReportExportLifecycleAndDeletion(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	result := measurement.Result{
		ID: "report-measurement", Provider: measurement.ProviderDevelopmentFake, ProtocolVersion: "fake-v1", ClientVersion: "test",
		SchemaVersion: measurement.SchemaVersion, MethodologyVersion: measurement.MethodologyVersion, ConfidenceVersion: measurement.ConfidenceVersion,
		StartedAt: now, CompletedAt: now.Add(time.Second), DownloadBPS: 100_000_000, UploadBPS: 20_000_000,
		MinRTTUS: 10_000, Status: measurement.StatusComplete, ConfidenceLevel: "high", ConfidenceScore: 90,
	}
	if err := a.store.SaveResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"csv", "pdf"} {
		body, contentType, err := a.Export(ctx, format)
		if err != nil {
			t.Fatalf("export %s: %v", format, err)
		}
		if len(body) == 0 || contentType == "" {
			t.Fatalf("empty %s export", format)
		}
		if format == "pdf" && !bytes.HasPrefix(body, []byte("%PDF")) {
			t.Fatal("PDF export has no PDF signature")
		}
	}
	reports, err := a.store.ListReports(ctx, 10)
	if err != nil || len(reports) != 2 {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
	for _, report := range reports {
		if report.State != reporting.Exported || report.ByteCount == 0 || report.ErrorCode != "" {
			t.Fatalf("invalid exported report: %+v", report)
		}
	}
	raw, err := a.Snapshot(ctx)
	if err != nil || len(raw.(Snapshot).Reports) != 2 {
		t.Fatalf("report history missing from snapshot: %+v err=%v", raw, err)
	}
	id := reports[0].ID
	if err := a.Action(ctx, "report-delete", []byte(`{"id":"`+id+`"}`)); err != nil {
		t.Fatal(err)
	}
	deleted, err := a.store.GetReport(ctx, id)
	if err != nil || deleted.State != reporting.Deleted {
		t.Fatalf("deleted report=%+v err=%v", deleted, err)
	}
	if err := a.Action(ctx, "report-delete", []byte(`{"id":"`+id+`"}`)); err == nil {
		t.Fatal("deleted report was deleted twice")
	}
}

func TestEmptyReportFailsVisibly(t *testing.T) {
	a := newTestApp(t)
	if _, _, err := a.Export(context.Background(), "pdf"); err == nil {
		t.Fatal("empty report unexpectedly succeeded")
	}
	reports, err := a.store.ListReports(context.Background(), 10)
	if err != nil || len(reports) != 1 || reports[0].State != reporting.Failed || reports[0].ErrorCode != "report.no_measurements" {
		t.Fatalf("failed report=%+v err=%v", reports, err)
	}
}

func TestSubscriberProfileAndComplaintPackage(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.Action(ctx, "plan", []byte(`{"offer_id":"converge-super-prime-2099"}`)); err != nil {
		t.Fatal(err)
	}
	profileBody := []byte(`{"full_name":"Test Subscriber","account_number":"ACC-123","service_address":"Cebu City","contact_email":"subscriber@example.com","provider_router":"Provider Router","test_connection":"ethernet","network_layout":"provider_router_direct"}`)
	if err := a.Action(ctx, "profile", profileBody); err != nil {
		t.Fatal(err)
	}

	ph := time.FixedZone("PHT", 8*60*60)
	nowLocal := time.Now().In(ph)
	// Keep every sample inside the rolling seven-day window while still
	// spanning seven local calendar dates. Anchoring at midnight made the
	// oldest samples expire progressively as the test ran later in the day.
	endDay := nowLocal.Add(-3 * time.Hour)
	for day := 0; day < complaint.TargetDays; day++ {
		for sample := 0; sample < 3; sample++ {
			started := endDay.AddDate(0, 0, -(complaint.TargetDays - 1 - day)).Add(time.Duration(sample) * time.Hour).UTC()
			result := measurement.Result{
				ID: fmt.Sprintf("complaint-%d-%d", day, sample), Provider: measurement.ProviderMLabNDT7,
				ProtocolVersion: "ndt7", ClientVersion: "test", SchemaVersion: measurement.SchemaVersion,
				MethodologyVersion: measurement.MethodologyVersion, ConfidenceVersion: measurement.ConfidenceVersion,
				StartedAt: started, CompletedAt: started.Add(time.Minute), ServerFQDN: "ndt.example",
				DownloadBPS: 400_000_000, UploadBPS: 350_000_000, MinRTTUS: 11_000,
				Status: measurement.StatusComplete, ConfidenceLevel: "high", ConfidenceScore: 95, PublicEligible: true,
				NetworkBefore: measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet, Online: true},
				NetworkAfter:  measurement.NetworkContext{ConnectionType: measurement.ConnectionEthernet, Online: true},
			}
			if err := a.store.SaveResult(ctx, result); err != nil {
				t.Fatal(err)
			}
		}
	}
	raw, err := a.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := raw.(Snapshot)
	if !snapshot.Complaint.Assessment.ComplaintReady || snapshot.Complaint.Assessment.QualifiedTests != 21 || snapshot.Complaint.Profile.AccountNumber != "ACC-123" {
		t.Fatalf("complaint state not ready: %+v", snapshot.Complaint)
	}
	pdf, pdfType, err := a.Export(ctx, "complaint-pdf")
	if err != nil || pdfType != "application/pdf" || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("complaint PDF type=%q size=%d err=%v", pdfType, len(pdf), err)
	}
	eml, emlType, err := a.Export(ctx, "complaint-eml")
	if err != nil || emlType != "message/rfc822" || !bytes.Contains(eml, []byte("fiberpulse-complaint-report.pdf")) {
		t.Fatalf("complaint EML type=%q size=%d err=%v", emlType, len(eml), err)
	}
	reports, err := a.store.ListReports(ctx, 10)
	if err != nil || len(reports) != 0 {
		t.Fatalf("complaint exports must not violate pdf/csv report history: reports=%+v err=%v", reports, err)
	}
}
