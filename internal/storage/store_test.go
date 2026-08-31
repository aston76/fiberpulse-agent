package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/health"
	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/reporting"
	"fiberpulse.dev/agent/internal/scheduler"
)

func TestPersistHealthRuntimeCommitsSampleAndMachineTogether(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 8, 28, 1, 2, 3, 0, time.UTC)
	sample := health.Sample{At: at, State: string(health.ConnectivityInternetUsable), Category: "healthy", Network: measurement.NetworkContext{Online: true}}
	runtime := health.ConnectivitySnapshot{State: health.ConnectivityInternetUsable, Stable: health.ConnectivityInternetUsable, LastObservedAt: at}
	if err := s.PersistHealthRuntime(ctx, sample, "connectivity_runtime_test", runtime); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM health_samples WHERE captured_at=?`, formatTime(at)).Scan(&count); err != nil || count != 1 {
		t.Fatalf("health count=%d err=%v", count, err)
	}
	var stored health.ConnectivitySnapshot
	found, err := s.GetSetting(ctx, "connectivity_runtime_test", &stored)
	if err != nil || !found || stored != runtime {
		t.Fatalf("runtime found=%v stored=%+v err=%v", found, stored, err)
	}
}

func TestPersistHealthRuntimeRollsBackBothWrites(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER reject_connectivity_runtime BEFORE INSERT ON settings
		WHEN NEW.key='connectivity_runtime_rejected'
		BEGIN SELECT RAISE(ABORT, 'runtime rejected'); END`); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 28, 1, 2, 3, 0, time.UTC)
	sample := health.Sample{At: at, State: string(health.ConnectivityOffline), Category: "internet_reachability"}
	runtime := health.ConnectivitySnapshot{State: health.ConnectivityOffline, Stable: health.ConnectivityOffline, LastObservedAt: at}
	if err := s.PersistHealthRuntime(ctx, sample, "connectivity_runtime_rejected", runtime); err == nil {
		t.Fatal("runtime rejection unexpectedly committed")
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM health_samples WHERE captured_at=?`, formatTime(at)).Scan(&count); err != nil || count != 0 {
		t.Fatalf("health write was not rolled back: count=%d err=%v", count, err)
	}
	var stored health.ConnectivitySnapshot
	found, err := s.GetSetting(ctx, "connectivity_runtime_rejected", &stored)
	if err != nil || found {
		t.Fatalf("runtime write was not rolled back: found=%v stored=%+v err=%v", found, stored, err)
	}
}

func TestStorePersistsConsentQuotaAndMeasurement(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetConsent(ctx, Consent{Scope: "mlab", Granted: true, PolicyVersion: "privacy-v1"}); err != nil {
		t.Fatal(err)
	}
	c, err := s.CurrentConsent(ctx, "mlab")
	if err != nil || !c.Granted {
		t.Fatalf("consent=%+v err=%v", c, err)
	}
	now := time.Now().UTC()
	if err := s.ReserveAttempt(ctx, scheduler.Automatic, now); err != nil {
		t.Fatal(err)
	}
	kind := scheduler.Automatic
	count, err := s.CountAttempts(ctx, now.Add(-time.Hour), &kind)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	r := measurement.Result{ID: "result-1", Provider: "development_fake", ProtocolVersion: "fake-v1", ClientVersion: "dev", SchemaVersion: measurement.SchemaVersion, MethodologyVersion: measurement.MethodologyVersion, ConfidenceVersion: measurement.ConfidenceVersion, StartedAt: now, CompletedAt: now.Add(time.Second), Status: measurement.StatusComplete, ConfidenceLevel: "high", ConfidenceScore: 100, PublicEligible: true}
	if err := s.SaveResult(ctx, r); err != nil {
		t.Fatal(err)
	}
	if inserted, err := s.SaveResultIfAbsent(ctx, r); err != nil || inserted {
		t.Fatalf("duplicate synchronized result inserted=%v err=%v", inserted, err)
	}
	imported := r
	imported.ID = "result-synchronized"
	imported.PublicEligible = false
	imported.ConfidenceReasons = []string{"account.synced_copy"}
	if inserted, err := s.SaveResultIfAbsent(ctx, imported); err != nil || !inserted {
		t.Fatalf("new synchronized result inserted=%v err=%v", inserted, err)
	}
	results, err := s.ListResults(ctx, 10)
	if err != nil || len(results) != 2 || (results[0].ID != imported.ID && results[1].ID != imported.ID) {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if err := s.IntegrityCheck(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSharingRevocationAtomicallyPurgesQueue(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: true, PolicyVersion: "privacy-v1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueShare(ctx, "event-1", "measurement", map[string]any{"value": 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: false, PolicyVersion: "privacy-v1"}); err != nil {
		t.Fatal(err)
	}
	count, err := s.ShareQueueCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("queue count=%d err=%v", count, err)
	}
	consent, err := s.CurrentConsent(ctx, "fiberpulse")
	if err != nil || consent.Granted {
		t.Fatalf("consent=%+v err=%v", consent, err)
	}
}

func TestCurrentConsentUsesInsertionOrderWhenTimestampsMatch(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: true, PolicyVersion: "privacy-v1", OccurredAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: false, PolicyVersion: "privacy-v1", OccurredAt: at}); err != nil {
		t.Fatal(err)
	}
	consent, err := s.CurrentConsent(ctx, "fiberpulse")
	if err != nil {
		t.Fatal(err)
	}
	if consent.Granted {
		t.Fatal("equal-timestamp revocation was hidden by the older grant")
	}
}

func TestNewShareIsDueEvenIfWallClockMovesBackwards(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	future := time.Date(2026, 8, 31, 1, 0, 0, 0, time.UTC)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO share_queue(id,event_type,payload_json,created_at,next_attempt_at) VALUES(?,?,?,?,?)`, "clock-step", "measurement", `{}`, formatTime(future), formatTime(future)); err != nil {
		t.Fatal(err)
	}
	items, err := s.DueShares(ctx, future.Add(-time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "clock-step" {
		t.Fatalf("new item was deferred by a backwards clock step: %+v", items)
	}
}

func TestSharingRevocationRollsBackWhenQueuePurgeFails(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: true, PolicyVersion: "privacy-v1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.QueueShare(ctx, "event-1", "measurement", map[string]any{"value": 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER reject_share_delete BEFORE DELETE ON share_queue BEGIN SELECT RAISE(ABORT, 'test purge failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConsent(ctx, Consent{Scope: "fiberpulse", Granted: false, PolicyVersion: "privacy-v1"}); err == nil {
		t.Fatal("revocation committed despite queue purge failure")
	}
	consent, err := s.CurrentConsent(ctx, "fiberpulse")
	if err != nil || !consent.Granted {
		t.Fatalf("consent transaction did not roll back: %+v err=%v", consent, err)
	}
	count, err := s.ShareQueueCount(ctx)
	if err != nil || count != 1 {
		t.Fatalf("queue transaction did not roll back: count=%d err=%v", count, err)
	}
}

