package plan

import (
	"testing"
	"time"
)

func TestCatalogOffersAreUniqueAndSane(t *testing.T) {
	seen := map[string]bool{}
	providers := map[string]bool{}
	for _, offer := range Catalog() {
		if seen[offer.ID] {
			t.Fatalf("duplicate offer id %q", offer.ID)
		}
		seen[offer.ID] = true
		providers[offer.ISP] = true
		if offer.ISP == "" || offer.Name == "" {
			t.Fatalf("offer %q lacks isp or name", offer.ID)
		}
		if offer.DownloadMbps <= 0 {
			t.Fatalf("offer %q has no advertised download", offer.ID)
		}
		if offer.UploadMbps < 0 {
			t.Fatalf("offer %q has negative upload", offer.ID)
		}
		if offer.VerifiedAt != CatalogVerifiedAt || offer.SourceURL == "" {
			t.Fatalf("offer %q lacks verification metadata", offer.ID)
		}
	}
	if len(providers) < 10 {
		t.Fatalf("catalog unexpectedly narrow: %d providers", len(providers))
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("converge-fiberx-400"); !ok {
		t.Fatal("expected converge-fiberx-400 to exist")
	}
	if _, ok := Find("nope"); ok {
		t.Fatal("unexpected offer")
	}
}

func TestAssessLevels(t *testing.T) {
	offer, _ := Find("converge-fiberx-400")
	mbps := func(v int64) int64 { return v * 1_000_000 }
	tests := []struct {
		name string
		down int64
		want Level
	}{
		{"exceeds plan", mbps(450), LevelOnPar},
		{"exactly at threshold", mbps(280), LevelOnPar}, // 70%
		{"just below threshold", mbps(279), LevelBelow},
		{"clearly slow", mbps(200), LevelBelow},
		{"edge of complaint", mbps(160), LevelBelow}, // 40%
		{"complaint worthy", mbps(159), LevelWellBelow},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Assess(offer, tc.down, 0); got.Level != tc.want {
				t.Fatalf("down=%d want %s got %s (%d%%)", tc.down, tc.want, got.Level, got.DownloadPct)
			}
		})
	}
}

func TestAssessComplaintFlagAndUpload(t *testing.T) {
	offer := Offer{ID: "x", ISP: "x", Name: "x", DownloadMbps: 100, UploadMbps: 100}
	verdict := Assess(offer, 20_000_000, 95_000_000)
	if !verdict.ComplaintWorthy {
		t.Fatal("20%% of plan should be complaint worthy")
	}
	if verdict.UploadPct != 95 {
		t.Fatalf("upload pct: got %d", verdict.UploadPct)
	}
	noUpload, _ := Find("pldt-unli-1699")
	if Assess(noUpload, 400_000_000, 500_000_000).UploadPct != 0 {
		t.Fatal("upload pct must stay zero when the plan does not advertise upload")
	}
}

func TestValidateCustom(t *testing.T) {
	got, err := ValidateCustom(Offer{ISP: "  Regional ISP ", Name: " Plan 500 ", DownloadMbps: 500, UploadMbps: 100, PricePHP: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "custom" || !got.Custom || got.ISP != "Regional ISP" || got.Name != "Plan 500" || got.PricePHP != 0 {
		t.Fatalf("unexpected custom offer: %+v", got)
	}
	if _, err := ValidateCustom(Offer{ISP: "ISP", Name: "Plan", DownloadMbps: 0}); err == nil {
		t.Fatal("zero-speed custom plan accepted")
	}
}

func TestTimeOfDayOfferUsesPhilippinePeriod(t *testing.T) {
	offer, ok := Find("converge-time-of-day-day-1699")
	if !ok {
		t.Fatal("time-of-day offer missing")
	}
	day := time.Date(2026, 8, 30, 2, 0, 0, 0, time.UTC)    // 10:00 PHT
	night := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) // 20:00 PHT
	if got := EffectiveDownloadMbps(offer, day); got != 600 {
		t.Fatalf("day speed=%d", got)
	}
	if got := EffectiveDownloadMbps(offer, night); got != 400 {
		t.Fatalf("night speed=%d", got)
	}
	verdict := AssessAt(offer, 400_000_000, 0, night)
	if verdict.DownloadPct != 100 || verdict.AdvertisedDownloadMbps != 400 {
		t.Fatalf("night verdict=%+v", verdict)
	}
}

func TestAssessZeroMeasurement(t *testing.T) {
	offer, _ := Find("globe-gfiber-500")
	verdict := Assess(offer, 0, 0)
	if verdict.Level != LevelWellBelow {
		t.Fatalf("zero measurement: got %s", verdict.Level)
	}
}
