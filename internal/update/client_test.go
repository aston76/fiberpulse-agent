package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type feedFixture struct {
	server     *httptest.Server
	artifact   []byte
	manifest   Manifest
	privateKey ed25519.PrivateKey
	publicHex  string
}

func newFeedFixture(t *testing.T, mutate func(*Manifest)) *feedFixture {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := []byte("fiberpulse-release-artifact-bytes")
	digest := sha256.Sum256(artifact)
	fixture := &feedFixture{artifact: artifact, privateKey: privateKey, publicHex: hex.EncodeToString(publicKey)}
	fixture.manifest = Manifest{
		Version:        "1.2.0",
		Channel:        "stable",
		Sequence:       7,
		SHA256:         hex.EncodeToString(digest[:]),
		Size:           int64(len(artifact)),
		MinimumVersion: "1.0.0",
		ExpiresAt:      time.Now().UTC().Add(time.Hour).Truncate(time.Second),
	}
	if mutate != nil {
		mutate(&fixture.manifest)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/download/fiberpulse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(fixture.artifact)
	})
	mux.HandleFunc("/download/latest-macos-universal.json", func(w http.ResponseWriter, r *http.Request) {
		manifest := fixture.manifest
		manifest.URL = fixture.server.URL + "/download/fiberpulse"
		raw, err := Sign(manifest, fixture.privateKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	})
	mux.HandleFunc("/download/trailing.json", func(w http.ResponseWriter, r *http.Request) {
		manifest := fixture.manifest
		manifest.URL = fixture.server.URL + "/download/fiberpulse"
		raw, err := Sign(manifest, fixture.privateKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(append(raw, []byte(" {}")...))
	})
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *feedFixture) client(t *testing.T, feedURL string) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{FeedURL: feedURL, PublicKeyHex: f.publicHex, Channel: "stable", CurrentVersion: "1.1.0", Platform: "macos-universal"})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestClientCheckReturnsSignedCandidateAndRawBytes(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	client := fixture.client(t, fixture.server.URL+"/download")
	manifest, raw, err := client.Check(context.Background(), State{})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.2.0" || manifest.Sequence != 7 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if _, err := loadManifest(raw, fixture.publicHex); err != nil {
		t.Fatalf("raw bytes do not re-verify: %v", err)
	}
}

func TestClientCheckReportsNoUpdateForOlderOrEqualVersions(t *testing.T) {
	fixture := newFeedFixture(t, func(m *Manifest) { m.Version = "1.1.0" })
	client := fixture.client(t, fixture.server.URL+"/download")
	if _, _, err := client.Check(context.Background(), State{}); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("expected ErrNoUpdate, got %v", err)
	}
}

func TestClientCheckReportsNoUpdateForReplayedSequence(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	client := fixture.client(t, fixture.server.URL+"/download")
	state := State{HighestSequence: 7, Version: "1.2.0", UpdatedAt: time.Now().UTC()}
	if _, _, err := client.Check(context.Background(), state); !errors.Is(err, ErrNoUpdate) {
		t.Fatalf("expected ErrNoUpdate, got %v", err)
	}
}

func TestClientCheckRejectsExpiredAndChannelMismatch(t *testing.T) {
	fixture := newFeedFixture(t, func(m *Manifest) { m.ExpiresAt = time.Now().UTC().Add(-time.Minute) })
	client := fixture.client(t, fixture.server.URL+"/download")
	if _, _, err := client.Check(context.Background(), State{}); err == nil || errors.Is(err, ErrNoUpdate) || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired manifest must fail validation: %v", err)
	}

	fixture = newFeedFixture(t, func(m *Manifest) { m.Channel = "canary" })
	client = fixture.client(t, fixture.server.URL+"/download")
	if _, _, err := client.Check(context.Background(), State{}); err == nil || !strings.Contains(err.Error(), "channel") {
		t.Fatalf("channel mismatch must fail validation: %v", err)
	}
}

