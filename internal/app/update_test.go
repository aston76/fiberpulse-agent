package app

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
	"sync"
	"testing"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/update"
)

type updateFeedServer struct {
	server    *httptest.Server
	publicHex string
	artifact  []byte
}

func newUpdateFeedServer(t *testing.T, version string, sequence uint64) *updateFeedServer {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &updateFeedServer{publicHex: hex.EncodeToString(publicKey), artifact: []byte("fiberpulse-agent-1.2.0-binary")}
	mux := http.NewServeMux()
	mux.HandleFunc("/fiberpulse", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fixture.artifact)
	})
	serveManifest := func(w http.ResponseWriter, r *http.Request) {
		digest := sha256.Sum256(fixture.artifact)
		raw, err := update.Sign(update.Manifest{
			Version:        version,
			Channel:        "stable",
			Sequence:       sequence,
			SHA256:         hex.EncodeToString(digest[:]),
			Size:           int64(len(fixture.artifact)),
			URL:            fixture.server.URL + "/fiberpulse",
			MinimumVersion: "1.0.0",
			ExpiresAt:      time.Now().UTC().Add(time.Hour).Truncate(time.Second),
		}, privateKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}
	mux.HandleFunc("/latest-windows-x64.json", serveManifest)
	mux.HandleFunc("/latest-macos-universal.json", serveManifest)
	fixture.server = httptest.NewServer(mux)
	t.Cleanup(fixture.server.Close)
	return fixture
}

type spawnRecorder struct {
	mu   sync.Mutex
	path string
	args []string
}

func (r *spawnRecorder) spawn(path string, args ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = path
	r.args = append([]string(nil), args...)
	return nil
}

func (r *spawnRecorder) called() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.path != ""
}

func newUpdateTestApp(t *testing.T, feed *updateFeedServer, recorder *spawnRecorder, mutate func(*UpdateConfig)) *App {
	t.Helper()
	dataDir := t.TempDir()
	config := &UpdateConfig{
		FeedURL:      feed.server.URL,
		PublicKeyHex: feed.publicHex,
		Channel:      "stable",
		Platform:     "windows-x64",
		Executable:   filepath.Join(dataDir, "fiberpulse"),
		UpdaterPath:  filepath.Join(dataDir, "fiberpulse-updater"),
		DataDir:      dataDir,
		Spawn:        recorder.spawn,
	}
	if mutate != nil {
		mutate(config)
	}
	a, err := New(Config{
		Version:      "1.0.0",
		DatabasePath: filepath.Join(t.TempDir(), "fiberpulse.db"),
		Provider:     &measurement.FakeProvider{Delay: time.Millisecond},
		Update:       config,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestUpdateStatusDisabledWithoutReleaseConfig(t *testing.T) {
	a := newTestApp(t)
	if status := a.currentUpdateStatus(); status.Status != updateStatusDisabled {
		t.Fatalf("status=%q", status.Status)
	}
	if err := a.CheckForUpdate(context.Background()); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("manual check without config: %v", err)
	}
	snapshot, err := a.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.(Snapshot).Update.Status != updateStatusDisabled {
		t.Fatalf("snapshot update status=%q", snapshot.(Snapshot).Update.Status)
	}
}

func TestManualUpdateCheckDownloadsStagesAndRestarts(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.2.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, nil)
	if err := a.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := a.currentUpdateStatus()
	if status.Status != updateStatusInstalling || status.AvailableVersion != "1.2.0" {
		t.Fatalf("status=%+v", status)
	}
	if !recorder.called() {
		t.Fatal("updater helper was not spawned")
	}
	joined := strings.Join(recorder.args, " ")
	for _, fragment := range []string{"-kind file", "-wait-pid", "-current-version 1.0.0", "-channel stable"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("updater arguments missing %q: %s", fragment, joined)
		}
	}
	staged, err := os.ReadFile(filepath.Join(a.updateConfig.DataDir, "staging", "agent-download"))
	if err != nil || string(staged) != string(feed.artifact) {
		t.Fatalf("staged artifact: %v %v", string(staged), err)
	}
	if _, err := os.Stat(filepath.Join(a.updateConfig.DataDir, "staging", "manifest.json")); err != nil {
		t.Fatalf("staged manifest missing: %v", err)
	}
	var guard persistedUpdateGuard
	if found, err := a.store.GetSetting(context.Background(), updateGuardSetting, &guard); err != nil || !found || guard.Version != "1.2.0" {
		t.Fatalf("safety guard not persisted: %+v %v %v", guard, found, err)
	}
	select {
	case <-a.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not begin its restart after staging the update")
	}
}

