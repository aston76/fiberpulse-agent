package baseline

import (
	"testing"
	"time"
)

func TestMaturityRequiresEnoughQualifiedSamples(t *testing.T) {
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	twoSamples := []Sample{
		{At: base, DownloadBPS: 100, HighConfidence: true},
		{At: base.Add(30 * 24 * time.Hour), DownloadBPS: 110, HighConfidence: true},
	}
	if result := Calculate(twoSamples); result.Maturity != "insufficient" {
		t.Fatalf("sparse baseline was promoted to %s", result.Maturity)
	}

	qualified := make([]Sample, 0, 10)
	for i := 0; i < 10; i++ {
		qualified = append(qualified, Sample{
			At:             base.Add(time.Duration(i%3) * 24 * time.Hour),
			DownloadBPS:    int64(100 + i),
			HighConfidence: true,
		})
	}
	if result := Calculate(qualified); result.Maturity != "provisional" {
		t.Fatalf("qualified baseline maturity=%s", result.Maturity)
	}
}
