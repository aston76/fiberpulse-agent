package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Options struct {
	Target         string
	Staged         string
	ManifestPath   string
	PublicKeyHex   string
	StatePath      string
	CurrentVersion string
	Channel        string
	HealthTimeout  time.Duration
	Now            func() time.Time
	VerifyPlatform func(string) error
}

type healthReceipt struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
}

type managedProcess struct {
	command *exec.Cmd
	done    chan struct{}
	mu      sync.Mutex
	err     error
}

func Apply(ctx context.Context, options Options) error {
	if err := validateOptions(options); err != nil {
		return err
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	verifyPlatform := platformVerify
	if options.VerifyPlatform != nil {
		verifyPlatform = options.VerifyPlatform
	}

	if err := verifyRegularFile(options.ManifestPath, "update manifest"); err != nil {
		return err
	}
	manifestRaw, err := os.ReadFile(options.ManifestPath)
	if err != nil {
		return fmt.Errorf("read update manifest: %w", err)
	}
	manifest, err := loadManifest(manifestRaw, options.PublicKeyHex)
	if err != nil {
		return err
	}
	state, err := loadState(options.StatePath)
	if err != nil {
		return err
	}
	if err := manifest.validate(now().UTC(), options.CurrentVersion, options.Channel, state); err != nil {
		return err
	}
	if err := verifyRegularFile(options.Target, "installed agent"); err != nil {
		return err
	}
	if err := verifyRegularFile(options.Staged, "staged agent"); err != nil {
		return err
	}
	if err := verifyArtifact(options.Staged, manifest); err != nil {
		return err
	}
	if err := verifyPlatform(options.Staged); err != nil {
		return fmt.Errorf("verify staged platform signature: %w", err)
	}

	backup := options.Target + ".previous"
	retired := backup + ".retired"
	if _, err := os.Lstat(retired); err == nil {
		return errors.New("retired update backup already exists; refusing to overwrite recovery data")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect retired update backup: %w", err)
	}
	hadPrevious := false
	if _, err := os.Lstat(backup); err == nil {
		if err := verifyRegularFile(backup, "previous agent backup"); err != nil {
			return err
		}
		if err := os.Rename(backup, retired); err != nil {
			return fmt.Errorf("rotate previous update backup: %w", err)
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect previous update backup: %w", err)
	}
	if err := os.Rename(options.Target, backup); err != nil {
		_ = restoreRetiredBackup(backup, retired, hadPrevious)
		return fmt.Errorf("backup installed agent: %w", err)
	}
	if err := copyFileExclusive(options.Staged, options.Target); err != nil {
		rollbackErr := restoreFiles(options.Target, backup, retired, hadPrevious)
		return errors.Join(fmt.Errorf("install staged agent: %w", err), rollbackErr)
	}

	healthPath, err := reserveHealthPath(filepath.Dir(options.Target))
	if err != nil {
		rollbackErr := restoreFiles(options.Target, backup, retired, hadPrevious)
		return errors.Join(err, rollbackErr)
	}
	defer os.Remove(healthPath)
	process, err := startManagedProcess(options.Target, "--post-update", manifest.Version, "--update-health-file", healthPath)
	if err != nil {
		rollbackErr := rollbackAndRestart(options.Target, backup, retired, hadPrevious, manifest.Version, nil)
		return errors.Join(fmt.Errorf("start updated agent: %w", err), rollbackErr)
	}
	if err := waitForHealth(ctx, process, healthPath, manifest.Version, options.HealthTimeout); err != nil {
		rollbackErr := rollbackAndRestart(options.Target, backup, retired, hadPrevious, manifest.Version, process)
		return errors.Join(err, rollbackErr)
	}
	if hadPrevious {
		if err := os.Remove(retired); err != nil {
			rollbackErr := rollbackAndRestart(options.Target, backup, retired, true, manifest.Version, process)
			return errors.Join(fmt.Errorf("remove retired update backup after successful health check: %w", err), rollbackErr)
		}
		hadPrevious = false
	}
	newState := State{HighestSequence: manifest.Sequence, Version: manifest.Version, UpdatedAt: now().UTC()}
	if err := saveState(options.StatePath, newState); err != nil {
		rollbackErr := rollbackAndRestart(options.Target, backup, retired, hadPrevious, manifest.Version, process)
		return errors.Join(fmt.Errorf("persist anti-rollback state: %w", err), rollbackErr)
	}
	return nil
}

func validateOptions(options Options) error {
	if options.Target == "" || options.Staged == "" || options.ManifestPath == "" || options.StatePath == "" || options.PublicKeyHex == "" || options.CurrentVersion == "" || options.Channel == "" {
		return errors.New("target, staged, manifest, state, public key, current version, and channel are required")
	}
	if !filepath.IsAbs(options.Target) || !filepath.IsAbs(options.Staged) || !filepath.IsAbs(options.ManifestPath) || !filepath.IsAbs(options.StatePath) {
		return errors.New("all updater paths must be absolute")
	}
	target, err := filepath.Abs(options.Target)
	if err != nil {
		return err
	}
	staged, err := filepath.Abs(options.Staged)
	if err != nil {
		return err
	}
	if target == staged {
		return errors.New("installed and staged agent paths must differ")
	}
	if options.HealthTimeout <= 0 || options.HealthTimeout > 2*time.Minute {
		return errors.New("health timeout must be between zero and two minutes")
	}
	return nil
}

func verifyRegularFile(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must be a regular file, not a link", label)
	}
	return nil
}

func verifyArtifact(path string, manifest Manifest) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open staged artifact: %w", err)
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return fmt.Errorf("hash staged artifact: %w", err)
	}
	if size != manifest.Size || hex.EncodeToString(digest.Sum(nil)) != manifest.SHA256 {
		return errors.New("staged artifact hash or size mismatch")
	}
	return nil
}