func TestStorePersistsIncidentLifecycle(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	record := incidents.Record{ID: "incident-1", Category: "dns", State: incidents.Suspected, SuspectedAt: base, UpdatedAt: base}
	if err := s.SaveIncident(ctx, record); err != nil {
		t.Fatal(err)
	}
	record.State = incidents.Active
	record.ActiveAt = base.Add(2 * time.Minute)
	record.UpdatedAt = record.ActiveAt
	if err := s.SaveIncident(ctx, record); err != nil {
		t.Fatal(err)
	}
	stored, err := s.ListIncidents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].State != incidents.Active || !stored[0].ActiveAt.Equal(record.ActiveAt) {
		t.Fatalf("stored incident=%+v", stored)
	}
	if err := s.DeleteIncident(ctx, record.ID); err != nil {
		t.Fatal(err)
	}
	stored, err = s.ListIncidents(ctx, 10)
	if err != nil || len(stored) != 0 {
		t.Fatalf("deleted incident remained: %+v err=%v", stored, err)
	}
}

func TestPersistIncidentRuntimeIsAtomic(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	record := incidents.Record{ID: "incident-atomic", Category: "dns", State: incidents.Suspected, SuspectedAt: base, UpdatedAt: base}
	initialRuntime := map[string]any{"state": "suspected", "bad_count": 2}
	if err := s.PersistIncidentRuntime(ctx, &record, "", "incident_runtime_test", initialRuntime); err != nil {
		t.Fatal(err)
	}

	invalid := record
	invalid.State = incidents.None
	if err := s.PersistIncidentRuntime(ctx, &invalid, record.ID, "incident_runtime_test", map[string]any{"state": "none"}); err == nil {
		t.Fatal("invalid incident unexpectedly committed")
	}
	stored, err := s.ListIncidents(ctx, 10)
	if err != nil || len(stored) != 1 || stored[0].State != incidents.Suspected {
		t.Fatalf("incident transaction did not roll back: %+v err=%v", stored, err)
	}
	var runtime map[string]any
	found, err := s.GetSetting(ctx, "incident_runtime_test", &runtime)
	if err != nil || !found || runtime["state"] != "suspected" || runtime["bad_count"] != float64(2) {
		t.Fatalf("runtime transaction did not roll back: found=%v runtime=%+v err=%v", found, runtime, err)
	}
}

