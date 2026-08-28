package incidents

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRecordJSONOmitsUnknownLifecycleDates(t *testing.T) {
	record := Record{
		ID:          "incident-1",
		Category:    "dns",
		State:       Suspected,
		SuspectedAt: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 8, 28, 0, 1, 0, 0, time.UTC),
	}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"active_at", "recovering_at", "resolved_at", "0001-01-01"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("unknown lifecycle date leaked into JSON: %s", body)
		}
	}
}
