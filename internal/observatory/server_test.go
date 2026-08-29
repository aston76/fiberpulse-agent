package observatory

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/sharing"
	"github.com/google/uuid"
)

func TestSignedMeasurementAppearsInPublicSearchWithoutInstallationID(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "observatory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server, err := NewServer(Config{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	identity, err := sharing.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	installationID := sharing.InstallationID(identity.Public)
	registration, _ := json.Marshal(map[string]string{"installation_id": installationID, "public_key": base64.StdEncoding.EncodeToString(identity.Public)})
	response, err := http.Post(httpServer.URL+"/api/v1/installations", "application/json", bytes.NewReader(registration))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("registration status=%d", response.StatusCode)
	}

	event := sharing.MeasurementEvent{
		EventID: uuid.NewString(), TimestampBucket: time.Now().UTC().Truncate(15 * time.Minute),
		MeasurementProvider: "mlab_ndt7", ProtocolVersion: "ndt7", AgentVersion: "test",
		SchemaVersion: "measurement-v1", MethodologyVersion: "methodology-v1", ConfidenceVersion: "confidence-v1",
		DownloadBPS: 812_500_000, UploadBPS: 407_200_000, MinRTTUS: 8_400, ConnectionType: "ethernet",
		ConfidenceScore: 96, ConfidenceLevel: "high", PublicEligible: true,
		PlanCountryCode: "PH", PlanCountryName: "Philippines", ISP: "Converge ICT", OfferName: "Super FiberX Prime 2099",
		SubscriptionType: "Super FiberX", AdvertisedDownloadMbps: 800, CatalogOffer: true,
	}
	body, _ := json.Marshal(event)
	request := signedRequest(t, httpServer.URL+"/api/v1/measurements", identity, installationID, 1, body)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("measurement status=%d", response.StatusCode)
	}

	response, err = http.Get(httpServer.URL + "/api/v1/public/measurements?q=Converge")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	publicBody, _ := io.ReadAll(response.Body)
	if bytes.Contains(publicBody, []byte(installationID)) || bytes.Contains(publicBody, identity.Public) || bytes.Contains(publicBody, []byte(event.EventID)) {
		t.Fatal("public response exposed a stable installation identifier")
	}
	var schema string
	if err := store.db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='measurements'").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains([]byte(schema), []byte("installation_id")) {
		t.Fatal("measurement rows retain a stable installation link")
	}
	var result SearchResult
	if err := json.Unmarshal(publicBody, &result); err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("result=%+v", result)
	}
	item := result.Items[0]
	if item.CountryCode != "PH" || item.CountryName != "Philippines" || item.ISP != "Converge ICT" || item.DownloadMbps != 812.5 {
		t.Fatalf("public item=%+v", item)
	}
	facets, err := store.Facets(context.Background())
	if err != nil || len(facets.Countries) != 1 || len(facets.Providers) != 1 || facets.Providers[0].Name != "Converge ICT" {
		t.Fatalf("facets=%+v err=%v", facets, err)
	}
}

func TestCloudflareLocationOverridesDeclaredPlanCountry(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "observatory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server, _ := NewServer(Config{Store: store, TrustCloudflareLocation: true})
	identity, _ := sharing.NewIdentity()
	installationID := sharing.InstallationID(identity.Public)
	if err := store.Register(context.Background(), installationID, identity.Public, time.Now()); err != nil {
		t.Fatal(err)
	}
	event := sharing.MeasurementEvent{EventID: uuid.NewString(), TimestampBucket: time.Now().UTC().Truncate(15 * time.Minute), MeasurementProvider: "mlab_ndt7", ProtocolVersion: "ndt7", AgentVersion: "test", SchemaVersion: "measurement-v1", MethodologyVersion: "methodology-v1", ConfidenceVersion: "confidence-v1", DownloadBPS: 100e6, UploadBPS: 50e6, MinRTTUS: 10_000, ConnectionType: "wifi", ConfidenceScore: 80, ConfidenceLevel: "medium", PublicEligible: true, PlanCountryCode: "FR", PlanCountryName: "France"}
	body, _ := json.Marshal(event)
	request := signedRequest(t, "http://observatory.test/api/v1/measurements", identity, installationID, 1, body)
	request.Header.Set("CF-IPCountry", "PH")
	request.Header.Set("CF-Region", "Central Visayas")
	request.Header.Set("CF-IPCity", "Cebu City")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	result, err := store.Search(context.Background(), SearchParams{Query: "Cebu", Page: 1, Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || result.Items[0].CountryCode != "PH" || result.Items[0].CountryName != "PH" || result.Items[0].City != "Cebu City" {
		t.Fatalf("location=%+v", result.Items)
	}
}

func TestRejectsPIIShapedUnknownFields(t *testing.T) {
	store, _ := OpenStore(filepath.Join(t.TempDir(), "observatory.db"))
	defer store.Close()
	server, _ := NewServer(Config{Store: store})
	identity, _ := sharing.NewIdentity()
	installationID := sharing.InstallationID(identity.Public)
	_ = store.Register(context.Background(), installationID, identity.Public, time.Now())
	body := []byte(`{"event_id":"` + uuid.NewString() + `","timestamp_bucket":"` + time.Now().UTC().Truncate(15*time.Minute).Format(time.RFC3339) + `","measurement_provider":"mlab_ndt7","protocol_version":"ndt7","agent_version":"test","schema_version":"measurement-v1","methodology_version":"methodology-v1","confidence_version":"confidence-v1","download_bps":1000000,"upload_bps":1000000,"min_rtt_us":1000,"connection_type":"ethernet","confidence_score":90,"confidence_level":"high","public_eligible":true,"catalog_offer":false,"email":"person@example.com"}`)
	request := signedRequest(t, "http://observatory.test/api/v1/measurements", identity, installationID, 1, body)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("PII-shaped field accepted: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func signedRequest(t *testing.T, target string, identity sharing.Identity, installationID string, sequence uint64, body []byte) *http.Request {
	t.Helper()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	signature, err := identity.Sign(http.MethodPost, "/api/v1/measurements", timestamp, nonce, sequence, body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-FiberPulse-Installation", installationID)
	request.Header.Set("X-FiberPulse-Timestamp", timestamp)
	request.Header.Set("X-FiberPulse-Nonce", nonce)
	request.Header.Set("X-FiberPulse-Sequence", strconv.FormatUint(sequence, 10))
	request.Header.Set("X-FiberPulse-Signature", signature)
	return request
}
