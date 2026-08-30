package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fiberpulse.dev/agent/internal/measurement"
	"fiberpulse.dev/agent/internal/update"
)

const (
	updateStatusDisabled    = "disabled"
	updateStatusIdle        = "idle"
	updateStatusChecking    = "checking"
	updateStatusUpToDate    = "up_to_date"
	updateStatusAvailable   = "available"
	updateStatusDeferred    = "deferred"
	updateStatusDownloading = "downloading"
	updateStatusInstalling  = "installing"
	updateStatusError       = "error"

	updateGuardSetting = "update_guard_v1"

	updateFirstCheckDelay  = 45 * time.Second
	updateFirstCheckJitter = 60 * time.Second
	updatePollInterval     = 6 * time.Hour
	updatePollJitter       = 30 * time.Minute
	updateTestRetryDelay   = 30 * time.Minute
	updateGuardWindow      = 24 * time.Hour
)

// ErrUpdateInProgress rejects measurement starts while an update download or
// installation would distort the line. The scheduler treats it like a busy
// measurement and retries a few minutes later.
var ErrUpdateInProgress = errors.New("an application update is in progress")

// UpdateConfig carries the release-channel wiring injected at build time. A
// nil config (development builds) disables discovery without failing startup.
type UpdateConfig struct {
	FeedURL            string
	PublicKeyHex       string
	Channel            string
	Platform           string
	BundlePath         string
	Executable         string
	UpdaterPath        string
	DataDir            string
	SkipPlatformVerify bool
	HTTPClient         *http.Client
	Spawn              func(path string, args ...string) error
	Now                func() time.Time
}

// UpdateStatus is the dashboard-facing view of the release channel.
type UpdateStatus struct {
	Status           string    `json:"status"`
	Channel          string    `json:"channel,omitempty"`
	CurrentVersion   string    `json:"current_version"`
	AvailableVersion string    `json:"available_version,omitempty"`
	CheckedAt        time.Time `json:"checked_at,omitempty"`
	Error            string    `json:"error,omitempty"`
}

type persistedUpdateGuard struct {
	Version     string    `json:"version"`
	AttemptedAt time.Time `json:"attempted_at"`
}

func (a *App) configureUpdates(config *UpdateConfig) error {
	if config == nil {
		a.updateStatus = UpdateStatus{Status: updateStatusDisabled, CurrentVersion: a.config.Version}
		return nil
	}
	if config.Executable == "" || config.UpdaterPath == "" || config.DataDir == "" {
		return errors.New("update executable, updater helper and data directory are required")
	}
	platform := config.Platform
	if platform == "" {
		var err error
		platform, err = update.DefaultPlatform()
		if err != nil {
			return err
		}
	}
	if config.BundlePath != "" && platform != "macos-universal" {
		return errors.New("bundle updates are only supported on macOS")
	}
	config.Platform = platform
	channel := config.Channel
	if channel == "" {
		channel = "stable"
	}
	now := time.Now
	if config.Now != nil {
		now = config.Now
	}
	client, err := update.NewClient(update.ClientConfig{
		FeedURL:        config.FeedURL,
		PublicKeyHex:   config.PublicKeyHex,
		Channel:        channel,
		CurrentVersion: a.config.Version,
		Platform:       platform,
		HTTPClient:     config.HTTPClient,
		Now:            now,
	})
	if err != nil {
		return err
	}
	a.updateClient = client
	a.updateConfig = config
	a.updateNow = now
	a.updateStatus = UpdateStatus{Status: updateStatusIdle, Channel: channel, CurrentVersion: a.config.Version}
	return nil
}

// CheckForUpdate runs an operator-triggered check. When a newer signed
// release exists and no measurement is running, it is downloaded and
// installed immediately; the agent then restarts into the new version.
func (a *App) CheckForUpdate(ctx context.Context) error {
	if a.updateClient == nil {
		return errors.New("automatic updates are not configured in this build")
	}
	_, err := a.runUpdateCheck(ctx, true)
	return err
}

// updateBusy reports whether an update transfer or install is using the line;
// StartTest refuses to run in that window so measurements stay clean.
func (a *App) updateBusy() bool {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	return a.updateStatus.Status == updateStatusDownloading || a.updateStatus.Status == updateStatusInstalling
}

func (a *App) currentUpdateStatus() UpdateStatus {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	return a.updateStatus
}

func (a *App) setUpdateStatus(status string, available string, err error) {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	a.updateStatus.Status = status
	a.updateStatus.AvailableVersion = available
	if err != nil {
		a.updateStatus.Error = err.Error()
	} else {
		a.updateStatus.Error = ""
	}
	if status == updateStatusUpToDate || status == updateStatusAvailable {
		a.updateStatus.CheckedAt = a.updateNow().UTC()
	}
}

