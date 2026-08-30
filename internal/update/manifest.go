package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type Manifest struct {
	Version        string    `json:"version"`
	Channel        string    `json:"channel"`
	Sequence       uint64    `json:"sequence"`
	SHA256         string    `json:"sha256"`
	Size           int64     `json:"size"`
	URL            string    `json:"url"`
	MinimumVersion string    `json:"minimum_version"`
	ExpiresAt      time.Time `json:"expires_at"`
	Signature      []byte    `json:"signature,omitempty"`
}

type State struct {
	HighestSequence uint64    `json:"highest_sequence"`
	Version         string    `json:"version"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func loadManifest(raw []byte, publicKeyHex string) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode update manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Manifest{}, err
	}
	key, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return Manifest{}, errors.New("invalid Ed25519 public key")
	}
	signature := append([]byte(nil), manifest.Signature...)
	manifest.Signature = nil
	unsigned, err := json.Marshal(manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("encode unsigned update manifest: %w", err)
	}
	if len(signature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(key), unsigned, signature) {
		return Manifest{}, errors.New("update manifest signature verification failed")
	}
	manifest.Signature = signature
	return manifest, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("update JSON contains trailing data")
		}
		return fmt.Errorf("decode trailing update JSON: %w", err)
	}
	return nil
}

// Sign returns the canonical signed encoding of the manifest: the struct is
// serialized without its signature, those exact compact bytes are signed with
// the offline Ed25519 release key, and the signature is embedded before the
// final encoding. loadManifest rejects anything that does not round-trip
// through this exact construction.
func Sign(manifest Manifest, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Ed25519 private key")
	}
	manifest.Signature = nil
	unsigned, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode unsigned update manifest: %w", err)
	}
	manifest.Signature = ed25519.Sign(privateKey, unsigned)
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode signed update manifest: %w", err)
	}
	return raw, nil
}

// FeedFileName maps a release platform identifier to the signed feed document
// published as a GitHub release asset.
func FeedFileName(platform string) (string, error) {
	switch platform {
	case "windows-x64":
		return "latest-windows-x64.json", nil
	case "macos-universal":
		return "latest-macos-universal.json", nil
	default:
		return "", fmt.Errorf("unsupported update platform %q", platform)
	}
}

// DefaultPlatform resolves the release platform identifier for the running
// operating system.
func DefaultPlatform() (string, error) {
	switch runtime.GOOS {
	case "windows":
		return "windows-x64", nil
	case "darwin":
		return "macos-universal", nil
	default:
		return "", fmt.Errorf("automatic updates are unsupported on %s", runtime.GOOS)
	}
}

func (m Manifest) validate(now time.Time, currentVersion, channel string, state State) error {
	if _, err := parseSemanticVersion(m.Version); err != nil {
		return fmt.Errorf("invalid update version: %w", err)
	}
	if _, err := parseSemanticVersion(m.MinimumVersion); err != nil {
		return fmt.Errorf("invalid minimum version: %w", err)
	}
	if _, err := parseSemanticVersion(currentVersion); err != nil {
		return fmt.Errorf("invalid current version: %w", err)
	}
	if m.Channel != "stable" && m.Channel != "canary" {
		return errors.New("update manifest channel must be stable or canary")
	}
	if channel != m.Channel {
		return fmt.Errorf("update channel mismatch: expected %s", channel)
	}
	if m.Sequence == 0 || m.Sequence <= state.HighestSequence {
		return fmt.Errorf("update sequence %d is not newer than %d", m.Sequence, state.HighestSequence)
	}
	if m.Size <= 0 {
		return errors.New("update artifact size must be positive")
	}
	digest, err := hex.DecodeString(m.SHA256)
	if err != nil || len(digest) != 32 || m.SHA256 != strings.ToLower(m.SHA256) {
		return errors.New("update artifact SHA-256 must be 64 lowercase hexadecimal characters")
	}
	parsedURL, err := url.Parse(m.URL)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil {
		return errors.New("update artifact URL must be an absolute URL without credentials")
	}
	artifactHost := parsedURL.Hostname()
	if parsedURL.Scheme != "https" && !(parsedURL.Scheme == "http" && (artifactHost == "127.0.0.1" || artifactHost == "localhost" || artifactHost == "::1")) {
		return errors.New("update artifact URL must use HTTPS outside loopback")
	}
	if m.ExpiresAt.IsZero() || !now.Before(m.ExpiresAt) {
		return errors.New("update manifest is expired")
	}
	minimumComparison, _ := compareSemanticVersions(currentVersion, m.MinimumVersion)
	if minimumComparison < 0 {
		return fmt.Errorf("current version %s is below required minimum %s", currentVersion, m.MinimumVersion)
	}
	updateComparison, _ := compareSemanticVersions(m.Version, currentVersion)
	if updateComparison <= 0 {
		return fmt.Errorf("update version %s is not newer than current version %s", m.Version, currentVersion)
	}
	return nil
}

type semanticVersion struct {
	major      uint64
	minor      uint64
	patch      uint64
	prerelease []string
}

func parseSemanticVersion(value string) (semanticVersion, error) {
	matches := semanticVersionPattern.FindStringSubmatch(value)
	if matches == nil {
		return semanticVersion{}, errors.New("expected SemVer without a leading v")
	}
	major, _ := strconv.ParseUint(matches[1], 10, 64)
	minor, _ := strconv.ParseUint(matches[2], 10, 64)
	patch, _ := strconv.ParseUint(matches[3], 10, 64)
	var prerelease []string
	if matches[4] != "" {
		prerelease = strings.Split(matches[4], ".")
		for _, identifier := range prerelease {
			if len(identifier) > 1 && identifier[0] == '0' && isNumeric(identifier) {
				return semanticVersion{}, errors.New("numeric prerelease identifiers cannot contain leading zeroes")
			}
		}
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: prerelease}, nil
}

func compareSemanticVersions(left, right string) (int, error) {
	a, err := parseSemanticVersion(left)
	if err != nil {
		return 0, err
	}
	b, err := parseSemanticVersion(right)
	if err != nil {
		return 0, err
	}
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1, nil
		}
		if pair[0] > pair[1] {
			return 1, nil
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0, nil
	}
	if len(a.prerelease) == 0 {
		return 1, nil
	}
	if len(b.prerelease) == 0 {
		return -1, nil
	}
	for index := 0; index < len(a.prerelease) && index < len(b.prerelease); index++ {
		leftIdentifier, rightIdentifier := a.prerelease[index], b.prerelease[index]
		leftNumeric, rightNumeric := isNumeric(leftIdentifier), isNumeric(rightIdentifier)
		switch {
		case leftNumeric && rightNumeric:
			leftNumber, _ := strconv.ParseUint(leftIdentifier, 10, 64)
			rightNumber, _ := strconv.ParseUint(rightIdentifier, 10, 64)
			if leftNumber < rightNumber {
				return -1, nil
			}
			if leftNumber > rightNumber {
				return 1, nil
			}
		case leftNumeric:
			return -1, nil
		case rightNumeric:
			return 1, nil
		default:
			if leftIdentifier < rightIdentifier {
				return -1, nil
			}
			if leftIdentifier > rightIdentifier {
				return 1, nil
			}
		}
	}
	if len(a.prerelease) < len(b.prerelease) {
		return -1, nil
	}
	if len(a.prerelease) > len(b.prerelease) {
		return 1, nil
	}
	return 0, nil
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}
