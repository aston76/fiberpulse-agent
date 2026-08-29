//go:build !windows

package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestApplyPublishesHealthyUpdateAndPersistsSequence(t *testing.T) {
	fixture := newUpdateFixture(t, true)
	if err := Apply(t.Context(), fixture.options); err != nil {
		t.Fatal(err)
	}
	assertFileEquals(t, fixture.target, fixture.staged)
	if got := readFile(t, fixture.target+".previous"); got != fixture.oldAgent {
		t.Fatalf("previous backup changed: %q", got)
	}
	if _, err := os.Stat(fixture.target + ".previous.retired"); !os.IsNotExist(err) {
		t.Fatalf("retired backup remains after successful update: %v", err)
	}
	state, err := loadState(fixture.statePath)
	if err != nil || state.HighestSequence != 2 || state.Version != "1.1.0" {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	stopFixtureProcess(t, fixture.pidPath)
}

func TestApplyRollsBackAndRestartsPreviousAgentWhenHealthFails(t *testing.T) {
	fixture := newUpdateFixture(t, false)
	if err := Apply(t.Context(), fixture.options); err == nil {
		t.Fatal("unhealthy update succeeded")
	}
	if got := readFile(t, fixture.target); got != fixture.oldAgent {
		t.Fatalf("installed agent was not restored: %q", got)
	}
	if _, err := os.Stat(fixture.statePath); !os.IsNotExist(err) {
		t.Fatalf("anti-rollback state advanced after failed update: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if raw, err := os.ReadFile(fixture.rollbackMarker); err == nil && strings.Contains(string(raw), "rollback-1.1.0") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restored agent was not restarted")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestApplyRejectsReplayBeforeTouchingInstalledAgent(t *testing.T) {
	fixture := newUpdateFixture(t, true)
	if err := saveState(fixture.statePath, State{HighestSequence: 2, Version: "1.1.0", UpdatedAt: fixture.now}); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, fixture.target)
	if err := Apply(t.Context(), fixture.options); err == nil {
		t.Fatal("replayed sequence accepted")
	}
	if got := readFile(t, fixture.target); got != before {
		t.Fatal("installed agent changed after replay rejection")
	}
}

func TestApplyRejectsSymlinkedAntiRollbackState(t *testing.T) {
	fixture := newUpdateFixture(t, true)
	realState := filepath.Join(fixture.directory, "real-update-state.json")
	if err := saveState(realState, State{HighestSequence: 1, Version: "1.0.0", UpdatedAt: fixture.now}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realState, fixture.statePath); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, fixture.target)
	err := Apply(t.Context(), fixture.options)
	if err == nil || !strings.Contains(err.Error(), "anti-rollback state must be a regular file") {
		t.Fatalf("symlinked anti-rollback state was not rejected: %v", err)
	}
	if got := readFile(t, fixture.target); got != before {
		t.Fatal("installed agent changed after symlink rejection")
	}
}

func TestApplyRejectsExpiredManifestAndPlatformFailure(t *testing.T) {
	fixture := newUpdateFixture(t, true)
	fixture.manifest.ExpiresAt = fixture.now.Add(-time.Second)
	fixture.writeManifest(t)
	if err := Apply(t.Context(), fixture.options); err == nil {
		t.Fatal("expired manifest accepted")
	}

	fixture = newUpdateFixture(t, true)
	fixture.options.VerifyPlatform = func(string) error { return fmt.Errorf("signature invalid") }
	if err := Apply(t.Context(), fixture.options); err == nil || !strings.Contains(err.Error(), "platform signature") {
		t.Fatalf("platform signature failure not enforced: %v", err)
	}
}

type updateFixture struct {
	directory      string
	target         string
	staged         string
	statePath      string
	manifestPath   string
	pidPath        string
	rollbackMarker string
	oldAgent       string
	now            time.Time
	manifest       Manifest
	privateKey     ed25519.PrivateKey
	options        Options
}

func newUpdateFixture(t *testing.T, healthy bool) *updateFixture {
	t.Helper()
	directory := t.TempDir()
	fixture := &updateFixture{
		directory:      directory,
		target:         filepath.Join(directory, "fiberpulse"),
		staged:         filepath.Join(directory, "fiberpulse-staged"),
		statePath:      filepath.Join(directory, "update-state.json"),
		manifestPath:   filepath.Join(directory, "manifest.json"),
		pidPath:        filepath.Join(directory, "updated.pid"),
		rollbackMarker: filepath.Join(directory, "rollback.marker"),
		now:            time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
	}
	fixture.oldAgent = fmt.Sprintf("#!/bin/sh\nprintf '%%s' \"$*\" > %s\nexit 0\n", shellQuote(fixture.rollbackMarker))
	writeExecutable(t, fixture.target, fixture.oldAgent)
	writeExecutable(t, fixture.target+".previous", "#!/bin/sh\nexit 0\n")
	if healthy {
		writeExecutable(t, fixture.staged, fmt.Sprintf("#!/bin/sh\nprintf '{\"version\":\"1.1.0\",\"pid\":%%s}' \"$$\" > \"$4\"\nprintf '%%s' \"$$\" > %s\nsleep 30\n", shellQuote(fixture.pidPath)))
	} else {
		writeExecutable(t, fixture.staged, "#!/bin/sh\nexit 7\n")
	}
	stagedRaw, err := os.ReadFile(fixture.staged)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(stagedRaw)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture.privateKey = privateKey
	fixture.manifest = Manifest{Version: "1.1.0", Channel: "stable", Sequence: 2, SHA256: hex.EncodeToString(digest[:]), Size: int64(len(stagedRaw)), URL: "https://updates.example/fiberpulse", MinimumVersion: "1.0.0", ExpiresAt: fixture.now.Add(time.Hour)}
	fixture.options = Options{Target: fixture.target, Staged: fixture.staged, ManifestPath: fixture.manifestPath, PublicKeyHex: hex.EncodeToString(publicKey), StatePath: fixture.statePath, CurrentVersion: "1.0.0", Channel: "stable", HealthTimeout: 2 * time.Second, Now: func() time.Time { return fixture.now }, VerifyPlatform: func(string) error { return nil }}
	fixture.writeManifest(t)
	return fixture
}

func (f *updateFixture) writeManifest(t *testing.T) {
	t.Helper()
	f.manifest.Signature = nil
	unsigned, err := json.Marshal(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	f.manifest.Signature = ed25519.Sign(f.privateKey, unsigned)
	raw, err := json.Marshal(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertFileEquals(t *testing.T, left, right string) {
	t.Helper()
	if readFile(t, left) != readFile(t, right) {
		t.Fatalf("files differ: %s %s", left, right)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func stopFixtureProcess(t *testing.T, pidPath string) {
	t.Helper()
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err == nil {
		_ = process.Kill()
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