func TestStorePersistsAndRecoversReportLifecycle(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Date(2026, 8, 28, 1, 0, 0, 0, time.UTC)
	report := reporting.Record{ID: "report-1", Format: "pdf", State: reporting.Drafting, PeriodStart: now.AddDate(-1, -1, 0), PeriodEnd: now, CreatedAt: now, UpdatedAt: now}
	if err := s.SaveReport(ctx, report); err != nil {
		t.Fatal(err)
	}
	if err := s.RecoverInterruptedReports(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.GetReport(ctx, report.ID)
	if err != nil || recovered.State != reporting.Failed || recovered.ErrorCode != "generation.interrupted" {
		t.Fatalf("recovered report=%+v err=%v", recovered, err)
	}
	reports, err := s.ListReports(ctx, 10)
	if err != nil || len(reports) != 1 || reports[0].ID != report.ID {
		t.Fatalf("reports=%+v err=%v", reports, err)
	}
}

func TestStoreRejectsInvalidReport(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "fiberpulse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveReport(context.Background(), reporting.Record{ID: "bad", Format: "html", State: reporting.Exported}); err == nil {
		t.Fatal("invalid report was persisted")
	}
}

func TestLegacyDevelopmentMeasurementsBecomeLocalOnlyOnOpen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fiberpulse.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	r := measurement.Result{ID: "legacy-fake", Provider: measurement.ProviderDevelopmentFake, ProtocolVersion: "fake-v1", ClientVersion: "dev", SchemaVersion: measurement.SchemaVersion, MethodologyVersion: measurement.MethodologyVersion, ConfidenceVersion: measurement.ConfidenceVersion, StartedAt: now, CompletedAt: now.Add(time.Second), Status: measurement.StatusComplete, ConfidenceLevel: "high", ConfidenceScore: 100, PublicEligible: true}
	if err := s.SaveResult(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	results, err := s.ListResults(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].PublicEligible {
		t.Fatalf("legacy development result remained eligible: %+v", results)
	}
	if len(results[0].ConfidenceReasons) != 1 || results[0].ConfidenceReasons[0] != "provider.not_public" {
		t.Fatalf("legacy development result lacks reason: %+v", results[0].ConfidenceReasons)
	}
}
