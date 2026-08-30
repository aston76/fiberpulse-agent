package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fiberpulse.dev/agent/internal/app"
)

// buildUpdateConfig resolves the release-channel wiring for this build. The
// feed URL and Ed25519 public key are injected at link time by the release
// workflow; development builds leave them empty and updates stay disabled.
// FIBERPULSE_UPDATE_FEED_URL and FIBERPULSE_UPDATE_CHANNEL remain available
// as operator overrides, mainly for staging and local end-to-end testing.
func buildUpdateConfig(dataDir string, logger *slog.Logger) (*app.UpdateConfig, error) {
	feedURL := updateFeedURL
	if override := os.Getenv("FIBERPULSE_UPDATE_FEED_URL"); override != "" {
		feedURL = override
	}
	publicKey := updatePublicKey
	if override := os.Getenv("FIBERPULSE_UPDATE_PUBLIC_KEY"); override != "" {
		publicKey = override
	}
	if feedURL == "" || publicKey == "" {
		return nil, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable for automatic updates: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve executable links for automatic updates: %w", err)
	}
	channel := os.Getenv("FIBERPULSE_UPDATE_CHANNEL")
	if channel == "" {
		channel = "stable"
	}
	if channel != "stable" && channel != "canary" {
		return nil, errors.New("FIBERPULSE_UPDATE_CHANNEL must be stable or canary")
	}
	updaterName := "fiberpulse-updater"
	if runtime.GOOS == "windows" {
		updaterName = "fiberpulse-updater.exe"
	}
	config := &app.UpdateConfig{
		FeedURL:      feedURL,
		PublicKeyHex: publicKey,
		Channel:      channel,
		Executable:   executable,
		UpdaterPath:  filepath.Join(filepath.Dir(executable), updaterName),
		DataDir:      dataDir,
	}
	if bundle, ok := enclosingBundle(executable); ok {
		config.BundlePath = bundle
	}
	if updateSkipPlatformVerify == "true" {
		logger.Warn("platform signature verification is disabled for this update channel (unsigned development release)")
		config.SkipPlatformVerify = true
	}
	if _, err := os.Lstat(config.UpdaterPath); err != nil {
		return nil, fmt.Errorf("updater helper is missing beside the agent (%s): %w", config.UpdaterPath, err)
	}
	logger.Info("automatic updates configured", "channel", channel, "bundle", config.BundlePath != "")
	return config, nil
}

// enclosingBundle returns the .app root when the executable runs from inside
// a macOS bundle, so the updater replaces the sealed bundle instead of the
// bare executable.
func enclosingBundle(executable string) (string, bool) {
	const marker = ".app/Contents/MacOS/"
	index := strings.Index(executable, marker)
	if index <= 0 {
		return "", false
	}
	return executable[:index+len(".app")], true
}
