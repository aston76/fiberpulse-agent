package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/incidents"
	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/reporting"
	"fiberpulse.dev/agent/internal/scheduler"
)

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
	results, err := s.ListResults(ctx, 10)
	if err != nil || len(results) != 1 || results[0].ID != r.ID {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if err := s.IntegrityCheck(ctx); err != nil {
		t.Fatal(err)
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