func TestClientCheckRejectsForgedSignature(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongClient, err := NewClient(ClientConfig{FeedURL: fixture.server.URL + "/download", PublicKeyHex: hex.EncodeToString(otherPublic), Channel: "stable", CurrentVersion: "1.1.0", Platform: "macos-universal"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := wrongClient.Check(context.Background(), State{}); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("forged manifest accepted: %v", err)
	}
}

func TestClientDownloadVerifiesHashAndSize(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	client := fixture.client(t, fixture.server.URL+"/download")
	manifest, _, err := client.Check(context.Background(), State{})
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), "agent-download")
	if err := client.Download(context.Background(), manifest, staged); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(fixture.artifact) {
		t.Fatal("staged artifact content mismatch")
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("staged artifact permissions: %v", info.Mode().Perm())
	}
	if err := client.Download(context.Background(), manifest, staged); err == nil {
		t.Fatal("re-download over an existing staged path succeeded")
	}
}

func TestClientDownloadRejectsModifiedArtifact(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	fixture.artifact = []byte("attacker-controlled-bytes")
	client := fixture.client(t, fixture.server.URL+"/download")
	manifest, _, err := client.Check(context.Background(), State{})
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), "agent-download")
	if err := client.Download(context.Background(), manifest, staged); err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("modified artifact accepted: %v", err)
	}
	if _, err := os.Lstat(staged); !os.IsNotExist(err) {
		t.Fatalf("partial download left behind: %v", err)
	}
}

func TestClientFollowsGitHubStyleRedirects(t *testing.T) {
	fixture := newFeedFixture(t, nil)
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".json") {
			http.Redirect(w, r, fixture.server.URL+"/download/latest-macos-universal.json", http.StatusFound)
			return
		}
		http.Redirect(w, r, fixture.server.URL+"/download/fiberpulse", http.StatusFound)
	}))
	defer redirector.Close()
	client := fixture.client(t, redirector.URL+"/releases")
	manifest, _, err := client.Check(context.Background(), State{})
	if err != nil {
		t.Fatal(err)
	}
	manifest.URL = redirector.URL + "/releases/fiberpulse"
	staged := filepath.Join(t.TempDir(), "agent-download")
	if err := client.Download(context.Background(), manifest, staged); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientValidatesOperatorConfiguration(t *testing.T) {
	if _, err := NewClient(ClientConfig{FeedURL: "http://updates.example.com", PublicKeyHex: strings.Repeat("00", 32), Channel: "stable", CurrentVersion: "1.0.0", Platform: "macos-universal"}); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("plaintext remote feed accepted: %v", err)
	}
	if _, err := NewClient(ClientConfig{FeedURL: "https://updates.example.com", PublicKeyHex: "abcd", Channel: "stable", CurrentVersion: "1.0.0", Platform: "macos-universal"}); err == nil {
		t.Fatal("short public key accepted")
	}
	if _, err := NewClient(ClientConfig{FeedURL: "https://updates.example.com", PublicKeyHex: strings.Repeat("00", 32), Channel: "nightly", CurrentVersion: "1.0.0", Platform: "macos-universal"}); err == nil {
		t.Fatal("unknown channel accepted")
	}
	if _, err := NewClient(ClientConfig{FeedURL: "https://updates.example.com", PublicKeyHex: strings.Repeat("00", 32), Channel: "stable", CurrentVersion: "dev", Platform: "macos-universal"}); err == nil {
		t.Fatal("non-semantic installed version accepted")
	}
	if _, err := NewClient(ClientConfig{FeedURL: "https://updates.example.com", PublicKeyHex: strings.Repeat("00", 32), Channel: "stable", CurrentVersion: "1.0.0", Platform: "plan9"}); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestSignRoundTripAndFeedFileNames(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Version: "2.0.0", Channel: "stable", Sequence: 3, SHA256: strings.Repeat("a1", 32), Size: 10, URL: "https://updates.example/x", MinimumVersion: "1.0.0", ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Second)}
	raw, err := Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loadManifest(raw, hex.EncodeToString(publicKey))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != manifest.Version || loaded.Sequence != manifest.Sequence {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	for platform, name := range map[string]string{"windows-x64": "latest-windows-x64.json", "macos-universal": "latest-macos-universal.json"} {
		got, err := FeedFileName(platform)
		if err != nil || got != name {
			t.Fatalf("FeedFileName(%s) = %s, %v", platform, got, err)
		}
	}
	if _, err := FeedFileName("freebsd"); err == nil {
		t.Fatal("unsupported platform mapped to a feed")
	}
}