func TestBundleUpdatePassesBundleLayoutToHelper(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.2.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, func(config *UpdateConfig) {
		bundle := filepath.Join(config.DataDir, "FiberPulse.app")
		config.Platform = "macos-universal"
		config.BundlePath = bundle
		config.Executable = filepath.Join(bundle, "Contents", "MacOS", "fiberpulse")
	})
	if err := os.MkdirAll(filepath.Dir(a.updateConfig.Executable), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(recorder.args, " ")
	for _, fragment := range []string{"-kind bundle", "-executable", "FiberPulse-update.zip"} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("updater arguments missing %q: %s", fragment, joined)
		}
	}
	if _, err := os.Stat(filepath.Join(a.updateConfig.DataDir, "staging", "FiberPulse-update.zip")); err != nil {
		t.Fatalf("bundle archive not staged: %v", err)
	}
	select {
	case <-a.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not begin its restart after staging the bundle update")
	}
}

func TestUpdateCheckDefersWhileMeasurementRuns(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.2.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, nil)
	if err := a.transitionTest(measurement.TestPreflight); err != nil {
		t.Fatal(err)
	}
	retry, err := a.runUpdateCheck(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if retry != updateTestRetryDelay {
		t.Fatalf("retry=%v", retry)
	}
	if status := a.currentUpdateStatus(); status.Status != updateStatusDeferred {
		t.Fatalf("status=%+v", status)
	}
	if recorder.called() {
		t.Fatal("update installed during a measurement")
	}
	if _, err := a.runUpdateCheck(context.Background(), true); !errors.Is(err, ErrMeasurementBusy) {
		t.Fatalf("manual check during a measurement: %v", err)
	}
	if err := a.transitionTest(measurement.TestFailed); err != nil {
		t.Fatal(err)
	}
	if err := a.transitionTest(measurement.TestIdle); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateGuardBlocksRepeatedInstallOfSameVersion(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.2.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, nil)
	guard := persistedUpdateGuard{Version: "1.2.0", AttemptedAt: time.Now().UTC()}
	if err := a.store.SetSetting(context.Background(), updateGuardSetting, guard); err != nil {
		t.Fatal(err)
	}
	if _, err := a.runUpdateCheck(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	status := a.currentUpdateStatus()
	if status.Status != updateStatusError || !strings.Contains(status.Error, "safety window") {
		t.Fatalf("status=%+v", status)
	}
	if recorder.called() {
		t.Fatal("guarded version was installed again automatically")
	}
	if err := a.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !recorder.called() {
		t.Fatal("operator-triggered update did not bypass the safety guard")
	}
	select {
	case <-a.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not begin its restart after the operator update")
	}
}

func TestStartTestRejectedWhileUpdateInstallRuns(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.2.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, nil)
	a.setUpdateStatus(updateStatusInstalling, "1.2.0", nil)
	if err := a.StartTest(context.Background(), "manual"); !errors.Is(err, ErrUpdateInProgress) {
		t.Fatalf("measurement started during update install: %v", err)
	}
}

func TestUpdateCheckReportsUpToDate(t *testing.T) {
	feed := newUpdateFeedServer(t, "1.0.0", 9)
	recorder := &spawnRecorder{}
	a := newUpdateTestApp(t, feed, recorder, nil)
	if err := a.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	status := a.currentUpdateStatus()
	if status.Status != updateStatusUpToDate || status.CheckedAt.IsZero() {
		t.Fatalf("status=%+v", status)
	}
	if recorder.called() {
		t.Fatal("updater spawned for an up-to-date agent")
	}
}