func copyFileExclusive(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	temporary := target + ".new"
	out, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	syncErr := out.Sync()
	closeErr := out.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if err := os.Rename(temporary, filepath.Clean(target)); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func reserveHealthPath(directory string) (string, error) {
	file, err := os.CreateTemp(directory, ".fiberpulse-health-*")
	if err != nil {
		return "", fmt.Errorf("reserve post-update health path: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func startManagedProcess(path string, arguments ...string) (*managedProcess, error) {
	command := exec.Command(path, arguments...)
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &managedProcess{command: command, done: make(chan struct{})}
	go func() {
		err := command.Wait()
		process.mu.Lock()
		process.err = err
		process.mu.Unlock()
		close(process.done)
	}()
	return process, nil
}

func (p *managedProcess) waitError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func waitForHealth(ctx context.Context, process *managedProcess, path, version string, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("post-update health check cancelled: %w", context.Cause(ctx))
		case <-process.done:
			err := process.waitError()
			if err == nil {
				err = errors.New("updated agent exited before confirming health")
			}
			return fmt.Errorf("updated agent exited before confirming health: %w", err)
		case <-timer.C:
			return errors.New("updated agent did not confirm health before timeout")
		case <-ticker.C:
			receipt, err := readHealthReceipt(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if receipt.Version != version || receipt.PID != process.command.Process.Pid {
				return errors.New("post-update health receipt does not match the launched agent")
			}
			select {
			case <-process.done:
				err := process.waitError()
				if err == nil {
					err = errors.New("updated agent exited immediately after confirming health")
				}
				return err
			default:
				return nil
			}
		}
	}
}

func readHealthReceipt(path string) (healthReceipt, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return healthReceipt{}, err
	}
	var receipt healthReceipt
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return healthReceipt{}, fmt.Errorf("decode post-update health receipt: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return healthReceipt{}, err
	}
	if receipt.Version == "" || receipt.PID <= 0 {
		return healthReceipt{}, errors.New("post-update health receipt is incomplete")
	}
	return receipt, nil
}

func rollbackAndRestart(target, backup, retired string, hadPrevious bool, failedVersion string, process *managedProcess) error {
	var result error
	if process != nil {
		if err := process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			result = errors.Join(result, fmt.Errorf("stop failed update: %w", err))
		}
		select {
		case <-process.done:
		case <-time.After(5 * time.Second):
			result = errors.Join(result, errors.New("failed update process did not exit during rollback"))
		}
	}
	result = errors.Join(result, restoreFiles(target, backup, retired, hadPrevious))
	if result != nil {
		return result
	}
	restored, err := startManagedProcess(target, "--post-update", "rollback-"+failedVersion)
	if err != nil {
		return fmt.Errorf("restart restored agent: %w", err)
	}
	_ = restored
	return nil
}

func restoreFiles(target, backup, retired string, hadPrevious bool) error {
	var result error
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		result = errors.Join(result, fmt.Errorf("remove failed update: %w", err))
	}
	if result == nil {
		if err := os.Rename(backup, target); err != nil {
			result = errors.Join(result, fmt.Errorf("restore previous agent: %w", err))
		}
	}
	return errors.Join(result, restoreRetiredBackup(backup, retired, hadPrevious))
}

func restoreRetiredBackup(backup, retired string, hadPrevious bool) error {
	if !hadPrevious {
		return nil
	}
	if _, err := os.Lstat(backup); err == nil {
		return errors.New("cannot restore retired backup because previous backup path is occupied")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(retired, backup); err != nil {
		return fmt.Errorf("restore retired update backup: %w", err)
	}
	return nil
}

func loadState(path string) (State, error) {
	info, statErr := os.Lstat(path)
	if os.IsNotExist(statErr) {
		return State{}, nil
	}
	if statErr != nil {
		return State{}, fmt.Errorf("inspect anti-rollback state: %w", statErr)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return State{}, errors.New("anti-rollback state must be a regular file, not a link")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read anti-rollback state: %w", err)
	}
	var state State
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode anti-rollback state: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return State{}, err
	}
	if state.HighestSequence > 0 {
		if _, err := parseSemanticVersion(state.Version); err != nil {
			return State{}, fmt.Errorf("invalid anti-rollback state version: %w", err)
		}
	}
	return state, nil
}

func saveState(path string, state State) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".new"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(temporary)
		return errors.Join(writeErr, syncErr, closeErr)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func platformVerificationOutput(command *exec.Cmd) error {
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}
	return nil
}
