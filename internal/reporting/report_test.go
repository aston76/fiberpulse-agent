package reporting

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/plan"
)

func TestCSVAndPDFExports(t *testing.T) {
	result := measurement.Result{
		StartedAt:       time.Date(2026, 8, 28, 8, 30, 0, 0, time.UTC),
		Provider:        "development_fake",
		ServerFQDN:      "fake.measurement.local",
		DownloadBPS:     100_000_000,
		UploadBPS:       20_000_000,
		MinRTTUS:        12_000,
		Status:          measurement.StatusComplete,
		ConfidenceScore: 100,
		ConfidenceLevel: "high",
		PublicEligible:  true,
	}
	csvBody, err := CSV([]measurement.Result{result})
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(string(csvBody))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][3] != "100000000" || records[1][6] != "complete" {
		t.Fatalf("unexpected CSV records: %#v", records)
	}
	pdfBody, err := PDF([]measurement.Result{result}, result.StartedAt.Add(-time.Hour), result.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdfBody) < 1_000 || !strings.HasPrefix(string(pdfBody), "%PDF-") {
		t.Fatalf("invalid PDF export: %d bytes", len(pdfBody))
	}
}

func TestBrandedPDFWithPlanAndManyRows(t *testing.T) {
	started := time.Date(2026, 8, 30, 3, 33, 0, 0, time.UTC)
	results := make([]measurement.Result, 0, 45)
	for i := 0; i < 45; i++ {
		results = append(results, measurement.Result{
			StartedAt:       started.Add(-time.Duration(i) * 3 * time.Hour),
			Provider:        measurement.ProviderMLabNDT7,
			ServerFQDN:      "ndt-mlab2-mnl02.measurement-lab.org",
			DownloadBPS:     448_000_000 - int64(i%5)*8_000_000,
			UploadBPS:       491_000_000 - int64(i%4)*7_000_000,
			MinRTTUS:        12_000 + int64(i%3)*900,
			Status:          measurement.StatusComplete,
			ConfidenceScore: 90 - i%4,
			ConfidenceLevel: "high",
			PublicEligible:  true,
		})
	}
	offer, ok := plan.Find("converge-fiberx-400")
	if !ok {
		t.Fatal("preview plan missing from catalog")
	}
	pdfBody, err := PDFWithPlan(results, started.AddDate(0, -1, 0), started, &offer)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdfBody) < 20_000 || !strings.HasPrefix(string(pdfBody), "%PDF-") {
		t.Fatalf("invalid branded PDF export: %d bytes", len(pdfBody))
	}
}

func TestMedianMetrics(t *testing.T) {
	results := []measurement.Result{
		{DownloadBPS: 100, UploadBPS: 10, MinRTTUS: 30},
		{DownloadBPS: 300, UploadBPS: 30, MinRTTUS: 10},
		{DownloadBPS: 200, UploadBPS: 20, MinRTTUS: 20},
	}
	download, upload, latency := medianMetrics(results)
	if download != 200 || upload != 20 || latency != 20 {
		t.Fatalf("unexpected medians: %.0f %.0f %.0f", download, upload, latency)
	}
}