// tryStartUpdateCheck enforces a single in-flight check or install.
func (a *App) tryStartUpdateCheck() bool {
	a.updateMu.Lock()
	defer a.updateMu.Unlock()
	switch a.updateStatus.Status {
	case updateStatusChecking, updateStatusDownloading, updateStatusInstalling:
		return false
	}
	a.updateStatus.Status = updateStatusChecking
	a.updateStatus.Error = ""
	return true
}

func (a *App) updateLoop() {
	delay := updateFirstCheckDelay + time.Duration(randomUnit()*float64(updateFirstCheckJitter))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-timer.C:
			retry, err := a.runUpdateCheck(a.ctx, false)
			if err != nil {
				a.config.Logger.Warn("automatic update check failed", "error", err)
			}
			interval := updatePollInterval + time.Duration((randomUnit()-0.5)*2*float64(updatePollJitter))
			if retry > 0 {
				interval = retry
			}
			timer.Reset(interval)
		}
	}
}

// runUpdateCheck performs one discovery cycle and, when a candidate exists
// and the line is idle, drives the staged install. It returns an optional
// retry delay for the automatic loop.
func (a *App) runUpdateCheck(ctx context.Context, manual bool) (time.Duration, error) {
	if a.updateClient == nil {
		return 0, errors.New("automatic updates are not configured in this build")
	}
	if !a.tryStartUpdateCheck() {
		return 0, nil
	}
	if a.measurementActive() {
		if manual {
			a.setUpdateStatus(updateStatusIdle, "", nil)
			return 0, ErrMeasurementBusy
		}
		a.setUpdateStatus(updateStatusDeferred, "", nil)
		return updateTestRetryDelay, nil
	}
	state, err := update.ReadState(a.updateStatePath())
	if err != nil {
		a.setUpdateStatus(updateStatusError, "", err)
		return 0, err
	}
	manifest, raw, err := a.updateClient.Check(ctx, state)
	if errors.Is(err, update.ErrNoUpdate) {
		a.setUpdateStatus(updateStatusUpToDate, "", nil)
		return 0, nil
	}
	if err != nil {
		a.setUpdateStatus(updateStatusError, "", err)
		return 0, err
	}
	if !manual {
		blocked, guardErr := a.updateGuardBlocks(ctx, manifest.Version)
		if guardErr != nil {
			a.setUpdateStatus(updateStatusError, "", guardErr)
			return 0, guardErr
		}
		if blocked {
			a.setUpdateStatus(updateStatusError, manifest.Version, fmt.Errorf("update to %s did not complete after the last attempt; retrying after the safety window", manifest.Version))
			return 0, nil
		}
	}
	a.setUpdateStatus(updateStatusAvailable, manifest.Version, nil)
	if a.measurementActive() {
		a.setUpdateStatus(updateStatusDeferred, manifest.Version, nil)
		return updateTestRetryDelay, nil
	}
	return 0, a.installCandidate(ctx, manifest, raw)
}

// installCandidate downloads the verified artifact, persists the exact signed
// manifest bytes, then hands off to the detached updater helper before
// quitting. The helper waits for this process to exit, swaps the install,
// launches the new version and rolls back on any failure.
func (a *App) installCandidate(ctx context.Context, manifest update.Manifest, raw []byte) error {
	config := a.updateConfig
	if config.BundlePath != "" {
		if err := ensureWritableDirectory(filepath.Dir(config.BundlePath)); err != nil {
			a.setUpdateStatus(updateStatusError, manifest.Version, err)
			return err
		}
	}
	staging := filepath.Join(config.DataDir, "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		a.setUpdateStatus(updateStatusError, manifest.Version, err)
		return fmt.Errorf("create update staging directory: %w", err)
	}
	manifestPath := filepath.Join(staging, "manifest.json")
	downloadName := "agent-download"
	if config.BundlePath != "" {
		downloadName = "FiberPulse-update.zip"
	}
	downloadPath := filepath.Join(staging, downloadName)
	for _, leftover := range []string{manifestPath, downloadPath} {
		if err := os.Remove(leftover); err != nil && !os.IsNotExist(err) {
			a.setUpdateStatus(updateStatusError, manifest.Version, err)
			return fmt.Errorf("clear update staging file: %w", err)
		}
	}
	a.setUpdateStatus(updateStatusDownloading, manifest.Version, nil)
	if err := a.updateClient.Download(ctx, manifest, downloadPath); err != nil {
		a.setUpdateStatus(updateStatusError, manifest.Version, err)
		return err
	}
	if err := writePrivateFile(manifestPath, raw); err != nil {
		a.setUpdateStatus(updateStatusError, manifest.Version, err)
		return err
	}
	guard := persistedUpdateGuard{Version: manifest.Version, AttemptedAt: a.updateNow().UTC()}
	if err := a.store.SetSetting(ctx, updateGuardSetting, guard); err != nil {
		a.setUpdateStatus(updateStatusError, manifest.Version, err)
		return fmt.Errorf("persist update safety guard: %w", err)
	}
	a.setUpdateStatus(updateStatusInstalling, manifest.Version, nil)
	if err := a.spawnUpdater(manifest, downloadPath, manifestPath); err != nil {
		_ = a.store.SetSetting(context.Background(), updateGuardSetting, persistedUpdateGuard{})
		a.setUpdateStatus(updateStatusError, manifest.Version, err)
		return err
	}
	a.config.Logger.Info("update staged, restarting into the new version", "version", manifest.Version)
	// The helper waits for this process to exit before touching the install.
	go func() {
		time.Sleep(250 * time.Millisecond)
		a.cancel()
	}()
	return nil
}

