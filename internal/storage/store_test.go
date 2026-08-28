package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
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
