package plan

import "testing"

func TestCatalogOffersAreUniqueAndSane(t *testing.T) {
	seen := map[string]bool{}
	for _, offer := range Catalog() {
		if seen[offer.ID] {
			t.Fatalf("duplicate offer id %q", offer.ID)
		}
		seen[offer.ID] = true
		if offer.ISP == "" || offer.Name == "" {
			t.Fatalf("offer %q lacks isp or name", offer.ID)
		}
		if offer.DownloadMbps <= 0 {
			t.Fatalf("offer %q has no advertised download", offer.ID)
		}
		if offer.UploadMbps < 0 {
			t.Fatalf("offer %q has negative upload", offer.ID)
		}
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
	noUpload, _ := Find("pldt-fibr-400")
	if Assess(noUpload, 400_000_000, 500_000_000).UploadPct != 0 {
		t.Fatal("upload pct must stay zero when the plan does not advertise upload")
	}
}

func TestAssessZeroMeasurement(t *testing.T) {
	offer, _ := Find("globe-gfiber-500")
	verdict := Assess(offer, 0, 0)
	if verdict.Level != LevelWellBelow {
		t.Fatalf("zero measurement: got %s", verdict.Level)
	}
}
