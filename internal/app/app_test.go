package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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
