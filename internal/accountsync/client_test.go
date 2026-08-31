package accountsync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (fn roundTrip) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestNewRejectsInsecureOrigin(t *testing.T) {
	if _, err := New("http://testspeednow.com", nil); err == nil {
		t.Fatal("expected insecure account origin to be rejected")
	}
}

func TestStartRejectsIncompleteResponse(t *testing.T) {
	client, err := New("https://testspeednow.com", roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 201, Body: http.NoBody, Header: make(http.Header)}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Start(context.Background(), "Mac"); err == nil {
		t.Fatal("expected incomplete response to fail")
	}
}

func TestMeasurementRoundTripSanitizesAndMakesImportedCopyPrivate(t *testing.T) {
	started := time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC)
	original := measurement.Result{
		ID: "measurement_roundtrip_001", Provider: measurement.ProviderMLabNDT7, ProtocolVersion: "ndt7",
		ClientVersion: "0.1.2", SchemaVersion: measurement.SchemaVersion, MethodologyVersion: measurement.MethodologyVersion,
		ConfidenceVersion: measurement.ConfidenceVersion, StartedAt: started, CompletedAt: started.Add(12 * time.Second),
		ServerFQDN: "ndt.example.net", DownloadBPS: 512_400_000, UploadBPS: 201_200_000, MinRTTUS: 8_700,
		BytesDown: 64_000_000, BytesUp: 25_000_000, DownloadDurationUS: 6_000_000, UploadDurationUS: 6_000_000,
		Status: measurement.StatusComplete, NetworkBefore: measurement.NetworkContext{InterfaceID: "en0", RouteID: "private-route", ConnectionType: measurement.ConnectionWiFi, Online: true, CapturedAt: started},
		NetworkAfter:    measurement.NetworkContext{InterfaceID: "en0", RouteID: "private-route", ConnectionType: measurement.ConnectionWiFi, Online: true, CapturedAt: started.Add(12 * time.Second)},
		ConfidenceScore: 95, ConfidenceLevel: "high", PublicEligible: true,
	}
	var stored measurement.Result
	client, err := New("https://testspeednow.com", roundTrip(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header=%q", request.Header.Get("Authorization"))
		}
		switch request.Method {
		case http.MethodPost:
			var payload struct {
				Items []measurementPayload `json:"items"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload.Items) != 1 {
				t.Fatalf("uploaded items=%d", len(payload.Items))
			}
			stored = payload.Items[0].Result
			if stored.NetworkBefore.InterfaceID != "" || stored.NetworkBefore.RouteID != "" || stored.NetworkAfter.InterfaceID != "" || stored.NetworkAfter.RouteID != "" {
				t.Fatalf("private network identifiers were uploaded: %+v %+v", stored.NetworkBefore, stored.NetworkAfter)
			}
			return jsonResponse(http.StatusOK, `{}`), nil
		case http.MethodGet:
			body, err := json.Marshal(map[string]any{"items": []any{map[string]any{"id": stored.ID, "result": stored}}})
			if err != nil {
				t.Fatal(err)
			}
			return jsonResponse(http.StatusOK, string(body)), nil
		default:
			t.Fatalf("unexpected method %s", request.Method)
			return nil, nil
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(context.Background(), "test-token", []measurement.Result{original}); err != nil {
		t.Fatal(err)
	}
	results, err := client.Measurements(context.Background(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != original.ID {
		t.Fatalf("results=%+v", results)
	}
	if results[0].PublicEligible || !contains(results[0].ConfidenceReasons, "account.synced_copy") {
		t.Fatalf("imported result must remain private: %+v", results[0])
	}
}

func TestMeasurementsRejectsInvalidRemoteResult(t *testing.T) {
	client, err := New("https://testspeednow.com", roundTrip(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{"id":"different_id","result":{"id":"measurement_001","status":"complete"}}]}`), nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Measurements(context.Background(), "test-token"); err == nil {
		t.Fatal("expected an invalid synchronized result to be rejected")
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