func (a *App) spawnUpdater(manifest update.Manifest, downloadPath, manifestPath string) error {
	config := a.updateConfig
	kind := update.KindFile
	target := config.Executable
	if config.BundlePath != "" {
		kind = update.KindBundle
		target = config.BundlePath
	}
	arguments := []string{
		"-target", target,
		"-staged", downloadPath,
		"-manifest", manifestPath,
		"-public-key", config.PublicKeyHex,
		"-state", a.updateStatePath(),
		"-current-version", a.config.Version,
		"-channel", a.updateStatus.Channel,
		"-kind", string(kind),
		"-timeout", "45s",
		"-wait-pid", fmt.Sprintf("%d", os.Getpid()),
	}
	if kind == update.KindBundle {
		arguments = append(arguments, "-executable", config.Executable)
	}
	if config.SkipPlatformVerify {
		arguments = append(arguments, "-skip-platform-verify")
	}
	spawn := defaultUpdaterSpawn(config.DataDir)
	if config.Spawn != nil {
		spawn = func(path string, args ...string) error { return config.Spawn(path, args...) }
	}
	if err := spawn(config.UpdaterPath, arguments...); err != nil {
		return fmt.Errorf("start updater helper: %w", err)
	}
	return nil
}

// updateGuardBlocks reports whether a previous attempt at this exact version
// already failed inside the safety window, which prevents a broken release
// from looping install/rollback cycles.
func (a *App) updateGuardBlocks(ctx context.Context, version string) (bool, error) {
	var guard persistedUpdateGuard
	if _, err := a.store.GetSetting(ctx, updateGuardSetting, &guard); err != nil {
		return false, fmt.Errorf("load update safety guard: %w", err)
	}
	if guard.Version == "" || guard.Version != version {
		return false, nil
	}
	return a.updateNow().UTC().Sub(guard.AttemptedAt) < updateGuardWindow, nil
}

func (a *App) updateStatePath() string {
	return filepath.Join(a.updateConfig.DataDir, "update-state.json")
}

func (a *App) measurementActive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.testMachine.State != measurement.TestIdle
}

// defaultUpdaterSpawn starts the helper detached with its diagnostics
// captured beside the database, then returns without waiting.
func defaultUpdaterSpawn(dataDir string) func(path string, args ...string) error {
	return func(path string, args ...string) error {
		command := exec.Command(path, args...)
		update.DetachProcess(command)
		logPath := filepath.Join(dataDir, "updater.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open updater diagnostics log: %w", err)
		}
		defer logFile.Close()
		command.Stdout = logFile
		command.Stderr = logFile
		if err := command.Start(); err != nil {
			return err
		}
		return nil
	}
}

// ensureWritableDirectory proves the update can stage beside the install
// before any download starts, failing fast with an actionable message.
func ensureWritableDirectory(directory string) error {
	file, err := os.CreateTemp(directory, ".fiberpulse-write-check-*")
	if err != nil {
		return fmt.Errorf("the folder containing the installed application is not writable; move FiberPulse to a folder you own: %w", err)
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}

func writePrivateFile(path string, raw []byte) error {
	temporary := path + ".new"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create staged update manifest: %w", err)
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("write staged update manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return fmt.Errorf("flush staged update manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("close staged update manifest: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish staged update manifest: %w", err)
	}
	return nil
}
