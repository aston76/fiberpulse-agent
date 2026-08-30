package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxManifestBytes bounds a signed feed document. A manifest is a small JSON
// object; anything larger is a serving error or an attack.
const MaxManifestBytes = 64 << 10

// MaxArtifactBytes bounds any downloadable artifact regardless of the size
// declared by the manifest.
const MaxArtifactBytes = 512 << 20

// ErrNoUpdate is returned when the feed is healthy but does not offer a
// version newer than the installed one. It is a normal steady state, not a
// failure.
var ErrNoUpdate = errors.New("no newer update is available")

type ClientConfig struct {
	FeedURL        string
	PublicKeyHex   string
	Channel        string
	CurrentVersion string
	Platform       string
	HTTPClient     *http.Client
	Now            func() time.Time
}

type Client struct {
	feedURL        string
	publicKeyHex   string
	channel        string
	currentVersion string
	httpClient     *http.Client
	now            func() time.Time
}

// NewClient validates the operator-provided release configuration. The feed
// URL is the base location hosting the per-platform feed documents, for
// example https://github.com/owner/repo/releases/latest/download.
func NewClient(config ClientConfig) (*Client, error) {
	platform := config.Platform
	if platform == "" {
		resolved, err := DefaultPlatform()
		if err != nil {
			return nil, err
		}
		platform = resolved
	}
	feedName, err := FeedFileName(platform)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(config.FeedURL), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return nil, errors.New("update feed URL must be absolute")
	}
	host := parsed.Hostname()
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1")) {
		return nil, errors.New("update feed URL must use HTTPS outside loopback")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return nil, errors.New("update feed URL must not contain credentials, query parameters or fragments")
	}
	key, err := hex.DecodeString(config.PublicKeyHex)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("update public key must be a 32-byte Ed25519 key in hexadecimal")
	}
	if config.Channel != "stable" && config.Channel != "canary" {
		return nil, errors.New("update channel must be stable or canary")
	}
	if _, err := parseSemanticVersion(config.CurrentVersion); err != nil {
		return nil, fmt.Errorf("invalid installed version: %w", err)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	now := time.Now
	if config.Now != nil {
		now = config.Now
	}
	return &Client{feedURL: parsed.String() + "/" + feedName, publicKeyHex: config.PublicKeyHex, channel: config.Channel, currentVersion: config.CurrentVersion, httpClient: httpClient, now: now}, nil
}

// FeedURL is the exact per-platform document the client polls.
func (c *Client) FeedURL() string { return c.feedURL }

// Check downloads the signed feed document and returns the candidate manifest
// together with its raw signed bytes so the updater can re-verify them from
// disk. ErrNoUpdate is returned when the feed is valid but not newer.
func (c *Client) Check(ctx context.Context, state State) (Manifest, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.feedURL, nil)
	if err != nil {
		return Manifest{}, nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-cache")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("download update feed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Manifest{}, nil, fmt.Errorf("update feed answered HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, MaxManifestBytes+1))
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("read update feed: %w", err)
	}
	if len(raw) > MaxManifestBytes {
		return Manifest{}, nil, errors.New("update feed document is too large")
	}
	manifest, err := loadManifest(raw, c.publicKeyHex)
	if err != nil {
		return Manifest{}, nil, err
	}
	if err := manifest.validate(c.now().UTC(), c.currentVersion, c.channel, state); err != nil {
		// A correctly signed feed that simply has nothing newer is the normal
		// steady state; distinguish it from corruption or attacks.
		if comparison, compareErr := compareSemanticVersions(manifest.Version, c.currentVersion); compareErr == nil && comparison <= 0 {
			return Manifest{}, nil, ErrNoUpdate
		}
		if manifest.Sequence != 0 && manifest.Sequence <= state.HighestSequence {
			return Manifest{}, nil, ErrNoUpdate
		}
		return Manifest{}, nil, err
	}
	return manifest, raw, nil
}

// Download streams the manifest artifact to stagedPath, enforcing the
// declared byte size and SHA-256. The file is written to a temporary sibling
// and moved into place only after full verification.
func (c *Client) Download(ctx context.Context, manifest Manifest, stagedPath string) error {
	if !filepath.IsAbs(stagedPath) {
		return errors.New("staged update path must be absolute")
	}
	if _, err := os.Lstat(stagedPath); err == nil {
		return errors.New("staged update path already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect staged update path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(stagedPath), 0o700); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifest.URL, nil)
	if err != nil {
		return err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download update artifact: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("update artifact answered HTTP %d", response.StatusCode)
	}
	limit := manifest.Size
	if limit > MaxArtifactBytes {
		return errors.New("update artifact exceeds the maximum supported size")
	}
	temporary, err := os.CreateTemp(filepath.Dir(stagedPath), filepath.Base(stagedPath)+".part-*")
	if err != nil {
		return fmt.Errorf("create partial update download: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func(err error) error {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
		return err
	}
	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, digest), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return cleanup(fmt.Errorf("store update artifact: %w", err))
	}
	if written != manifest.Size {
		return cleanup(errors.New("update artifact size mismatch"))
	}
	if hex.EncodeToString(digest.Sum(nil)) != manifest.SHA256 {
		return cleanup(errors.New("update artifact SHA-256 mismatch"))
	}
	if err := temporary.Sync(); err != nil {
		return cleanup(fmt.Errorf("flush update artifact: %w", err))
	}
	if err := temporary.Close(); err != nil {
		return cleanup(fmt.Errorf("close update artifact: %w", err))
	}
	if err := os.Chmod(temporaryPath, 0o755); err != nil {
		return cleanup(fmt.Errorf("mark update artifact executable: %w", err))
	}
	if err := os.Rename(temporaryPath, stagedPath); err != nil {
		return cleanup(fmt.Errorf("publish staged update artifact: %w", err))
	}
	return nil
}
