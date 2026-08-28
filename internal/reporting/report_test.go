package reporting

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
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
