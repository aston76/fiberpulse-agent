package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/scheduler"
	"fiberpulse.dev/agent/internal/storage"
)

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
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Errorf("close app: %v", err)
		}
	})
	return a
}

func TestPauseStateNeverHidesMissingMLabConsent(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	if err := a.SetPaused(ctx, true); err != nil {
		t.Fatal(err)
	}
	if a.state != "consent_required" || !a.paused {
		t.Fatalf("state=%q paused=%v", a.state, a.paused)
	}
	if err := a.Action(ctx, "consent", []byte(`{"scope":"mlab","granted":true,"language":"en"}`)); err != nil {
		t.Fatal(err)
	}
	if a.state != "paused" {
		t.Fatalf("grant while paused produced state %q", a.state)
	}
	if err := a.SetPaused(ctx, false); err != nil {
		t.Fatal(err)
	}
	if a.state != "monitoring" || a.paused {
		t.Fatalf("state=%q paused=%v", a.state, a.paused)
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
	if a.state != "paused" || !a.paused {
		t.Fatalf("state=%q paused=%v", a.state, a.paused)
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
