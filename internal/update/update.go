package update

import (
	"archive/zip"
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
	Target             string
	Staged             string
	ManifestPath       string
	PublicKeyHex       string
	StatePath          string
	CurrentVersion     string
	Channel            string
	Kind               Kind
	Executable         string
	WaitPID            int
	WaitTimeout        time.Duration
	HealthTimeout      time.Duration
	SkipPlatformVerify bool
	Now                func() time.Time
	VerifyPlatform     func(string) error
	ProcessAlive       func(int) bool
}

// Kind selects the replacement strategy. KindFile swaps a single regular
// file (Windows agent). KindBundle swaps a complete macOS .app directory so
// the bundle seal stays valid.
type Kind string

const (
	KindFile   Kind = "file"
	KindBundle Kind = "bundle"
)

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

	kind := options.Kind
	if kind == "" {
		kind = KindFile
	}
	executable := options.Executable
	if kind == KindFile {
		executable = options.Target
	}
	alive := defaultProcessAlive
	if options.ProcessAlive != nil {
		alive = options.ProcessAlive
	}
	waitTimeout := options.WaitTimeout
	if waitTimeout == 0 {
		waitTimeout = time.Minute
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
	if err := waitForProcessExit(ctx, options.WaitPID, alive, waitTimeout); err != nil {
		return err
	}
	if err := verifyTargetPath(options.Target, "installed agent", kind); err != nil {
		return err
	}
	if err := verifyRegularFile(options.Staged, "staged agent archive"); err != nil {
		return err
	}
	if err := verifyArtifact(options.Staged, manifest); err != nil {
		return err
	}
	installSource := options.Staged
	extractRoot := ""
	if kind == KindBundle {
		relativeExecutable, relErr := filepath.Rel(options.Target, executable)
		if relErr != nil {
			return fmt.Errorf("resolve bundle executable location: %w", relErr)
		}
		extracted, root, extractErr := extractBundleArchive(options.Staged, filepath.Dir(options.Target), filepath.Base(options.Target), relativeExecutable)
		if extractErr != nil {
			return extractErr
		}
		extractRoot = root
		defer os.RemoveAll(extractRoot)
		installSource = extracted
	}
	if options.SkipPlatformVerify {
		if kind == KindBundle {
			return errors.New("platform signature verification cannot be skipped for a bundle update")
		}
	} else if err := verifyPlatform(installSource); err != nil {
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
		if err := verifyTargetPath(backup, "previous agent backup", kind); err != nil {
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
	if err := installStaged(installSource, options.Target, kind); err != nil {
		rollbackErr := restoreFiles(options.Target, backup, retired, hadPrevious, kind)
		return errors.Join(fmt.Errorf("install staged agent: %w", err), rollbackErr)
	}

	healthPath, err := reserveHealthPath(filepath.Dir(executable))
	if err != nil {
		rollbackErr := restoreFiles(options.Target, backup, retired, hadPrevious, kind)
		return errors.Join(err, rollbackErr)
	}
	defer os.Remove(healthPath)
	process, err := startManagedProcess(executable, "--post-update", manifest.Version, "--update-health-file", healthPath)
	if err != nil {
		rollbackErr := rollbackAndRestart(executable, options.Target, backup, retired, hadPrevious, kind, manifest.Version, nil)
		return errors.Join(fmt.Errorf("start updated agent: %w", err), rollbackErr)
	}
	if err := waitForHealth(ctx, process, healthPath, manifest.Version, options.HealthTimeout); err != nil {
		rollbackErr := rollbackAndRestart(executable, options.Target, backup, retired, hadPrevious, kind, manifest.Version, process)
		return errors.Join(err, rollbackErr)
	}
	if hadPrevious {
		if err := removeInstalledPath(retired, kind); err != nil {
			rollbackErr := rollbackAndRestart(executable, options.Target, backup, retired, true, kind, manifest.Version, process)
			return errors.Join(fmt.Errorf("remove retired update backup after successful health check: %w", err), rollbackErr)
		}
		hadPrevious = false
	}
	newState := State{HighestSequence: manifest.Sequence, Version: manifest.Version, UpdatedAt: now().UTC()}
	if err := saveState(options.StatePath, newState); err != nil {
		rollbackErr := rollbackAndRestart(executable, options.Target, backup, retired, hadPrevious, kind, manifest.Version, process)
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
	kind := options.Kind
	if kind == "" {
		kind = KindFile
	}
	if kind != KindFile && kind != KindBundle {
		return errors.New("update kind must be file or bundle")
	}
	if kind == KindBundle {
		if options.Executable == "" || !filepath.IsAbs(options.Executable) {
			return errors.New("bundle updates require an absolute executable path inside the bundle")
		}
		relative, err := filepath.Rel(options.Target, options.Executable)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return errors.New("bundle executable must live inside the installed bundle")
		}
	}
	if options.WaitPID < 0 {
		return errors.New("wait PID cannot be negative")
	}
	if options.WaitTimeout < 0 || options.WaitTimeout > 2*time.Minute {
		return errors.New("wait timeout must be between zero and two minutes")
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

// verifyTargetPath validates an installed (or backed-up) agent location: a
// regular file for KindFile, a real directory for KindBundle. Symbolic
// links are always rejected so replacement never escapes the install root.
func verifyTargetPath(path, label string, kind Kind) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symbolic link", label)
	}
	if kind == KindBundle {
		if !info.IsDir() {
			return fmt.Errorf("%s must be a bundle directory", label)
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", label)
	}
	return nil
}

// installStaged publishes the verified artifact at the target location.
// Bundles move atomically from their extraction root, which always shares
// the target parent directory and therefore the same filesystem.
func installStaged(staged, target string, kind Kind) error {
	if kind == KindBundle {
		if err := os.Rename(staged, target); err != nil {
			return fmt.Errorf("move staged bundle into place: %w", err)
		}
		return nil
	}
	return copyFileExclusive(staged, target)
}

// extractBundleArchive unpacks a verified zip into a private temporary
// directory inside the installed bundle parent. Extraction is deliberately
// strict: only regular files living under the expected bundle directory,
// no links, no AppleDouble residue, no path escapes and a total size cap,
// so a malformed archive can never write outside the staging root.
func extractBundleArchive(archivePath, parent, bundleName, relativeExecutable string) (string, string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", "", fmt.Errorf("open update bundle archive: %w", err)
	}
	defer reader.Close()
	root, err := os.MkdirTemp(parent, ".fiberpulse-extract-*")
	if err != nil {
		return "", "", fmt.Errorf("create bundle extraction directory beside the installed bundle: %w", err)
	}
	fail := func(err error) (string, string, error) {
		_ = os.RemoveAll(root)
		return "", "", err
	}
	var total int64
	for _, member := range reader.File {
		if !validBundleMemberName(member.Name, bundleName) {
			if ignorableBundleMember(member.Name) {
				continue
			}
			return fail(fmt.Errorf("update archive contains an unexpected entry %q", member.Name))
		}
		mode := member.FileInfo().Mode()
		if mode&os.ModeSymlink != 0 || !mode.IsRegular() {
			return fail(fmt.Errorf("update archive entry %q must be a regular file", member.Name))
		}
		total += int64(member.UncompressedSize64)
		if total > MaxArtifactBytes || int64(member.UncompressedSize64) > MaxArtifactBytes {
			return fail(errors.New("extracted update bundle exceeds the maximum supported size"))
		}
		if err := extractBundleMember(member, root, mode.Perm()); err != nil {
			return fail(err)
		}
	}
	bundle := filepath.Join(root, bundleName)
	if info, err := os.Lstat(bundle); err != nil || !info.IsDir() {
		return fail(fmt.Errorf("update archive must contain the %s bundle directory", bundleName))
	}
	stagedExecutable := filepath.Join(bundle, relativeExecutable)
	if err := verifyRegularFile(stagedExecutable, "extracted bundle executable"); err != nil {
		return fail(err)
	}
	if info, err := os.Stat(stagedExecutable); err != nil || info.Mode().Perm()&0o100 == 0 {
		return fail(errors.New("extracted bundle executable lost its executable permission"))
	}
	return bundle, root, nil
}

// validBundleMemberName reports whether a zip entry belongs inside the
// expected bundle directory without any escape attempt.
func validBundleMemberName(name, bundleName string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsRune(name, 0) {
		return false
	}
	segments := strings.Split(name, "/")
	if segments[0] != bundleName {
		return false
	}
	for _, segment := range segments[1:] {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return len(segments) > 1
}

// ignorableBundleMember reports AppleDouble metadata that macOS archivers
// add around real files; it carries no code and is skipped, never written.
func ignorableBundleMember(name string) bool {
	if name == "__MACOSX" || strings.HasPrefix(name, "__MACOSX/") {
		return true
	}
	segments := strings.Split(name, "/")
	return strings.HasPrefix(segments[len(segments)-1], "._")
}

func extractBundleMember(member *zip.File, root string, permissions os.FileMode) error {
	destination := filepath.Join(root, filepath.FromSlash(member.Name))
	parent, err := filepath.Abs(filepath.Dir(destination))
	if err != nil {
		return err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if parent != cleanRoot && !strings.HasPrefix(parent, cleanRoot+string(filepath.Separator)) {
		return fmt.Errorf("update archive entry %q escapes the extraction root", member.Name)
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create extracted bundle directory: %w", err)
	}
	source, err := member.Open()
	if err != nil {
		return fmt.Errorf("open archived bundle entry %q: %w", member.Name, err)
	}
	defer source.Close()
	if permissions == 0 {
		permissions = 0o644
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, permissions)
	if err != nil {
		return fmt.Errorf("create extracted bundle entry %q: %w", member.Name, err)
	}
	if _, err := io.Copy(out, io.LimitReader(source, MaxArtifactBytes+1)); err != nil {
		_ = out.Close()
		return fmt.Errorf("extract bundle entry %q: %w", member.Name, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close extracted bundle entry %q: %w", member.Name, err)
	}
	return nil
}

// waitForProcessExit blocks until the previous agent process has released
// the installation. Replacing a running executable fails on Windows and
// would race the single-instance lock on every platform.
func waitForProcessExit(ctx context.Context, pid int, alive func(int) bool, timeout time.Duration) error {
	if pid <= 0 || !alive(pid) {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for previous agent exit cancelled: %w", context.Cause(ctx))
		case <-timer.C:
			return fmt.Errorf("previous agent process %d is still running", pid)
		case <-ticker.C:
			if !alive(pid) {
				return nil
			}
		}
	}
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

func rollbackAndRestart(executable, target, backup, retired string, hadPrevious bool, kind Kind, failedVersion string, process *managedProcess) error {
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
	result = errors.Join(result, restoreFiles(target, backup, retired, hadPrevious, kind))
	if result != nil {
		return result
	}
	restored, err := startManagedProcess(executable, "--post-update", "rollback-"+failedVersion)
	if err != nil {
		return fmt.Errorf("restart restored agent: %w", err)
	}
	_ = restored
	return nil
}

func restoreFiles(target, backup, retired string, hadPrevious bool, kind Kind) error {
	var result error
	if err := removeInstalledPath(target, kind); err != nil && !os.IsNotExist(err) {
		result = errors.Join(result, fmt.Errorf("remove failed update: %w", err))
	}
	if result == nil {
		if err := os.Rename(backup, target); err != nil {
			result = errors.Join(result, fmt.Errorf("restore previous agent: %w", err))
		}
	}
	return errors.Join(result, restoreRetiredBackup(backup, retired, hadPrevious))
}

// removeInstalledPath deletes a path this updater itself installed or
// rotated: a file for KindFile, a directory tree for KindBundle. It is
// only ever called on exact paths the updater created during this run.
func removeInstalledPath(path string, kind Kind) error {
	if kind == KindBundle {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return os.RemoveAll(path)
	}
	return os.Remove(path)
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

// ReadState loads the anti-rollback state written by previous updater runs.
// A missing file yields a zero state, which accepts any first update.
func ReadState(path string) (State, error) {
	return loadState(path)
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
